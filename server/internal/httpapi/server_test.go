package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func newTestServer(db Pinger) http.Handler {
	return New(Deps{Log: quiet, DB: db, WebURL: "http://localhost:5173", WebOrigins: []string{"http://localhost:5173"}}).Handler()
}

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(fakeDB{}).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("every response should carry a request ID")
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"database up", nil, http.StatusOK},
		{"database down", errors.New("connection refused"), http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newTestServer(fakeDB{err: tt.err}).ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestCORS(t *testing.T) {
	tests := []struct {
		origin string
		want   string
	}{
		{"http://localhost:5173", "http://localhost:5173"},
		{"https://evil.example", ""},
	}
	for _, tt := range tests {
		req := httptest.NewRequest("OPTIONS", "/healthz", nil)
		req.Header.Set("Origin", tt.origin)
		rec := httptest.NewRecorder()
		newTestServer(fakeDB{}).ServeHTTP(rec, req)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.want {
			t.Errorf("origin %q: allow-origin = %q, want %q", tt.origin, got, tt.want)
		}
	}
}

func TestProtectedRoutesNeedToken(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(fakeDB{}).ServeHTTP(rec, httptest.NewRequest("GET", "/api/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestGoogleDisabledWithoutConfig(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(fakeDB{}).ServeHTTP(rec, httptest.NewRequest("GET", "/api/auth/google/start", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
