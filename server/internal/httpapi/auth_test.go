package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"rift/server/internal/auth"
	"rift/server/internal/mail"
	"rift/server/internal/testdb"
)

// captureMailer keeps sent messages instead of emailing them.
type captureMailer struct {
	mu   sync.Mutex
	sent []mail.Message
}

func (m *captureMailer) Send(_ context.Context, msg mail.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return nil
}

func (m *captureMailer) lastToken(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		t.Fatal("no email sent")
	}
	text := m.sent[len(m.sent)-1].Text
	start := strings.Index(text, "token=")
	if start < 0 {
		t.Fatalf("no token in email: %q", text)
	}
	raw := strings.Fields(text[start+len("token="):])[0]
	tok, _ := url.QueryUnescape(raw)
	return tok
}

type fakeGoogle struct{ id auth.GoogleIdentity }

func (f fakeGoogle) AuthCodeURL(state, _ string) string {
	return "https://accounts.example/auth?state=" + url.QueryEscape(state)
}
func (f fakeGoogle) Exchange(context.Context, string, string) (auth.GoogleIdentity, error) {
	return f.id, nil
}

type harness struct {
	srv    *httptest.Server
	client *http.Client
	mailer *captureMailer
}

func newHarness(t *testing.T, google auth.GoogleAuth) *harness {
	t.Helper()
	pool := testdb.New(t)
	mailer := &captureMailer{}
	var h harness
	h.srv = httptest.NewServer(nil)
	h.srv.Config.Handler = New(Deps{
		Log:          quiet,
		DB:           pool,
		Auth:         auth.NewService(pool, auth.NewAccessTokens([]byte("0123456789abcdef0123456789abcdef"))),
		Google:       google,
		Mailer:       mailer,
		WebURL:       h.srv.URL,
		WebOrigins:   []string{h.srv.URL},
		CookieSecure: false,
	}).Handler()
	t.Cleanup(h.srv.Close)
	jar, _ := cookiejar.New(nil)
	h.client = &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	h.mailer = mailer
	return &h
}

func (h *harness) do(t *testing.T, method, path, body, bearer string) (*http.Response, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, h.srv.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", h.srv.URL)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

const deviceJSON = `{"name":"Windows PC","kind":"desktop","client":"Edge on Windows"}`

func (h *harness) signInByEmail(t *testing.T, email string) string {
	t.Helper()
	resp, _ := h.do(t, "POST", "/api/auth/email", `{"email":"`+email+`"}`, "")
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("request link: status %d", resp.StatusCode)
	}
	resp, body := h.do(t, "POST", "/api/auth/email/verify", `{"token":"`+h.mailer.lastToken(t)+`","device":`+deviceJSON+`}`, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify: status %d %v", resp.StatusCode, body)
	}
	return body["access_token"].(string)
}

func TestEmailSignInRefreshAndLogout(t *testing.T) {
	h := newHarness(t, nil)
	access := h.signInByEmail(t, "reader@example.com")

	resp, me := h.do(t, "GET", "/api/me", "", access)
	if resp.StatusCode != http.StatusOK || me["email"] != "reader@example.com" {
		t.Fatalf("/api/me: %d %v", resp.StatusCode, me)
	}

	// The refresh token is never in a response body, only the HttpOnly cookie.
	resp, refreshed := h.do(t, "POST", "/api/auth/refresh", "", "")
	if resp.StatusCode != http.StatusOK || refreshed["access_token"] == "" {
		t.Fatalf("refresh: %d %v", resp.StatusCode, refreshed)
	}
	if _, leaked := refreshed["refresh_token"]; leaked {
		t.Fatal("refresh token must not appear in JSON")
	}

	resp, _ = h.do(t, "POST", "/api/auth/logout", "", "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout: %d", resp.StatusCode)
	}
	resp, _ = h.do(t, "POST", "/api/auth/refresh", "", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("refresh after logout: %d, want 401", resp.StatusCode)
	}
}

