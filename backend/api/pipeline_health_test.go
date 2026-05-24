package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/domain"
)

type pipelineHealthTestStore struct {
	fakeStore
	opts domain.PipelineHealthOptions
}

func (s *pipelineHealthTestStore) GetPipelineHealth(ctx context.Context, opts domain.PipelineHealthOptions) (domain.PipelineHealth, error) {
	s.opts = opts
	uploadID := "upload-stale"
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	eventAt := now.Add(-10 * time.Minute)
	return domain.PipelineHealth{
		GeneratedAt: now,
		RecentSince: opts.RecentSince,
		StaleBefore: opts.StaleBefore,
		Severity:    "critical",
		Alerts: []domain.PipelineHealthAlert{
			{
				Code:      "stale_uploads",
				Severity:  "critical",
				Message:   "Uploads are stuck beyond the stale threshold",
				Value:     1,
				Threshold: 0,
			},
		},
		Summary: domain.PipelineHealthSummary{
			TotalUploads:        7,
			ProcessingUploads:   2,
			ReviewUploads:       1,
			FailedUploads:       1,
			PublishedUploads:    3,
			ActiveJobs:          2,
			FailedJobs:          1,
			StaleUploads:        1,
			RecentIssues:        1,
			SecurityBlocked:     1,
			SecurityUnavailable: 0,
		},
		UploadStatusCount: []domain.PipelineStatusCount{
			{Status: "indexing", Count: 2},
			{Status: "ready", Count: 3},
		},
		JobStatusCount: []domain.PipelineStatusCount{
			{Status: "queued", Count: 2},
			{Status: "failed", Count: 1},
		},
		StageStatusCount: []domain.PipelineStageStatusCount{
			{Stage: "security", Status: "blocked", Count: 1},
		},
		Security: domain.PipelineSecurityStats{
			Passed:  4,
			Blocked: 1,
		},
		Latency: domain.PipelineLatencyStats{
			CompletedCount: 3,
			AverageSeconds: 42,
			P50Seconds:     39,
			P95Seconds:     61,
			MaxSeconds:     64,
		},
		StaleUploads: []domain.PipelineHealthIssue{
			{
				UploadID:    &uploadID,
				Title:       "Hợp đồng vay",
				FileName:    "hop-dong-vay.docx",
				Status:      "indexing",
				Stage:       "embed_index",
				EventStatus: "active",
				Message:     "Upload has not advanced before the stale threshold",
				AgeSeconds:  1800,
				CreatedAt:   now.Add(-40 * time.Minute),
				UpdatedAt:   now.Add(-30 * time.Minute),
			},
		},
		RecentIssues: []domain.PipelineHealthIssue{
			{
				Title:          "Malware sample",
				FileName:       "malware.docx",
				Status:         "",
				Stage:          "security",
				EventStatus:    "blocked",
				Message:        "upload failed malware scan",
				AgeSeconds:     600,
				CreatedAt:      eventAt,
				UpdatedAt:      eventAt,
				EventCreatedAt: &eventAt,
			},
		},
	}, nil
}

func TestGetPipelineHealthReturnsOperationalSnapshot(t *testing.T) {
	store := &pipelineHealthTestStore{}
	h := &Handler{Store: store}
	app := fiber.New()
	app.Get("/admin/pipeline/health", h.GetPipelineHealth)

	req := httptest.NewRequest(http.MethodGet, "/admin/pipeline/health?recent_hours=48&stale_minutes=30&limit=5", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got pipelineHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if store.opts.Limit != 5 {
		t.Fatalf("limit = %d, want 5", store.opts.Limit)
	}
	if age := time.Since(store.opts.RecentSince); age < 47*time.Hour || age > 49*time.Hour {
		t.Fatalf("recent_since age = %s, want about 48h", age)
	}
	if got.Summary.StaleUploads != 1 || got.Summary.SecurityBlocked != 1 {
		t.Fatalf("summary = %#v", got.Summary)
	}
	if got.Severity != "critical" || len(got.Alerts) != 1 || got.Alerts[0].Code != "stale_uploads" {
		t.Fatalf("severity/alerts = %q %#v", got.Severity, got.Alerts)
	}
	if len(got.StaleUploads) != 1 || got.StaleUploads[0].Stage != "embed_index" {
		t.Fatalf("stale_uploads = %#v", got.StaleUploads)
	}
	if len(got.RecentIssues) != 1 || got.RecentIssues[0].Stage != "security" {
		t.Fatalf("recent_issues = %#v", got.RecentIssues)
	}
}

func TestGetPipelineHealthRejectsInvalidWindowParam(t *testing.T) {
	h := &Handler{Store: &pipelineHealthTestStore{}}
	app := fiber.New()
	app.Get("/admin/pipeline/health", h.GetPipelineHealth)

	req := httptest.NewRequest(http.MethodGet, "/admin/pipeline/health?recent_hours=nope", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestMetricsIncludesPipelineHealthGauges(t *testing.T) {
	h := &Handler{Store: &pipelineHealthTestStore{}}
	app := fiber.New()
	app.Get("/metrics", h.Metrics)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	body := string(bodyBytes)
	for _, want := range []string{
		"pipeline_metrics_up 1",
		`pipeline_health_status{severity="critical"} 1`,
		"pipeline_stale_uploads 1",
		`pipeline_health_alert{code="stale_uploads",severity="critical"} 1`,
		"vector_search_debug_total",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q:\n%s", want, body)
		}
	}
}
