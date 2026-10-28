package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"rift/server/internal/store"
	"rift/server/internal/testdb"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestBackoffGrowsAndCaps(t *testing.T) {
	prev := time.Duration(0)
	for attempt := int32(1); attempt <= 6; attempt++ {
		d := Backoff(attempt)
		base := 2 * time.Second << (attempt - 1)
		if d < base*8/10 || d > base*12/10 {
			t.Errorf("attempt %d: %v outside ±20%% of %v", attempt, d, base)
		}
		if d <= prev/2 {
			t.Errorf("attempt %d: backoff should grow (%v after %v)", attempt, d, prev)
		}
		prev = d
	}
	if d := Backoff(40); d > time.Hour*12/10 {
		t.Errorf("backoff must cap near an hour, got %v", d)
	}
}

// runUntil runs workers until done() reports true or the deadline passes.
func runUntil(t *testing.T, workers []*Worker, done func() bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for _, w := range workers {
		wg.Add(1)
		go func() { defer wg.Done(); w.Run(ctx) }()
	}
	deadline := time.Now().Add(20 * time.Second)
	for !done() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	wg.Wait()
	if !done() {
		t.Fatal("timed out waiting for jobs")
	}
}

func newWorker(q store.Querier) *Worker {
	w := NewWorker(q, quiet)
	w.PollInterval = 10 * time.Millisecond
	return w
}

func TestEveryJobRunsExactlyOnceAcrossWorkers(t *testing.T) {
	pool := testdb.New(t)
	q := store.New(pool)
	ctx := context.Background()

	const total = 200
	for i := range total {
		if _, err := Enqueue(ctx, q, "count", map[string]int{"n": i}); err != nil {
			t.Fatal(err)
		}
	}

	var mu sync.Mutex
	seen := make(map[int]int)
	handler := func(_ context.Context, payload json.RawMessage) error {
		var p struct{ N int }
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		mu.Lock()
		seen[p.N]++
		mu.Unlock()
		return nil
	}

	// Five independent workers, as if on five instances, sharing one queue.
	var workers []*Worker
	for range 5 {
		w := newWorker(q)
		w.Handle("count", handler)
		workers = append(workers, w)
	}
	runUntil(t, workers, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(seen) == total
	})

	for n, times := range seen {
		if times != 1 {
			t.Errorf("job %d ran %d times", n, times)
		}
	}
}

func TestFailedJobRetriesThenSucceeds(t *testing.T) {
	pool := testdb.New(t)
	q := store.New(pool)
	ctx := context.Background()
	if _, err := Enqueue(ctx, q, "flaky", struct{}{}); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	w := newWorker(q)
	w.Handle("flaky", func(context.Context, json.RawMessage) error {
		if calls.Add(1) == 1 {
			return errors.New("transient")
		}
		return nil
	})
	// Make the retry due immediately instead of after the real backoff.
	go func() {
		for calls.Load() < 1 {
			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(50 * time.Millisecond)
		_, _ = pool.Exec(ctx, "UPDATE jobs SET run_at = now() WHERE state = 'pending'")
	}()
	runUntil(t, []*Worker{w}, func() bool { return jobState(t, pool, "flaky") == "done" })

	if calls.Load() != 2 {
		t.Fatalf("handler ran %d times, want 2", calls.Load())
	}
}

func TestPermanentFailureStopsRetrying(t *testing.T) {
	pool := testdb.New(t)
	q := store.New(pool)
	ctx := context.Background()
	if _, err := Enqueue(ctx, q, "doomed", struct{}{}); err != nil {
		t.Fatal(err)
	}
	if _, err := Enqueue(ctx, q, "unknown-kind", struct{}{}); err != nil {
		t.Fatal(err)
	}

	w := newWorker(q)
	w.Handle("doomed", func(context.Context, json.RawMessage) error {
		return errors.Join(ErrPermanent, errors.New("payload refers to a deleted book"))
	})
	runUntil(t, []*Worker{w}, func() bool {
		return jobState(t, pool, "doomed") == "failed" && jobState(t, pool, "unknown-kind") == "failed"
	})
}

func TestDedupeKeepsOnePendingCopy(t *testing.T) {
	pool := testdb.New(t)
	q := store.New(pool)
	ctx := context.Background()

	first, err := Enqueue(ctx, q, "cleanup", struct{}{}, Dedupe("periodic"))
	if err != nil || !first {
		t.Fatalf("first enqueue should insert (inserted=%v, err=%v)", first, err)
	}
	second, err := Enqueue(ctx, q, "cleanup", struct{}{}, Dedupe("periodic"))
	if err != nil || second {
		t.Fatalf("duplicate while pending should be skipped (inserted=%v, err=%v)", second, err)
	}

	if _, err := pool.Exec(ctx, "UPDATE jobs SET state = 'done' WHERE kind = 'cleanup'"); err != nil {
		t.Fatal(err)
	}
	third, err := Enqueue(ctx, q, "cleanup", struct{}{}, Dedupe("periodic"))
	if err != nil || !third {
		t.Fatalf("once finished, the key is free again (inserted=%v, err=%v)", third, err)
	}
}

func TestExpiredLeaseIsReclaimed(t *testing.T) {
	pool := testdb.New(t)
	q := store.New(pool)
	ctx := context.Background()
	if _, err := Enqueue(ctx, q, "orphaned", struct{}{}); err != nil {
		t.Fatal(err)
	}
	// Simulate a worker that claimed the job and then crashed.
	if _, err := q.ClaimJobs(ctx, store.ClaimJobsParams{LeaseSeconds: 60, BatchSize: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE jobs SET locked_until = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}

	w := newWorker(q)
	w.Handle("orphaned", func(context.Context, json.RawMessage) error { return nil })
	runUntil(t, []*Worker{w}, func() bool { return jobState(t, pool, "orphaned") == "done" })
}

func jobState(t *testing.T, pool *pgxpool.Pool, kind string) string {
	t.Helper()
	var state string
	if err := pool.QueryRow(context.Background(), "SELECT state::text FROM jobs WHERE kind = $1 ORDER BY id DESC LIMIT 1", kind).Scan(&state); err != nil {
		return ""
	}
	return state
}
