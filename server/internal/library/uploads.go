package library

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"rift/server/internal/auth"
	"rift/server/internal/blob"
	"rift/server/internal/database"
	"rift/server/internal/keys"
	"rift/server/internal/metrics"
	"rift/server/internal/store"
)

type StartUpload struct {
	SHA256 string // hex SHA-256 of the plaintext file
	Size   int64
	Format string
	Title  string
	Author *string
}

// UploadTicket tells the client what to do next.
type UploadTicket struct {
	// Status is "created" (upload every part), "resuming" (upload the listed
	// parts) or "exists" (this file is already in the library; nothing to upload).
	Status   string    `json:"status"`
	UploadID uuid.UUID `json:"upload_id,omitzero"`
	Book     Book      `json:"book"`
	// BookKey encrypts the file client-side. Sent over TLS, never logged.
	BookKey   []byte    `json:"book_key,omitempty"`
	Parts     []PartURL `json:"parts,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitzero"`
}

type PartURL struct {
	Number int32  `json:"number"`
	Size   int64  `json:"size"` // exact ciphertext bytes the URL accepts
	URL    string `json:"url"`
}

func (r StartUpload) validate() ([]byte, error) {
	sum, err := hex.DecodeString(r.SHA256)
	if err != nil || len(sum) != 32 {
		return nil, invalid("sha256 must be 64 hex characters")
	}
	if r.Size < 1 || r.Size > BookSizeLimit {
		return nil, invalid("books can be up to 50 MB")
	}
	if r.Format != "pdf" && r.Format != "epub" {
		return nil, invalid("format must be pdf or epub")
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(r.Title)); n < 1 || n > 300 {
		return nil, invalid("title must be 1-300 characters")
	}
	if r.Author != nil && utf8.RuneCountInString(*r.Author) > 300 {
		return nil, invalid("author must be at most 300 characters")
	}
	return sum, nil
}

// StartUpload reserves space and opens a multipart upload, or resumes the
// in-flight upload of the same file, or reports that the file is already in
// the library (same SHA-256 from any device).
func (s *Service) StartUpload(ctx context.Context, p auth.Principal, req StartUpload) (UploadTicket, error) {
	sum, err := req.validate()
	if err != nil {
		return UploadTicket{}, err
	}

	var (
		ticket   UploadTicket
		book     store.Book
		upload   store.Upload
		resuming bool
	)
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		user, err := q.LockUser(ctx, p.UserID)
		if err != nil {
			return err
		}

		existing, err := q.GetBookBySHA(ctx, store.GetBookBySHAParams{UserID: p.UserID, Sha256: sum})
		switch {
		case err == nil && existing.Status == store.BookStatusReady:
			ticket = UploadTicket{Status: "exists", Book: bookView(existing)}
			return nil
		case err == nil:
			book, resuming = existing, true
			upload, err = q.GetUploadByBook(ctx, existing.ID)
			return err
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}

		// The user row is locked, so concurrent uploads from two devices
		// can't both pass this check.
		used, err := q.UserUsedBytes(ctx, p.UserID)
		if err != nil {
			return err
		}
		if used+req.Size > UserQuota {
			return &QuotaError{Used: used, Size: req.Size, Quota: UserQuota}
		}

		userKey, err := s.userKey(ctx, q, p.UserID, user.WrappedKey)
		if err != nil {
			return err
		}
		bookID := uuid.New()
		wrapped, err := keys.WrapBookKey(userKey, bookID, keys.NewKey())
		if err != nil {
			return err
		}
		storageKey := fmt.Sprintf("u/%s/b/%s", p.UserID, bookID)

		s3UploadID, err := s.blob.CreateMultipart(ctx, storageKey)
		if err != nil {
			return err
		}
		title := strings.TrimSpace(req.Title)
		book, err = q.CreateBook(ctx, store.CreateBookParams{
			ID: bookID, UserID: p.UserID, Sha256: sum, Format: store.BookFormat(req.Format),
			Title: title, Author: req.Author, SizeBytes: req.Size,
			StorageKey: storageKey, WrappedKey: wrapped,
		})
		if err == nil {
			upload, err = q.CreateUpload(ctx, store.CreateUploadParams{
				BookID: bookID, UserID: p.UserID, DeviceID: &p.DeviceID,
				S3UploadID: s3UploadID, PartCount: PartCount(req.Size),
			})
		}
		if err != nil {
			// The transaction will roll back; don't leave parts behind in storage.
			_ = s.blob.AbortMultipart(context.WithoutCancel(ctx), storageKey, s3UploadID)
		}
		return err
	})
	if err != nil {
		return UploadTicket{}, err
	}
	if ticket.Status == "exists" {
		metrics.Uploads.WithLabelValues("deduplicated").Inc()
		return ticket, nil
	}
	if !resuming {
		metrics.Uploads.WithLabelValues("started").Inc()
	}
	return s.ticket(ctx, p.UserID, book, upload)
}

