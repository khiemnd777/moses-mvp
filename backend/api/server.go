package api

import (
	"context"
	"log/slog"
	"strings"
	"time"

	adminapi "github.com/khiemnd777/legal_api/admin"
	"github.com/khiemnd777/legal_api/admin/repository"
	adminservice "github.com/khiemnd777/legal_api/admin/service"
	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/core/embedding"
	"github.com/khiemnd777/legal_api/core/ingest"
	"github.com/khiemnd777/legal_api/core/retrieval"
	"github.com/khiemnd777/legal_api/infra"
	"github.com/khiemnd777/legal_api/internal/auth"
	"github.com/khiemnd777/legal_api/observability"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

type Server struct {
	App         *fiber.App
	Store       *infra.Store
	Storage     *infra.Storage
	Embedder    *embedding.Client
	Qdrant      *infra.QdrantClient
	Retriever   *retrieval.Service
	Answer      *answer.Client
	Tones       map[string]string
	Ingest      *ingest.Service
	Logger      *slog.Logger
	TraceRepo   observability.TraceRepository
	AuthService *auth.Service
}

func NewServer(store *infra.Store, storage *infra.Storage, embedder *embedding.Client, qdrant *infra.QdrantClient, ans *answer.Client, authService *auth.Service, tones map[string]string, logger *slog.Logger, ingestCfg ingest.Config) *Server {
	return NewServerWithCORS(store, storage, embedder, qdrant, ans, authService, tones, logger, ingestCfg, []string{"http://localhost:5173"})
}

func NewServerWithCORS(store *infra.Store, storage *infra.Storage, embedder *embedding.Client, qdrant *infra.QdrantClient, ans *answer.Client, authService *auth.Service, tones map[string]string, logger *slog.Logger, ingestCfg ingest.Config, allowedOrigins []string) *Server {
	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins: strings.Join(allowedOrigins, ","),
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))
	retriever := &retrieval.Service{Store: store, Qdrant: qdrant, Embed: embedder, Logger: logger}
	ingestSvc := &ingest.Service{Store: store, Qdrant: qdrant, Embed: embedder, Config: ingestCfg, Logger: logger}
	return &Server{
		App:         app,
		Store:       store,
		Storage:     storage,
		Embedder:    embedder,
		Qdrant:      qdrant,
		Retriever:   retriever,
		Answer:      ans,
		Tones:       tones,
		Ingest:      ingestSvc,
		Logger:      logger,
		TraceRepo:   observability.NewSQLTraceRepository(store.DB),
		AuthService: authService,
	}
}

