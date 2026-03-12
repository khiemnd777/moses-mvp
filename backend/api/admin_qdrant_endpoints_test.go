package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	adminapi "github.com/khiemnd777/legal_api/admin"
	"github.com/khiemnd777/legal_api/admin/service"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
)

type stubQdrantService struct {
	mu sync.Mutex

	listCollectionsFn      func(context.Context) ([]infra.CollectionDetails, error)
	expectedDimensionFn    func(context.Context) int
	getCollectionFn        func(context.Context, string) (infra.CollectionDetails, bool, error)
	searchDebugFn          func(context.Context, service.SearchDebugInput) (service.SearchDebugResult, error)
	vectorHealthFn         func(context.Context, infra.VectorScanMode, int, int, int, int, time.Duration) (infra.VectorConsistencyReport, error)
	deleteByFilterFn       func(context.Context, service.DeleteByFilterInput) (service.DeleteByFilterResult, error)
	enqueueReindexDocFn    func(context.Context, service.ReindexDocumentInput) ([]domain.IngestJob, []bool, error)
	enqueueReindexAllFn    func(context.Context, service.ReindexAllInput) ([]domain.IngestJob, []bool, error)
	lastVectorHealthInputs struct {
		mode            infra.VectorScanMode
		chunkBatchSize  int
		vectorBatchSize int
		maxChunks       int
		maxVectors      int
		maxDuration     time.Duration
	}
}

func (s *stubQdrantService) ListCollections(ctx context.Context) ([]infra.CollectionDetails, error) {
	if s.listCollectionsFn != nil {
		return s.listCollectionsFn(ctx)
	}
	count := int64(12)
	return []infra.CollectionDetails{{
		Name:        "legal_chunks",
		Status:      "green",
		VectorSize:  1536,
		Distance:    "Cosine",
		PointsCount: &count,
	}}, nil
}

func (s *stubQdrantService) ExpectedDimension(ctx context.Context) int {
	if s.expectedDimensionFn != nil {
		return s.expectedDimensionFn(ctx)
	}
	return 1536
}

func (s *stubQdrantService) GetCollection(ctx context.Context, name string) (infra.CollectionDetails, bool, error) {
	if s.getCollectionFn != nil {
		return s.getCollectionFn(ctx, name)
	}
	count := int64(12)
	return infra.CollectionDetails{Name: name, Status: "green", VectorSize: 1536, Distance: "Cosine", PointsCount: &count}, true, nil
}

func (s *stubQdrantService) SearchDebug(ctx context.Context, in service.SearchDebugInput) (service.SearchDebugResult, error) {
	if s.searchDebugFn != nil {
		return s.searchDebugFn(ctx, in)
	}
	return service.SearchDebugResult{
		QueryHash:     "abc123",
		Collection:    "legal_chunks",
		TopK:          3,
		FilterSummary: "none",
		Hits:          []service.SearchDebugHit{{Rank: 1, PointID: "v1_0", Score: 0.91}},
	}, nil
}

func (s *stubQdrantService) VectorHealth(ctx context.Context, mode infra.VectorScanMode, chunkBatchSize, vectorBatchSize, maxChunks, maxVectors int, maxDuration time.Duration) (infra.VectorConsistencyReport, error) {
	s.mu.Lock()
	s.lastVectorHealthInputs.mode = mode
	s.lastVectorHealthInputs.chunkBatchSize = chunkBatchSize
	s.lastVectorHealthInputs.vectorBatchSize = vectorBatchSize
	s.lastVectorHealthInputs.maxChunks = maxChunks
	s.lastVectorHealthInputs.maxVectors = maxVectors
	s.lastVectorHealthInputs.maxDuration = maxDuration
	s.mu.Unlock()
	if s.vectorHealthFn != nil {
		return s.vectorHealthFn(ctx, mode, chunkBatchSize, vectorBatchSize, maxChunks, maxVectors, maxDuration)
	}
	return infra.VectorConsistencyReport{Mode: string(mode), ScannedChunkCount: 10, ScannedVectorCount: 10}, nil
}

func (s *stubQdrantService) DeleteByFilter(ctx context.Context, in service.DeleteByFilterInput) (service.DeleteByFilterResult, error) {
	if s.deleteByFilterFn != nil {
		return s.deleteByFilterFn(ctx, in)
	}
	scope := int64(4)
	return service.DeleteByFilterResult{Collection: "legal_chunks", FilterSummary: "document_id=1", EstimatedScope: &scope, ScopeEstimated: true}, nil
}

func (s *stubQdrantService) EnqueueReindexDocument(ctx context.Context, in service.ReindexDocumentInput) ([]domain.IngestJob, []bool, error) {
	if s.enqueueReindexDocFn != nil {
		return s.enqueueReindexDocFn(ctx, in)
	}
	return []domain.IngestJob{{ID: "job-1", DocumentVersionID: "ver-1", Status: "queued"}}, []bool{true}, nil
}

