// Package jobs is a durable background job queue on Postgres.
//
// Jobs are rows. Enqueue inside the caller's transaction and the job exists
// exactly when the change that caused it commits (a transactional outbox).
// Workers claim batches with FOR UPDATE SKIP LOCKED, hold a lease while
// running, and retry failures with exponential backoff. A worker that dies
// mid-job simply lets its lease expire; another worker picks the job up.
// Handlers must therefore be idempotent: a job can run more than once.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"

	"rift/server/internal/store"
)

// Handler runs one job. Return nil when done, an error to retry, or an error
// wrapping ErrPermanent to fail without retrying (bad payload, gone resource).
type Handler func(ctx context.Context, payload json.RawMessage) error

var ErrPermanent = errors.New("permanent job failure")

type EnqueueOption func(*store.EnqueueJobParams)

// At delays the job until t.
func At(t time.Time) EnqueueOption {
	return func(p *store.EnqueueJobParams) { p.RunAt = t }
}

// Dedupe skips the enqueue while a job of the same kind and key is pending or running.
func Dedupe(key string) EnqueueOption {
	return func(p *store.EnqueueJobParams) { p.DedupeKey = &key }
}

// MaxAttempts overrides the default of 8 attempts.
func MaxAttempts(n int32) EnqueueOption {
	return func(p *store.EnqueueJobParams) { p.MaxAttempts = n }
}

// Enqueue adds a job using q, which may be a transaction. It reports false
// when a dedupe key suppressed the job.
func Enqueue(ctx context.Context, q store.Querier, kind string, payload any, opts ...EnqueueOption) (bool, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("encode %s payload: %w", kind, err)
	}
	params := store.EnqueueJobParams{Kind: kind, Payload: raw, RunAt: time.Now(), MaxAttempts: 8}
	for _, opt := range opts {
		opt(&params)
	}
	if _, err := q.EnqueueJob(ctx, params); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil // deduplicated
		}
		return false, fmt.Errorf("enqueue %s: %w", kind, err)
	}
	return true, nil
}

// Backoff is the delay before retry number `attempt` (1-based): exponential
// from 2s, capped at an hour, with ±20% jitter so retries of jobs that failed
// together (e.g. during an outage) don't all return at the same instant.
func Backoff(attempt int32) time.Duration {
	d := 2 * time.Second << min(attempt-1, 11) // 2s, 4s, 8s ... ~68min
	d = min(d, time.Hour)
	jitter := 0.8 + rand.Float64()*0.4
	return time.Duration(float64(d) * jitter)
}

func logger(log *slog.Logger, j store.Job) *slog.Logger {
	return log.With("job_id", j.ID, "kind", j.Kind, "attempt", j.Attempts)
}
