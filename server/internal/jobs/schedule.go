package jobs

import (
	"context"
	"log/slog"
	"time"

	"rift/server/internal/metrics"
	"rift/server/internal/store"
)

// Periodic is a job enqueued on a fixed interval.
type Periodic struct {
	Kind     string
	Interval time.Duration
}

// Schedule enqueues each periodic job on its interval until ctx ends. Every
// instance runs a scheduler; the dedupe key keeps at most one copy of each
// periodic job pending, so instances never pile duplicates into the queue.
func Schedule(ctx context.Context, q store.Querier, log *slog.Logger, periodic ...Periodic) {
	for _, p := range periodic {
		go func() {
			enqueue := func() {
				if _, err := Enqueue(ctx, q, p.Kind, struct{}{}, Dedupe("periodic")); err != nil && ctx.Err() == nil {
					log.Error("schedule periodic job", "kind", p.Kind, "err", err)
				}
			}
			enqueue()
			t := time.NewTicker(p.Interval)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					enqueue()
				}
			}
		}()
	}
}

// ReportQueueDepth refreshes the unfinished-jobs gauge every interval.
func ReportQueueDepth(ctx context.Context, q store.Querier, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		rows, err := q.CountUnfinishedJobs(ctx)
		if err == nil {
			metrics.JobsUnfinished.Reset()
			for _, r := range rows {
				metrics.JobsUnfinished.WithLabelValues(r.Kind, string(r.State)).Set(float64(r.Total))
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
