// Package httpapi exposes Rift's HTTP API.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"rift/server/internal/auth"
	"rift/server/internal/library"
	"rift/server/internal/mail"
	"rift/server/internal/ratelimit"
	"rift/server/internal/realtime"
	"rift/server/internal/store"
)

// Pinger reports whether a dependency is reachable. *pgxpool.Pool satisfies it.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	Log     *slog.Logger
	DB      Pinger
	Store   *store.Queries // idempotency keys
	Auth    *auth.Service
	Library *library.Service
	Broker  *realtime.Broker
	Google  auth.GoogleAuth // nil disables Google sign-in
	Mailer  mail.Mailer

	WebURL       string   // the web app origin, e.g. https://rift.app
	WebOrigins   []string // allowed CORS / cookie-request origins
	CookieSecure bool
	TrustProxy   bool // take the client IP from X-Forwarded-For
}

type Server struct {
	Deps

	ipSignIn    *ratelimit.Limiter // sign-in attempts per IP
	emailSignIn *ratelimit.Limiter // link emails per address
	ipRefresh   *ratelimit.Limiter // refreshes per IP
}

func New(d Deps) *Server {
	return &Server{
		Deps:        d,
		ipSignIn:    ratelimit.New(20, time.Hour, 10),
		emailSignIn: ratelimit.New(5, time.Hour, 3),
		ipRefresh:   ratelimit.New(120, time.Hour, 30),
	}
}

// Handler returns the root handler with middleware applied.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	route := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, instrument(pattern, h))
	}

	route("GET /healthz", s.handleHealth)
	route("GET /readyz", s.handleReady)

	route("POST /api/auth/email", s.handleEmailLink)
	route("POST /api/auth/email/verify", s.handleEmailVerify)
	route("GET /api/auth/google/start", s.handleGoogleStart)
	route("GET /api/auth/google/callback", s.handleGoogleCallback)
	route("POST /api/auth/refresh", s.handleRefresh)
	route("POST /api/auth/logout", s.handleLogout)

	route("GET /api/me", s.authed(s.handleMe))
	route("GET /api/devices", s.authed(s.handleListDevices))
	route("DELETE /api/devices/{id}", s.authed(s.handleRevokeDevice))

	route("POST /api/uploads", s.authed(s.idempotent(s.handleStartUpload)))
	route("GET /api/uploads/{id}", s.authed(s.handleGetUpload))
	route("POST /api/uploads/{id}/complete", s.authed(s.handleCompleteUpload))
	route("DELETE /api/uploads/{id}", s.authed(s.handleAbortUpload))

	route("GET /api/sync", s.authed(s.handleSync))
	route("GET /api/events", s.authed(s.handleEvents))

	route("PATCH /api/books/{id}", s.authed(s.idempotent(s.handleUpdateBook)))
	route("DELETE /api/books/{id}", s.authed(s.idempotent(s.handleTrashBook)))
	route("POST /api/books/{id}/restore", s.authed(s.idempotent(s.handleRestoreBook)))
	route("GET /api/books/{id}/download", s.authed(s.handleDownload))
	route("PUT /api/books/{id}/position", s.authed(s.idempotent(s.handleRecordPosition)))

	return s.recoverer(s.requestID(s.logRequests(s.cors(mux))))
}

// handleHealth reports that the process is up. It never touches dependencies.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReady reports whether the server can serve traffic, i.e. the database answers.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.DB.Ping(ctx); err != nil {
		s.Log.Warn("readiness check failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
