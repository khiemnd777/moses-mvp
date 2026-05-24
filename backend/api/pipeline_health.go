package api

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/domain"
)

type pipelineHealthReader interface {
	GetPipelineHealth(ctx context.Context, opts domain.PipelineHealthOptions) (domain.PipelineHealth, error)
}

type pipelineHealthResponse struct {
	GeneratedAt       time.Time                          `json:"generated_at"`
	RecentSince       time.Time                          `json:"recent_since"`
	StaleBefore       time.Time                          `json:"stale_before"`
	Severity          string                             `json:"severity"`
	Alerts            []pipelineHealthAlertResponse      `json:"alerts"`
	Summary           pipelineHealthSummaryResponse      `json:"summary"`
	UploadStatusCount []pipelineStatusCountResponse      `json:"upload_status_counts"`
	JobStatusCount    []pipelineStatusCountResponse      `json:"job_status_counts"`
	StageStatusCount  []pipelineStageStatusCountResponse `json:"stage_status_counts"`
	Security          pipelineSecurityStatsResponse      `json:"security"`
	Latency           pipelineLatencyStatsResponse       `json:"latency"`
	StaleUploads      []pipelineHealthIssueResponse      `json:"stale_uploads"`
	RecentIssues      []pipelineHealthIssueResponse      `json:"recent_issues"`
}