// GetUpload returns the parts still missing, with fresh URLs. Clients call it
// to resume after a crash or a lost connection.
func (s *Service) GetUpload(ctx context.Context, p auth.Principal, uploadID uuid.UUID) (UploadTicket, error) {
	var (
		book   store.Book
		upload store.Upload
	)
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		var err error
		upload, err = q.LockUpload(ctx, store.LockUploadParams{ID: uploadID, UserID: p.UserID})
		if err != nil {
			return err
		}
		book, err = q.GetBook(ctx, store.GetBookParams{ID: upload.BookID, UserID: p.UserID})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return UploadTicket{}, ErrNotFound
	}
	if err != nil {
		return UploadTicket{}, err
	}
	if upload.State == store.UploadStateCompleted {
		return UploadTicket{Status: "exists", Book: bookView(book)}, nil
	}
	if upload.State != store.UploadStateUploading {
		return UploadTicket{}, ErrUploadGone
	}
	return s.ticket(ctx, p.UserID, book, upload)
}

// ticket presigns a URL for every part not yet stored at its exact size.
func (s *Service) ticket(ctx context.Context, userID uuid.UUID, book store.Book, upload store.Upload) (UploadTicket, error) {
	have := map[int32]bool{}
	parts, err := s.blob.ListParts(ctx, book.StorageKey, upload.S3UploadID)
	if errors.Is(err, blob.ErrNotFound) {
		return UploadTicket{}, ErrUploadGone
	}
	if err != nil {
		return UploadTicket{}, err
	}
	for _, pt := range parts {
		have[pt.Number] = pt.Size == PartCipherSize(book.SizeBytes, pt.Number)
	}

	bookKey, err := s.bookKey(ctx, userID, book)
	if err != nil {
		return UploadTicket{}, err
	}
	status := "created"
	if len(parts) > 0 {
		status = "resuming"
	}
	t := UploadTicket{
		Status: status, UploadID: upload.ID, Book: bookView(book), BookKey: bookKey,
		ExpiresAt: s.now().Add(uploadURLTTL),
	}
	for n := int32(1); n <= upload.PartCount; n++ {
		if have[n] {
			continue
		}
		size := PartCipherSize(book.SizeBytes, n)
		url, err := s.blob.PresignUploadPart(ctx, book.StorageKey, upload.S3UploadID, n, size, uploadURLTTL)
		if err != nil {
			return UploadTicket{}, err
		}
		t.Parts = append(t.Parts, PartURL{Number: n, Size: size, URL: url})
	}
	return t, nil
}

// CompleteUpload verifies every part arrived at its exact size, assembles the
// object, checks the result, and publishes the book to every device.
// Completing an already completed upload returns the book again.
func (s *Service) CompleteUpload(ctx context.Context, p auth.Principal, uploadID uuid.UUID) (Book, error) {
	var out store.Book
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		// Locking the upload makes concurrent completes take turns.
		upload, err := q.LockUpload(ctx, store.LockUploadParams{ID: uploadID, UserID: p.UserID})
		if err != nil {
			return err
		}
		book, err := q.GetBook(ctx, store.GetBookParams{ID: upload.BookID, UserID: p.UserID})
		if err != nil {
			return err
		}
		switch upload.State {
		case store.UploadStateCompleted:
			out = book
			return nil
		case store.UploadStateAbandoned:
			return ErrUploadGone
		}

		if err := s.assemble(ctx, book, upload); err != nil {
			return err
		}

		// The user lock orders this write with every other change for the user.
		if _, err := q.LockUser(ctx, p.UserID); err != nil {
			return err
		}
		version, err := q.NextVersion(ctx, p.UserID)
		if err != nil {
			return err
		}
		if err := q.SetUploadState(ctx, store.SetUploadStateParams{ID: upload.ID, State: store.UploadStateCompleted}); err != nil {
			return err
		}
		if out, err = q.MarkBookReady(ctx, store.MarkBookReadyParams{ID: book.ID, Version: version}); err != nil {
			return err
		}
		return record(ctx, q, p.UserID, version, store.EntityTypeBook, out.ID, "upsert", bookView(out))
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Book{}, ErrNotFound
	}
	if err != nil {
		var inc *IncompleteError
		if !errors.As(err, &inc) {
			metrics.Uploads.WithLabelValues("complete_failed").Inc()
		}
		return Book{}, err
	}
	metrics.Uploads.WithLabelValues("completed").Inc()
	return bookView(out), nil
}

