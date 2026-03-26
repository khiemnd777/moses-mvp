package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/core/retrieval"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/observability"
	"github.com/khiemnd777/legal_api/pkg/logging"
)

type followupStore struct {
	*fakeStore
	messagesByConversation map[string][]domain.Message
}

func (s *followupStore) ListMessagesByConversation(ctx context.Context, conversationID string) ([]domain.Message, error) {
	if s.messagesByConversation == nil {
		return nil, nil
	}
	return append([]domain.Message{}, s.messagesByConversation[conversationID]...), nil
}

type queryCapturingRetriever struct {
	lastQuery string
	calls     int
	results   []retrieval.Result
}

func (r *queryCapturingRetriever) Search(ctx context.Context, query string, opts retrieval.SearchOptions) ([]retrieval.Result, error) {
	r.calls++
	r.lastQuery = query
	return append([]retrieval.Result{}, r.results...), nil
}

func TestPrepareChatResponseUsesConversationHistoryForRetrievalQuery(t *testing.T) {
	store := &followupStore{
		fakeStore: &fakeStore{},
		messagesByConversation: map[string][]domain.Message{
			"convo-1": {
				{Role: "user", Content: "Tu van cho toi ve quyen va nghia vu cua vo chong sau khi ly hon?"},
				{Role: "assistant", Content: "Noi ve quyen, nghia vu, tai san va con chung sau ly hon."},
			},
		},
	}
	retriever := &queryCapturingRetriever{
		results: []retrieval.Result{
			{
				ChunkID:    "chunk-1",
				Text:       "Dieu 95 ve ly hon",
				VersionID:  "version-1",
				ChunkIndex: 1,
				Score:      0.99,
				Metadata: map[string]interface{}{
					"document_title": "Luat Hon nhan va Gia dinh",
					"article":        "95",
					"document_type":  "law",
				},
			},
		},
	}
	handler := NewHandler(
		store,
		nil,
		nil,
		retriever,
		answer.NewClient("test-key", "gpt-test"),
		map[string]string{"default": "Tra loi bang tieng Viet."},
		nil,
		logging.New(),
		newMemoryTraceRepo(),
	)

	traceSvc, err := observability.NewTraceService(context.Background(), newMemoryTraceRepo(), logging.New(), "trace-1", "answer", "Cam on! Toi muon hoi them ve cac khoan chi phi hau ly hon?")
	if err != nil {
		t.Fatalf("NewTraceService() error = %v", err)
	}

	_, history, decision, _, _, _, err := handler.prepareChatResponse(
		context.Background(),
		"convo-1",
		"Cam on! Toi muon hoi them ve cac khoan chi phi hau ly hon?",
		ChatFilters{Tone: "default", TopK: 5},
		runtimeAnswerConfig{
			Policy: domain.AIGuardPolicy{
				Enabled:            true,
				MinRetrievedChunks: 1,
				MinSimilarityScore: 0,
				OnEmptyRetrieval:   "refuse",
				OnLowConfidence:    "refuse",
			},
			Tone: "Tra loi bang tieng Viet.",
		},
		traceSvc,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("prepareChatResponse() error = %v", err)
	}
	if !decision.Allow() {
		t.Fatalf("expected guard to allow high-confidence retrieval")
	}
	if len(history) != 2 {
		t.Fatalf("expected history to be loaded, got %d messages", len(history))
	}
	if !strings.Contains(retriever.lastQuery, "Tu van cho toi ve quyen va nghia vu cua vo chong sau khi ly hon?") {
		t.Fatalf("expected retrieval query to include prior user turn, got %q", retriever.lastQuery)
	}
	if !strings.Contains(retriever.lastQuery, "Cam on! Toi muon hoi them ve cac khoan chi phi hau ly hon?") {
		t.Fatalf("expected retrieval query to include current question, got %q", retriever.lastQuery)
	}
}

func TestPrepareChatResponseSkipsRetrievalForGreeting(t *testing.T) {
	store := &followupStore{fakeStore: &fakeStore{}}
	retriever := &queryCapturingRetriever{
		results: []retrieval.Result{
			{ChunkID: "chunk-1", Text: "unexpected", VersionID: "version-1", Score: 0.99},
		},
	}
	handler := NewHandler(
		store,
		nil,
		nil,
		retriever,
		answer.NewClient("test-key", "gpt-test"),
		map[string]string{"default": "Tra loi bang tieng Viet."},
		nil,
		logging.New(),
		newMemoryTraceRepo(),
	)

	traceSvc, err := observability.NewTraceService(context.Background(), newMemoryTraceRepo(), logging.New(), "trace-2", "answer", "Xin chao!")
	if err != nil {
		t.Fatalf("NewTraceService() error = %v", err)
	}

	results, history, decision, sources, _, _, err := handler.prepareChatResponse(
		context.Background(),
		"convo-2",
		"Xin chào!",
		ChatFilters{Tone: "default", TopK: 5},
		runtimeAnswerConfig{
			Policy: domain.AIGuardPolicy{
				Enabled:            true,
				MinRetrievedChunks: 1,
				MinSimilarityScore: 0,
				OnEmptyRetrieval:   "refuse",
				OnLowConfidence:    "refuse",
			},
			Tone: "Tra loi bang tieng Viet.",
		},
		traceSvc,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("prepareChatResponse() error = %v", err)
	}
	if decision.Allow() {
		t.Fatalf("expected greeting to bypass legal answer flow")
	}
	if decision.PromptType != smallTalkPromptType {
		t.Fatalf("expected prompt type %q, got %q", smallTalkPromptType, decision.PromptType)
	}
	if retriever.calls != 0 {
		t.Fatalf("expected retriever not to be called, got %d", retriever.calls)
	}
	if len(results) != 0 {
		t.Fatalf("expected no retrieval results, got %d", len(results))
	}
	if len(history) != 0 {
		t.Fatalf("expected no history load for greeting shortcut, got %d", len(history))
	}
	if len(sources) != 0 {
		t.Fatalf("expected no sources for greeting shortcut, got %d", len(sources))
	}
}

func TestRunChatTurnReturnsNoCitationsForGreeting(t *testing.T) {
	store := &followupStore{fakeStore: &fakeStore{}}
	retriever := &queryCapturingRetriever{}
	handler := NewHandler(
		store,
		nil,
		nil,
		retriever,
		answer.NewClient("test-key", "gpt-test"),
		map[string]string{"default": "Tra loi bang tieng Viet."},
		nil,
		logging.New(),
		newMemoryTraceRepo(),
	)

	result, err := handler.runChatTurn(context.Background(), nil, "", "Xin chào!", ChatFilters{Tone: "default", TopK: 5})
	if err != nil {
		t.Fatalf("runChatTurn() error = %v", err)
	}
	if retriever.calls != 0 {
		t.Fatalf("expected retriever not to be called, got %d", retriever.calls)
	}
	if got := string(result.AssistantMessage.CitationsJSON); got != "[]" {
		t.Fatalf("expected empty citations json, got %q", got)
	}
	if !strings.Contains(result.AssistantMessage.Content, "Chào bạn!") {
		t.Fatalf("expected greeting response, got %q", result.AssistantMessage.Content)
	}
}
