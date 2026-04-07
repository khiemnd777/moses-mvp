package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/core/guard"
	"github.com/khiemnd777/legal_api/core/ingest"
	cprompt "github.com/khiemnd777/legal_api/core/prompt"
	"github.com/khiemnd777/legal_api/core/retrieval"
	"github.com/khiemnd777/legal_api/core/schema"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
	"github.com/khiemnd777/legal_api/observability"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	Store        handlerStore
	Storage      *infra.Storage
	Qdrant       *infra.QdrantClient
	Retriever    retriever
	AnswerCli    *answer.Client
	Tones        map[string]string
	IngestSvc    *ingest.Service
	Logger       *slog.Logger
	TraceRepo    observability.TraceRepository
	PromptRouter *cprompt.Router
	GuardEngine  *guard.Engine

	runtimeCfgMu       sync.RWMutex
	runtimeCfg         runtimeAnswerConfig
	runtimeCfgLoadedAt time.Time
	runtimeCfgReady    bool
	runtimeCfgTTL      time.Duration
}

func NewHandler(store handlerStore, storage *infra.Storage, qdrant *infra.QdrantClient, retriever retriever, ans *answer.Client, tones map[string]string, ingestSvc *ingest.Service, logger *slog.Logger, traceRepo observability.TraceRepository) *Handler {
	ttl := 30 * time.Second
	return &Handler{
		Store:         store,
		Storage:       storage,
		Qdrant:        qdrant,
		Retriever:     retriever,
		AnswerCli:     ans,
		Tones:         tones,
		IngestSvc:     ingestSvc,
		Logger:        logger,
		TraceRepo:     traceRepo,
		PromptRouter:  cprompt.NewRouter(store, ttl, cprompt.DefaultPromptType),
		GuardEngine:   guard.NewEngine(),
		runtimeCfgTTL: ttl,
	}
}

type errorEnvelope struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}

type docTypeResponse struct {
	ID        string             `json:"id"`
	Code      string             `json:"code"`
	Name      string             `json:"name"`
	Form      schema.DocTypeForm `json:"form"`
	FormHash  string             `json:"form_hash"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type documentResponse struct {
	ID          string                  `json:"id"`
	DocTypeID   string                  `json:"doc_type_id"`
	DocTypeCode string                  `json:"doc_type_code"`
	Title       string                  `json:"title"`
	Assets      []documentAssetResponse `json:"assets"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
}

type documentAssetResponse struct {
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
	Versions    []int     `json:"versions"`
}