func (s *stubQdrantService) EnqueueReindexAll(ctx context.Context, in service.ReindexAllInput) ([]domain.IngestJob, []bool, error) {
	if s.enqueueReindexAllFn != nil {
		return s.enqueueReindexAllFn(ctx, in)
	}
	return []domain.IngestJob{{ID: "job-1", DocumentVersionID: "ver-1", Status: "queued"}}, []bool{true}, nil
}

func setupQdrantAdminApp(t *testing.T, adminKey string, svc *stubQdrantService) *fiber.App {
	t.Helper()
	app := fiber.New()
	group := app.Group("/admin", adminAuthMiddleware(adminKey))
	h := adminapi.NewQdrantControlPlaneHandler(svc, slog.Default())
	adminapi.RegisterRoutes(group, nil, nil, nil, nil, h)
	return app
}

func doJSON(t *testing.T, app *fiber.App, method, path, key, actor string, payload interface{}) (*http.Response, map[string]interface{}) {
	t.Helper()
	var body []byte
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = encoded
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("X-Admin-Key", key)
	}
	if actor != "" {
		req.Header.Set("X-Admin-Actor", actor)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func assertErrorEnvelope(t *testing.T, out map[string]interface{}, code string) {
	t.Helper()
	errObj, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object, got: %#v", out)
	}
	if got := errObj["code"]; got != code {
		t.Fatalf("expected error code %q, got %#v", code, got)
	}
	if _, ok := errObj["message"].(string); !ok {
		t.Fatalf("expected error.message string, got: %#v", errObj)
	}
}

func TestQdrantEndpoints_AdminAuthAndSuccess(t *testing.T) {
	svc := &stubQdrantService{}
	app := setupQdrantAdminApp(t, "secret", svc)

	tests := []struct {
		name          string
		method        string
		path          string
		payload       interface{}
		expectStatus  int
		expectRespKey string
	}{
		{name: "list collections", method: http.MethodGet, path: "/admin/qdrant/collections", expectStatus: http.StatusOK, expectRespKey: "collections"},
		{name: "get collection", method: http.MethodGet, path: "/admin/qdrant/collections/legal_chunks", expectStatus: http.StatusOK, expectRespKey: "collection"},
		{name: "search debug", method: http.MethodPost, path: "/admin/qdrant/search_debug", payload: map[string]interface{}{"query_text": "luat", "top_k": 3}, expectStatus: http.StatusOK, expectRespKey: "hits"},
		{name: "vector health", method: http.MethodGet, path: "/admin/qdrant/vector_health", expectStatus: http.StatusOK, expectRespKey: "scan_mode"},
		{name: "delete by filter", method: http.MethodPost, path: "/admin/qdrant/delete_by_filter", payload: map[string]interface{}{"collection": "legal_chunks", "dry_run": true, "filter": map[string]interface{}{"must": []map[string]interface{}{{"key": "document_id", "match": map[string]interface{}{"value": "doc-1"}}}}}, expectStatus: http.StatusOK, expectRespKey: "filter_summary"},
		{name: "reindex document", method: http.MethodPost, path: "/admin/qdrant/reindex_document", payload: map[string]interface{}{"document_version_id": "ver-1"}, expectStatus: http.StatusAccepted, expectRespKey: "items"},
		{name: "reindex all", method: http.MethodPost, path: "/admin/qdrant/reindex_all", payload: map[string]interface{}{"confirm": true, "reason": "ops-check"}, expectStatus: http.StatusAccepted, expectRespKey: "accepted_count"},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unauthResp, unauthBody := doJSON(t, app, tc.method, tc.path, "", "authz-actor", tc.payload)
			if unauthResp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", unauthResp.StatusCode)
			}
			assertErrorEnvelope(t, unauthBody, "unauthorized")

			resp, body := doJSON(t, app, tc.method, tc.path, "secret", "actor-success-"+time.Now().Format("150405")+"-"+string(rune('A'+i)), tc.payload)
			if resp.StatusCode != tc.expectStatus {
				t.Fatalf("expected %d, got %d body=%#v", tc.expectStatus, resp.StatusCode, body)
			}
			if _, ok := body["status"].(string); !ok {
				t.Fatalf("expected status field, got %#v", body)
			}
			if _, ok := body[tc.expectRespKey]; !ok {
				t.Fatalf("expected key %q in response: %#v", tc.expectRespKey, body)
			}
		})
	}
}

