package library

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"rift/server/internal/auth"
	"rift/server/internal/database"
	"rift/server/internal/store"
)

// BookPatch edits a book's metadata. Nil fields are left alone.
type BookPatch struct {
	Title  *string
	Author *string
	// SetFinished applies Finished (which may be nil: "derive from progress").
	SetFinished bool
	Finished    *bool
}

func (p BookPatch) validate() error {
	if p.Title != nil {
		if n := utf8.RuneCountInString(strings.TrimSpace(*p.Title)); n < 1 || n > 300 {
			return invalid("title must be 1-300 characters")
		}
	}
	if p.Author != nil && utf8.RuneCountInString(*p.Author) > 300 {
		return invalid("author must be at most 300 characters")
	}
	return nil
}

func (s *Service) UpdateBook(ctx context.Context, p auth.Principal, bookID uuid.UUID, patch BookPatch) (Book, error) {
	if err := patch.validate(); err != nil {
		return Book{}, err
	}
	if patch.Title != nil {
		t := strings.TrimSpace(*patch.Title)
		patch.Title = &t
	}
	return s.mutateBook(ctx, p, bookID, false, func(q *store.Queries, version int64) (store.Book, error) {
		return q.UpdateBookMeta(ctx, store.UpdateBookMetaParams{
			ID: bookID, Title: patch.Title, Author: patch.Author,
			SetFinished: patch.SetFinished, Finished: patch.Finished, Version: version,
		})
	})
}

// TrashBook removes a book from every device. It stays restorable, and keeps
// counting against quota, until it is purged after TrashRetention.
func (s *Service) TrashBook(ctx context.Context, p auth.Principal, bookID uuid.UUID) (Book, error) {
	return s.mutateBook(ctx, p, bookID, false, func(q *store.Queries, version int64) (store.Book, error) {
		return q.SetBookTrashed(ctx, store.SetBookTrashedParams{ID: bookID, Trashed: true, Version: version})
	})
}

func (s *Service) RestoreBook(ctx context.Context, p auth.Principal, bookID uuid.UUID) (Book, error) {
	return s.mutateBook(ctx, p, bookID, true, func(q *store.Queries, version int64) (store.Book, error) {
		return q.SetBookTrashed(ctx, store.SetBookTrashedParams{ID: bookID, Trashed: false, Version: version})
	})
}

// mutateBook runs one versioned change to a ready book and records it.
func (s *Service) mutateBook(ctx context.Context, p auth.Principal, bookID uuid.UUID, allowTrashed bool,
	change func(q *store.Queries, version int64) (store.Book, error)) (Book, error) {
	var out store.Book
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		version, err := q.NextVersion(ctx, p.UserID)
		if err != nil {
			return err
		}
		book, err := q.LockBook(ctx, store.LockBookParams{ID: bookID, UserID: p.UserID})
		if err != nil {
			return err
		}
		if book.Status != store.BookStatusReady || (book.TrashedAt != nil && !allowTrashed) {
			return ErrNotFound
		}
		if out, err = change(q, version); err != nil {
			return err
		}
		return record(ctx, q, p.UserID, version, store.EntityTypeBook, out.ID, "upsert", bookView(out))
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Book{}, ErrNotFound
	}
	return bookView(out), err
}

// Download is what a device needs to fetch and decrypt a book.
type Download struct {
	URL        string    `json:"url"`
	BookKey    []byte    `json:"book_key"`
	SizeBytes  int64     `json:"size_bytes"`
	CipherSize int64     `json:"cipher_size"`
	ExpiresAt  time.Time `json:"expires_at"`
}

func (s *Service) Download(ctx context.Context, p auth.Principal, bookID uuid.UUID) (Download, error) {
	book, err := s.q.GetBook(ctx, store.GetBookParams{ID: bookID, UserID: p.UserID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (book.Status != store.BookStatusReady || book.TrashedAt != nil)) {
		return Download{}, ErrNotFound
	}
	if err != nil {
		return Download{}, err
	}
	key, err := s.bookKey(ctx, p.UserID, book)
	if err != nil {
		return Download{}, err
	}
	url, err := s.blob.PresignGet(ctx, book.StorageKey, downloadURLTTL)
	if err != nil {
		return Download{}, err
	}
	return Download{
		URL: url, BookKey: key, SizeBytes: book.SizeBytes,
		CipherSize: CipherSize(book.SizeBytes), ExpiresAt: s.now().Add(downloadURLTTL),
	}, nil
}

// PurgeTrash permanently deletes books trashed longer than TrashRetention:
// the stored file first, then the row, leaving tombstones in the change feed
// so offline devices drop the book too. Run as a periodic job; safe to repeat.
func (s *Service) PurgeTrash(ctx context.Context) (int, error) {
	books, err := s.q.ListPurgeableBooks(ctx, store.ListPurgeableBooksParams{
		AfterDays: int32(TrashRetention.Hours() / 24), BatchSize: 100,
	})
	if err != nil {
		return 0, err
	}
	for i, b := range books {
		if err := s.blob.Delete(ctx, b.StorageKey); err != nil {
			return i, err
		}
		err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
			version, err := q.NextVersion(ctx, b.UserID)
			if err != nil {
				return err
			}
			if err := q.DeleteBook(ctx, b.ID); err != nil {
				return err
			}
			tombstone := map[string]uuid.UUID{"id": b.ID}
			if err := record(ctx, q, b.UserID, version, store.EntityTypeBook, b.ID, "deleted", tombstone); err != nil {
				return err
			}
			// Every change gets its own version: (user, version) is the feed's key.
			posVersion, err := q.NextVersion(ctx, b.UserID)
			if err != nil {
				return err
			}
			return record(ctx, q, b.UserID, posVersion, store.EntityTypePosition, b.ID, "deleted", map[string]uuid.UUID{"book_id": b.ID})
		})
		if err != nil {
			return i, err
		}
	}
	return len(books), nil
}
