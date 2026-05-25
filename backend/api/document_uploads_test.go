package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/core/uploadscan"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
)

type documentUploadTestStore struct {
	fakeStore
	createdInput        domain.DocumentUploadInput
	eventInputs         []domain.DocumentUploadEventInput
	uploads             map[string]domain.DocumentUpload
	docTypesByCode      map[string]domain.DocType
	approvedUploadID    string
	approvedDocTypeCode string
	approvedActor       string
	approvedReason      string
	approvedAnalysis    []byte
}

func (s *documentUploadTestStore) CreateDocumentUpload(ctx context.Context, input domain.DocumentUploadInput) (domain.DocumentUpload, error) {
	s.createdInput = input
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	upload := domain.DocumentUpload{
		ID:            "upload-1",
		Title:         input.Title,
		FileName:      input.FileName,
		ContentType:   input.ContentType,
		StoragePath:   input.StoragePath,
		FileSizeBytes: input.FileSizeBytes,
		SHA256:        input.SHA256,
		Status:        input.Status,
		AnalysisJSON:  input.AnalysisJSON,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	for _, eventInput := range input.Events {
		if eventInput.UploadID == nil {
			uploadID := upload.ID
			eventInput.UploadID = &uploadID
		}
		event, err := s.CreateDocumentUploadEvent(ctx, eventInput)
		if err != nil {
			return domain.DocumentUpload{}, err
		}
		upload.Events = append(upload.Events, event)
	}
	return upload, nil
}

func (s *documentUploadTestStore) CreateDocumentUploadEvent(ctx context.Context, input domain.DocumentUploadEventInput) (domain.DocumentUploadEvent, error) {
	s.eventInputs = append(s.eventInputs, input)
	now := time.Date(2026, 5, 24, 10, 0, len(s.eventInputs), 0, time.UTC)
	return domain.DocumentUploadEvent{
		ID:            "event-" + strconv.Itoa(len(s.eventInputs)),
		UploadID:      input.UploadID,
		EventType:     input.EventType,
		Stage:         input.Stage,
		Status:        input.Status,
		Message:       input.Message,
		EvidenceJSON:  input.EvidenceJSON,
		Actor:         input.Actor,
		FileName:      input.FileName,
		ContentType:   input.ContentType,
		FileSizeBytes: input.FileSizeBytes,
		SHA256:        input.SHA256,
		CreatedAt:     now,
	}, nil
}

func (s *documentUploadTestStore) GetDocumentUpload(ctx context.Context, id string) (domain.DocumentUpload, error) {
	upload, ok := s.uploads[id]
	if !ok {
		return domain.DocumentUpload{}, sql.ErrNoRows
	}
	return upload, nil
}

func (s *documentUploadTestStore) ListDocumentUploads(ctx context.Context, limit int) ([]domain.DocumentUpload, error) {
	out := make([]domain.DocumentUpload, 0, len(s.uploads))
	for _, upload := range s.uploads {
		out = append(out, upload)
	}
	return out, nil
}

func (s *documentUploadTestStore) GetDocTypeByCode(ctx context.Context, code string) (domain.DocType, error) {
	docType, ok := s.docTypesByCode[code]
	if !ok {
		return domain.DocType{}, sql.ErrNoRows
	}
	return docType, nil
}

func (s *documentUploadTestStore) ApproveDocumentUpload(ctx context.Context, id string, docType domain.DocType, analysisJSON []byte, actor, reason string) (domain.DocumentUploadPromotion, error) {
	upload, ok := s.uploads[id]
	if !ok {
		return domain.DocumentUploadPromotion{}, sql.ErrNoRows
	}
	s.approvedUploadID = id
	s.approvedDocTypeCode = docType.Code
	s.approvedActor = actor
	s.approvedReason = reason
	s.approvedAnalysis = analysisJSON
	documentID := "document-approved"
	assetID := "asset-approved"
	versionID := "version-approved"
	upload.Status = "indexing"
	upload.AnalysisJSON = analysisJSON
	upload.DocumentID = &documentID
	upload.DocumentAssetID = &assetID
	upload.DocumentVersionID = &versionID
	s.uploads[id] = upload
	return domain.DocumentUploadPromotion{
		DocumentID:        documentID,
		DocumentAssetID:   assetID,
		DocumentVersionID: versionID,
		IngestJobID:       "job-approved",
	}, nil
}

func TestUploadDocumentStoresUploadWithoutPublishingDocument(t *testing.T) {
	root := t.TempDir()
	store := &documentUploadTestStore{}
	var scanned uploadscan.File
	h := &Handler{
		Store:   store,
		Storage: infra.NewStorage(root),
		UploadScanner: uploadScannerFunc(func(ctx context.Context, file uploadscan.File) error {
			scanned = file
			return nil
		}),
	}

	app := fiber.New()
	app.Post("/document-uploads", h.UploadDocument)

	body, contentType := multipartUploadBody(t, "file", "Nghi-dinh-123.txt", []byte("noi dung van ban phap luat"))
	req := httptest.NewRequest(http.MethodPost, "/document-uploads", body)
	req.Header.Set("Content-Type", contentType)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, payload)
	}

	var got documentUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Status != "uploaded" {
		t.Fatalf("status = %q, want uploaded", got.Status)
	}
	if len(got.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(got.Events))
	}
	if got.Events[0].Stage != "upload" || got.Events[0].Status != "done" {
		t.Fatalf("upload event = %#v", got.Events[0])
	}
	if got.Events[1].Stage != "security" || got.Events[1].Status != "done" {
		t.Fatalf("security event = %#v", got.Events[1])
	}
	if got.Title != "Nghi-dinh-123" {
		t.Fatalf("title = %q", got.Title)
	}
	if store.createdInput.StoragePath == "" {
		t.Fatal("storage path was not persisted")
	}
	if _, err := os.Stat(filepath.Join(root, store.createdInput.StoragePath)); err != nil {
		t.Fatalf("stored upload missing: %v", err)
	}
	sum := sha256.Sum256([]byte("noi dung van ban phap luat"))
	if store.createdInput.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha256 = %q", store.createdInput.SHA256)
	}
	if store.createdInput.Status != "uploaded" {
		t.Fatalf("input status = %q", store.createdInput.Status)
	}
	if len(store.eventInputs) != 2 {
		t.Fatalf("recorded events = %d, want 2", len(store.eventInputs))
	}
	if store.eventInputs[1].SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("security event sha256 = %q", store.eventInputs[1].SHA256)
	}
	if scanned.Name != "Nghi-dinh-123.txt" || string(scanned.Content) != "noi dung van ban phap luat" {
		t.Fatalf("scanner saw unexpected file: %#v", scanned)
	}
}