func TestQdrantEndpoints_ValidationAndMalformedPayload(t *testing.T) {
	svc := &stubQdrantService{
		searchDebugFn: func(ctx context.Context, in service.SearchDebugInput) (service.SearchDebugResult, error) {
			if in.TopK > 50 {
				return service.SearchDebugResult{}, service.ErrInvalidTopK
			}
			return service.SearchDebugResult{Collection: "legal_chunks", QueryHash: "x", TopK: 1, FilterSummary: "none"}, nil
		},
		enqueueReindexDocFn: func(ctx context.Context, in service.ReindexDocumentInput) ([]domain.IngestJob, []bool, error) {
			if in.DocumentID == "" && in.DocumentVersionID == "" {
				return nil, nil, service.ErrInvalidReindexScope
			}
			return []domain.IngestJob{{ID: "job", DocumentVersionID: "v", Status: "queued"}}, []bool{true}, nil
		},
		enqueueReindexAllFn: func(ctx context.Context, in service.ReindexAllInput) ([]domain.IngestJob, []bool, error) {
			if !in.Confirm {
				return nil, nil, service.ErrInvalidReindexScope
			}
			return []domain.IngestJob{{ID: "job", DocumentVersionID: "v", Status: "queued"}}, []bool{true}, nil
		},
	}
	app := setupQdrantAdminApp(t, "secret", svc)

	resp, body := doJSON(t, app, http.MethodPost, "/admin/qdrant/search_debug", "secret", "actor-a", map[string]interface{}{"query_text": "   "})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	assertErrorEnvelope(t, body, "validation")

	req := httptest.NewRequest(http.MethodPost, "/admin/qdrant/search_debug", bytes.NewBufferString("{"))
	req.Header.Set("X-Admin-Key", "secret")
	req.Header.Set("Content-Type", "application/json")
	malformedResp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test malformed: %v", err)
	}
	defer malformedResp.Body.Close()
	if malformedResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected malformed payload 400, got %d", malformedResp.StatusCode)
	}

	resp, body = doJSON(t, app, http.MethodPost, "/admin/qdrant/reindex_document", "secret", "actor-b", map[string]interface{}{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	assertErrorEnvelope(t, body, "validation")

	resp, body = doJSON(t, app, http.MethodPost, "/admin/qdrant/reindex_all", "secret", "actor-c", map[string]interface{}{"confirm": true})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	assertErrorEnvelope(t, body, "validation")
}

func TestQdrantEndpoints_DeleteByFilterDryRunConfirmAndRateLimit(t *testing.T) {
	svc := &stubQdrantService{}
	app := setupQdrantAdminApp(t, "secret", svc)

	dryRunPayload := map[string]interface{}{
		"collection": "legal_chunks",
		"dry_run":    true,
		"confirm":    false,
		"filter": map[string]interface{}{
			"must": []map[string]interface{}{{"key": "document_id", "match": map[string]interface{}{"value": "doc-1"}}},
		},
	}
	resp, body := doJSON(t, app, http.MethodPost, "/admin/qdrant/delete_by_filter", "secret", "actor-delete-dry", dryRunPayload)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 dry_run, got %d body=%#v", resp.StatusCode, body)
	}
	if body["dry_run"] != true {
		t.Fatalf("expected dry_run=true, got %#v", body)
	}
	if body["confirmed"] != false {
		t.Fatalf("expected confirmed=false, got %#v", body)
	}

	confirmPayload := map[string]interface{}{
		"collection": "legal_chunks",
		"dry_run":    false,
		"confirm":    true,
		"filter": map[string]interface{}{
			"must": []map[string]interface{}{{"key": "document_id", "match": map[string]interface{}{"value": "doc-2"}}},
		},
	}
	resp, body = doJSON(t, app, http.MethodPost, "/admin/qdrant/delete_by_filter", "secret", "actor-delete-confirm", confirmPayload)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 confirm, got %d body=%#v", resp.StatusCode, body)
	}
	if body["dry_run"] != false {
		t.Fatalf("expected dry_run=false, got %#v", body)
	}
	if body["confirmed"] != true {
		t.Fatalf("expected confirmed=true, got %#v", body)
	}

	resp, body = doJSON(t, app, http.MethodPost, "/admin/qdrant/delete_by_filter", "secret", "actor-rate", confirmPayload)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected first 200 for rate limit actor, got %d", resp.StatusCode)
	}
	resp, body = doJSON(t, app, http.MethodPost, "/admin/qdrant/delete_by_filter", "secret", "actor-rate", confirmPayload)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d body=%#v", resp.StatusCode, body)
	}
	assertErrorEnvelope(t, body, "rate_limited")
}

func TestQdrantEndpoints_VectorHealthBoundedModeParsing(t *testing.T) {
	svc := &stubQdrantService{}
	app := setupQdrantAdminApp(t, "secret", svc)

	resp, body := doJSON(t, app, http.MethodGet, "/admin/qdrant/vector_health?mode=full&batch_size=99999&max_vectors_scanned=111&max_chunks=222&max_scan_duration_ms=999999", "secret", "actor-health", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%#v", resp.StatusCode, body)
	}

	svc.mu.Lock()
	captured := svc.lastVectorHealthInputs
	svc.mu.Unlock()

	if captured.mode != infra.VectorScanFull {
		t.Fatalf("expected full mode, got %q", captured.mode)
	}
	if captured.chunkBatchSize != 2000 || captured.vectorBatchSize != 2000 {
		t.Fatalf("expected bounded batch size 2000, got chunk=%d vector=%d", captured.chunkBatchSize, captured.vectorBatchSize)
	}
	if captured.maxDuration != 120*time.Second {
		t.Fatalf("expected bounded duration 120s, got %s", captured.maxDuration)
	}
	if captured.maxVectors != 111 || captured.maxChunks != 222 {
		t.Fatalf("unexpected scan limits: vectors=%d chunks=%d", captured.maxVectors, captured.maxChunks)
	}
}