type ingestJobResponse struct {
	ID                string    `json:"id"`
	DocumentVersionID string    `json:"document_version_id"`
	Status            string    `json:"status"`
	ErrorMessage      *string   `json:"error_message,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func toDocTypeResponse(d domain.DocType) (docTypeResponse, error) {
	var form schema.DocTypeForm
	if len(d.FormJSON) > 0 {
		if err := json.Unmarshal(d.FormJSON, &form); err != nil {
			return docTypeResponse{}, err
		}
	}
	return docTypeResponse{
		ID:        d.ID,
		Code:      d.Code,
		Name:      d.Name,
		Form:      form,
		FormHash:  d.FormHash,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}, nil
}

func toDocumentResponse(d domain.Document, assets []domain.DocumentAssetWithVersions) documentResponse {
	respAssets := make([]documentAssetResponse, 0, len(assets))
	for _, a := range assets {
		respAssets = append(respAssets, documentAssetResponse{
			FileName:    a.FileName,
			ContentType: a.ContentType,
			CreatedAt:   a.CreatedAt,
			Versions:    a.Versions,
		})
	}
	return documentResponse{
		ID:          d.ID,
		DocTypeID:   d.DocTypeID,
		DocTypeCode: d.DocTypeCode,
		Title:       d.Title,
		Assets:      respAssets,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

func toIngestJobResponse(j domain.IngestJob) ingestJobResponse {
	return ingestJobResponse{
		ID:                j.ID,
		DocumentVersionID: j.DocumentVersionID,
		Status:            j.Status,
		ErrorMessage:      j.ErrorMessage,
		CreatedAt:         j.CreatedAt,
		UpdatedAt:         j.UpdatedAt,
	}
}

func respondError(c *fiber.Ctx, code int, errCode, message string, details interface{}) error {
	var env errorEnvelope
	env.Error.Code = errCode
	env.Error.Message = message
	env.Error.Details = details
	return c.Status(code).JSON(env)
}

func (h *Handler) CreateDocType(c *fiber.Ctx) error {
	var req struct {
		Code string             `json:"code"`
		Name string             `json:"name"`
		Form schema.DocTypeForm `json:"form"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid_request", "invalid json", err.Error())
	}
	if req.Code == "" || req.Name == "" {
		return respondError(c, 400, "validation", "code and name are required", nil)
	}
	if req.Form.Version == 0 {
		req.Form.Version = 1
	}
	req.Form.DocType = schema.DocType{Code: req.Code, Name: req.Name}
	req.Form = req.Form.AlignMappingRules()
	if err := req.Form.Validate(); err != nil {
		return respondError(c, 400, "validation", err.Error(), nil)
	}
	formJSON, _ := json.Marshal(req.Form)
	hash, _ := req.Form.Hash()
	id, err := h.Store.CreateDocType(c.Context(), req.Code, req.Name, formJSON, hash)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to create doc type", err.Error())
	}
	h.Retriever.InvalidateQueryUnderstandingCache()
	docType, err := h.Store.GetDocType(c.Context(), id)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to load doc type", err.Error())
	}
	resp, err := toDocTypeResponse(docType)
	if err != nil {
		return respondError(c, 500, "decode_error", "failed to decode doc type form", err.Error())
	}
	return c.JSON(resp)
}

func (h *Handler) ListDocTypes(c *fiber.Ctx) error {
	items, err := h.Store.ListDocTypes(c.Context())
	if err != nil {
		return respondError(c, 500, "db_error", "failed to list doc types", err.Error())
	}
	resp := make([]docTypeResponse, 0, len(items))
	for _, item := range items {
		mapped, err := toDocTypeResponse(item)
		if err != nil {
			return respondError(c, 500, "decode_error", "failed to decode doc type form", err.Error())
		}
		resp = append(resp, mapped)
	}
	return c.JSON(resp)
}