func TestUploadDocumentRejectsUnsupportedFileType(t *testing.T) {
	h := &Handler{
		Store:   &documentUploadTestStore{},
		Storage: infra.NewStorage(t.TempDir()),
	}
	app := fiber.New()
	app.Post("/document-uploads", h.UploadDocument)

	body, contentType := multipartUploadBody(t, "file", "script.exe", []byte("bad"))
	req := httptest.NewRequest(http.MethodPost, "/document-uploads", body)
	req.Header.Set("Content-Type", contentType)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, payload)
	}
}

func TestUploadDocumentRejectsInvalidFileSignature(t *testing.T) {
	h := &Handler{
		Store:   &documentUploadTestStore{},
		Storage: infra.NewStorage(t.TempDir()),
	}
	app := fiber.New()
	app.Post("/document-uploads", h.UploadDocument)

	body, contentType := multipartUploadBody(t, "file", "fake.pdf", []byte("not a pdf"))
	req := httptest.NewRequest(http.MethodPost, "/document-uploads", body)
	req.Header.Set("Content-Type", contentType)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, payload)
	}
}

func TestUploadDocumentRejectsMalwareBeforeStorage(t *testing.T) {
	root := t.TempDir()
	store := &documentUploadTestStore{}
	h := &Handler{
		Store:   store,
		Storage: infra.NewStorage(root),
		UploadScanner: uploadScannerFunc(func(ctx context.Context, file uploadscan.File) error {
			return uploadscan.MalwareDetectedError{Signature: "Eicar-Test-Signature"}
		}),
	}
	app := fiber.New()
	app.Post("/document-uploads", h.UploadDocument)

	body, contentType := multipartUploadBody(t, "file", "eicar.txt", []byte("X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*"))
	req := httptest.NewRequest(http.MethodPost, "/document-uploads", body)
	req.Header.Set("Content-Type", contentType)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, payload)
	}
	var got errorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Error.Code != "malware_detected" {
		t.Fatalf("error code = %q", got.Error.Code)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read storage root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no stored files, found %d", len(entries))
	}
	if len(store.eventInputs) != 1 {
		t.Fatalf("recorded events = %d, want 1", len(store.eventInputs))
	}
	if store.eventInputs[0].Stage != "security" || store.eventInputs[0].Status != "blocked" {
		t.Fatalf("malware event = %#v", store.eventInputs[0])
	}
	if store.eventInputs[0].UploadID != nil {
		t.Fatalf("malware audit upload id = %v, want nil", *store.eventInputs[0].UploadID)
	}
}

