package admin

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/admin/service"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
	"github.com/khiemnd777/legal_api/observability"
)

const (
	defaultAdminTimeoutMs = 20000
	maxAdminTimeoutMs     = 120000
	defaultPreviewLength  = 320
	maxSearchQueryChars   = 2000

	defaultQuickMaxVectorsScanned = 1000
	defaultQuickMaxScanDurationMs = 8000
	defaultFullMaxVectorsScanned  = 50000
	defaultFullMaxScanDurationMs  = 30000
	absoluteMaxScanDurationMs     = 120000
	defaultScanBatchSize          = 256
	maxScanBatchSize              = 2000
)

type QdrantControlPlaneHandler struct {
	Service qdrantControlPlaneAPI
	Logger  *slog.Logger
}

type qdrantControlPlaneAPI interface {
	ListCollections(ctx context.Context) ([]infra.CollectionDetails, error)
	ExpectedDimension(ctx context.Context) int
	GetCollection(ctx context.Context, name string) (infra.CollectionDetails, bool, error)
	SearchDebug(ctx context.Context, in service.SearchDebugInput) (service.SearchDebugResult, error)
	VectorHealth(ctx context.Context, mode infra.VectorScanMode, chunkBatchSize, vectorBatchSize, maxChunks, maxVectors int, maxDuration time.Duration) (infra.VectorConsistencyReport, error)
	DeleteByFilter(ctx context.Context, in service.DeleteByFilterInput) (service.DeleteByFilterResult, error)
	EnqueueReindexDocument(ctx context.Context, in service.ReindexDocumentInput) ([]domain.IngestJob, []bool, error)
	EnqueueReindexAll(ctx context.Context, in service.ReindexAllInput) ([]domain.IngestJob, []bool, error)
}

func NewQdrantControlPlaneHandler(svc qdrantControlPlaneAPI, logger *slog.Logger) *QdrantControlPlaneHandler {
	return &QdrantControlPlaneHandler{
		Service: svc,
		Logger:  logger,
	}
}