func (h *Handler) UpdateDocTypeForm(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Form schema.DocTypeForm `json:"form"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid_request", "invalid json", err.Error())
	}
	if req.Form.Version == 0 {
		req.Form.Version = 1
	}
	req.Form = req.Form.AlignMappingRules()
	if err := req.Form.Validate(); err != nil {
		return respondError(c, 400, "validation", err.Error(), nil)
	}
	formJSON, _ := json.Marshal(req.Form)
	hash, _ := req.Form.Hash()
	if err := h.Store.UpdateDocTypeForm(c.Context(), id, formJSON, hash); err != nil {
		return respondError(c, 500, "db_error", "failed to update form", err.Error())
	}
	h.Retriever.InvalidateQueryUnderstandingCache()
	docType, err := h.Store.GetDocType(c.Context(), id)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to load doc type", err.Error())
	}
	resp, err := toDocTypeResponse(docType)
	if err != nil {
		return respondError(c, 500, "decode_error", "failed to decode doc type form", err.Error())
	}
	return c.JSON(resp)
}

func (h *Handler) DeleteDocType(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return respondError(c, 400, "validation", "doc type id is required", nil)
	}
	documentCount, err := h.Store.CountDocumentsByDocType(c.Context(), id)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to check doc type usage", err.Error())
	}
	if documentCount > 0 {
		return respondError(c, 409, "conflict", "doc type is in use by existing documents", fiber.Map{"document_count": documentCount})
	}
	deleted, err := h.Store.DeleteDocType(c.Context(), id)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to delete doc type", err.Error())
	}
	if !deleted {
		return respondError(c, 404, "not_found", "doc type not found", nil)
	}
	h.Retriever.InvalidateQueryUnderstandingCache()
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) ListDocuments(c *fiber.Ctx) error {
	items, err := h.Store.ListDocuments(c.Context())
	if err != nil {
		return respondError(c, 500, "db_error", "failed to list documents", err.Error())
	}
	resp := make([]documentResponse, 0, len(items))
	for _, item := range items {
		assets, err := h.Store.ListDocumentAssets(c.Context(), item.ID)
		if err != nil {
			return respondError(c, 500, "db_error", "failed to list document assets", err.Error())
		}
		resp = append(resp, toDocumentResponse(item, assets))
	}
	return c.JSON(resp)
}

func (h *Handler) CreateDocument(c *fiber.Ctx) error {
	var req struct {
		DocTypeCode string `json:"doc_type_code"`
		Title       string `json:"title"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid_request", "invalid json", err.Error())
	}
	if req.DocTypeCode == "" || req.Title == "" {
		return respondError(c, 400, "validation", "doc_type_code and title are required", nil)
	}
	docType, err := h.Store.GetDocTypeByCode(c.Context(), req.DocTypeCode)
	if err != nil {
		return respondError(c, 404, "not_found", "doc type not found", err.Error())
	}
	id, err := h.Store.CreateDocument(c.Context(), docType.ID, req.Title)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to create document", err.Error())
	}
	doc, err := h.Store.GetDocument(c.Context(), id)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to load document", err.Error())
	}
	return c.JSON(toDocumentResponse(doc, nil))
}

