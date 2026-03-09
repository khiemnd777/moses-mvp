package admin

import (
	"database/sql"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/observability"
)

type AIAnswerTraceHandler struct {
	Repo observability.TraceRepository
}

func NewAIAnswerTraceHandler(repo observability.TraceRepository) *AIAnswerTraceHandler {
	return &AIAnswerTraceHandler{Repo: repo}
}

func (h *AIAnswerTraceHandler) List(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	items, err := h.Repo.List(c.Context(), limit)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to list answer traces", err.Error())
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *AIAnswerTraceHandler) Get(c *fiber.Ctx) error {
	item, err := h.Repo.GetByTraceID(c.Context(), c.Params("traceID"))
	if err != nil {
		if err == sql.ErrNoRows {
			return respondError(c, fiber.StatusNotFound, "not_found", "answer trace not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to load answer trace", err.Error())
	}
	return c.JSON(item)
}
