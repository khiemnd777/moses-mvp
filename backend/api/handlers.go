package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/core/ingest"
	"github.com/khiemnd777/legal_api/core/retrieval"
	"github.com/khiemnd777/legal_api/core/schema"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	Store     *infra.Store
	Storage   *infra.Storage
	Retriever *retrieval.Service
	AnswerCli *answer.Client
	Guard     string
	Tones     map[string]string
	IngestSvc *ingest.Service
	Logger    *slog.Logger
}

func NewHandler(store *infra.Store, storage *infra.Storage, retriever *retrieval.Service, ans *answer.Client, guard string, tones map[string]string, ingestSvc *ingest.Service, logger *slog.Logger) *Handler {
	return &Handler{Store: store, Storage: storage, Retriever: retriever, AnswerCli: ans, Guard: guard, Tones: tones, IngestSvc: ingestSvc, Logger: logger}
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
	for _, path := range assetPaths {
		if err := h.Storage.Remove(path); err != nil {
			h.Logger.Error("failed to remove document asset file", slog.String("document_id", id), slog.String("path", path), slog.String("error", err.Error()))
		}
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
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
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
	_ = h.Store.LogQuery(c.Context(), req.Query)
	results, err := h.Retriever.Search(c.Context(), req.Query, req.TopK)
	if err != nil {
		return respondError(c, 500, "search_error", "failed to search", err.Error())
	}
	resp := make([]fiber.Map, 0, len(results))
	for _, r := range results {
		resp = append(resp, fiber.Map{
			"chunk_id":    r.ChunkID,
			"text":        r.Text,
			"citation_id": r.CitationID,
		})
	}
	return c.JSON(fiber.Map{"results": resp})
}

func (h *Handler) Answer(c *fiber.Ctx) error {
	var req answerRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid_request", "invalid json", err.Error())
	}
	question, filters := normalizeAnswerRequest(req, h.Tones)
	if question == "" {
		return respondError(c, 400, "validation", "question is required", nil)
	}
	ctx := c.Context()
	_ = h.Store.LogQuery(ctx, question)
	results, err := h.Retriever.Search(ctx, question, filters.TopK)
	if err != nil {
		return respondError(c, 500, "search_error", "failed to search", err.Error())
	}
	sources := make([]answer.Source, 0, len(results))
	for _, r := range results {
		sources = append(sources, answer.Source{Text: r.Text, CitationID: r.CitationID})
	}
	tone := h.Tones[defaultToneKey]
	if v, ok := h.Tones[filters.Tone]; ok {
		tone = v
	}
	ansSvc := &answer.Service{Client: h.AnswerCli, Guard: h.Guard, Tone: tone}
	ans, err := ansSvc.Generate(ctx, question, sources)
	if err != nil {
		return respondError(c, 500, "answer_error", "failed to generate answer", err.Error())
	}
	_ = h.Store.LogAnswer(ctx, question, ans)
	return c.JSON(fiber.Map{"answer": ans, "citations": sources})
}