// assemble completes the multipart upload, tolerating a retry after an
// earlier attempt completed in storage but failed to commit in the database.
func (s *Service) assemble(ctx context.Context, book store.Book, upload store.Upload) error {
	want := CipherSize(book.SizeBytes)
	parts, err := s.blob.ListParts(ctx, book.StorageKey, upload.S3UploadID)
	if errors.Is(err, blob.ErrNotFound) {
		// The multipart upload is gone: either a previous attempt completed it,
		// or it was aborted. The object's size tells which.
		if size, err := s.blob.Size(ctx, book.StorageKey); err == nil && size == want {
			return nil
		}
		return ErrUploadGone
	}
	if err != nil {
		return err
	}

	byNumber := map[int32]blob.Part{}
	for _, pt := range parts {
		byNumber[pt.Number] = pt
	}
	var missing []int32
	ordered := make([]blob.Part, 0, upload.PartCount)
	for n := int32(1); n <= upload.PartCount; n++ {
		pt, ok := byNumber[n]
		if !ok || pt.Size != PartCipherSize(book.SizeBytes, n) {
			missing = append(missing, n)
			continue
		}
		ordered = append(ordered, pt)
	}
	if len(missing) > 0 {
		return &IncompleteError{Missing: missing}
	}
	if err := s.blob.CompleteMultipart(ctx, book.StorageKey, upload.S3UploadID, ordered); err != nil {
		return err
	}
	// Belt and braces: the signed part sizes should already guarantee this.
	if size, err := s.blob.Size(ctx, book.StorageKey); err != nil || size != want {
		_ = s.blob.Delete(context.WithoutCancel(ctx), book.StorageKey)
		return ErrIntegrity
	}
	return nil
}

// AbortUpload cancels an in-flight upload and frees its reserved space.
func (s *Service) AbortUpload(ctx context.Context, p auth.Principal, uploadID uuid.UUID) error {
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		upload, err := q.LockUpload(ctx, store.LockUploadParams{ID: uploadID, UserID: p.UserID})
		if err != nil {
			return err
		}
		if upload.State != store.UploadStateUploading {
			return nil
		}
		book, err := q.GetBook(ctx, store.GetBookParams{ID: upload.BookID, UserID: p.UserID})
		if err != nil {
			return err
		}
		if err := s.blob.AbortMultipart(ctx, book.StorageKey, upload.S3UploadID); err != nil {
			return err
		}
		return q.DeleteBook(ctx, book.ID) // cascades to the upload row
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err == nil {
		metrics.Uploads.WithLabelValues("aborted").Inc()
	}
	return err
}

// CleanupStaleUploads aborts uploads still open after `after`, freeing their
// storage parts and reserved quota. Run as a periodic job.
func (s *Service) CleanupStaleUploads(ctx context.Context, after time.Duration) (int, error) {
	stale, err := s.q.ListStaleUploads(ctx, store.ListStaleUploadsParams{AfterHours: int32(after.Hours()), BatchSize: 100})
	if err != nil {
		return 0, err
	}
	cleaned := 0
	for _, u := range stale {
		if err := s.blob.AbortMultipart(ctx, u.StorageKey, u.S3UploadID); err != nil {
			return cleaned, err
		}
		err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
			locked, err := q.LockUpload(ctx, store.LockUploadParams{ID: u.ID, UserID: u.UserID})
			if err != nil || locked.State != store.UploadStateUploading {
				return err // completed in the meantime, or already gone
			}
			return q.DeleteBook(ctx, u.BookID)
		})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return cleaned, err
		}
		cleaned++
	}
	if cleaned > 0 {
		metrics.Uploads.WithLabelValues("abandoned").Add(float64(cleaned))
	}
	return cleaned, nil
}

// userKey unwraps the user's key, creating it on first use.
func (s *Service) userKey(ctx context.Context, q *store.Queries, userID uuid.UUID, wrapped []byte) ([]byte, error) {
	if len(wrapped) > 0 {
		return s.keys.UnwrapUserKey(userID, wrapped)
	}
	key := keys.NewKey()
	if err := q.SetUserKey(ctx, store.SetUserKeyParams{ID: userID, WrappedKey: s.keys.WrapUserKey(userID, key)}); err != nil {
		return nil, err
	}
	return key, nil
}

func (s *Service) bookKey(ctx context.Context, userID uuid.UUID, book store.Book) ([]byte, error) {
	user, err := s.q.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	userKey, err := s.keys.UnwrapUserKey(userID, user.WrappedKey)
	if err != nil {
		return nil, err
	}
	return keys.UnwrapBookKey(userKey, book.ID, book.WrappedKey)
}
