package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"rift/server/internal/metrics"
	"rift/server/internal/store"
)

type Worker struct {
	q        store.Querier
	log      *slog.Logger
	handlers map[string]Handler

	// Concurrency is the most jobs this worker runs at once.
	Concurrency int
	// PollInterval is how long to wait when the queue had nothing due.
	PollInterval time.Duration
	// Lease is how long a claimed job stays reserved; a job running longer
	// than this may be picked up again by another worker.
	Lease time.Duration
}

func NewWorker(q store.Querier, log *slog.Logger) *Worker {
	return &Worker{
		q:            q,
		log:          log,
		handlers:     make(map[string]Handler),
		Concurrency:  4,
		PollInterval: time.Second,
		Lease:        5 * time.Minute,
	}
}

func (w *Worker) Handle(kind string, h Handler) {
	w.handlers[kind] = h
}

// Run claims and executes jobs until ctx is cancelled, then waits for running
// jobs to finish.
func (w *Worker) Run(ctx context.Context) {
	sem := make(chan struct{}, w.Concurrency)
	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		free := w.Concurrency - len(sem)
		claimed := 0
		if free > 0 {
			jobs, err := w.q.ClaimJobs(ctx, store.ClaimJobsParams{
				LeaseSeconds: int32(w.Lease.Seconds()),
				BatchSize:    int32(free),
			})
			if err != nil && ctx.Err() == nil {
				w.log.Error("claim jobs", "err", err)
			}
			claimed = len(jobs)
			for _, job := range jobs {
				sem <- struct{}{}
				wg.Add(1)
				go func() {
					defer func() { <-sem; wg.Done() }()
					w.run(ctx, job)
				}()
			}
		}

		// A full batch means more may be due: go again without sleeping.
		wait := w.PollInterval
		if claimed > 0 && claimed == free {
			wait = 0
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

func (w *Worker) run(ctx context.Context, job store.Job) {
	log := logger(w.log, job)
	start := time.Now()
	err := w.call(ctx, job)
	metrics.JobDuration.WithLabelValues(job.Kind).Observe(time.Since(start).Seconds())

	// Settle the job even if the worker is shutting down.
	settle := context.WithoutCancel(ctx)

	switch {
	case err == nil:
		if err := w.q.CompleteJob(settle, job.ID); err != nil {
			log.Error("mark job done", "err", err)
		}
		metrics.JobsProcessed.WithLabelValues(job.Kind, "done").Inc()

	case errors.Is(err, ErrPermanent) || job.Attempts >= job.MaxAttempts:
		msg := err.Error()
		if err := w.q.FailJob(settle, store.FailJobParams{ID: job.ID, LastError: &msg}); err != nil {
			log.Error("mark job failed", "err", err)
		}
		metrics.JobsProcessed.WithLabelValues(job.Kind, "failed").Inc()
		log.Error("job failed permanently", "err", msg)

	default:
		msg := err.Error()
		next := time.Now().Add(Backoff(job.Attempts))
		if err := w.q.RetryJob(settle, store.RetryJobParams{ID: job.ID, RunAt: next, LastError: &msg}); err != nil {
			log.Error("schedule job retry", "err", err)
		}
		metrics.JobsProcessed.WithLabelValues(job.Kind, "retry").Inc()
		log.Warn("job failed, will retry", "err", msg, "retry_at", next)
	}
}

// call runs the handler, turning a missing handler or a panic into an error.
func (w *Worker) call(ctx context.Context, job store.Job) (err error) {
	h, ok := w.handlers[job.Kind]
	if !ok {
		return fmt.Errorf("%w: no handler for %q", ErrPermanent, job.Kind)
	}
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("panic: %v", v)
		}
	}()
	jobCtx, cancel := context.WithTimeout(ctx, w.Lease)
	defer cancel()
	return h(jobCtx, job.Payload)
}