func (h *Handler) DeleteDocument(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return respondError(c, 400, "validation", "document id is required", nil)
	}
	// Consistency policy (Phase 1.5):
	// 1) Delete from Postgres first (source of truth for retrieval path).
	// 2) Cleanup vectors in Qdrant after DB commit.
	// If step 2 fails, enqueue a durable repair task. This avoids serving live DB
	// rows with missing vectors (worse for retrieval correctness) and makes any
	// orphan vectors explicitly repairable and auditable.
	versionIDs, err := h.Store.ListDocumentVersionIDsByDocument(c.Context(), id)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to load document versions", err.Error())
	}

	assetPaths, err := h.Store.ListDocumentAssetPaths(c.Context(), id)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to load document assets", err.Error())
	}
	deleted, err := h.Store.DeleteDocument(c.Context(), id)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to delete document", err.Error())
	}
	if !deleted {
		return respondError(c, 404, "not_found", "document not found", nil)
	}
	vectorCleanupFailed := false
	if h.Qdrant == nil {
		for _, path := range assetPaths {
			if err := h.Storage.Remove(path); err != nil {
				h.Logger.Error("failed to remove document asset file", slog.String("document_id", id), slog.String("path", path), slog.String("error", err.Error()))
			}
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
	for _, versionID := range versionIDs {
		filter := infra.Filter{
			Must: []infra.FieldCondition{{
				Key:   "document_version_id",
				Match: infra.FieldMatch{Value: versionID},
			}},
		}
		started := time.Now()
		h.Logger.Info("vector_cleanup_started",
			slog.String("document_id", id),
			slog.String("document_version_id", versionID),
			slog.String("collection", h.Qdrant.Collection),
		)
		if err := h.Qdrant.DeleteByFilter(c.Context(), h.Qdrant.Collection, filter); err != nil {
			vectorCleanupFailed = true
			enqueued, enqueueErr := h.Store.EnqueueDeleteVectorsRepair(c.Context(), h.Qdrant.Collection, id, versionID, filter)
			if enqueueErr != nil {
				h.Logger.Error("vector_cleanup_failed_repair_enqueue_failed",
					slog.String("document_id", id),
					slog.String("document_version_id", versionID),
					slog.String("collection", h.Qdrant.Collection),
					slog.String("error", err.Error()),
					slog.String("enqueue_error", enqueueErr.Error()),
				)
				continue
			}
			h.Logger.Error("vector_cleanup_failed_repair_enqueued",
				slog.String("document_id", id),
				slog.String("document_version_id", versionID),
				slog.String("collection", h.Qdrant.Collection),
				slog.String("error", err.Error()),
				slog.Bool("repair_enqueued", enqueued),
			)
			continue
		}
		h.Logger.Info("vector_cleanup_completed",
			slog.String("document_id", id),
			slog.String("document_version_id", versionID),
			slog.String("collection", h.Qdrant.Collection),
			slog.Duration("duration", time.Since(started)),
		)
	}
	for _, path := range assetPaths {
		if err := h.Storage.Remove(path); err != nil {
			h.Logger.Error("failed to remove document asset file", slog.String("document_id", id), slog.String("path", path), slog.String("error", err.Error()))
		}
	}
	if vectorCleanupFailed {
		return respondError(c, 500, "partial_delete", "document deleted but vector cleanup pending repair", nil)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) AddDocumentAsset(c *fiber.Ctx) error {
	documentID := c.Params("id")
	if fileHeader, err := c.FormFile("file"); err == nil && fileHeader != nil {
		file, err := fileHeader.Open()
		if err != nil {
			return respondError(c, 400, "invalid_request", "failed to read upload", err.Error())
		}
		defer file.Close()
		content, err := io.ReadAll(file)
		if err != nil {
			return respondError(c, 500, "storage_error", "failed to read upload", err.Error())
		}
		if fileHeader.Filename == "" {
			return respondError(c, 400, "validation", "file name is required", nil)
		}
		contentType := fileHeader.Header.Get("Content-Type")
		path := filepath.Join(documentID, uuid.NewString()+"_"+fileHeader.Filename)
		if err := h.Storage.Write(path, content); err != nil {
			return respondError(c, 500, "storage_error", "failed to store asset", err.Error())
		}
		id, err := h.Store.CreateDocumentAsset(c.Context(), documentID, fileHeader.Filename, contentType, path)
		if err != nil {
			return respondError(c, 500, "db_error", "failed to create asset", err.Error())
		}
		return c.JSON(fiber.Map{"id": id})
	}

	var req struct {
		FileName    string `json:"file_name"`
		ContentType string `json:"content_type"`
		Content     string `json:"content"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid_request", "invalid json", err.Error())
	}
	if req.FileName == "" || req.Content == "" {
		return respondError(c, 400, "validation", "file_name and content are required", nil)
	}
	path := filepath.Join(documentID, uuid.NewString()+"_"+req.FileName)
	if err := h.Storage.Write(path, []byte(req.Content)); err != nil {
		return respondError(c, 500, "storage_error", "failed to store asset", err.Error())
	}
	id, err := h.Store.CreateDocumentAsset(c.Context(), documentID, req.FileName, req.ContentType, path)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to create asset", err.Error())
	}
	return c.JSON(fiber.Map{"id": id})
}

func (h *Handler) CreateDocumentVersion(c *fiber.Ctx) error {
	documentID := c.Params("id")
	var req struct {
		AssetID string `json:"asset_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid_request", "invalid json", err.Error())
	}
	if req.AssetID == "" {
		return respondError(c, 400, "validation", "asset_id is required", nil)
	}
	asset, err := h.Store.GetDocumentAsset(c.Context(), req.AssetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return respondError(c, 404, "not_found", "asset not found", nil)
		}
		return respondError(c, 500, "db_error", "failed to load asset", err.Error())
	}
	if asset.DocumentID != documentID {
		return respondError(c, 400, "validation", "asset_id does not belong to the document", nil)
	}
	id, err := h.Store.CreateDocumentVersion(c.Context(), documentID, req.AssetID)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to create document version", err.Error())
	}
	return c.JSON(fiber.Map{"id": id})
}

