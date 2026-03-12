package admin

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

type errorEnvelope struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}

func respondError(c *fiber.Ctx, code int, errCode, message string, details interface{}) error {
	var env errorEnvelope
	env.Error.Code = errCode
	env.Error.Message = message
	env.Error.Details = details
	return c.Status(code).JSON(env)
}

func RegisterRoutes(
	group fiber.Router,
	guardHandler *GuardPolicyHandler,
	promptHandler *PromptHandler,
	retrievalConfigHandler *RetrievalConfigHandler,
	answerTraceHandler *AIAnswerTraceHandler,
	qdrantHandler *QdrantControlPlaneHandler,
) {
	group.Get("/ai/guard-policies", guardHandler.List)
	group.Get("/ai/guard-policies/:id", guardHandler.Get)
	group.Post("/ai/guard-policies", guardHandler.Create)
	group.Put("/ai/guard-policies/:id", guardHandler.Update)
	group.Delete("/ai/guard-policies/:id", guardHandler.Delete)

	group.Get("/ai/prompts", promptHandler.List)
	group.Get("/ai/prompts/:id", promptHandler.Get)
	group.Post("/ai/prompts", promptHandler.Create)
	group.Put("/ai/prompts/:id", promptHandler.Update)
	group.Delete("/ai/prompts/:id", promptHandler.Delete)
	group.Post("/ai/prompts/test", promptHandler.Test)

	group.Get("/ai/retrieval-configs", retrievalConfigHandler.List)
	group.Get("/ai/retrieval-configs/:id", retrievalConfigHandler.Get)
	group.Post("/ai/retrieval-configs", retrievalConfigHandler.Create)
	group.Put("/ai/retrieval-configs/:id", retrievalConfigHandler.Update)
	group.Delete("/ai/retrieval-configs/:id", retrievalConfigHandler.Delete)
	group.Post("/ai/retrieval-configs/:id/enable", retrievalConfigHandler.Enable)
	group.Post("/ai/retrieval-configs/:id/disable", retrievalConfigHandler.Disable)

	group.Get("/ai/answer-traces", answerTraceHandler.List)
	group.Get("/ai/answer-traces/:traceID", answerTraceHandler.Get)

	qdrantGroup := group.Group("/qdrant")
	qdrantGroup.Get("/collections", qdrantHandler.ListCollections)
	qdrantGroup.Get("/collections/:name", qdrantHandler.GetCollection)
	qdrantGroup.Post("/search_debug", qdrantRateLimitMiddleware(qdrantRatePolicy{Limit: 5, Window: time.Second}), qdrantHandler.SearchDebug)
	qdrantGroup.Get("/vector_health", qdrantRateLimitMiddleware(qdrantRatePolicy{Limit: 2, Window: time.Second}), qdrantHandler.VectorHealth)
	qdrantGroup.Post("/delete_by_filter", qdrantRateLimitMiddleware(qdrantRatePolicy{Limit: 1, Window: time.Second}), qdrantHandler.DeleteByFilter)
	qdrantGroup.Post("/reindex_document", qdrantRateLimitMiddleware(qdrantRatePolicy{Limit: 1, Window: 10 * time.Second}), qdrantHandler.ReindexDocument)
	qdrantGroup.Post("/reindex_all", qdrantRateLimitMiddleware(qdrantRatePolicy{Limit: 1, Window: 10 * time.Second}), qdrantHandler.ReindexAll)
}
