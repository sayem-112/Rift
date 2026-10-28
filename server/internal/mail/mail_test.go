package mail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResendRequest(t *testing.T) {
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m := NewResend("re_test", "rift <a@b.c>")
	m.Endpoint = srv.URL
	if err := m.Send(context.Background(), SignInLink("x@y.z", "https://app/verify?token=abc&x=<1>")); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer re_test" {
		t.Errorf("auth header = %q", auth)
	}
	if got["to"].([]any)[0] != "x@y.z" || got["from"] != "rift <a@b.c>" {
		t.Errorf("unexpected body: %v", got)
	}
	if html := got["html"].(string); strings.Contains(html, "<1>") {
		t.Errorf("link not escaped in HTML: %s", html)
	}
}

func TestResendError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"invalid from"}`, http.StatusUnprocessableEntity)
	}))
	defer srv.Close()

	m := NewResend("re_test", "bad")
	m.Endpoint = srv.URL
	err := m.Send(context.Background(), Message{To: "x@y.z"})
	if err == nil || !strings.Contains(err.Error(), "422") {
		t.Fatalf("want 422 error, got %v", err)
	}
}
