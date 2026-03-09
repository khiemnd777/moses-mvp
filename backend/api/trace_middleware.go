package api

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/khiemnd777/legal_api/observability"
)

func answerTraceMiddleware(logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		traceID := uuid.NewString()
		ctx := observability.WithTraceID(c.UserContext(), traceID)
		c.SetUserContext(ctx)
		c.Set("X-Trace-Id", traceID)
		observability.LogInfo(ctx, logger, "api", "answer request received", map[string]interface{}{
			"method": c.Method(),
			"path":   c.Path(),
		})
		return c.Next()
	}
}
