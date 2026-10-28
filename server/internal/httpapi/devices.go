package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"rift/server/internal/auth"
)

type deviceResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	Client     string    `json:"client"`
	LastSeenAt time.Time `json:"last_seen_at"`
	IsCurrent  bool      `json:"is_current"`
}

// GET /api/me
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	user, err := s.Auth.User(r.Context(), p.UserID)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Sign in to continue.")
			return
		}
		s.logger(r).Error("load user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "Something went wrong.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":        user.ID,
		"email":     user.Email,
		"name":      user.Name,
		"device_id": p.DeviceID,
	})
}

// GET /api/devices
func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	devices, err := s.Auth.Devices(r.Context(), p.UserID)
	if err != nil {
		s.logger(r).Error("list devices", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "Something went wrong.")
		return
	}
	out := make([]deviceResponse, 0, len(devices))
	for _, d := range devices {
		out = append(out, deviceResponse{
			ID: d.ID.String(), Name: d.Name, Kind: d.Kind, Client: d.Client,
			LastSeenAt: d.LastSeenAt, IsCurrent: d.ID == p.DeviceID,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": out})
}

// DELETE /api/devices/{id}
func (s *Server) handleRevokeDevice(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "That device isn't signed in.")
		return
	}
	switch err := s.Auth.RevokeDevice(r.Context(), p.UserID, id); {
	case errors.Is(err, auth.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "That device isn't signed in.")
	case err != nil:
		s.logger(r).Error("revoke device", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "Couldn't sign that device out. Try again.")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