type pipelineHealthAlertResponse struct {
	Code      string  `json:"code"`
	Severity  string  `json:"severity"`
	Message   string  `json:"message"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
}

type pipelineHealthSummaryResponse struct {
	TotalUploads        int `json:"total_uploads"`
	ProcessingUploads   int `json:"processing_uploads"`
	ReviewUploads       int `json:"review_uploads"`
	FailedUploads       int `json:"failed_uploads"`
	PublishedUploads    int `json:"published_uploads"`
	ActiveJobs          int `json:"active_jobs"`
	FailedJobs          int `json:"failed_jobs"`
	StaleUploads        int `json:"stale_uploads"`
	RecentIssues        int `json:"recent_issues"`
	SecurityBlocked     int `json:"security_blocked"`
	SecurityUnavailable int `json:"security_unavailable"`
}

type pipelineStatusCountResponse struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type pipelineStageStatusCountResponse struct {
	Stage  string `json:"stage"`
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type pipelineSecurityStatsResponse struct {
	Passed      int `json:"passed"`
	Blocked     int `json:"blocked"`
	Unavailable int `json:"unavailable"`
}

type pipelineLatencyStatsResponse struct {
	CompletedCount int     `json:"completed_count"`
	AverageSeconds float64 `json:"average_seconds"`
	P50Seconds     float64 `json:"p50_seconds"`
	P95Seconds     float64 `json:"p95_seconds"`
	MaxSeconds     float64 `json:"max_seconds"`
}

type pipelineHealthIssueResponse struct {
	UploadID       *string    `json:"upload_id,omitempty"`
	Title          string     `json:"title"`
	FileName       string     `json:"file_name"`
	Status         string     `json:"status"`
	Stage          string     `json:"stage"`
	EventStatus    string     `json:"event_status"`
	Message        string     `json:"message"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	AgeSeconds     int64      `json:"age_seconds"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	EventCreatedAt *time.Time `json:"event_created_at,omitempty"`
}

func (h *Handler) GetPipelineHealth(c *fiber.Ctx) error {
	reader, ok := h.Store.(pipelineHealthReader)
	if !ok {
		return respondError(c, fiber.StatusNotImplemented, "not_implemented", "pipeline health is not available for this store", nil)
	}
	recentHours, err := boundedIntQuery(c, "recent_hours", 24, 1, 168)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "validation", "recent_hours must be an integer", nil)
	}
	staleMinutes, err := boundedIntQuery(c, "stale_minutes", 15, 1, 1440)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "validation", "stale_minutes must be an integer", nil)
	}
	limit, err := boundedIntQuery(c, "limit", 20, 1, 100)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "validation", "limit must be an integer", nil)
	}

	now := time.Now().UTC()
	health, err := reader.GetPipelineHealth(c.Context(), domain.PipelineHealthOptions{
		RecentSince: now.Add(-time.Duration(recentHours) * time.Hour),
		StaleBefore: now.Add(-time.Duration(staleMinutes) * time.Minute),
		Limit:       limit,
	})
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to load pipeline health", err.Error())
	}
	return c.JSON(toPipelineHealthResponse(health))
}

func boundedIntQuery(c *fiber.Ctx, name string, fallback, minValue, maxValue int) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if value < minValue {
		return minValue, nil
	}
	if value > maxValue {
		return maxValue, nil
	}
	return value, nil
}

func toPipelineHealthResponse(health domain.PipelineHealth) pipelineHealthResponse {
	return pipelineHealthResponse{
		GeneratedAt:       health.GeneratedAt,
		RecentSince:       health.RecentSince,
		StaleBefore:       health.StaleBefore,
		Severity:          health.Severity,
		Alerts:            toPipelineHealthAlertResponses(health.Alerts),
		Summary:           toPipelineHealthSummaryResponse(health.Summary),
		UploadStatusCount: toPipelineStatusCountResponses(health.UploadStatusCount),
		JobStatusCount:    toPipelineStatusCountResponses(health.JobStatusCount),
		StageStatusCount:  toPipelineStageStatusCountResponses(health.StageStatusCount),
		Security:          toPipelineSecurityStatsResponse(health.Security),
		Latency:           toPipelineLatencyStatsResponse(health.Latency),
		StaleUploads:      toPipelineHealthIssueResponses(health.StaleUploads),
		RecentIssues:      toPipelineHealthIssueResponses(health.RecentIssues),
	}
}

func toPipelineHealthAlertResponses(alerts []domain.PipelineHealthAlert) []pipelineHealthAlertResponse {
	out := make([]pipelineHealthAlertResponse, 0, len(alerts))
	for _, item := range alerts {
		out = append(out, pipelineHealthAlertResponse{
			Code:      item.Code,
			Severity:  item.Severity,
			Message:   item.Message,
			Value:     item.Value,
			Threshold: item.Threshold,
		})
	}
	return out
}

func toPipelineHealthSummaryResponse(summary domain.PipelineHealthSummary) pipelineHealthSummaryResponse {
	return pipelineHealthSummaryResponse{
		TotalUploads:        summary.TotalUploads,
		ProcessingUploads:   summary.ProcessingUploads,
		ReviewUploads:       summary.ReviewUploads,
		FailedUploads:       summary.FailedUploads,
		PublishedUploads:    summary.PublishedUploads,
		ActiveJobs:          summary.ActiveJobs,
		FailedJobs:          summary.FailedJobs,
		StaleUploads:        summary.StaleUploads,
		RecentIssues:        summary.RecentIssues,
		SecurityBlocked:     summary.SecurityBlocked,
		SecurityUnavailable: summary.SecurityUnavailable,
	}
}

func toPipelineStatusCountResponses(counts []domain.PipelineStatusCount) []pipelineStatusCountResponse {
	out := make([]pipelineStatusCountResponse, 0, len(counts))
	for _, item := range counts {
		out = append(out, pipelineStatusCountResponse{Status: item.Status, Count: item.Count})
	}
	return out
}

func toPipelineStageStatusCountResponses(counts []domain.PipelineStageStatusCount) []pipelineStageStatusCountResponse {
	out := make([]pipelineStageStatusCountResponse, 0, len(counts))
	for _, item := range counts {
		out = append(out, pipelineStageStatusCountResponse{Stage: item.Stage, Status: item.Status, Count: item.Count})
	}
	return out
}

func toPipelineSecurityStatsResponse(stats domain.PipelineSecurityStats) pipelineSecurityStatsResponse {
	return pipelineSecurityStatsResponse{
		Passed:      stats.Passed,
		Blocked:     stats.Blocked,
		Unavailable: stats.Unavailable,
	}
}

func toPipelineLatencyStatsResponse(stats domain.PipelineLatencyStats) pipelineLatencyStatsResponse {
	return pipelineLatencyStatsResponse{
		CompletedCount: stats.CompletedCount,
		AverageSeconds: stats.AverageSeconds,
		P50Seconds:     stats.P50Seconds,
		P95Seconds:     stats.P95Seconds,
		MaxSeconds:     stats.MaxSeconds,
	}
}

func toPipelineHealthIssueResponses(issues []domain.PipelineHealthIssue) []pipelineHealthIssueResponse {
	out := make([]pipelineHealthIssueResponse, 0, len(issues))
	for _, item := range issues {
		out = append(out, pipelineHealthIssueResponse{
			UploadID:       item.UploadID,
			Title:          item.Title,
			FileName:       item.FileName,
			Status:         item.Status,
			Stage:          item.Stage,
			EventStatus:    item.EventStatus,
			Message:        item.Message,
			ErrorMessage:   item.ErrorMessage,
			AgeSeconds:     item.AgeSeconds,
			CreatedAt:      item.CreatedAt,
			UpdatedAt:      item.UpdatedAt,
			EventCreatedAt: item.EventCreatedAt,
		})
	}
	return out
}
