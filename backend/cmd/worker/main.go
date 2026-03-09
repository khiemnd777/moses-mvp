package main

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/khiemnd777/legal_api/core/embedding"
	"github.com/khiemnd777/legal_api/core/ingest"
	"github.com/khiemnd777/legal_api/infra"
	"github.com/khiemnd777/legal_api/pkg/config"
	"github.com/khiemnd777/legal_api/pkg/logging"

	_ "github.com/lib/pq"
)

func main() {
	logger := logging.New()
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "config/config.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}
	db, err := sql.Open("postgres", cfg.Postgres.DSN)
	if err != nil {
		logger.Error("failed to open db", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := infra.WaitForPostgres(
		context.Background(),
		db,
		infra.WaitPostgresOptions{
			MaxRetries: 15,
			Interval:   2 * time.Second,
			Timeout:    2 * time.Second,
		},
	); err != nil {
		log.Fatal("failed to wait for postgres:", err)
	}

	store := infra.NewStore(db)
	if err := store.Ping(context.Background()); err != nil {
		logger.Error("failed to ping db", slog.String("error", err.Error()))
		os.Exit(1)
	}
	_ = store.EnsureDocTypeSeed(context.Background())
	_ = store.EnsureAIConfigSeed(context.Background())

	storage := infra.NewStorage(cfg.Storage.RootDir)
	embed := embedding.NewClient(cfg.OpenAI.APIKey, cfg.OpenAI.EmbeddingsModel)
	qdrant := infra.NewQdrantClient(cfg.Qdrant.URL, cfg.Qdrant.Collection)
	_ = qdrant.EnsureCollection(context.Background(), 1536)

	service := &ingest.Service{
		Store:  store,
		Qdrant: qdrant,
		Embed:  embed,
		Config: ingest.Config{ChunkSize: cfg.Ingest.ChunkSize, ChunkOverlap: cfg.Ingest.ChunkOverlap},
		Logger: logger,
	}

	const (
		pollInterval      = 3 * time.Second
		processingTimeout = 15 * time.Minute
		maxAttempts       = 5
		retryScanLimit    = 50
	)

	for {
		ctx := context.Background()

		recovered, err := store.ResetStaleProcessingJobs(ctx, time.Now().Add(-processingTimeout))
		if err != nil {
			logger.Error("failed to reset stale processing jobs", slog.String("error", err.Error()))
			time.Sleep(pollInterval)
			continue
		}
		if recovered > 0 {
			logger.Info("stale_ingest_jobs_reset", slog.Int64("job_count", recovered))
		}

		if err := requeueFailedJobs(ctx, logger, store, retryScanLimit, maxAttempts); err != nil {
			logger.Error("failed to requeue failed jobs", slog.String("error", err.Error()))
			time.Sleep(pollInterval)
			continue
		}

		job, claimed, err := store.ClaimNextIngestJob(ctx)
		if err != nil {
			logger.Error("failed to claim ingest job", slog.String("error", err.Error()))
			time.Sleep(pollInterval)
			continue
		}
		if !claimed {
			time.Sleep(pollInterval)
			continue
		}

		attempt := infra.DecodeJobAttempt(job) + 1
		version, doc, asset, doctype, err := store.GetDocumentVersionBundle(ctx, job.DocumentVersionID)
		if err != nil {
			logger.Error("failed to load ingest bundle",
				slog.String("job_id", job.ID),
				slog.String("document_version_id", job.DocumentVersionID),
				slog.Int("attempt", attempt),
				slog.String("error", err.Error()),
			)
			_ = store.MarkJobFailed(ctx, job.ID, attempt, err.Error())
			continue
		}
		bundle := ingest.Bundle{Version: version, Document: doc, Asset: asset, DocType: doctype, Storage: storage}
		if err := service.Run(ctx, job, bundle); err != nil {
			logger.Error("ingest_job_failed",
				slog.String("job_id", job.ID),
				slog.String("document_id", doc.ID),
				slog.String("document_version_id", job.DocumentVersionID),
				slog.Int("attempt", attempt),
				slog.String("error", err.Error()),
			)
			_ = store.MarkJobFailed(ctx, job.ID, attempt, err.Error())
			continue
		}
		if err := store.MarkJobCompleted(ctx, job.ID); err != nil {
			logger.Error("failed to mark job completed",
				slog.String("job_id", job.ID),
				slog.String("document_version_id", job.DocumentVersionID),
				slog.String("error", err.Error()),
			)
			time.Sleep(pollInterval)
			continue
		}
	}
}

func requeueFailedJobs(ctx context.Context, logger *slog.Logger, store *infra.Store, limit, maxAttempts int) error {
	failedJobs, err := store.ListFailedIngestJobs(ctx, limit)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, job := range failedJobs {
		attempt := infra.DecodeJobAttempt(job)
		if attempt >= maxAttempts {
			continue
		}
		if now.Sub(job.UpdatedAt) < retryBackoff(attempt) {
			continue
		}
		if err := store.RequeueJob(ctx, job.ID); err != nil {
			return err
		}
		logger.Info("failed_ingest_job_requeued",
			slog.String("job_id", job.ID),
			slog.String("document_version_id", job.DocumentVersionID),
			slog.Int("attempt", attempt),
			slog.Duration("backoff", retryBackoff(attempt)),
		)
	}
	return nil
}

func retryBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 5 * time.Second
	}
	backoff := float64(5*time.Second) * math.Pow(2, float64(attempt-1))
	if backoff > float64(5*time.Minute) {
		backoff = float64(5 * time.Minute)
	}
	return time.Duration(backoff)
}
