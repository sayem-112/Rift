package httpapi

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5"

	"rift/server/internal/auth"
	"rift/server/internal/store"
)

// idempotent makes a mutating endpoint safe to retry. A client sends a unique
// Idempotency-Key per logical action (e.g. one position update) and reuses
// it on every retry:
//
//   - first request: the key is claimed, the handler runs, and its response
//     is stored (server errors release the key so a retry runs fresh);
//   - retry after completion: the stored response is replayed, nothing re-runs;
//   - retry while the first is still running: 409, try again shortly;
//   - same key with a different request: 422, the client has a bug.
//
// The claim is a single INSERT … ON CONFLICT DO NOTHING, so two concurrent
// copies of a request can't both run. Keys expire after 24 hours.
func (s *Server) idempotent(next func(http.ResponseWriter, *http.Request, auth.Principal)) func(http.ResponseWriter, *http.Request, auth.Principal) {
	return func(w http.ResponseWriter, r *http.Request, p auth.Principal) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next(w, r, p)
			return
		}
		if len(key) < 8 || len(key) > 128 {
			writeError(w, http.StatusBadRequest, "bad_idempotency_key", "Idempotency-Key must be 8-128 characters.")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large", "The request body is too large.")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		sum := sha256.Sum256(append([]byte(r.Method+" "+r.URL.Path+"\n"), body...))
		ctx := r.Context()

		_, err = s.Store.ClaimIdempotencyKey(ctx, store.ClaimIdempotencyKeyParams{UserID: p.UserID, Key: key, RequestHash: sum[:]})
		if errors.Is(err, pgx.ErrNoRows) {
			s.replay(w, r, p, key, sum[:])
			return
		}
		if err != nil {
			s.logger(r).Error("claim idempotency key", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "Something went wrong. Try again.")
			return
		}

		rec := &captureWriter{ResponseWriter: w, status: http.StatusOK}
		next(rec, r, p)

		if rec.status >= 500 {
			_ = s.Store.ReleaseIdempotencyKey(ctx, store.ReleaseIdempotencyKeyParams{UserID: p.UserID, Key: key})
			return
		}
		status := int32(rec.status)
		if err := s.Store.FinishIdempotencyKey(ctx, store.FinishIdempotencyKeyParams{
			UserID: p.UserID, Key: key, Status: &status, Response: rec.body.Bytes(),
		}); err != nil {
			s.logger(r).Error("store idempotent response", "err", err)
		}
	}
}

func (s *Server) replay(w http.ResponseWriter, r *http.Request, p auth.Principal, key string, hash []byte) {
	row, err := s.Store.GetIdempotencyKey(r.Context(), store.GetIdempotencyKeyParams{UserID: p.UserID, Key: key})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Released between our claim attempt and this read: let the client retry.
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusConflict, "request_in_progress", "This request is still being processed. Try again in a moment.")
	case err != nil:
		s.logger(r).Error("read idempotency key", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "Something went wrong. Try again.")
	case !bytes.Equal(row.RequestHash, hash):
		writeError(w, http.StatusUnprocessableEntity, "idempotency_key_reused", "This Idempotency-Key was already used for a different request.")
	case row.Status == nil:
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusConflict, "request_in_progress", "This request is still being processed. Try again in a moment.")
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Idempotent-Replayed", "true")
		w.WriteHeader(int(*row.Status))
		_, _ = w.Write(row.Response)
	}
}

// captureWriter passes a response through while keeping a copy of it.
type captureWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (c *captureWriter) WriteHeader(code int) {
	c.status = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *captureWriter) Write(b []byte) (int, error) {
	c.body.Write(b)
	return c.ResponseWriter.Write(b)
}

func (c *captureWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }
