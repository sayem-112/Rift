// Package library owns a user's books, their uploads, reading positions and
// the change feed that keeps every device in sync.
//
// Every write follows the same shape inside one transaction:
//
//  1. take the next version from the user's counter (locks the user row),
//  2. change the entity,
//  3. upsert its latest state into the change feed under that version,
//  4. pg_notify, so every API instance pushes it to open event streams.
//
// The notification only fires on commit, so subscribers never see a change
// that was rolled back.
package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"rift/server/internal/blob"
	"rift/server/internal/keys"
	"rift/server/internal/store"
)

const (
	BookSizeLimit int64 = 50 << 20
	UserQuota     int64 = 200 << 20

	// TrashRetention is how long a deleted book can be restored before its file
	// is purged and its space freed.
	TrashRetention = 30 * 24 * time.Hour

	uploadURLTTL   = time.Hour
	downloadURLTTL = 15 * time.Minute
)

var (
	ErrNotFound   = errors.New("not found")
	ErrUploadGone = errors.New("upload was cancelled or expired")
	// ErrIntegrity means the stored object doesn't match the declared size.
	ErrIntegrity = errors.New("uploaded file doesn't match its declared size")
)

// ValidationError is a client mistake; Message is safe to show.
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func invalid(format string, args ...any) error {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

// QuotaError means the book doesn't fit in the user's remaining space.
type QuotaError struct{ Used, Size, Quota int64 }

func (e *QuotaError) Error() string {
	return fmt.Sprintf("quota exceeded: %d used + %d > %d", e.Used, e.Size, e.Quota)
}

// IncompleteError lists parts the client still has to upload.
type IncompleteError struct{ Missing []int32 }

func (e *IncompleteError) Error() string {
	return fmt.Sprintf("upload incomplete: missing parts %v", e.Missing)
}

// Blob is the object storage the library needs; *blob.S3 implements it.
type Blob interface {
	CreateMultipart(ctx context.Context, key string) (string, error)
	PresignUploadPart(ctx context.Context, key, uploadID string, part int32, contentLength int64, ttl time.Duration) (string, error)
	ListParts(ctx context.Context, key, uploadID string) ([]blob.Part, error)
	CompleteMultipart(ctx context.Context, key, uploadID string, parts []blob.Part) error
	AbortMultipart(ctx context.Context, key, uploadID string) error
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	Size(ctx context.Context, key string) (int64, error)
	Delete(ctx context.Context, key string) error
}

type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
	keys *keys.Keyring
	blob Blob
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool, keyring *keys.Keyring, b Blob) *Service {
	return &Service{pool: pool, q: store.New(pool), keys: keyring, blob: b, now: time.Now}
}

// Book is a book as clients see it, in the change feed and API responses.
type Book struct {
	ID        uuid.UUID  `json:"id"`
	Title     string     `json:"title"`
	Author    *string    `json:"author"`
	Format    string     `json:"format"`
	SizeBytes int64      `json:"size_bytes"`
	Finished  *bool      `json:"finished"`
	AddedAt   time.Time  `json:"added_at"`
	TrashedAt *time.Time `json:"trashed_at"`
	Version   int64      `json:"version"`
}

func bookView(b store.Book) Book {
	return Book{
		ID: b.ID, Title: b.Title, Author: b.Author, Format: string(b.Format), SizeBytes: b.SizeBytes,
		Finished: b.Finished, AddedAt: b.CreatedAt, TrashedAt: b.TrashedAt, Version: b.Version,
	}
}

// Position is the winning reading position for a book.
type Position struct {
	BookID   uuid.UUID       `json:"book_id"`
	DeviceID uuid.UUID       `json:"device_id"`
	Locator  json.RawMessage `json:"locator"`
	Progress float32         `json:"progress"`
	Label    string          `json:"label"`
	ReadAt   time.Time       `json:"read_at"`
	Version  int64           `json:"version"`
}

func positionView(p store.CurrentPosition) Position {
	return Position{
		BookID: p.BookID, DeviceID: p.DeviceID, Locator: p.Locator, Progress: p.Progress,
		Label: p.Label, ReadAt: p.ReadAt, Version: p.Version,
	}
}

// record appends an entity's latest state to the change feed and wakes
// listeners on commit. Call it inside the transaction that made the change,
// with the version that transaction took from NextVersion.
func record(ctx context.Context, q *store.Queries, userID uuid.UUID, version int64, entity store.EntityType, id uuid.UUID, op string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := q.UpsertChange(ctx, store.UpsertChangeParams{
		UserID: userID, Version: version, Entity: entity, EntityID: id, Op: op, Data: raw,
	}); err != nil {
		return fmt.Errorf("record change: %w", err)
	}
	return q.NotifyChange(ctx, userID.String())
}