func (h *Handler) DeleteDocumentVersion(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return respondError(c, 400, "validation", "document version id is required", nil)
	}
	version, err := h.Store.GetDocumentVersion(c.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return respondError(c, 404, "not_found", "document version not found", nil)
		}
		return respondError(c, 500, "db_error", "failed to load document version", err.Error())
	}
	deleted, err := h.Store.DeleteDocumentVersion(c.Context(), id)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to delete document version", err.Error())
	}
	if !deleted {
		return respondError(c, 404, "not_found", "document version not found", nil)
	}
	if h.Qdrant == nil {
		return c.SendStatus(fiber.StatusNoContent)
	}
	filter := infra.Filter{
		Must: []infra.FieldCondition{{
			Key:   "document_version_id",
			Match: infra.FieldMatch{Value: id},
		}},
	}
	started := time.Now()
	h.Logger.Info("vector_cleanup_started",
		slog.String("document_id", version.DocumentID),
		slog.String("document_version_id", id),
		slog.String("collection", h.Qdrant.Collection),
	)
	if err := h.Qdrant.DeleteByFilter(c.Context(), h.Qdrant.Collection, filter); err != nil {
		enqueued, enqueueErr := h.Store.EnqueueDeleteVectorsRepair(c.Context(), h.Qdrant.Collection, version.DocumentID, id, filter)
		if enqueueErr != nil {
			h.Logger.Error("vector_cleanup_failed_repair_enqueue_failed",
				slog.String("document_id", version.DocumentID),
				slog.String("document_version_id", id),
				slog.String("collection", h.Qdrant.Collection),
				slog.String("error", err.Error()),
				slog.String("enqueue_error", enqueueErr.Error()),
			)
			return respondError(c, 500, "partial_delete", "document version deleted but vector cleanup repair enqueue failed", enqueueErr.Error())
		}
		h.Logger.Error("vector_cleanup_failed_repair_enqueued",
			slog.String("document_id", version.DocumentID),
			slog.String("document_version_id", id),
			slog.String("collection", h.Qdrant.Collection),
			slog.String("error", err.Error()),
			slog.Bool("repair_enqueued", enqueued),
		)
		return respondError(c, 500, "partial_delete", "document version deleted but vector cleanup pending repair", nil)
	}
	h.Logger.Info("vector_cleanup_completed",
		slog.String("document_id", version.DocumentID),
		slog.String("document_version_id", id),
		slog.String("collection", h.Qdrant.Collection),
		slog.Duration("duration", time.Since(started)),
	)
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) ListIngestJobs(c *fiber.Ctx) error {
	items, err := h.Store.ListIngestJobs(c.Context())
	if err != nil {
		return respondError(c, 500, "db_error", "failed to list ingest jobs", err.Error())
	}
	resp := make([]ingestJobResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, toIngestJobResponse(item))
	}
	return c.JSON(resp)
}

func (h *Handler) DeleteIngestJob(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return respondError(c, 400, "validation", "job id is required", nil)
	}
	deleted, err := h.Store.DeleteIngestJob(c.Context(), id)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to delete ingest job", err.Error())
	}
	if !deleted {
		return respondError(c, 404, "not_found", "ingest job not found", nil)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) EnqueueIngest(c *fiber.Ctx) error {
	versionID := c.Params("id")
	if versionID == "" {
		return respondError(c, 400, "validation", "version id is required", nil)
	}
	if _, err := h.Store.GetDocumentVersion(c.Context(), versionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return respondError(c, 404, "not_found", "document version not found", nil)
		}
		return respondError(c, 500, "db_error", "failed to load document version", err.Error())
	}
	job, created, err := h.Store.EnqueueIngestJob(c.Context(), versionID)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to enqueue ingest", err.Error())
	}
	return c.JSON(fiber.Map{"id": job.ID, "status": job.Status, "created": created})
}

