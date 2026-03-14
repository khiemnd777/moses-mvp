package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/core/retrieval"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
	"github.com/khiemnd777/legal_api/observability"
	"github.com/khiemnd777/legal_api/pkg/logging"
)

type fakeStore struct{}

func (f *fakeStore) CreateDocType(ctx context.Context, code, name string, formJSON []byte, formHash string) (string, error) {
	return "", nil
}
func (f *fakeStore) ListDocTypes(ctx context.Context) ([]domain.DocType, error) { return nil, nil }
func (f *fakeStore) UpdateDocTypeForm(ctx context.Context, id string, formJSON []byte, formHash string) error {
	return nil
}
func (f *fakeStore) CountDocumentsByDocType(ctx context.Context, docTypeID string) (int, error) {
	return 0, nil
}
func (f *fakeStore) DeleteDocType(ctx context.Context, id string) (bool, error) { return false, nil }
func (f *fakeStore) GetDocType(ctx context.Context, id string) (domain.DocType, error) {
	return domain.DocType{}, nil
}
func (f *fakeStore) GetDocTypeByCode(ctx context.Context, code string) (domain.DocType, error) {
	return domain.DocType{}, nil
}
func (f *fakeStore) CreateDocument(ctx context.Context, docTypeID, title string) (string, error) {
	return "", nil
}
func (f *fakeStore) GetDocument(ctx context.Context, id string) (domain.Document, error) {
	return domain.Document{}, nil
}
func (f *fakeStore) ListDocuments(ctx context.Context) ([]domain.Document, error) { return nil, nil }
func (f *fakeStore) DeleteDocument(ctx context.Context, id string) (bool, error)  { return false, nil }
func (f *fakeStore) ListDocumentVersionIDsByDocument(ctx context.Context, documentID string) ([]string, error) {
	return nil, nil
}
func (f *fakeStore) ListChunkIDsByVersion(ctx context.Context, documentVersionID string) ([]string, error) {
	return nil, nil
}
func (f *fakeStore) EnqueueDeleteVectorsRepair(ctx context.Context, collection, documentID, documentVersionID string, filter infra.Filter) (bool, error) {
	return true, nil
}
func (f *fakeStore) EnqueueRebuildVectorsRepair(ctx context.Context, collection, documentVersionID string) (bool, error) {
	return true, nil
}
func (f *fakeStore) ListDocumentAssetPaths(ctx context.Context, documentID string) ([]string, error) {
	return nil, nil
}
func (f *fakeStore) ListDocumentAssets(ctx context.Context, documentID string) ([]domain.DocumentAssetWithVersions, error) {
	return nil, nil
}
func (f *fakeStore) CreateDocumentAsset(ctx context.Context, documentID, fileName, contentType, storagePath string) (string, error) {
	return "", nil
}
func (f *fakeStore) GetDocumentAsset(ctx context.Context, id string) (domain.DocumentAsset, error) {
	return domain.DocumentAsset{}, nil
}
func (f *fakeStore) CreateDocumentVersion(ctx context.Context, documentID, assetID string) (string, error) {
	return "", nil
}
func (f *fakeStore) GetDocumentVersion(ctx context.Context, id string) (domain.DocumentVersion, error) {
	return domain.DocumentVersion{}, nil
}
func (f *fakeStore) GetDocumentVersionBundle(ctx context.Context, id string) (domain.DocumentVersion, domain.Document, domain.DocumentAsset, domain.DocType, error) {
	return domain.DocumentVersion{}, domain.Document{}, domain.DocumentAsset{}, domain.DocType{}, nil
}
func (f *fakeStore) DeleteDocumentVersion(ctx context.Context, id string) (bool, error) {
	return false, nil
}
func (f *fakeStore) ListIngestJobs(ctx context.Context) ([]domain.IngestJob, error) { return nil, nil }
func (f *fakeStore) DeleteIngestJob(ctx context.Context, id string) (bool, error)   { return false, nil }
func (f *fakeStore) EnqueueIngestJob(ctx context.Context, documentVersionID string) (domain.IngestJob, bool, error) {
	return domain.IngestJob{}, false, nil
}
func (f *fakeStore) LogQuery(ctx context.Context, q string) error     { return nil }
func (f *fakeStore) LogAnswer(ctx context.Context, q, a string) error { return nil }
func (f *fakeStore) CreateConversation(ctx context.Context, title string, userID *string) (domain.Conversation, error) {
	return domain.Conversation{}, nil
}
func (f *fakeStore) ListConversations(ctx context.Context, userID *string) ([]domain.Conversation, error) {
	return nil, nil
}
func (f *fakeStore) GetConversation(ctx context.Context, id string) (domain.Conversation, error) {
	return domain.Conversation{}, nil
}
func (f *fakeStore) DeleteConversation(ctx context.Context, id string) (bool, error) {
	return true, nil
}
func (f *fakeStore) UpdateConversationTitle(ctx context.Context, id, title string) error { return nil }
func (f *fakeStore) CreateMessage(ctx context.Context, conversationID, role, content string, citationsJSON []byte, traceID *string) (domain.Message, error) {
	return domain.Message{}, nil
}
func (f *fakeStore) UpdateMessage(ctx context.Context, id, content string, citationsJSON []byte, traceID *string) error {
	return nil
}
func (f *fakeStore) ListMessagesByConversation(ctx context.Context, conversationID string) ([]domain.Message, error) {
	return nil, nil
}
func (f *fakeStore) GetActiveAIGuardPolicy(ctx context.Context) (domain.AIGuardPolicy, error) {
	return domain.AIGuardPolicy{
		Name:               "default",
		Enabled:            true,
		MinRetrievedChunks: 1,
		MinSimilarityScore: 0,
		OnEmptyRetrieval:   "refuse",
		OnLowConfidence:    "refuse",
	}, nil
}
func (f *fakeStore) GetActiveAIPromptByType(ctx context.Context, promptType string) (domain.AIPrompt, error) {
	return domain.AIPrompt{
		Name:         "test",
		PromptType:   promptType,
		SystemPrompt: "You are a legal assistant.",
		Temperature:  0.2,
		MaxTokens:    256,
		Retry:        1,
		Enabled:      true,
	}, nil
}
func (f *fakeStore) ListEnabledAIPrompts(ctx context.Context) ([]domain.AIPrompt, error) {
	return []domain.AIPrompt{
		{
			Name:         "legal_guard_prompt",
			PromptType:   "legal_guard",
			SystemPrompt: "You are a legal assistant.",
			Temperature:  0.2,
			MaxTokens:    256,
			Retry:        1,
			Enabled:      true,
		},
		{
			Name:         "legal_answer_prompt",
			PromptType:   "legal_answer",
			SystemPrompt: "You are a legal assistant. Answer with legal reasoning format.",
			Temperature:  0.2,
			MaxTokens:    256,
			Retry:        1,
			Enabled:      true,
		},
		{
			Name:         "legal_refusal_prompt",
			PromptType:   "legal_refusal",
			SystemPrompt: "Không đủ căn cứ pháp lý trong dữ liệu truy xuất để trả lời chắc chắn.",
			Temperature:  0.2,
			MaxTokens:    64,
			Retry:        0,
			Enabled:      true,
		},
		{
			Name:         "legal_clarification_prompt",
			PromptType:   "legal_clarification",
			SystemPrompt: "Vui lòng cung cấp thêm dữ kiện pháp lý hoặc điều khoản cụ thể cần tra cứu.",
			Temperature:  0.2,
			MaxTokens:    64,
			Retry:        0,
			Enabled:      true,
		},
	}, nil
}

