// Command rift runs the Rift API server, its background job worker, and an
// internal metrics/pprof endpoint.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"rift/server/internal/auth"
	"rift/server/internal/blob"
	"rift/server/internal/config"
	"rift/server/internal/database"
	"rift/server/internal/httpapi"
	"rift/server/internal/jobs"
	"rift/server/internal/keys"
	"rift/server/internal/library"
	"rift/server/internal/mail"
	"rift/server/internal/metrics"
	"rift/server/internal/realtime"
	"rift/server/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		return err
	}
	log.Info("database ready")

	authSvc := auth.NewService(pool, auth.NewAccessTokens(cfg.TokenSecret))

	keyring, err := keys.NewKeyring(cfg.MasterKey)
	if err != nil {
		return err
	}
	storage := blob.NewS3(blob.Config{
		Endpoint: cfg.S3.Endpoint, Region: cfg.S3.Region, Bucket: cfg.S3.Bucket,
		AccessKey: cfg.S3.AccessKey, SecretKey: cfg.S3.SecretKey,
	})
	if cfg.S3.CreateBucket {
		if err := storage.EnsureBucket(ctx); err != nil {
			return err
		}
	}
	librarySvc := library.NewService(pool, keyring, storage)
	broker := realtime.NewBroker(cfg.DatabaseURL, log)
	queries := store.New(pool)

	var mailer mail.Mailer = mail.LogMailer{Log: log}
	if cfg.Mail.ResendAPIKey != "" {
		mailer = mail.NewResend(cfg.Mail.ResendAPIKey, cfg.Mail.From)
	}
	var google auth.GoogleAuth
	if cfg.Google.Enabled() {
		google = auth.NewGoogle(cfg.Google.ClientID, cfg.Google.ClientSecret, cfg.WebURL+"/api/auth/google/callback")
	} else {
		log.Info("google sign-in disabled: GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET not set")
	}

	api := &http.Server{
		Addr: cfg.Addr,
		Handler: httpapi.New(httpapi.Deps{
			Log:          log,
			DB:           pool,
			Store:        queries,
			Auth:         authSvc,
			Library:      librarySvc,
			Broker:       broker,
			Google:       google,
			Mailer:       mailer,
			WebURL:       cfg.WebURL,
			WebOrigins:   cfg.WebOrigins,
			CookieSecure: cfg.CookieSecure,
			TrustProxy:   cfg.TrustProxy,
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	internal := &http.Server{
		Addr:              cfg.InternalAddr,
		Handler:           metrics.InternalHandler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	worker := jobs.NewWorker(queries, log)
	worker.Handle(jobKindAuthCleanup, func(ctx context.Context, _ json.RawMessage) error { return authSvc.Cleanup(ctx) })
	worker.Handle(jobKindJobsCleanup, func(ctx context.Context, _ json.RawMessage) error {
		_, err := queries.DeleteFinishedJobs(ctx)
		return err
	})
	worker.Handle(jobKindUploadsCleanup, func(ctx context.Context, _ json.RawMessage) error {
		_, err := librarySvc.CleanupStaleUploads(ctx, 24*time.Hour)
		return err
	})
	worker.Handle(jobKindTrashPurge, func(ctx context.Context, _ json.RawMessage) error {
		_, err := librarySvc.PurgeTrash(ctx)
		return err
	})
	worker.Handle(jobKindIdempotencyCleanup, func(ctx context.Context, _ json.RawMessage) error {
		_, err := queries.DeleteExpiredIdempotencyKeys(ctx)
		return err
	})

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return serve(gctx, log, "api", api) })
	g.Go(func() error { return serve(gctx, log, "internal", internal) })
	g.Go(func() error { worker.Run(gctx); return nil })
	g.Go(func() error { broker.Run(gctx); return nil })
	g.Go(func() error {
		jobs.Schedule(gctx, queries, log,
			jobs.Periodic{Kind: jobKindAuthCleanup, Interval: time.Hour},
			jobs.Periodic{Kind: jobKindJobsCleanup, Interval: 6 * time.Hour},
			jobs.Periodic{Kind: jobKindUploadsCleanup, Interval: time.Hour},
			jobs.Periodic{Kind: jobKindTrashPurge, Interval: time.Hour},
			jobs.Periodic{Kind: jobKindIdempotencyCleanup, Interval: time.Hour},
		)
		return nil
	})
	g.Go(func() error { jobs.ReportQueueDepth(gctx, queries, 30*time.Second); return nil })
	return g.Wait()
}

const (
	jobKindAuthCleanup        = "auth.cleanup"
	jobKindJobsCleanup        = "jobs.cleanup"
	jobKindUploadsCleanup     = "uploads.cleanup"
	jobKindTrashPurge         = "trash.purge"
	jobKindIdempotencyCleanup = "idempotency.cleanup"
)

// serve runs srv until ctx ends, then shuts it down gracefully.
func serve(ctx context.Context, log *slog.Logger, name string, srv *http.Server) error {
	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "server", name, "addr", srv.Addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Info("shutting down", "server", name)
	return srv.Shutdown(shutdownCtx)
}