func TestLinkCannotBeReused(t *testing.T) {
	h := newHarness(t, nil)
	h.signInByEmail(t, "reader@example.com")
	resp, body := h.do(t, "POST", "/api/auth/email/verify", `{"token":"`+h.mailer.lastToken(t)+`","device":`+deviceJSON+`}`, "")
	if resp.StatusCode != http.StatusUnauthorized || body["error"].(map[string]any)["code"] != "link_invalid" {
		t.Fatalf("reused link: %d %v", resp.StatusCode, body)
	}
}

func TestEmailLinkRateLimit(t *testing.T) {
	h := newHarness(t, nil)
	var last int
	for range 4 { // burst of 3 per address
		resp, _ := h.do(t, "POST", "/api/auth/email", `{"email":"reader@example.com"}`, "")
		last = resp.StatusCode
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("4th link request: %d, want 429", last)
	}
}

func TestRefreshRejectsForeignOrigin(t *testing.T) {
	h := newHarness(t, nil)
	h.signInByEmail(t, "reader@example.com")
	req, _ := http.NewRequest("POST", h.srv.URL+"/api/auth/refresh", nil)
	req.Header.Set("Origin", "https://evil.example")
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin refresh: %d, want 403", resp.StatusCode)
	}
}

func TestDevicesListAndRevoke(t *testing.T) {
	h := newHarness(t, nil)
	pcToken := h.signInByEmail(t, "reader@example.com")

	// A second device signs in with its own cookie jar.
	other := *h
	jar, _ := cookiejar.New(nil)
	other.client = &http.Client{Jar: jar}
	phoneToken := other.signInByEmail(t, "reader@example.com")

	_, list := h.do(t, "GET", "/api/devices", "", pcToken)
	devices := list["devices"].([]any)
	if len(devices) != 2 {
		t.Fatalf("want 2 devices, got %v", list)
	}
	var phoneID string
	for _, d := range devices {
		if dev := d.(map[string]any); dev["is_current"] == false {
			phoneID = dev["id"].(string)
		}
	}

	resp, _ := h.do(t, "DELETE", "/api/devices/"+phoneID, "", pcToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke: %d", resp.StatusCode)
	}
	resp, body := h.do(t, "GET", "/api/me", "", phoneToken)
	if resp.StatusCode != http.StatusUnauthorized || body["error"].(map[string]any)["code"] != "device_revoked" {
		t.Fatalf("revoked device: %d %v", resp.StatusCode, body)
	}
}

func TestGoogleSignInFlow(t *testing.T) {
	h := newHarness(t, fakeGoogle{id: auth.GoogleIdentity{Subject: "g-1", Email: "reader@example.com", EmailVerified: true}})

	resp, _ := h.do(t, "GET", "/api/auth/google/start?name=Phone&kind=phone&client=Chrome", "", "")
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("start: %d", resp.StatusCode)
	}
	state, _ := url.Parse(resp.Header.Get("Location"))

	// A forged callback with the wrong state is refused.
	resp, _ = h.do(t, "GET", "/api/auth/google/callback?code=x&state=forged", "", "")
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "error=") {
		t.Fatalf("forged state should fail, redirected to %q", loc)
	}

	// The cookie was consumed by the failed attempt; start again for the real one.
	resp, _ = h.do(t, "GET", "/api/auth/google/start?name=Phone&kind=phone&client=Chrome", "", "")
	state, _ = url.Parse(resp.Header.Get("Location"))
	resp, _ = h.do(t, "GET", "/api/auth/google/callback?code=x&state="+url.QueryEscape(state.Query().Get("state")), "", "")
	if loc := resp.Header.Get("Location"); loc != h.srv.URL+"/" {
		t.Fatalf("callback should land on the app, got %q", loc)
	}
	resp, body := h.do(t, "POST", "/api/auth/refresh", "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the callback should have set the refresh cookie: %d %v", resp.StatusCode, body)
	}
}