type fakeRetriever struct {
	results []retrieval.Result
}

func (f *fakeRetriever) Search(ctx context.Context, query string, opts retrieval.SearchOptions) ([]retrieval.Result, error) {
	return f.results, nil
}

type memoryTraceRepo struct {
	mu    sync.Mutex
	items map[string]observability.TraceRecord
}

func newMemoryTraceRepo() *memoryTraceRepo {
	return &memoryTraceRepo{items: map[string]observability.TraceRecord{}}
}

func (m *memoryTraceRepo) Create(ctx context.Context, record observability.TraceRecord) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[record.TraceID] = record
	return record.TraceID, nil
}

func (m *memoryTraceRepo) Update(ctx context.Context, traceID string, record observability.TraceRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.items[traceID]
	if record.Mode != "" {
		current.Mode = record.Mode
	}
	if record.UserQuery != "" {
		current.UserQuery = record.UserQuery
	}
	if record.NormalizedQuery != "" {
		current.NormalizedQuery = record.NormalizedQuery
	}
	if len(record.RetrievalFiltersJSON) > 0 {
		current.RetrievalFiltersJSON = record.RetrievalFiltersJSON
	}
	if len(record.RetrievedChunkIDsJSON) > 0 {
		current.RetrievedChunkIDsJSON = record.RetrievedChunkIDsJSON
	}
	if record.PromptSnapshot != "" {
		current.PromptSnapshot = record.PromptSnapshot
	}
	if record.ModelName != "" {
		current.ModelName = record.ModelName
	}
	if record.Temperature != 0 {
		current.Temperature = record.Temperature
	}
	if record.MaxTokens != 0 {
		current.MaxTokens = record.MaxTokens
	}
	if record.Retry != 0 {
		current.Retry = record.Retry
	}
	if record.ResponseText != "" {
		current.ResponseText = record.ResponseText
	}
	current.StreamCompleted = record.StreamCompleted
	if record.LatencyMS != 0 {
		current.LatencyMS = record.LatencyMS
	}
	if record.ErrorMessage != "" {
		current.ErrorMessage = record.ErrorMessage
	}
	m.items[traceID] = current
	return nil
}