func (h *Handler) Search(c *fiber.Ctx) error {
	var req struct {
		Query   string      `json:"query"`
		TopK    int         `json:"top_k"`
		Filters ChatFilters `json:"filters"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid_request", "invalid json", err.Error())
	}
	if req.Query == "" {
		return respondError(c, 400, "validation", "query is required", nil)
	}
	if req.TopK <= 0 {
		req.TopK = 5
	}
	req.Filters.TopK = req.TopK
	req.Filters.Domain = strings.TrimSpace(req.Filters.Domain)
	req.Filters.DocType = strings.TrimSpace(req.Filters.DocType)
	req.Filters.DocumentNumber = strings.TrimSpace(req.Filters.DocumentNumber)
	req.Filters.ArticleNumber = strings.TrimSpace(req.Filters.ArticleNumber)
	req.Filters.EffectiveStatus = normalizeEffectiveStatus(req.Filters.EffectiveStatus)

	_ = h.Store.LogQuery(c.Context(), req.Query)
	results, err := h.Retriever.Search(c.Context(), req.Query, retrieval.SearchOptions{
		TopK:            req.TopK,
		Domain:          req.Filters.Domain,
		DocType:         req.Filters.DocType,
		EffectiveStatus: req.Filters.EffectiveStatus,
		DocumentNumber:  req.Filters.DocumentNumber,
		ArticleNumber:   req.Filters.ArticleNumber,
	})
	if err != nil {
		return respondError(c, 500, "search_error", "failed to search", err.Error())
	}
	resp := make([]fiber.Map, 0, len(results))
	for _, r := range results {
		source := buildAnswerSources([]retrieval.Result{r})
		citation := answer.Citation{ID: r.ChunkID, Excerpt: excerptText(r.Text, 320)}
		if len(source) > 0 {
			citation = source[0].Citation
		}
		resp = append(resp, fiber.Map{
			"chunk_id":   r.ChunkID,
			"text":       r.Text,
			"score":      r.Score,
			"citation":   citation,
			"metadata":   r.Metadata,
			"version_id": r.VersionID,
		})
	}
	return c.JSON(fiber.Map{"results": resp})
}

func (h *Handler) DebugDocTypeQuery(c *fiber.Ctx) error {
	var req struct {
		Query   string      `json:"query"`
		TopK    int         `json:"top_k"`
		Filters ChatFilters `json:"filters"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid_request", "invalid json", err.Error())
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		return respondError(c, 400, "validation", "query is required", nil)
	}
	if req.TopK <= 0 {
		req.TopK = 5
	}
	req.Filters.TopK = req.TopK
	req.Filters.Domain = strings.TrimSpace(req.Filters.Domain)
	req.Filters.DocType = strings.TrimSpace(req.Filters.DocType)
	req.Filters.DocumentNumber = strings.TrimSpace(req.Filters.DocumentNumber)
	req.Filters.ArticleNumber = strings.TrimSpace(req.Filters.ArticleNumber)
	req.Filters.EffectiveStatus = normalizeEffectiveStatus(req.Filters.EffectiveStatus)

	debug, err := h.Retriever.DebugSearch(c.Context(), req.Query, retrieval.SearchOptions{
		TopK:            req.TopK,
		Domain:          req.Filters.Domain,
		DocType:         req.Filters.DocType,
		EffectiveStatus: req.Filters.EffectiveStatus,
		DocumentNumber:  req.Filters.DocumentNumber,
		ArticleNumber:   req.Filters.ArticleNumber,
	})
	if err != nil {
		return respondError(c, 500, "search_error", "failed to debug query", err.Error())
	}
	resultRows := make([]fiber.Map, 0, len(debug.Results))
	for _, r := range debug.Results {
		resultRows = append(resultRows, fiber.Map{
			"chunk_id":   r.ChunkID,
			"text":       r.Text,
			"score":      r.Score,
			"metadata":   r.Metadata,
			"version_id": r.VersionID,
		})
	}
	return c.JSON(fiber.Map{
		"query":                 req.Query,
		"normalized_query":      debug.Analysis.NormalizedQuery,
		"canonical_query":       debug.Analysis.CanonicalQuery,
		"matched_doc_types":     debug.Analysis.MatchedDocTypes,
		"matched_query_rules":   debug.Analysis.MatchedQueryRules,
		"query_profile_hashes":  debug.Analysis.QueryProfileHashes,
		"inferred_legal_domain": debug.Analysis.LegalDomain,
		"inferred_legal_topic":  debug.Analysis.LegalTopic,
		"inferred_intent":       debug.Analysis.Intent,
		"applied_filters":       debug.AppliedFilters,
		"preferred_doc_types":   debug.PreferredDocTypes,
		"fallback_stages":       debug.FallbackStages,
		"results":               resultRows,
	})
}

