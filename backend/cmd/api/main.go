package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/khiemnd777/legal_api/api"
	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/core/embedding"
	"github.com/khiemnd777/legal_api/core/ingest"
	"github.com/khiemnd777/legal_api/infra"
	"github.com/khiemnd777/legal_api/internal/auth"
	envconfig "github.com/khiemnd777/legal_api/internal/config"
	appconfig "github.com/khiemnd777/legal_api/pkg/config"
	"github.com/khiemnd777/legal_api/pkg/logging"
	"github.com/khiemnd777/legal_api/pkg/prompt"

	_ "github.com/lib/pq"
)

func main() {
	envCfg, err := envconfig.Load()
	if err != nil {
		log.Fatal(err)
	}

	logger := logging.New()
	slog.SetDefault(logger)
	cfg, err := appconfig.Load(envCfg.ConfigPath)
	if err != nil {
		logger.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}
	databaseURL := cfg.Postgres.DSN
	if envCfg.DatabaseURL != "" {
		databaseURL = envCfg.DatabaseURL
	}
	if databaseURL == "" {
		logger.Error("database connection is empty; set DATABASE_URL or postgres.dsn")
		os.Exit(1)
	}
	db, err := sql.Open("postgres", databaseURL)
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
	if err := store.EnsureVectorRepairSchema(context.Background()); err != nil {
		logger.Error("failed to ensure vector repair schema", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defaultTone, err := prompt.Load(cfg.Prompts.ToneDefault)
	if err != nil {
		logger.Error("failed to load default tone", slog.String("error", err.Error()))
		os.Exit(1)
	}
	acadTone, err := prompt.Load(cfg.Prompts.ToneAcademic)
	if err != nil {
		logger.Error("failed to load academic tone", slog.String("error", err.Error()))
		os.Exit(1)
	}
	procTone, err := prompt.Load(cfg.Prompts.ToneProcedure)
	if err != nil {
		logger.Error("failed to load procedure tone", slog.String("error", err.Error()))
		os.Exit(1)
	}

	storage := infra.NewStorage(cfg.Storage.RootDir)
	embed := embedding.NewClient(cfg.OpenAI.APIKey, cfg.OpenAI.EmbeddingsModel)
	qdrant := infra.NewQdrantClient(cfg.Qdrant.URL, cfg.Qdrant.Collection)
	vectorDim, err := embedding.ExpectedDimensions(cfg.OpenAI.EmbeddingsModel)
	if err != nil {
		logger.Error("failed to resolve embedding dimension", slog.String("model", cfg.OpenAI.EmbeddingsModel), slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := qdrant.EnsureCollection(context.Background(), vectorDim); err != nil {
		logger.Error("failed to ensure qdrant collection", slog.String("collection", cfg.Qdrant.Collection), slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := qdrant.ValidateCollectionDimension(context.Background(), vectorDim); err != nil {
		logger.Error("qdrant collection validation failed", slog.String("collection", cfg.Qdrant.Collection), slog.String("error", err.Error()))
		os.Exit(1)
	}
	ansClient := answer.NewClient(cfg.OpenAI.APIKey, cfg.OpenAI.ChatModel)
	tones := map[string]string{
		"default":   defaultTone.Content,
		"academic":  acadTone.Content,
		"procedure": procTone.Content,
	}
	authService := auth.NewService(store, auth.Config{
		Secret:   envCfg.JWTSecret,
		Issuer:   "legal_api",
		TokenTTL: time.Hour,
	})
	if err := auth.BootstrapAdmin(context.Background(), store, envCfg.AdminBootstrapPassword, logger); err != nil {
		logger.Error("failed to bootstrap admin user", slog.String("error", err.Error()))
		os.Exit(1)
	}

	server := api.NewServer(store, storage, embed, qdrant, ansClient, authService, tones, logger, ingest.Config{ChunkSize: cfg.Ingest.ChunkSize, ChunkOverlap: cfg.Ingest.ChunkOverlap})
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, envCfg.ServerPort)
	logger.Info("api server starting", slog.String("addr", addr))
	if err := server.Start(context.Background(), addr); err != nil {
		logger.Error("api server stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