func TestUploadDocumentFailsClosedWhenScannerUnavailable(t *testing.T) {
	store := &documentUploadTestStore{}
	h := &Handler{
		Store:   store,
		Storage: infra.NewStorage(t.TempDir()),
		UploadScanner: uploadScannerFunc(func(ctx context.Context, file uploadscan.File) error {
			return uploadscan.ScannerUnavailableError{Err: context.DeadlineExceeded}
		}),
	}
	app := fiber.New()
	app.Post("/document-uploads", h.UploadDocument)

	body, contentType := multipartUploadBody(t, "file", "clean.txt", []byte("noi dung hop le"))
	req := httptest.NewRequest(http.MethodPost, "/document-uploads", body)
	req.Header.Set("Content-Type", contentType)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, payload)
	}
	if len(store.eventInputs) != 1 {
		t.Fatalf("recorded events = %d, want 1", len(store.eventInputs))
	}
	if store.eventInputs[0].Stage != "security" || store.eventInputs[0].Status != "unavailable" {
		t.Fatalf("scanner unavailable event = %#v", store.eventInputs[0])
	}
}

func TestListDocumentUploadsReturnsReviewActionDescriptors(t *testing.T) {
	store := &documentUploadTestStore{
		uploads: map[string]domain.DocumentUpload{
			"upload-review": {
				ID:           "upload-review",
				Title:        "Needs review",
				FileName:     "review.txt",
				ContentType:  "text/plain",
				StoragePath:  "document_uploads/review.txt",
				Status:       "classification_low_confidence",
				AnalysisJSON: []byte(`{"candidates":[{"code":"vn_decree","name":"Nghị định","score":8}]}`),
				CreatedAt:    time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC),
				UpdatedAt:    time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	h := &Handler{Store: store}
	app := fiber.New()
	app.Get("/document-uploads", h.ListDocumentUploads)

	req := httptest.NewRequest(http.MethodGet, "/document-uploads", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, payload)
	}

	var got []documentUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("uploads = %d, want 1", len(got))
	}
	approve := got[0].Actions["approve"]
	if !approve.Enabled || approve.Method != "POST" || approve.Href != "/document-uploads/upload-review/actions/approve" {
		t.Fatalf("approve descriptor = %#v", approve)
	}
	reindex := got[0].Actions["reindex"]
	if reindex.Enabled || reindex.Reason == "" {
		t.Fatalf("reindex descriptor = %#v", reindex)
	}
}

func TestApproveDocumentUploadUsesOperatorSelectedDocType(t *testing.T) {
	store := &documentUploadTestStore{
		uploads: map[string]domain.DocumentUpload{
			"upload-review": {
				ID:           "upload-review",
				Title:        "Needs review",
				FileName:     "review.txt",
				ContentType:  "text/plain",
				StoragePath:  "document_uploads/review.txt",
				Status:       "classification_low_confidence",
				AnalysisJSON: []byte(`{"confidence":0.4,"candidates":[{"code":"vn_decree","name":"Nghị định","score":8}]}`),
				CreatedAt:    time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC),
				UpdatedAt:    time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC),
			},
		},
		docTypesByCode: map[string]domain.DocType{
			"vn_decree": {ID: "doc-type-1", Code: "vn_decree", Name: "Nghị định"},
		},
	}
	h := &Handler{Store: store}
	app := fiber.New()
	app.Post("/document-uploads/:id/actions/approve", h.ApproveDocumentUpload)

	req := httptest.NewRequest(
		http.MethodPost,
		"/document-uploads/upload-review/actions/approve",
		bytes.NewBufferString(`{"doc_type_code":"vn_decree","reason":"correct profile"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Actor", "reviewer-1")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, payload)
	}

	var got documentUploadActionResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Status != "indexing" || got.DocumentUpload == nil || got.DocumentUpload.Status != "indexing" {
		t.Fatalf("action response = %#v", got)
	}
	if store.approvedUploadID != "upload-review" || store.approvedDocTypeCode != "vn_decree" || store.approvedActor != "reviewer-1" {
		t.Fatalf("approval store state: upload=%q doc_type=%q actor=%q", store.approvedUploadID, store.approvedDocTypeCode, store.approvedActor)
	}
	var analysis map[string]interface{}
	if err := json.Unmarshal(store.approvedAnalysis, &analysis); err != nil {
		t.Fatalf("decode approved analysis: %v", err)
	}
	if analysis["approved_doc_type_code"] != "vn_decree" || analysis["status_reason"] != "approved_by_operator" {
		t.Fatalf("approved analysis = %#v", analysis)
	}
}

type uploadScannerFunc func(ctx context.Context, file uploadscan.File) error

func (f uploadScannerFunc) Scan(ctx context.Context, file uploadscan.File) error {
	return f(ctx, file)
}

func multipartUploadBody(t *testing.T, fieldName, fileName string, content []byte) (*bytes.Buffer, string) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &body, writer.FormDataContentType()
}