func (h *Handler) Answer(c *fiber.Ctx) error {
	started := time.Now()
	var req answerRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid_request", "invalid json", err.Error())
	}
	question, filters := normalizeAnswerRequest(req, h.Tones)
	if question == "" {
		return respondError(c, 400, "validation", "question is required", nil)
	}
	ctx := c.UserContext()
	ctx, traceSvc, traceID, err := h.startAnswerTrace(ctx, "answer", question)
	if err != nil {
		return respondError(c, 500, "trace_error", "failed to create answer trace", err.Error())
	}
	_ = h.Store.LogQuery(ctx, question)
	runtimeCfg, err := h.loadRuntimeAnswerConfig(ctx, filters.Tone)
	if err != nil {
		traceSvc.OnError(err, traceLatency(started))
		return respondError(c, 500, "config_error", "failed to load answer runtime config", err.Error())
	}
	analysis := h.Retriever.AnalyzeQuery(ctx, question)
	if decision, ok, normalized := h.detectSmallTalkDecision(ctx, question); ok {
		traceSvc.OnRetrieval(normalized, map[string]interface{}{
			"canonical_query":       analysis.CanonicalQuery,
			"matched_doc_types":     analysis.MatchedDocTypes,
			"matched_query_rules":   analysis.MatchedQueryRules,
			"query_profile_hashes":  analysis.QueryProfileHashes,
			"inferred_intent":       analysis.Intent,
			"inferred_legal_domain": analysis.LegalDomain,
			"inferred_legal_topic":  analysis.LegalTopic,
			"legal_domain":          filters.Domain,
			"document_type":         filters.DocType,
			"effective_status":      filters.EffectiveStatus,
			"document_number":       filters.DocumentNumber,
			"article_number":        filters.ArticleNumber,
			"retrieved_chunks":      0,
			"max_similarity":        0.0,
			"guard_decision":        string(decision.Decision),
			"prompt_type_used":      decision.PromptType,
			"smalltalk_detected":    true,
			"retrieval": fiber.Map{
				"chunks":         0,
				"max_similarity": 0.0,
			},
			"guard": fiber.Map{
				"decision": string(decision.Decision),
			},
			"prompt": fiber.Map{
				"type": decision.PromptType,
			},
		}, []string{})
		traceSvc.OnResponse(decision.Message, true, traceLatency(started))
		return c.JSON(fiber.Map{"answer": decision.Message, "citations": []answer.Citation{}, "trace_id": traceID})
	}
	results, err := h.Retriever.Search(ctx, question, retrieval.SearchOptions{
		TopK:            filters.TopK,
		Domain:          filters.Domain,
		DocType:         filters.DocType,
		EffectiveStatus: filters.EffectiveStatus,
		DocumentNumber:  filters.DocumentNumber,
		ArticleNumber:   filters.ArticleNumber,
	})
	if err != nil {
		traceSvc.OnError(err, traceLatency(started))
		return respondError(c, 500, "search_error", "failed to search", err.Error())
	}
	decision, diag, err := h.evaluateGuardPolicy(ctx, runtimeCfg.Policy, results)
	if err != nil {
		traceSvc.OnError(err, traceLatency(started))
		return respondError(c, 500, "config_error", "failed to evaluate guard decision", err.Error())
	}
	promptTypeUsed := decision.PromptType
	if decision.Allow() {
		if _, usedPromptType, routeErr := h.getAnswerPrompt(ctx); routeErr == nil {
			promptTypeUsed = usedPromptType
		}
	}
	traceSvc.OnRetrieval(analysis.NormalizedQuery, map[string]interface{}{
		"canonical_query":       analysis.CanonicalQuery,
		"matched_doc_types":     analysis.MatchedDocTypes,
		"matched_query_rules":   analysis.MatchedQueryRules,
		"query_profile_hashes":  analysis.QueryProfileHashes,
		"inferred_intent":       analysis.Intent,
		"inferred_legal_domain": analysis.LegalDomain,
		"inferred_legal_topic":  analysis.LegalTopic,
		"legal_domain":          filters.Domain,
		"document_type":         filters.DocType,
		"effective_status":      filters.EffectiveStatus,
		"document_number":       filters.DocumentNumber,
		"article_number":        filters.ArticleNumber,
		"retrieved_chunks":      diag.RetrievedChunks,
		"max_similarity":        diag.MaxSimilarity,
		"guard_decision":        string(decision.Decision),
		"prompt_type_used":      promptTypeUsed,
		"retrieval": fiber.Map{
			"chunks":         diag.RetrievedChunks,
			"max_similarity": diag.MaxSimilarity,
		},
		"guard": fiber.Map{
			"decision": string(decision.Decision),
		},
		"prompt": fiber.Map{
			"type": promptTypeUsed,
		},
	}, traceChunkIDs(results))
	if !decision.Allow() {
		traceSvc.OnResponse(decision.Message, true, traceLatency(started))
		return c.JSON(fiber.Map{"answer": decision.Message, "citations": []answer.Citation{}, "trace_id": traceID})
	}
	promptCfg, _, err := h.getAnswerPrompt(ctx)
	if err != nil {
		traceSvc.OnError(err, traceLatency(started))
		return respondError(c, 500, "config_error", "failed to route legal answer prompt", err.Error())
	}
	sources := buildAnswerSources(results)
	ansSvc := &answer.Service{
		Client:       h.AnswerCli,
		SystemPrompt: promptCfg.SystemPrompt,
		Tone:         runtimeCfg.Tone,
		Temperature:  promptCfg.Temperature,
		MaxTokens:    promptCfg.MaxTokens,
		Retry:        promptCfg.Retry,
	}
	traceSvc.OnPromptSnapshot(ansSvc.PromptSnapshot(question, sources))
	traceSvc.OnLLMCall(h.AnswerCli.Model, promptCfg.Temperature, promptCfg.MaxTokens, promptCfg.Retry)
	ans, err := ansSvc.Generate(ctx, question, sources)
	if err != nil {
		traceSvc.OnError(err, traceLatency(started))
		return respondError(c, 500, "answer_error", "failed to generate answer", err.Error())
	}
	finalAnswer, citations, valid, validationErr := h.validateGeneratedLegalAnswer(ctx, ans, sources)
	if validationErr != nil {
		traceSvc.OnError(validationErr, traceLatency(started))
		return respondError(c, 500, "validation_error", "failed to validate legal answer", validationErr.Error())
	}
	if !valid {
		citations = []answer.Citation{}
	}
	_ = h.Store.LogAnswer(ctx, question, finalAnswer)
	traceSvc.OnResponse(finalAnswer, true, traceLatency(started))
	return c.JSON(fiber.Map{"answer": finalAnswer, "citations": citations, "trace_id": traceID})
}
