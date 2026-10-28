package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"rift/server/internal/auth"
	"rift/server/internal/metrics"
)

const (
	heartbeatEvery = 25 * time.Second // under typical 30-60s proxy idle timeouts
	eventsPageSize = 200
)

// GET /api/events: a server-sent event stream of the user's changes.
//
// Each event is one change-feed entry, with its version as the SSE event ID.
// A reconnecting client sends Last-Event-ID and resumes exactly where it left
// off; nothing is lost while it was away because events are read from the
// change feed, not from memory. (The access token goes in the Authorization
// header, so clients use fetch() streaming rather than EventSource, which
// can't send headers.)
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	ctx := r.Context()
	rc := http.NewResponseController(w)

	cursor, err := s.startCursor(r, p)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Last-Event-ID / since must be a version number.")
		return
	}

	signal, unsubscribe := s.Broker.Subscribe(p.UserID)
	defer unsubscribe()
	metrics.EventStreams.Inc()
	defer metrics.EventStreams.Dec()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no") // don't let proxies buffer the stream
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 3000\n\n")

	send := func() error {
		for {
			page, err := s.Library.Changes(ctx, p.UserID, cursor, eventsPageSize)
			if err != nil {
				return err
			}
			for _, ch := range page.Changes {
				data, _ := json.Marshal(ch)
				if _, err := fmt.Fprintf(w, "id: %d\nevent: change\ndata: %s\n\n", ch.Version, data); err != nil {
					return err
				}
				metrics.EventsSent.Inc()
			}
			cursor = page.Cursor
			if !page.HasMore {
				return rc.Flush()
			}
		}
	}

	if err := send(); err != nil {
		return
	}
	heartbeat := time.NewTicker(heartbeatEvery)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-signal:
			if err := send(); err != nil {
				return
			}
		case <-heartbeat.C:
			// Revoking a device closes its open streams within one heartbeat.
			if !s.Auth.DeviceActive(ctx, p.DeviceID) {
				fmt.Fprint(w, "event: revoked\ndata: {}\n\n")
				_ = rc.Flush()
				return
			}
			// The comment keeps proxies from timing out an idle stream; the
			// catch-up covers any notification that never arrived.
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			if err := send(); err != nil {
				return
			}
		}
	}
}

// startCursor resumes from Last-Event-ID, else ?since=, else "now": a client
// that hasn't synced yet should call /api/sync first.
func (s *Server) startCursor(r *http.Request, p auth.Principal) (int64, error) {
	raw := r.Header.Get("Last-Event-ID")
	if raw == "" {
		raw = r.URL.Query().Get("since")
	}
	if raw != "" {
		return strconv.ParseInt(raw, 10, 64)
	}
	return s.Library.CurrentVersion(r.Context(), p.UserID)
}
