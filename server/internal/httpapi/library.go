package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"rift/server/internal/auth"
	"rift/server/internal/library"
)

// libraryError maps library errors to responses. Messages are written to be shown.
func (s *Server) libraryError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		v   *library.ValidationError
		q   *library.QuotaError
		inc *library.IncompleteError
	)
	switch {
	case errors.As(err, &v):
		writeError(w, http.StatusBadRequest, "invalid_request", v.Message)
	case errors.As(err, &q):
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": map[string]any{
			"code":    "quota_exceeded",
			"message": "There isn't enough space in your library for this book.",
			"used":    q.Used, "quota": q.Quota,
		}})
	case errors.As(err, &inc):
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{
			"code":    "upload_incomplete",
			"message": "Some parts of the file haven't arrived yet.",
			"missing": inc.Missing,
		}})
	case errors.Is(err, library.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "That book isn't in your library.")
	case errors.Is(err, library.ErrUploadGone):
		writeError(w, http.StatusGone, "upload_gone", "This upload was cancelled or expired. Start it again.")
	case errors.Is(err, library.ErrIntegrity):
		writeError(w, http.StatusUnprocessableEntity, "integrity", "The uploaded file didn't match its declared size. Upload it again.")
	default:
		s.logger(r).Error("library request failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "Something went wrong. Try again.")
	}
}

func pathID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Not found.")
		return uuid.Nil, false
	}
	return id, true
}

// POST /api/uploads
func (s *Server) handleStartUpload(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	var body struct {
		SHA256    string  `json:"sha256"`
		SizeBytes int64   `json:"size_bytes"`
		Format    string  `json:"format"`
		Title     string  `json:"title"`
		Author    *string `json:"author"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	ticket, err := s.Library.StartUpload(r.Context(), p, library.StartUpload{
		SHA256: body.SHA256, Size: body.SizeBytes, Format: body.Format, Title: body.Title, Author: body.Author,
	})
	if err != nil {
		s.libraryError(w, r, err)
		return
	}
	status := http.StatusCreated
	if ticket.Status == "exists" {
		status = http.StatusOK
	}
	writeJSON(w, status, ticket)
}

// GET /api/uploads/{id}
func (s *Server) handleGetUpload(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ticket, err := s.Library.GetUpload(r.Context(), p, id)
	if err != nil {
		s.libraryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ticket)
}

// POST /api/uploads/{id}/complete
func (s *Server) handleCompleteUpload(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	book, err := s.Library.CompleteUpload(r.Context(), p, id)
	if err != nil {
		s.libraryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}

// DELETE /api/uploads/{id}
func (s *Server) handleAbortUpload(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.Library.AbortUpload(r.Context(), p, id); err != nil {
		s.libraryError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/sync?since=<version>&limit=<n>
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	since, err := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	if err != nil && r.URL.Query().Get("since") != "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "since must be a version number.")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	page, err := s.Library.Changes(r.Context(), p.UserID, since, limit)
	if err != nil {
		s.libraryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// PATCH /api/books/{id}  {title?, author?, finished?: true|false|null}
func (s *Server) handleUpdateBook(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Title    *string         `json:"title"`
		Author   *string         `json:"author"`
		Finished json.RawMessage `json:"finished"` // absent: unchanged; null: derive from progress
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	patch := library.BookPatch{Title: body.Title, Author: body.Author}
	if len(body.Finished) > 0 {
		patch.SetFinished = true
		if err := json.Unmarshal(body.Finished, &patch.Finished); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "finished must be true, false or null.")
			return
		}
	}
	book, err := s.Library.UpdateBook(r.Context(), p, id, patch)
	if err != nil {
		s.libraryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}

// DELETE /api/books/{id}: moves the book to the trash.
func (s *Server) handleTrashBook(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	book, err := s.Library.TrashBook(r.Context(), p, id)
	if err != nil {
		s.libraryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}

// POST /api/books/{id}/restore
func (s *Server) handleRestoreBook(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	book, err := s.Library.RestoreBook(r.Context(), p, id)
	if err != nil {
		s.libraryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}

// GET /api/books/{id}/download
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	dl, err := s.Library.Download(r.Context(), p, id)
	if err != nil {
		s.libraryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dl)
}

// PUT /api/books/{id}/position
func (s *Server) handleRecordPosition(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Locator     json.RawMessage `json:"locator"`
		Progress    float32         `json:"progress"`
		Label       string          `json:"label"`
		ReadAt      time.Time       `json:"read_at"`
		ClientNow   time.Time       `json:"client_now"`
		BaseVersion int64           `json:"base_version"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	result, err := s.Library.RecordPosition(r.Context(), p, id, library.RecordPosition{
		Locator: body.Locator, Progress: body.Progress, Label: body.Label,
		ReadAt: body.ReadAt, ClientNow: body.ClientNow, BaseVersion: body.BaseVersion,
	})
	if err != nil {
		s.libraryError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
