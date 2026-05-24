package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/observability"
)

func (h *Handler) Metrics(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	output := observability.Metrics.RenderPrometheus()
	reader, ok := h.Store.(pipelineHealthReader)
	if !ok {
		return c.SendString(output + observability.RenderPipelineMetricsUnavailablePrometheus())
	}

	now := time.Now().UTC()
	health, err := reader.GetPipelineHealth(c.Context(), domain.PipelineHealthOptions{
		RecentSince: now.Add(-24 * time.Hour),
		StaleBefore: now.Add(-15 * time.Minute),
		Limit:       20,
	})
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("pipeline_metrics_unavailable", "error", err.Error())
		}
		return c.SendString(output + observability.RenderPipelineMetricsUnavailablePrometheus())
	}
	return c.SendString(output + observability.RenderPipelineHealthPrometheus(health))
}