func (m *memoryTraceRepo) List(ctx context.Context, limit int) ([]observability.AnswerTrace, error) {
	return nil, nil
}

func (m *memoryTraceRepo) GetByTraceID(ctx context.Context, traceID string) (observability.AnswerTrace, error) {
	return observability.AnswerTrace{}, nil
}

func (m *memoryTraceRepo) snapshot(traceID string) observability.TraceRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.items[traceID]
}

func TestAnswerGeneratesTraceIDAndPersistsTrace(t *testing.T) {
	openAIServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Day la cau tra loi"}}]}`))
	}))
	defer openAIServer.Close()

	traceRepo := newMemoryTraceRepo()
	handler := newTestHandler(openAIServer.URL, traceRepo)
	app := fiber.New()
	app.Post("/answer", answerTraceMiddleware(logging.New()), handler.Answer)

	req := httptest.NewRequest(http.MethodPost, "/answer", strings.NewReader(`{"question":"thu tuc ly hon"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, int(5*time.Second/time.Millisecond))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var payload struct {
		Answer    string            `json:"answer"`
		Citations []answer.Citation `json:"citations"`
		TraceID   string            `json:"trace_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if payload.TraceID == "" {
		t.Fatalf("expected trace_id in response")
	}
	if got := resp.Header.Get("X-Trace-Id"); got == "" || got != payload.TraceID {
		t.Fatalf("expected X-Trace-Id header to match response trace_id, got %q", got)
	}
	if len(payload.Citations) == 0 || payload.Citations[0].ID == "" {
		t.Fatalf("expected citations with stable structure")
	}

	record := traceRepo.snapshot(payload.TraceID)
	if record.TraceID == "" {
		t.Fatalf("expected trace record to be persisted")
	}
	if record.PromptSnapshot == "" {
		t.Fatalf("expected prompt snapshot to be stored")
	}
	if len(record.RetrievedChunkIDsJSON) == 0 {
		t.Fatalf("expected retrieved chunk ids to be stored")
	}
	if record.ResponseText == "" {
		t.Fatalf("expected response text to be stored")
	}
}

func TestAnswerStreamGeneratesTraceIDAndPersistsTrace(t *testing.T) {
	openAIServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Xin chao \"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ban\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer openAIServer.Close()

	traceRepo := newMemoryTraceRepo()
	handler := newTestHandler(openAIServer.URL, traceRepo)
	app := fiber.New()
	app.Post("/answer/stream", answerTraceMiddleware(logging.New()), handler.AnswerStream)

	req := httptest.NewRequest(http.MethodPost, "/answer/stream", strings.NewReader(`{"question":"thu tuc ly hon"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, int(5*time.Second/time.Millisecond))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	raw := string(body)

	if !strings.Contains(raw, "event: meta") || !strings.Contains(raw, "\"trace_id\":") {
		t.Fatalf("expected stream meta event with trace_id, got %s", raw)
	}
	if !strings.Contains(raw, "event: citations") {
		t.Fatalf("expected citations event, got %s", raw)
	}

	traceID := extractStreamTraceID(raw)
	if traceID == "" {
		t.Fatalf("expected to extract trace_id from stream")
	}
	record := traceRepo.snapshot(traceID)
	if record.TraceID == "" {
		t.Fatalf("expected trace record to be persisted")
	}
	if !record.StreamCompleted {
		t.Fatalf("expected stream_completed=true")
	}
	if record.ResponseText == "" {
		t.Fatalf("expected stream response text to be stored")
	}
}

func newTestHandler(baseURL string, repo observability.TraceRepository) *Handler {
	client := answer.NewClient("test-key", "gpt-test")
	client.BaseURL = baseURL
	return NewHandler(
		&fakeStore{},
		nil,
		nil,
		&fakeRetriever{results: []retrieval.Result{{
			ChunkID:    "chunk-1",
			Text:       "Dieu 1 ve thu tuc ly hon",
			VersionID:  "version-1",
			ChunkIndex: 1,
			Score:      0.99,
			Metadata: map[string]interface{}{
				"document_title": "Luat Hon nhan va Gia dinh",
				"article":        "1",
				"document_type":  "law",
			},
		}}},
		client,
		map[string]string{"default": "Tra loi bang tieng Viet."},
		nil,
		logging.New(),
		repo,
	)
}

func extractStreamTraceID(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		if !strings.HasPrefix(line, "data: ") || !strings.Contains(line, "\"trace_id\"") {
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &payload); err == nil {
			if traceID, _ := payload["trace_id"].(string); traceID != "" {
				return traceID
			}
		}
	}
	return ""
}
