package library

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"rift/server/internal/auth"
	"rift/server/internal/keys"
	"rift/server/internal/store"
	"rift/server/internal/testblob"
	"rift/server/internal/testdb"
)

// This file plays the web client: it encrypts files in Rift's record format,
// uploads parts to presigned URLs, and downloads and decrypts them again.

type env struct {
	svc *Service
	q   *store.Queries
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := testdb.New(t)
	b := testblob.New(t)
	ring, err := keys.NewKeyring(keys.NewKey())
	if err != nil {
		t.Fatal(err)
	}
	return &env{svc: NewService(pool, ring, b), q: store.New(pool)}
}

// newUser creates a user with one device and returns its principal.
func (e *env) newUser(t *testing.T) auth.Principal {
	t.Helper()
	ctx := context.Background()
	u, err := e.q.UpsertUserByEmail(ctx, store.UpsertUserByEmailParams{Email: uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	return auth.Principal{UserID: u.ID, DeviceID: e.newDevice(t, u.ID, "Phone")}
}

func (e *env) newDevice(t *testing.T, userID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	d, err := e.q.CreateDevice(context.Background(), store.CreateDeviceParams{UserID: userID, Name: name, Kind: "phone"})
	if err != nil {
		t.Fatal(err)
	}
	return d.ID
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func startReq(plain []byte, title string) StartUpload {
	sum := sha256.Sum256(plain)
	return StartUpload{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(plain)), Format: "epub", Title: title}
}

// encryptPart encrypts part n (1-based) of plain as 1 MiB AES-GCM records.
func encryptPart(t *testing.T, key, plain []byte, n int32) []byte {
	t.Helper()
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	start := int64(n-1) * PartPlainSize
	end := min(start+PartPlainSize, int64(len(plain)))
	var out bytes.Buffer
	for off := start; off < end; off += RecordPlainSize {
		rec := plain[off:min(off+RecordPlainSize, end)]
		nonce := randomBytes(aead.NonceSize())
		out.Write(nonce)
		out.Write(aead.Seal(nil, nonce, rec, nil))
	}
	return out.Bytes()
}

func decryptFile(t *testing.T, key, ct []byte) []byte {
	t.Helper()
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	var out bytes.Buffer
	for len(ct) > 0 {
		n := min(int64(len(ct)), RecordPlainSize+RecordOverhead)
		rec := ct[:n]
		pt, err := aead.Open(nil, rec[:12], rec[12:], nil)
		if err != nil {
			t.Fatalf("decrypt record: %v", err)
		}
		out.Write(pt)
		ct = ct[n:]
	}
	return out.Bytes()
}

func putPart(url string, body []byte) (int, error) {
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.ContentLength = int64(len(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// uploadParts sends the given parts (all of them when only is empty).
func uploadParts(t *testing.T, ticket UploadTicket, plain []byte, only ...int32) {
	t.Helper()
	want := map[int32]bool{}
	for _, n := range only {
		want[n] = true
	}
	for _, part := range ticket.Parts {
		if len(only) > 0 && !want[part.Number] {
			continue
		}
		body := encryptPart(t, ticket.BookKey, plain, part.Number)
		if int64(len(body)) != part.Size {
			t.Fatalf("part %d: client produced %d bytes, server expects %d", part.Number, len(body), part.Size)
		}
		status, err := putPart(part.URL, body)
		if err != nil || status != http.StatusOK {
			t.Fatalf("upload part %d: status %d, err %v", part.Number, status, err)
		}
	}
}

// addBook uploads plain as a complete book.
func (e *env) addBook(t *testing.T, p auth.Principal, plain []byte, title string) Book {
	t.Helper()
	ctx := context.Background()
	ticket, err := e.svc.StartUpload(ctx, p, startReq(plain, title))
	if err != nil {
		t.Fatal(err)
	}
	uploadParts(t, ticket, plain)
	book, err := e.svc.CompleteUpload(ctx, p, ticket.UploadID)
	if err != nil {
		t.Fatal(err)
	}
	return book
}

func download(t *testing.T, e *env, p auth.Principal, bookID uuid.UUID) []byte {
	t.Helper()
	dl, err := e.svc.Download(context.Background(), p, bookID)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(dl.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	ct, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || int64(len(ct)) != dl.CipherSize {
		t.Fatalf("download: status %d, %d bytes (want %d)", resp.StatusCode, len(ct), dl.CipherSize)
	}
	return decryptFile(t, dl.BookKey, ct)
}

func ago(d time.Duration) time.Time { return time.Now().Add(-d) }

func locator(page int) []byte { return []byte(fmt.Sprintf(`{"page":%d}`, page)) }
