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
	"github.com/khiemnd777/legal_api/pkg/config"
	"github.com/khiemnd777/legal_api/pkg/logging"
	"github.com/khiemnd777/legal_api/pkg/prompt"

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
	_ = qdrant.EnsureCollection(context.Background(), 1536)
	ansClient := answer.NewClient(cfg.OpenAI.APIKey, cfg.OpenAI.ChatModel)
	tones := map[string]string{
		"default":   defaultTone.Content,
		"academic":  acadTone.Content,
		"procedure": procTone.Content,
	}
	adminAPIKey := os.Getenv("ADMIN_API_KEY")

	server := api.NewServer(store, storage, embed, qdrant, ansClient, adminAPIKey, tones, logger, ingest.Config{ChunkSize: cfg.Ingest.ChunkSize, ChunkOverlap: cfg.Ingest.ChunkOverlap})
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Info("api server starting", slog.String("addr", addr))
	if err := server.Start(context.Background(), addr); err != nil {
		logger.Error("api server stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
