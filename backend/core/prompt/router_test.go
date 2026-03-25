package prompt

import (
	"context"
	"testing"

	"github.com/khiemnd777/legal_api/domain"
)

type routerStoreStub struct {
	items []domain.AIPrompt
}

func (s routerStoreStub) ListEnabledAIPrompts(ctx context.Context) ([]domain.AIPrompt, error) {
	return s.items, nil
}

func TestRouterGetPromptExactDoesNotFallbackToDefault(t *testing.T) {
	router := NewRouter(routerStoreStub{
		items: []domain.AIPrompt{
			{PromptType: "legal_guard", SystemPrompt: "guard"},
			{PromptType: "legal_qa", SystemPrompt: "qa"},
		},
	}, 0, DefaultPromptType)

	_, found, err := router.GetPromptExact(context.Background(), "legal_refusal")
	if err != nil {
		t.Fatalf("GetPromptExact() error = %v", err)
	}
	if found {
		t.Fatalf("expected missing exact prompt, got found=true")
	}
}
