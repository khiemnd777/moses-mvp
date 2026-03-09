package api

import (
	"context"
	"log/slog"

	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/core/embedding"
	"github.com/khiemnd777/legal_api/core/ingest"
	"github.com/khiemnd777/legal_api/core/retrieval"
	"github.com/khiemnd777/legal_api/infra"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

type Server struct {
	App       *fiber.App
	Store     *infra.Store
	Storage   *infra.Storage
	Embedder  *embedding.Client
	Retriever *retrieval.Service
	Answer    *answer.Client
	Tones     map[string]string
	Ingest    *ingest.Service
	Logger    *slog.Logger
}

func NewServer(store *infra.Store, storage *infra.Storage, embedder *embedding.Client, qdrant *infra.QdrantClient, ans *answer.Client, tones map[string]string, logger *slog.Logger, ingestCfg ingest.Config) *Server {
	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:5173",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))
	retriever := &retrieval.Service{Store: store, Qdrant: qdrant, Embed: embedder}
	ingestSvc := &ingest.Service{Store: store, Qdrant: qdrant, Embed: embedder, Config: ingestCfg, Logger: logger}
	return &Server{
		App:       app,
		Store:     store,
		Storage:   storage,
		Embedder:  embedder,
		Retriever: retriever,
		Answer:    ans,
		Tones:     tones,
		Ingest:    ingestSvc,
		Logger:    logger,
	}
}

func (s *Server) RegisterRoutes() {
	h := NewHandler(s.Store, s.Storage, s.Retriever, s.Answer, s.Tones, s.Ingest, s.Logger)
	s.App.Post("/doc-types", h.CreateDocType)
	s.App.Get("/doc-types", h.ListDocTypes)
	s.App.Put("/doc-types/:id/form", h.UpdateDocTypeForm)
	s.App.Delete("/doc-types/:id", h.DeleteDocType)
	s.App.Get("/documents", h.ListDocuments)
	s.App.Post("/documents", h.CreateDocument)
	s.App.Delete("/documents/:id", h.DeleteDocument)
	s.App.Post("/documents/:id/assets", h.AddDocumentAsset)
	s.App.Post("/documents/:id/versions", h.CreateDocumentVersion)
	s.App.Get("/ingest-jobs", h.ListIngestJobs)
	s.App.Delete("/ingest-jobs/:id", h.DeleteIngestJob)
	s.App.Post("/document-versions/:id/ingest", h.EnqueueIngest)
	s.App.Post("/search", h.Search)
	s.App.Post("/answer", h.Answer)
	s.App.Post("/answer/stream", h.AnswerStream)
}

func (s *Server) Start(ctx context.Context, addr string) error {
	s.RegisterRoutes()
	return s.App.Listen(addr)
}
