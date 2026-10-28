package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"rift/server/internal/auth"
	"rift/server/internal/metrics"
)

type ctxKey int

const (
	ctxRequestID ctxKey = iota
	ctxPrincipal
)

// requestID tags each request with an ID (the caller's, if sane) that appears
// in every log line and is echoed back for support and debugging.
func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if len(id) == 0 || len(id) > 64 {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxRequestID, id)))
	})
}

func (s *Server) logger(r *http.Request) *slog.Logger {
	id, _ := r.Context().Value(ctxRequestID).(string)
	return s.Log.With("request_id", id)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger(r).Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start).Round(time.Microsecond),
		)
	})
}

// instrument records per-route metrics. It wraps each route individually so
// the label is the route pattern ("DELETE /api/devices/{id}"), never the raw
// path, which would explode metric cardinality.
func instrument(pattern string, next http.Handler) http.Handler {
	method, route, ok := strings.Cut(pattern, " ")
	if !ok {
		method, route = "ANY", pattern
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		metrics.HTTPDuration.WithLabelValues(route, method).Observe(time.Since(start).Seconds())
		metrics.HTTPRequests.WithLabelValues(route, method, strconv.Itoa(rec.status/100)+"xx").Inc()
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && slices.Contains(s.WebOrigins, origin) {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Max-Age", "600")
			h.Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.logger(r).Error("panic serving request", "panic", v, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal", "Something went wrong.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// authed requires a valid access token from a device that is still signed in.
func (s *Server) authed(next func(http.ResponseWriter, *http.Request, auth.Principal)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Sign in to continue.")
			return
		}
		p, err := s.Auth.Authenticate(r.Context(), token)
		switch {
		case err == auth.ErrDeviceRevoked:
			writeError(w, http.StatusUnauthorized, "device_revoked", "This device was signed out.")
			return
		case err != nil:
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Your session expired. Sign in again.")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxPrincipal, p)), p)
	}
}

// sameOrigin guards cookie-authenticated endpoints. The refresh cookie is
// already SameSite=Strict; checking Origin too blocks cross-site requests from
// browsers that ignore SameSite.
func (s *Server) sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == "" || origin == s.WebURL || slices.Contains(s.WebOrigins, origin)
}

func (s *Server) clientIP(r *http.Request) string {
	if s.TrustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			first, _, _ := strings.Cut(fwd, ",")
			return strings.TrimSpace(first)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the underlying writer (to flush
// event streams through the middleware).
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