func (s *Server) RegisterRoutes() {
	h := NewHandler(s.Store, s.Storage, s.Qdrant, s.Retriever, s.Answer, s.Tones, s.Ingest, s.Logger, s.TraceRepo)
	traceMiddleware := answerTraceMiddleware(s.Logger)
	authMiddleware := auth.RequireAuth(s.AuthService.JWTManager(), s.Store)
	authHandlers := auth.NewHandlers(s.AuthService, auth.NewLoginRateLimiter(5, time.Minute))

	s.App.Post("/auth/login", authHandlers.Login)
	s.App.Get("/auth/me", authMiddleware, authHandlers.Me)
	s.App.Post("/auth/change-password", authMiddleware, authHandlers.ChangePassword)

	s.App.Post("/chat", traceMiddleware, h.Answer)
	s.App.Post("/search", h.Search)
	s.App.Get("/health", s.Health)
	s.App.Get("/metrics", observability.MetricsHandler)
	s.App.Post("/answer", traceMiddleware, h.Answer)
	s.App.Post("/answer/stream", traceMiddleware, h.AnswerStream)

	s.App.Use("/playground", authMiddleware)
	s.App.Use("/tuning", authMiddleware)
	s.App.Use("/ingest", authMiddleware)

	protectedGroup := s.App.Group("", authMiddleware)
	protectedGroup.Post("/doc-types", h.CreateDocType)
	protectedGroup.Get("/doc-types", h.ListDocTypes)
	protectedGroup.Put("/doc-types/:id/form", h.UpdateDocTypeForm)
	protectedGroup.Delete("/doc-types/:id", h.DeleteDocType)
	protectedGroup.Get("/documents", h.ListDocuments)
	protectedGroup.Post("/documents", h.CreateDocument)
	protectedGroup.Delete("/documents/:id", h.DeleteDocument)
	protectedGroup.Post("/documents/:id/assets", h.AddDocumentAsset)
	protectedGroup.Post("/documents/:id/versions", h.CreateDocumentVersion)
	protectedGroup.Delete("/document-versions/:id", h.DeleteDocumentVersion)
	protectedGroup.Get("/ingest-jobs", h.ListIngestJobs)
	protectedGroup.Delete("/ingest-jobs/:id", h.DeleteIngestJob)
	protectedGroup.Post("/document-versions/:id/ingest", h.EnqueueIngest)
	protectedGroup.Post("/conversations", h.CreateConversation)
	protectedGroup.Get("/conversations", h.ListConversations)
	protectedGroup.Get("/conversations/:id", h.GetConversation)
	protectedGroup.Delete("/conversations/:id", h.DeleteConversation)
	protectedGroup.Post("/messages", h.CreateMessage)
	protectedGroup.Get("/messages", h.ListMessages)
	protectedGroup.Post("/messages/stream", h.StreamMessage)
	protectedGroup.Get("/assets/:id/download", h.DownloadAsset)

	adminGroup := s.App.Group("/admin", authMiddleware)
	guardRepo := repository.NewGuardPolicyRepository(s.Store)
	promptRepo := repository.NewPromptRepository(s.Store)
	retrievalCfgRepo := repository.NewRetrievalConfigRepository(s.Store)
	guardSvc := adminservice.NewGuardPolicyService(guardRepo)
	promptSvc := adminservice.NewPromptService(promptRepo)
	retrievalCfgSvc := adminservice.NewRetrievalConfigService(retrievalCfgRepo)
	guardHandler := adminapi.NewGuardPolicyHandler(guardSvc, h.InvalidateRuntimeAnswerConfigCache)
	defaultTone := s.Tones[defaultToneKey]
	promptHandler := adminapi.NewPromptHandler(promptSvc, s.Retriever, s.Answer, defaultTone, h.InvalidateRuntimeAnswerConfigCache)
	onRetrievalConfigChanged := func() {
		s.Retriever.InvalidateRuntimeConfigCache()
	}
	retrievalConfigHandler := adminapi.NewRetrievalConfigHandler(retrievalCfgSvc, onRetrievalConfigChanged)
	answerTraceHandler := adminapi.NewAIAnswerTraceHandler(s.TraceRepo)
	qdrantControlSvc := adminservice.NewQdrantControlPlaneService(s.Store, s.Qdrant, s.Embedder, s.Logger)
	qdrantHandler := adminapi.NewQdrantControlPlaneHandler(qdrantControlSvc, s.Logger)
	adminapi.RegisterRoutes(adminGroup, guardHandler, promptHandler, retrievalConfigHandler, answerTraceHandler, qdrantHandler)
}

func (s *Server) Start(ctx context.Context, addr string) error {
	s.RegisterRoutes()
	return s.App.Listen(addr)
}

func (s *Server) Health(c *fiber.Ctx) error {
	ctx := c.UserContext()
	status := fiber.Map{
		"postgres": "ok",
		"qdrant":   "ok",
		"openai":   "ok",
	}
	httpStatus := fiber.StatusOK
	if err := s.Store.Ping(ctx); err != nil {
		status["postgres"] = "error"
		httpStatus = fiber.StatusServiceUnavailable
	}
	if err := s.Qdrant.HealthCheck(ctx); err != nil {
		status["qdrant"] = "error"
		httpStatus = fiber.StatusServiceUnavailable
	}
	if err := s.Answer.HealthCheck(ctx); err != nil {
		status["openai"] = "error"
		httpStatus = fiber.StatusServiceUnavailable
	}
	return c.Status(httpStatus).JSON(status)
}

func adminAuthMiddleware(adminKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		adminKey = strings.TrimSpace(adminKey)
		if adminKey == "" {
			// MVP mode: allow admin routes when no key is configured.
			return c.Next()
		}
		clientKey := strings.TrimSpace(c.Get("X-Admin-Key"))
		if clientKey == "" {
			auth := strings.TrimSpace(c.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				clientKey = strings.TrimSpace(auth[7:])
			}
		}
		if clientKey == "" || clientKey != adminKey {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "unauthorized",
					"message": "invalid admin credentials",
				},
			})
		}
		return c.Next()
	}
}
