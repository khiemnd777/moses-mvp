package api

import (
	"context"
	"encoding/json"
	"fmt"
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

var fakeStoreClock = time.Now().UTC()

type fakeStore struct {
	mu              sync.Mutex
	conversationSeq int
	messageSeq      int
	createdMessages []domain.Message
	updatedMessages []domain.Message
}

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
func (f *fakeStore) GetChunksByIDs(ctx context.Context, ids []string) ([]domain.Chunk, error) {
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
	f.mu.Lock()
	defer f.mu.Unlock()
	f.conversationSeq++
	now := fakeStoreClock.Add(time.Duration(f.conversationSeq) * time.Second)
	return domain.Conversation{
		ID:        fmt.Sprintf("conversation-%d", f.conversationSeq),
		Title:     title,
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
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
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messageSeq++
	now := fakeStoreClock.Add(time.Duration(f.messageSeq) * time.Second)
	msg := domain.Message{
		ID:             fmt.Sprintf("message-%d", f.messageSeq),
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		CitationsJSON:  citationsJSON,
		TraceID:        traceID,
		CreatedAt:      now,
	}
	f.createdMessages = append(f.createdMessages, msg)
	return msg, nil
}
func (f *fakeStore) UpdateMessage(ctx context.Context, id, content string, citationsJSON []byte, traceID *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updatedMessages = append(f.updatedMessages, domain.Message{
		ID:            id,
		Content:       content,
		CitationsJSON: citationsJSON,
		TraceID:       traceID,
	})
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
			SystemPrompt: "Không đủ căn cứ pháp lý trong dữ liệu truy xuất để đưa ra kết luận.",
			Temperature:  0.2,
			MaxTokens:    64,
			Retry:        0,
			Enabled:      true,
		},
		{
			Name:         "legal_clarification_prompt",
			PromptType:   "legal_clarification",
			SystemPrompt: "Chưa đủ căn cứ pháp lý rõ ràng. Vui lòng bổ sung tình huống, văn bản, hoặc điều khoản cần tra cứu.",
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
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"1. Vấn đề pháp lý\nXác định thủ tục ly hôn.\n\n2. Áp dụng pháp luật\nĐiều 1.\n\n3. Phân tích pháp lý\nNội dung phù hợp với nguồn truy xuất.\n\n4. Kết luận\nCó thể tiếp tục theo quy định tại Điều 1."}}]}`))
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

func TestValidateGeneratedLegalAnswerPreservesTerminalMessages(t *testing.T) {
	handler := newTestHandler("http://example.invalid", newMemoryTraceRepo())

	refusal := "Không đủ căn cứ pháp lý trong dữ liệu truy xuất để đưa ra kết luận."
	got, citations, valid, err := handler.validateGeneratedLegalAnswer(context.Background(), refusal, nil)
	if err != nil {
		t.Fatalf("validate refusal: %v", err)
	}
	if !valid {
		t.Fatalf("expected refusal to be treated as valid")
	}
	if got != refusal {
		t.Fatalf("expected refusal to be preserved, got %q", got)
	}
	if len(citations) != 0 {
		t.Fatalf("expected no citations for refusal, got %d", len(citations))
	}

	clarification := "Chưa đủ căn cứ pháp lý rõ ràng. Vui lòng bổ sung tình huống, văn bản, hoặc điều khoản cần tra cứu."
	got, citations, valid, err = handler.validateGeneratedLegalAnswer(context.Background(), clarification, nil)
	if err != nil {
		t.Fatalf("validate clarification: %v", err)
	}
	if !valid {
		t.Fatalf("expected clarification to be treated as valid")
	}
	if got != clarification {
		t.Fatalf("expected clarification to be preserved, got %q", got)
	}
	if len(citations) != 0 {
		t.Fatalf("expected no citations for clarification, got %d", len(citations))
	}
}

func TestValidateGeneratedLegalAnswerPreservesParaphrasedTerminalMessages(t *testing.T) {
	handler := newTestHandler("http://example.invalid", newMemoryTraceRepo())

	refusal := "Nguồn hiện có chưa đủ căn cứ pháp lý để kết luận chắc chắn về vấn đề này."
	got, citations, valid, err := handler.validateGeneratedLegalAnswer(context.Background(), refusal, nil)
	if err != nil {
		t.Fatalf("validate paraphrased refusal: %v", err)
	}
	if !valid {
		t.Fatalf("expected paraphrased refusal to be treated as valid")
	}
	if got != refusal {
		t.Fatalf("expected paraphrased refusal to be preserved, got %q", got)
	}
	if len(citations) != 0 {
		t.Fatalf("expected no citations for paraphrased refusal, got %d", len(citations))
	}

	clarification := "Vui lòng bổ sung thêm tình huống cụ thể hoặc điều khoản cần tra cứu để tôi đối chiếu đúng nguồn."
	got, citations, valid, err = handler.validateGeneratedLegalAnswer(context.Background(), clarification, nil)
	if err != nil {
		t.Fatalf("validate paraphrased clarification: %v", err)
	}
	if !valid {
		t.Fatalf("expected paraphrased clarification to be treated as valid")
	}
	if got != clarification {
		t.Fatalf("expected paraphrased clarification to be preserved, got %q", got)
	}
	if len(citations) != 0 {
		t.Fatalf("expected no citations for paraphrased clarification, got %d", len(citations))
	}
}

func TestValidateGeneratedLegalAnswerReturnsOnlySupportingCitations(t *testing.T) {
	handler := newTestHandler("http://example.invalid", newMemoryTraceRepo())
	sources := []answer.Source{
		{
			Text: "Điều 54. Hòa giải tại Tòa án.",
			Citation: answer.Citation{
				ID:            "citation-54",
				ChunkID:       "chunk-54",
				DocumentTitle: "Luật Hôn nhân và gia đình 2014",
				LawName:       "Luật Hôn nhân và gia đình 2014",
				DocumentType:  "LUẬT",
				Article:       "54",
			},
		},
		{
			Text: "Điều 95. Điều kiện mang thai hộ vì mục đích nhân đạo.",
			Citation: answer.Citation{
				ID:            "citation-95",
				ChunkID:       "chunk-95",
				DocumentTitle: "Luật Hôn nhân và gia đình 2014",
				LawName:       "Luật Hôn nhân và gia đình 2014",
				DocumentType:  "LUẬT",
				Article:       "95",
			},
		},
		{
			Text: "Điều 6. Giá trị pháp lý của Giấy khai sinh.",
			Citation: answer.Citation{
				ID:             "citation-6",
				ChunkID:        "chunk-6",
				DocumentTitle:  "Nghị định số 123/2015/NĐ-CP ngày 15 tháng 11 năm 2015 quy định chi tiết một số điều và biện pháp thi hành Luật Hộ tịch",
				LawName:        "Nghị định số 123/2015/NĐ-CP ngày 15 tháng 11 năm 2015 quy định chi tiết một số điều và biện pháp thi hành Luật Hộ tịch",
				DocumentNumber: "123/2015/NĐ-CP",
				DocumentType:   "NGHỊ ĐỊNH",
				Article:        "6",
			},
		},
	}

	answerText := "1. Vấn đề pháp lý\nXác định quyền, nghĩa vụ sau ly hôn.\n\n2. Áp dụng pháp luật\nLuật Hôn nhân và gia đình 2014: Điều 54.\n\n3. Phân tích pháp lý\nĐiều 54 Luật Hôn nhân và gia đình 2014 yêu cầu hòa giải tại Tòa án.\n\n4. Kết luận\nÁp dụng Điều 54 Luật Hôn nhân và gia đình 2014."
	got, citations, valid, err := handler.validateGeneratedLegalAnswer(context.Background(), answerText, sources)
	if err != nil {
		t.Fatalf("validate legal answer: %v", err)
	}
	if !valid {
		t.Fatalf("expected answer to be valid")
	}
	if got != answerText {
		t.Fatalf("expected answer to be preserved, got %q", got)
	}
	if len(citations) != 1 {
		t.Fatalf("expected exactly 1 supporting citation, got %d", len(citations))
	}
	if citations[0].Article != "54" {
		t.Fatalf("expected citation article 54, got %q", citations[0].Article)
	}
}

func TestValidateGeneratedLegalAnswerSuppressesCitationsForNegativeFinding(t *testing.T) {
	handler := newTestHandler("http://example.invalid", newMemoryTraceRepo())
	sources := []answer.Source{
		{
			Text: "Điều 95. Điều kiện mang thai hộ vì mục đích nhân đạo.",
			Citation: answer.Citation{
				ID:            "citation-95",
				ChunkID:       "chunk-95",
				DocumentTitle: "Luật Hôn nhân và gia đình 2014",
				LawName:       "Luật Hôn nhân và gia đình 2014",
				DocumentType:  "LUẬT",
				Article:       "95",
			},
		},
		{
			Text: "Điều 6. Giá trị pháp lý của Giấy khai sinh.",
			Citation: answer.Citation{
				ID:             "citation-6",
				ChunkID:        "chunk-6",
				DocumentTitle:  "Nghị định số 123/2015/NĐ-CP ngày 15 tháng 11 năm 2015 quy định chi tiết một số điều và biện pháp thi hành Luật Hộ tịch",
				LawName:        "Nghị định số 123/2015/NĐ-CP ngày 15 tháng 11 năm 2015 quy định chi tiết một số điều và biện pháp thi hành Luật Hộ tịch",
				DocumentNumber: "123/2015/NĐ-CP",
				DocumentType:   "NGHỊ ĐỊNH",
				Article:        "6",
			},
		},
	}

	answerText := "1. Vấn đề pháp lý\nTư vấn về các khoản chi phí phát sinh sau khi ly hôn.\n\n2. Pháp luật áp dụng\nCác văn bản pháp luật được cung cấp không có quy định cụ thể về các khoản chi phí phát sinh hậu ly hôn.\n\n3. Phân tích pháp lý\nTrong các tài liệu pháp lý được trích dẫn, không có quy định nào đề cập trực tiếp hoặc gián tiếp về các khoản chi phí phát sinh sau khi ly hôn.\n\n4. Kết luận\nPháp luật hiện hành trong các tài liệu được cung cấp không quy định cụ thể về các khoản chi phí phát sinh sau khi ly hôn."
	got, citations, valid, err := handler.validateGeneratedLegalAnswer(context.Background(), answerText, sources)
	if err != nil {
		t.Fatalf("validate negative finding answer: %v", err)
	}
	if !valid {
		t.Fatalf("expected negative finding answer to stay valid")
	}
	if got != answerText {
		t.Fatalf("expected answer to be preserved, got %q", got)
	}
	if len(citations) != 0 {
		t.Fatalf("expected no citations for negative finding answer, got %d", len(citations))
	}
}

func TestStreamMessagePersistsStreamedContentOnValidationFailure(t *testing.T) {
	openAIServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Day la cau tra loi khong dung format.\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer openAIServer.Close()

	store := &fakeStore{}
	traceRepo := newMemoryTraceRepo()
	client := answer.NewClient("test-key", "gpt-test")
	client.BaseURL = openAIServer.URL
	handler := NewHandler(
		store,
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
		traceRepo,
	)

	app := fiber.New()
	app.Post("/messages/stream", answerTraceMiddleware(logging.New()), handler.StreamMessage)

	req := httptest.NewRequest(http.MethodPost, "/messages/stream", strings.NewReader(`{"content":"thu tuc ly hon","filters":{"tone":"default","topK":5,"effectiveStatus":"active","domain":"marriage_family","docType":"law"}}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, int(5*time.Second/time.Millisecond))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	raw := string(body)

	if !strings.Contains(raw, "Day la cau tra loi khong dung format.") {
		t.Fatalf("expected streamed content in response, got %s", raw)
	}
	if strings.Contains(raw, "Không đủ căn cứ pháp lý trong dữ liệu truy xuất để đưa ra kết luận.") {
		t.Fatalf("did not expect refusal fallback in streamed response, got %s", raw)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.updatedMessages) == 0 {
		t.Fatalf("expected message update to be recorded")
	}
	if got := store.updatedMessages[len(store.updatedMessages)-1].Content; got != "Day la cau tra loi khong dung format." {
		t.Fatalf("expected persisted content to match streamed content, got %q", got)
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
