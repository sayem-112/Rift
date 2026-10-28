package library

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"rift/server/internal/auth"
	"rift/server/internal/database"
	"rift/server/internal/store"
)

// RecordPosition is a device reporting where it is in a book.
type RecordPosition struct {
	Locator  json.RawMessage // {page, offset} for PDF, {cfi} for EPUB; opaque to the server
	Progress float32
	Label    string
	// ReadAt is when the reader was at this spot, by the device's clock.
	ReadAt time.Time
	// ClientNow is the device's clock when it sent the request. With the
	// server's own clock at arrival, it gives the device's clock offset.
	ClientNow time.Time
	// BaseVersion is the position version the device last saw for this book.
	BaseVersion int64
}

type PositionResult struct {
	// Applied is false when another device's newer reading won the conflict.
	Applied bool     `json:"applied"`
	Current Position `json:"current"`
}

func (r RecordPosition) validate() error {
	if len(r.Locator) == 0 || len(r.Locator) > 2048 || !json.Valid(r.Locator) || r.Locator[0] != '{' {
		return invalid("locator must be a JSON object under 2 KB")
	}
	if r.Progress < 0 || r.Progress > 1 {
		return invalid("progress must be between 0 and 1")
	}
	if utf8.RuneCountInString(r.Label) > 80 {
		return invalid("label must be at most 80 characters")
	}
	if r.ReadAt.IsZero() || r.ClientNow.IsZero() {
		return invalid("read_at and client_now are required")
	}
	return nil
}

// CorrectedReadAt maps a device timestamp onto the server's clock.
//
// The offset is measured on this very request (server time at arrival minus
// the device's time at sending), so it needs no stored state and tracks a
// drifting clock. Network latency adds a small positive error, far below the
// gap between two separate reading sessions. The result never lies in the
// future: a device can't win every conflict by claiming to read tomorrow.
func CorrectedReadAt(readAt, clientNow, serverNow time.Time) time.Time {
	corrected := readAt.Add(serverNow.Sub(clientNow))
	if corrected.After(serverNow) {
		return serverNow
	}
	return corrected
}

// RecordPosition stores a device's position and decides whether it becomes
// the book's current position.
//
//   - Fast path: the device saw the current version, so nothing happened
//     elsewhere in between; its write is simply the newest.
//   - Conflict: another device wrote since (e.g. both read offline). The
//     reading that happened later, on corrected clocks, wins.
//
// Either way the device's own latest position is kept, so a client can undo
// an auto-jump back to where this device was.
func (s *Service) RecordPosition(ctx context.Context, p auth.Principal, bookID uuid.UUID, req RecordPosition) (PositionResult, error) {
	if err := req.validate(); err != nil {
		return PositionResult{}, err
	}
	readAt := CorrectedReadAt(req.ReadAt, req.ClientNow, s.now())

	var result PositionResult
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		// Lock the user first, like every other write, then the book.
		if _, err := q.LockUser(ctx, p.UserID); err != nil {
			return err
		}
		book, err := q.LockBook(ctx, store.LockBookParams{ID: bookID, UserID: p.UserID})
		if err != nil {
			return err
		}
		if book.Status != store.BookStatusReady || book.TrashedAt != nil {
			return ErrNotFound
		}

		if err := q.UpsertDevicePosition(ctx, store.UpsertDevicePositionParams{
			BookID: bookID, DeviceID: p.DeviceID, Locator: req.Locator,
			Progress: req.Progress, Label: req.Label, ReadAt: readAt,
		}); err != nil {
			return err
		}

		current, err := q.GetCurrentPosition(ctx, bookID)
		hasCurrent := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		wins := !hasCurrent || req.BaseVersion == current.Version || readAt.After(current.ReadAt)
		if !wins {
			result = PositionResult{Applied: false, Current: positionView(current)}
			return nil
		}

		version, err := q.NextVersion(ctx, p.UserID)
		if err != nil {
			return err
		}
		current, err = q.SetCurrentPosition(ctx, store.SetCurrentPositionParams{
			BookID: bookID, DeviceID: p.DeviceID, Locator: req.Locator, Progress: req.Progress,
			Label: req.Label, ReadAt: readAt, Version: version,
		})
		if err != nil {
			return err
		}
		result = PositionResult{Applied: true, Current: positionView(current)}
		return record(ctx, q, p.UserID, version, store.EntityTypePosition, bookID, "upsert", result.Current)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PositionResult{}, ErrNotFound
	}
	return result, err
}

// Change is one entry of the change feed.
type Change struct {
	Version int64           `json:"version"`
	Entity  string          `json:"entity"` // "book" or "position"
	ID      uuid.UUID       `json:"id"`
	Op      string          `json:"op"` // "upsert" or "deleted"
	Data    json.RawMessage `json:"data"`
}

type ChangePage struct {
	Changes []Change `json:"changes"`
	// Cursor is the version to pass as `since` next time.
	Cursor  int64 `json:"cursor"`
	HasMore bool  `json:"has_more"`
}

// CurrentVersion is the user's latest change version.
func (s *Service) CurrentVersion(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.q.CurrentVersion(ctx, userID)
}

// Changes returns what changed after version `since`, oldest first. A new
// device passes 0 and pages until HasMore is false.
func (s *Service) Changes(ctx context.Context, userID uuid.UUID, since int64, limit int) (ChangePage, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	rows, err := s.q.ListChanges(ctx, store.ListChangesParams{UserID: userID, Version: since, Limit: int32(limit + 1)})
	if err != nil {
		return ChangePage{}, err
	}
	page := ChangePage{Changes: []Change{}, Cursor: since}
	if len(rows) > limit {
		page.HasMore = true
		rows = rows[:limit]
	}
	for _, r := range rows {
		page.Changes = append(page.Changes, Change{
			Version: r.Version, Entity: string(r.Entity), ID: r.EntityID, Op: r.Op, Data: r.Data,
		})
		page.Cursor = r.Version
	}
	if len(rows) == 0 && !page.HasMore {
		// Nothing new: report the user's current version so the client cursor
		// moves past versions whose entries were compacted away.
		current, err := s.q.CurrentVersion(ctx, userID)
		if err != nil {
			return ChangePage{}, err
		}
		page.Cursor = max(since, current)
	}
	return page, nil
}
