package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rift/server/internal/auth"
	"rift/server/internal/keys"
	"rift/server/internal/library"
	"rift/server/internal/realtime"
	"rift/server/internal/store"
	"rift/server/internal/testblob"
	"rift/server/internal/testdb"
)

// twoInstances runs two API servers against one database and one bucket, as
// if behind a load balancer: each with its own change listener.
func twoInstances(t *testing.T) (a, b *harness) {
	t.Helper()
	pool := testdb.New(t)
	bucket := testblob.New(t)
	ring, err := keys.NewKeyring(keys.NewKey())
	if err != nil {
		t.Fatal(err)
	}
	authSvc := auth.NewService(pool, auth.NewAccessTokens([]byte("0123456789abcdef0123456789abcdef")))
	lib := library.NewService(pool, ring, bucket)

	instance := func() *harness {
		broker := realtime.NewBroker(pool.Config().ConnString(), quiet)
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		go broker.Run(ctx)
		select {
		case <-broker.Ready():
		case <-time.After(10 * time.Second):
			t.Fatal("broker never started listening")
		}

		h := &harness{mailer: &captureMailer{}}
		h.srv = httptest.NewServer(nil)
		t.Cleanup(h.srv.Close)
		h.srv.Config.Handler = New(Deps{
			Log: quiet, DB: pool, Store: store.New(pool), Auth: authSvc, Library: lib, Broker: broker,
			Mailer: h.mailer, WebURL: h.srv.URL, WebOrigins: []string{h.srv.URL},
		}).Handler()
		jar, _ := cookiejar.New(nil)
		h.client = &http.Client{Jar: jar}
		return h
	}
	return instance(), instance()
}

// doKey is do() with an Idempotency-Key.
func (h *harness) doKey(t *testing.T, method, path, body, bearer, key string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, h.srv.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return resp, buf.Bytes()
}

// uploadSmallBook runs the whole upload protocol over HTTP for a one-record file.
func uploadSmallBook(t *testing.T, h *harness, token, title string) string {
	t.Helper()
	plain := make([]byte, 700)
	_, _ = rand.Read(plain)
	sum := sha256.Sum256(plain)

	resp, raw := h.doKey(t, "POST", "/api/uploads", fmt.Sprintf(
		`{"sha256":"%s","size_bytes":%d,"format":"pdf","title":%q}`, hex.EncodeToString(sum[:]), len(plain), title), token, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("start upload: %d %s", resp.StatusCode, raw)
	}
	var ticket struct {
		UploadID string `json:"upload_id"`
		BookKey  string `json:"book_key"`
		Parts    []struct {
			URL  string `json:"url"`
			Size int64  `json:"size"`
		} `json:"parts"`
	}
	_ = json.Unmarshal(raw, &ticket)

	key, _ := base64.StdEncoding.DecodeString(ticket.BookKey)
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	nonce := make([]byte, 12)
	_, _ = rand.Read(nonce)
	record := append(nonce, aead.Seal(nil, nonce, plain, nil)...)
	if int64(len(record)) != ticket.Parts[0].Size {
		t.Fatalf("record is %d bytes, server expects %d", len(record), ticket.Parts[0].Size)
	}
	put, _ := http.NewRequest("PUT", ticket.Parts[0].URL, bytes.NewReader(record))
	putResp, err := http.DefaultClient.Do(put)
	if err != nil || putResp.StatusCode != http.StatusOK {
		t.Fatalf("put part: %v %v", err, putResp)
	}
	putResp.Body.Close()

	resp, raw = h.doKey(t, "POST", "/api/uploads/"+ticket.UploadID+"/complete", "", token, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("complete: %d %s", resp.StatusCode, raw)
	}
	var book struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &book)
	return book.ID
}

func positionBody(page int, base int64) string {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return fmt.Sprintf(`{"locator":{"page":%d},"progress":0.5,"label":"p. %d","read_at":%q,"client_now":%q,"base_version":%d}`,
		page, page, now, now, base)
}

func TestIdempotentPositionWrite(t *testing.T) {
	a, _ := twoInstances(t)
	token := a.signInByEmail(t, "reader@example.com")
	bookID := uploadSmallBook(t, a, token, "Dune")

	// A retry resends the identical request bytes.
	write := positionBody(12, 0)
	first, body1 := a.doKey(t, "PUT", "/api/books/"+bookID+"/position", write, token, "pos-key-0001")
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first write: %d %s", first.StatusCode, body1)
	}
	retry, body2 := a.doKey(t, "PUT", "/api/books/"+bookID+"/position", write, token, "pos-key-0001")
	if retry.Header.Get("Idempotent-Replayed") != "true" || !bytes.Equal(body1, body2) {
		t.Fatalf("retry should replay the original response:\n%s\n%s", body1, body2)
	}

	// A replay doesn't apply twice: the feed holds one position change.
	_, feed := a.doKey(t, "GET", "/api/sync?since=0", "", token, "")
	if n := strings.Count(string(feed), `"entity":"position"`); n != 1 {
		t.Fatalf("want 1 position change, got %d: %s", n, feed)
	}

	misuse, _ := a.doKey(t, "PUT", "/api/books/"+bookID+"/position", positionBody(99, 0), token, "pos-key-0001")
	if misuse.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("same key, different body: %d, want 422", misuse.StatusCode)
	}
}

func TestPositionReachesStreamOnAnotherInstance(t *testing.T) {
	a, b := twoInstances(t)
	token := a.signInByEmail(t, "reader@example.com")
	bookID := uploadSmallBook(t, a, token, "Meditations")

	// The phone holds an event stream open on instance B...
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", b.srv.URL+"/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("open stream: %v %v", err, resp)
	}
	defer resp.Body.Close()

	lines := make(chan string, 64)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	if first := <-lines; !strings.HasPrefix(first, "retry:") {
		t.Fatalf("stream should open with a retry hint, got %q", first)
	}

	// ...while the PC saves a position through instance A.
	start := time.Now()
	if resp, raw := a.doKey(t, "PUT", "/api/books/"+bookID+"/position", positionBody(120, 0), token, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("write position: %d %s", resp.StatusCode, raw)
	}

	deadline := time.After(5 * time.Second)
	var id string
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("stream closed before the change arrived")
			}
			if strings.HasPrefix(line, "id: ") {
				id = strings.TrimPrefix(line, "id: ")
			}
			if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"entity":"position"`) && strings.Contains(line, `"page":120`) {
				t.Logf("cross-instance delivery took %v (event id %s)", time.Since(start).Round(time.Millisecond), id)
				return
			}
		case <-deadline:
			t.Fatal("position written on instance A never reached the stream on instance B")
		}
	}
}

func TestStreamResumesFromLastEventID(t *testing.T) {
	a, _ := twoInstances(t)
	token := a.signInByEmail(t, "reader@example.com")
	bookID := uploadSmallBook(t, a, token, "Walden")

	// Learn the current cursor, then make a change while "offline".
	_, feed := a.doKey(t, "GET", "/api/sync?since=0", "", token, "")
	var page struct{ Cursor int64 }
	_ = json.Unmarshal(feed, &page)
	a.doKey(t, "PUT", "/api/books/"+bookID+"/position", positionBody(7, 0), token, "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", a.srv.URL+"/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Last-Event-ID", fmt.Sprint(page.Cursor))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if strings.Contains(sc.Text(), `"page":7`) {
			return // the change made while disconnected was replayed on reconnect
		}
	}
	t.Fatal("reconnecting with Last-Event-ID should replay missed changes")
}