func (h *QdrantControlPlaneHandler) ListCollections(c *fiber.Ctx) error {
	started := time.Now()
	ctx, cancel := h.withTimeout(c)
	defer cancel()

	items, err := h.Service.ListCollections(ctx)
	if err != nil {
		h.logOperation(c, "list_collections", slog.String("result_status", "failed"), slog.String("error", err.Error()))
		return respondError(c, fiber.StatusBadGateway, "qdrant_error", "failed to inspect qdrant collections", nil)
	}
	expected := h.Service.ExpectedDimension(ctx)
	out := make([]QdrantCollectionSummary, 0, len(items))
	for _, item := range items {
		out = append(out, toCollectionSummary(item, expected))
	}
	resp := ListQdrantCollectionsResponse{
		Status:      "ok",
		Summary:     fmt.Sprintf("loaded %d collections", len(out)),
		Collections: out,
	}
	h.logOperation(c, "list_collections",
		slog.String("result_status", "ok"),
		slog.Int("collection_count", len(out)),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	return c.JSON(resp)
}

func (h *QdrantControlPlaneHandler) GetCollection(c *fiber.Ctx) error {
	started := time.Now()
	ctx, cancel := h.withTimeout(c)
	defer cancel()

	name := c.Params("name")
	item, found, err := h.Service.GetCollection(ctx, name)
	if err != nil {
		if err == service.ErrInvalidCollectionName {
			return respondError(c, fiber.StatusBadRequest, "validation", "invalid collection name", nil)
		}
		h.logOperation(c, "get_collection", slog.String("result_status", "failed"), slog.String("collection", name), slog.String("error", err.Error()))
		return respondError(c, fiber.StatusBadGateway, "qdrant_error", "failed to inspect qdrant collection", nil)
	}
	if !found {
		resp := GetQdrantCollectionResponse{
			Status:  "not_found",
			Summary: "collection does not exist",
			Found:   false,
		}
		h.logOperation(c, "get_collection",
			slog.String("result_status", "not_found"),
			slog.String("collection", name),
			slog.Int64("duration_ms", time.Since(started).Milliseconds()),
		)
		return c.JSON(resp)
	}
	expected := h.Service.ExpectedDimension(ctx)
	resp := GetQdrantCollectionResponse{
		Status:     "ok",
		Summary:    "collection inspection completed",
		Found:      true,
		Collection: ptrCollectionSummary(toCollectionSummary(item, expected)),
	}
	h.logOperation(c, "get_collection",
		slog.String("result_status", "ok"),
		slog.String("collection", item.Name),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	return c.JSON(resp)
}

func (h *QdrantControlPlaneHandler) SearchDebug(c *fiber.Ctx) error {
	started := time.Now()
	observability.Metrics.IncSearchDebugTotal()
	defer func() {
		observability.Metrics.ObserveSearchDebugDuration(time.Since(started).Seconds())
	}()

	var req SearchDebugRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json", err.Error())
	}
	if l := utf8.RuneCountInString(strings.TrimSpace(req.QueryText)); l == 0 || l > maxSearchQueryChars {
		return respondError(c, fiber.StatusBadRequest, "validation", fmt.Sprintf("query_text is required and must be <= %d chars", maxSearchQueryChars), nil)
	}
	ctx, cancel := h.withTimeout(c)
	defer cancel()

	in := service.SearchDebugInput{
		QueryText:           strings.TrimSpace(req.QueryText),
		TopK:                req.TopK,
		Collection:          req.Collection,
		IncludePayload:      req.IncludePayload,
		IncludeChunkPreview: req.IncludeChunkPreview,
	}
	if req.MetadataFilters != nil {
		in.Filters = service.SearchDebugFilter{
			LegalDomain:     req.MetadataFilters.LegalDomain,
			DocumentType:    req.MetadataFilters.DocumentType,
			EffectiveStatus: req.MetadataFilters.EffectiveStatus,
			DocumentNumber:  req.MetadataFilters.DocumentNumber,
			ArticleNumber:   req.MetadataFilters.ArticleNumber,
		}
	}
	out, err := h.Service.SearchDebug(ctx, in)
	if err != nil {
		switch err {
		case service.ErrMissingQueryText:
			return respondError(c, fiber.StatusBadRequest, "validation", "query_text is required", nil)
		case service.ErrInvalidQueryLength:
			return respondError(c, fiber.StatusBadRequest, "validation", fmt.Sprintf("query_text must be <= %d chars", service.MaxQueryTextLength), nil)
		case service.ErrInvalidTopK:
			return respondError(c, fiber.StatusBadRequest, "validation", "top_k must be between 1 and 50", nil)
		case service.ErrInvalidSearchFilters:
			return respondError(c, fiber.StatusBadRequest, "validation", "invalid metadata_filters payload", nil)
		case service.ErrInvalidCollectionName:
			return respondError(c, fiber.StatusBadRequest, "validation", "invalid collection name", nil)
		default:
			h.logOperation(c, "search_debug",
				slog.String("result_status", "failed"),
				slog.String("collection", strings.TrimSpace(req.Collection)),
				slog.Int("top_k", req.TopK),
				slog.String("error", err.Error()),
			)
			return respondError(c, fiber.StatusBadGateway, "qdrant_error", "search_debug failed", nil)
		}
	}
	hits := make([]SearchDebugHit, 0, len(out.Hits))
	for _, hit := range out.Hits {
		respHit := SearchDebugHit{
			Rank:    hit.Rank,
			PointID: hit.PointID,
			Score:   hit.Score,
			Payload: hit.Payload,
		}
		if hit.Chunk != nil {
			respHit.Chunk = &SearchDebugChunk{
				ChunkID:           hit.Chunk.ID,
				DocumentVersionID: hit.Chunk.DocumentVersionID,
				ChunkIndex:        hit.Chunk.Index,
				Preview:           service.TruncateText(hit.Chunk.Text, defaultPreviewLength),
				Citation:          fmt.Sprintf("chunk:%s", hit.Chunk.ID),
			}
		}
		hits = append(hits, respHit)
	}
	resp := SearchDebugResponse{
		Status:        "ok",
		Summary:       fmt.Sprintf("retrieved %d hits", len(hits)),
		QueryHash:     out.QueryHash,
		TopK:          out.TopK,
		FilterSummary: out.FilterSummary,
		Collection:    out.Collection,
		DurationMS:    time.Since(started).Milliseconds(),
		HitCount:      len(hits),
		Hits:          hits,
	}
	h.logOperation(c, "search_debug",
		slog.String("result_status", "ok"),
		slog.String("collection", out.Collection),
		slog.String("query_hash", out.QueryHash),
		slog.Int("top_k", out.TopK),
		slog.String("filter_summary", out.FilterSummary),
		slog.Int("hit_count", len(hits)),
		slog.Int("affected_count", len(hits)),
		slog.Int64("duration_ms", resp.DurationMS),
	)
	h.logSummary(c, "search_debug_summary",
		slog.String("collection", out.Collection),
		slog.Int("hits", len(hits)),
		slog.Int64("duration_ms", resp.DurationMS),
	)
	return c.JSON(resp)
}

func (h *QdrantControlPlaneHandler) VectorHealth(c *fiber.Ctx) error {
	started := time.Now()
	defer func() {
		observability.Metrics.ObserveHealthScanDuration(time.Since(started).Seconds())
	}()

	ctx, cancel := h.withTimeout(c)
	defer cancel()

	mode := infra.VectorScanQuick
	if strings.EqualFold(strings.TrimSpace(c.Query("mode")), "full") || strings.EqualFold(strings.TrimSpace(c.Query("full_scan")), "true") {
		mode = infra.VectorScanFull
	}
	batchSize, _ := strconv.Atoi(c.Query("batch_size", strconv.Itoa(defaultScanBatchSize)))
	if batchSize <= 0 {
		batchSize = defaultScanBatchSize
	}
	if batchSize > maxScanBatchSize {
		batchSize = maxScanBatchSize
	}
	chunkBatchSize, _ := strconv.Atoi(c.Query("chunk_batch_size", strconv.Itoa(batchSize)))
	vectorBatchSize, _ := strconv.Atoi(c.Query("vector_batch_size", strconv.Itoa(batchSize)))
	if chunkBatchSize <= 0 {
		chunkBatchSize = batchSize
	}
	if vectorBatchSize <= 0 {
		vectorBatchSize = batchSize
	}
	if chunkBatchSize > maxScanBatchSize {
		chunkBatchSize = maxScanBatchSize
	}
	if vectorBatchSize > maxScanBatchSize {
		vectorBatchSize = maxScanBatchSize
	}

	maxVectors, _ := strconv.Atoi(c.Query("max_vectors_scanned", c.Query("max_vectors", "0")))
	if maxVectors <= 0 {
		if mode == infra.VectorScanQuick {
			maxVectors = defaultQuickMaxVectorsScanned
		} else {
			maxVectors = defaultFullMaxVectorsScanned
		}
	}
	maxChunks, _ := strconv.Atoi(c.Query("max_chunks", "0"))
	if maxChunks <= 0 {
		maxChunks = maxVectors
	}

	maxScanDurationMs, _ := strconv.Atoi(c.Query("max_scan_duration_ms", "0"))
	if maxScanDurationMs <= 0 {
		if mode == infra.VectorScanQuick {
			maxScanDurationMs = defaultQuickMaxScanDurationMs
		} else {
			maxScanDurationMs = defaultFullMaxScanDurationMs
		}
	}
	if maxScanDurationMs > absoluteMaxScanDurationMs {
		maxScanDurationMs = absoluteMaxScanDurationMs
	}

	report, err := h.Service.VectorHealth(ctx, mode, chunkBatchSize, vectorBatchSize, maxChunks, maxVectors, time.Duration(maxScanDurationMs)*time.Millisecond)
	if err != nil {
		observability.Metrics.IncConsistencyErrorTotal()
		h.logOperation(c, "vector_health",
			slog.String("result_status", "failed"),
			slog.String("scan_mode", string(mode)),
			slog.Int("max_vectors_scanned", maxVectors),
			slog.Int("max_scan_duration_ms", maxScanDurationMs),
			slog.String("error", err.Error()),
		)
		return respondError(c, fiber.StatusBadGateway, "qdrant_error", "vector health scan failed", nil)
	}
	scannedBatches := 0
	if chunkBatchSize > 0 {
		scannedBatches += (report.ScannedChunkCount + chunkBatchSize - 1) / chunkBatchSize
	}
	if vectorBatchSize > 0 {
		scannedBatches += (report.ScannedVectorCount + vectorBatchSize - 1) / vectorBatchSize
	}
	repairable := report.MissingVectorCount > 0 || report.OrphanVectorCount > 0
	recommendation := "no repair needed"
	if repairable {
		recommendation = "repair tasks recommended for missing/orphan vectors"
	}
	samples := []string{}
	samples = append(samples, report.MissingVectorChunkIDs...)
	if len(samples) < 10 {
		remaining := 10 - len(samples)
		for i, id := range report.OrphanVectorPointIDs {
			if i >= remaining {
				break
			}
			samples = append(samples, id)
		}
	}
	resp := VectorHealthResponse{
		Status:                    "ok",
		Summary:                   "vector health diagnostics completed",
		ScanMode:                  report.Mode,
		ScannedBatches:            scannedBatches,
		ScannedVectors:            report.ScannedVectorCount,
		ScannedChunks:             report.ScannedChunkCount,
		DurationMS:                time.Since(started).Milliseconds(),
		Bounded:                   report.Bounded,
		OrphanVectorsCount:        report.OrphanVectorCount,
		MissingVectorsCount:       report.MissingVectorCount,
		ChunkVectorCountMismatch:  report.ChunkVectorCountMismatch,
		DimensionMismatchDetected: report.DimensionMismatch || report.EmbeddingDimensionMismatchCnt > 0,
		RepairableIssuesDetected:  repairable,
		RepairRecommendation:      recommendation,
		Samples:                   samples,
	}
	observability.Metrics.SetOrphanCount(report.OrphanVectorCount)
	observability.Metrics.SetMissingCount(report.MissingVectorCount)
	h.logOperation(c, "vector_health",
		slog.String("result_status", "ok"),
		slog.String("scan_mode", report.Mode),
		slog.Int("batch_size", batchSize),
		slog.Int("max_vectors_scanned", maxVectors),
		slog.Int("max_scan_duration_ms", maxScanDurationMs),
		slog.Int("scanned_batches", scannedBatches),
		slog.Int("scanned_vectors", report.ScannedVectorCount),
		slog.Int("scanned_chunks", report.ScannedChunkCount),
		slog.Int("missing_vectors", report.MissingVectorCount),
		slog.Int("orphan_vectors", report.OrphanVectorCount),
		slog.Bool("repairable_issues", repairable),
		slog.Int("affected_count", report.MissingVectorCount+report.OrphanVectorCount),
		slog.Int64("duration_ms", resp.DurationMS),
	)
	h.logSummary(c, "vector_health_summary",
		slog.String("scan_mode", report.Mode),
		slog.Int("scanned_batches", scannedBatches),
		slog.Int("scanned_vectors", report.ScannedVectorCount),
		slog.Int("missing_vectors", report.MissingVectorCount),
		slog.Int("orphan_vectors", report.OrphanVectorCount),
		slog.Bool("repairable_issues", repairable),
		slog.Int64("duration_ms", resp.DurationMS),
	)
	return c.JSON(resp)
}

func (h *QdrantControlPlaneHandler) DeleteByFilter(c *fiber.Ctx) error {
	started := time.Now()
	observability.Metrics.IncDeleteByFilterTotal()
	var req DeleteByFilterRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json", err.Error())
	}
	ctx, cancel := h.withTimeout(c)
	defer cancel()

	out, err := h.Service.DeleteByFilter(ctx, service.DeleteByFilterInput{
		Collection: req.Collection,
		Filter:     req.Filter,
		Confirm:    req.Confirm,
		DryRun:     req.DryRun,
	})
	if err != nil {
		switch err {
		case service.ErrInvalidDeleteRequest:
			return respondError(c, fiber.StatusBadRequest, "validation", "set either dry_run=true or confirm=true", nil)
		case service.ErrInvalidCollectionName:
			return respondError(c, fiber.StatusBadRequest, "validation", "invalid collection name", nil)
		default:
			if strings.Contains(err.Error(), "rejected") || strings.Contains(err.Error(), "requires") || strings.Contains(err.Error(), "ambiguous") {
				return respondError(c, fiber.StatusBadRequest, "validation", err.Error(), nil)
			}
			h.logOperation(c, "delete_by_filter",
				slog.String("result_status", "failed"),
				slog.String("collection", strings.TrimSpace(req.Collection)),
				slog.String("filter_summary", infra.SummarizeFilter(req.Filter)),
				slog.String("reason", strings.TrimSpace(req.Reason)),
				slog.String("error", err.Error()),
			)
			return respondError(c, fiber.StatusBadGateway, "qdrant_error", "delete_by_filter failed", nil)
		}
	}
	summary := "delete executed"
	if req.DryRun {
		summary = "dry run completed"
	}
	resp := DeleteByFilterResponse{
		Status:         "ok",
		Summary:        summary,
		Collection:     out.Collection,
		DryRun:         req.DryRun,
		Confirmed:      req.Confirm,
		FilterSummary:  out.FilterSummary,
		EstimatedScope: out.EstimatedScope,
		ScopeEstimated: out.ScopeEstimated,
	}
	h.logOperation(c, "delete_by_filter",
		slog.String("result_status", "ok"),
		slog.String("collection", out.Collection),
		slog.String("filter_summary", out.FilterSummary),
		slog.String("reason", strings.TrimSpace(req.Reason)),
		slog.Bool("dry_run", req.DryRun),
		slog.Bool("confirm", req.Confirm),
		slog.Int64("affected_count", valueOrZero(out.EstimatedScope)),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	h.logSummary(c, "delete_by_filter_summary",
		slog.String("collection", out.Collection),
		slog.String("filter_summary", out.FilterSummary),
		slog.Int64("estimated_scope", valueOrZero(out.EstimatedScope)),
		slog.Bool("dry_run", req.DryRun),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	return c.JSON(resp)
}

func (h *QdrantControlPlaneHandler) ReindexDocument(c *fiber.Ctx) error {
	started := time.Now()
	observability.Metrics.IncReindexDocumentTotal()
	var req ReindexDocumentRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json", err.Error())
	}
	ctx, cancel := h.withTimeout(c)
	defer cancel()

	jobs, createdFlags, err := h.Service.EnqueueReindexDocument(ctx, service.ReindexDocumentInput{
		DocumentID:        req.DocumentID,
		DocumentVersionID: req.DocumentVersionID,
	})
	if err != nil {
		if err == service.ErrInvalidReindexScope {
			return respondError(c, fiber.StatusBadRequest, "validation", "provide either document_id or document_version_id", nil)
		}
		h.logOperation(c, "reindex_document",
			slog.String("result_status", "failed"),
			slog.String("document_id", strings.TrimSpace(req.DocumentID)),
			slog.String("document_version_id", strings.TrimSpace(req.DocumentVersionID)),
			slog.String("reason", strings.TrimSpace(req.Reason)),
			slog.String("error", err.Error()),
		)
		return respondError(c, fiber.StatusBadGateway, "db_error", "failed to enqueue reindex document", nil)
	}
	createdCount := 0
	items := make([]ReindexEnqueueItem, 0, len(jobs))
	for i := range jobs {
		if createdFlags[i] {
			createdCount++
		}
		items = append(items, ReindexEnqueueItem{
			DocumentVersionID: jobs[i].DocumentVersionID,
			JobID:             jobs[i].ID,
			JobStatus:         jobs[i].Status,
			Created:           createdFlags[i],
		})
	}
	resp := ReindexAcceptedResponse{
		Status:        "accepted",
		Summary:       fmt.Sprintf("accepted %d reindex targets", len(items)),
		AcceptedCount: len(items),
		CreatedCount:  createdCount,
		SkippedCount:  len(items) - createdCount,
		Items:         items,
	}
	jobSample := ""
	if len(items) > 0 {
		jobSample = items[0].JobID
	}
	h.logOperation(c, "reindex_document",
		slog.String("result_status", "accepted"),
		slog.String("document_id", strings.TrimSpace(req.DocumentID)),
		slog.String("document_version_id", strings.TrimSpace(req.DocumentVersionID)),
		slog.Bool("force", req.Force),
		slog.String("reason", strings.TrimSpace(req.Reason)),
		slog.Int("accepted_count", resp.AcceptedCount),
		slog.Int("created_count", resp.CreatedCount),
		slog.Int("affected_count", resp.AcceptedCount),
		slog.String("job_id_sample", jobSample),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	h.logSummary(c, "reindex_document_summary",
		slog.Int("accepted_count", resp.AcceptedCount),
		slog.Int("created_count", resp.CreatedCount),
		slog.Int("skipped_count", resp.SkippedCount),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	return c.Status(fiber.StatusAccepted).JSON(resp)
}

func (h *QdrantControlPlaneHandler) ReindexAll(c *fiber.Ctx) error {
	started := time.Now()
	observability.Metrics.IncReindexAllTotal()
	var req ReindexAllRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json", err.Error())
	}
	if strings.TrimSpace(req.Reason) == "" {
		return respondError(c, fiber.StatusBadRequest, "validation", "reason is required for reindex_all", nil)
	}
	ctx, cancel := h.withTimeout(c)
	defer cancel()

	jobs, createdFlags, err := h.Service.EnqueueReindexAll(ctx, service.ReindexAllInput{
		Confirm:     req.Confirm,
		DocTypeCode: req.DocTypeCode,
		Collection:  req.Collection,
		Status:      req.Status,
		Limit:       req.Limit,
		Reason:      req.Reason,
	})
	if err != nil {
		if err == service.ErrInvalidReindexScope {
			return respondError(c, fiber.StatusBadRequest, "validation", "invalid reindex_all scope or missing confirm=true", nil)
		}
		h.logOperation(c, "reindex_all",
			slog.String("result_status", "failed"),
			slog.String("collection", strings.TrimSpace(req.Collection)),
			slog.String("doc_type_code", strings.TrimSpace(req.DocTypeCode)),
			slog.String("reason", strings.TrimSpace(req.Reason)),
			slog.String("error", err.Error()),
		)
		return respondError(c, fiber.StatusBadGateway, "db_error", "failed to enqueue reindex_all", nil)
	}
	createdCount := 0
	for _, created := range createdFlags {
		if created {
			createdCount++
		}
	}
	resp := ReindexAcceptedResponse{
		Status:        "accepted",
		Summary:       fmt.Sprintf("accepted %d reindex targets", len(jobs)),
		Scope:         map[string]string{"doc_type_code": strings.TrimSpace(req.DocTypeCode), "collection": strings.TrimSpace(req.Collection), "status": strings.TrimSpace(req.Status)},
		AcceptedCount: len(jobs),
		CreatedCount:  createdCount,
		SkippedCount:  len(jobs) - createdCount,
	}
	jobSample := ""
	if len(jobs) > 0 {
		jobSample = jobs[0].ID
	}
	h.logOperation(c, "reindex_all",
		slog.String("result_status", "accepted"),
		slog.String("collection", strings.TrimSpace(req.Collection)),
		slog.String("doc_type_code", strings.TrimSpace(req.DocTypeCode)),
		slog.String("status_filter", strings.TrimSpace(req.Status)),
		slog.Bool("confirm", req.Confirm),
		slog.Bool("force", req.Force),
		slog.String("reason", strings.TrimSpace(req.Reason)),
		slog.Int("accepted_count", resp.AcceptedCount),
		slog.Int("created_count", resp.CreatedCount),
		slog.Int("affected_count", resp.AcceptedCount),
		slog.String("job_id_sample", jobSample),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	h.logSummary(c, "reindex_all_summary",
		slog.Int("accepted_count", resp.AcceptedCount),
		slog.Int("created_count", resp.CreatedCount),
		slog.Int("skipped_count", resp.SkippedCount),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	return c.Status(fiber.StatusAccepted).JSON(resp)
}

func (h *QdrantControlPlaneHandler) withTimeout(c *fiber.Ctx) (context.Context, context.CancelFunc) {
	timeoutMs, _ := strconv.Atoi(c.Query("timeout_ms", strconv.Itoa(defaultAdminTimeoutMs)))
	if timeoutMs <= 0 {
		timeoutMs = defaultAdminTimeoutMs
	}
	if timeoutMs > maxAdminTimeoutMs {
		timeoutMs = maxAdminTimeoutMs
	}
	return context.WithTimeout(c.UserContext(), time.Duration(timeoutMs)*time.Millisecond)
}

func (h *QdrantControlPlaneHandler) logOperation(c *fiber.Ctx, op string, attrs ...slog.Attr) {
	logger := h.Logger
	if logger == nil {
		logger = slog.Default()
	}
	base := []slog.Attr{
		slog.String("component", "admin_qdrant_control_plane"),
		slog.String("operation", op),
		slog.String("route", c.Path()),
		slog.String("admin_identity", adminActor(c)),
	}
	base = append(base, attrs...)
	logger.LogAttrs(c.UserContext(), slog.LevelInfo, "admin_qdrant_operation", base...)
}

func (h *QdrantControlPlaneHandler) logSummary(c *fiber.Ctx, message string, attrs ...slog.Attr) {
	logger := h.Logger
	if logger == nil {
		logger = slog.Default()
	}
	base := []slog.Attr{
		slog.String("component", "admin_qdrant_control_plane"),
		slog.String("admin_identity", adminActor(c)),
	}
	base = append(base, attrs...)
	logger.LogAttrs(c.UserContext(), slog.LevelInfo, message, base...)
}

func adminActor(c *fiber.Ctx) string {
	if actor := strings.TrimSpace(c.Get("X-Admin-Actor")); actor != "" {
		return actor
	}
	return "admin_api_key"
}

func toCollectionSummary(item infra.CollectionDetails, expectedDim int) QdrantCollectionSummary {
	validation := QdrantValidationSummary{
		Available: expectedDim > 0,
	}
	if expectedDim > 0 {
		validation.ExpectedDimension = expectedDim
		validation.Passed = item.VectorSize == expectedDim
		if validation.Passed {
			validation.Message = "dimension validated"
		} else {
			validation.Message = fmt.Sprintf("expected=%d actual=%d", expectedDim, item.VectorSize)
		}
	}
	fields := make([]QdrantPayloadSchemaField, 0)
	for _, entry := range service.BuildPayloadSummary(item.PayloadSchema) {
		fields = append(fields, QdrantPayloadSchemaField{
			Key:  entry["key"],
			Type: entry["type"],
		})
	}
	return QdrantCollectionSummary{
		CollectionName:       item.Name,
		Status:               item.Status,
		PointsCount:          item.PointsCount,
		VectorCount:          item.VectorsCount,
		IndexedVectorsCount:  item.IndexedVectorsCount,
		VectorDimension:      item.VectorSize,
		DistanceMetric:       item.Distance,
		Validation:           validation,
		PayloadSchemaSummary: fields,
	}
}

func ptrCollectionSummary(item QdrantCollectionSummary) *QdrantCollectionSummary {
	return &item
}

func valueOrZero(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
