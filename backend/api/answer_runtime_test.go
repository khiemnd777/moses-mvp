package api

import (
	"context"
	"testing"

	"github.com/khiemnd777/legal_api/core/guard"
	"github.com/khiemnd777/legal_api/core/retrieval"
	cprompt "github.com/khiemnd777/legal_api/core/prompt"
	"github.com/khiemnd777/legal_api/domain"
)

type promptStoreStub struct {
	items []domain.AIPrompt
}

func (s promptStoreStub) ListEnabledAIPrompts(ctx context.Context) ([]domain.AIPrompt, error) {
	return s.items, nil
}

func TestEvaluateGuardPolicyDoesNotLeakGuardSystemPromptWhenRefusalPromptMissing(t *testing.T) {
	handler := &Handler{
		PromptRouter: cprompt.NewRouter(promptStoreStub{
			items: []domain.AIPrompt{
				{
					Name:         "legal_guard_prompt",
					PromptType:   "legal_guard",
					SystemPrompt: "You are a Vietnamese legal assistant.\nUse ONLY the provided legal sources.",
					Enabled:      true,
				},
			},
		}, 0, cprompt.DefaultPromptType),
		GuardEngine: guard.NewEngine(),
	}

	decision, _, err := handler.evaluateGuardPolicy(context.Background(), domain.AIGuardPolicy{
		OnEmptyRetrieval: "refuse",
	}, nil)
	if err != nil {
		t.Fatalf("evaluateGuardPolicy() error = %v", err)
	}
	if decision.Message != defaultRefusalMessage {
		t.Fatalf("expected default refusal message, got %q", decision.Message)
	}
}

func TestEvaluateGuardPolicyTreatsFallbackLLMAsGenerationPath(t *testing.T) {
	handler := &Handler{
		PromptRouter: cprompt.NewRouter(promptStoreStub{
			items: []domain.AIPrompt{
				{
					Name:         "legal_answer_prompt",
					PromptType:   legalAnswerPromptType,
					SystemPrompt: "answer prompt",
					Enabled:      true,
				},
			},
		}, 0, cprompt.DefaultPromptType),
		GuardEngine: guard.NewEngine(),
	}

	decision, _, err := handler.evaluateGuardPolicy(context.Background(), domain.AIGuardPolicy{
		MinRetrievedChunks: 2,
		MinSimilarityScore: 0.9,
		OnLowConfidence:    "fallback_llm",
	}, []retrieval.Result{{
		ChunkID: "chunk-1",
		Score:   0.95,
	}})
	if err != nil {
		t.Fatalf("evaluateGuardPolicy() error = %v", err)
	}
	if decision.Decision != guard.DecisionFallbackLLM {
		t.Fatalf("expected fallback_llm decision, got %q", decision.Decision)
	}
	if !decision.Allow() {
		t.Fatalf("fallback_llm should continue through generation path")
	}
	if decision.PromptType != legalAnswerPromptType {
		t.Fatalf("expected answer prompt type, got %q", decision.PromptType)
	}
}

func TestGetAnswerPromptFallsBackToLegacyLegalQAPrompt(t *testing.T) {
	handler := &Handler{
		PromptRouter: cprompt.NewRouter(promptStoreStub{
			items: []domain.AIPrompt{
				{
					Name:         "legal_qa_prompt",
					PromptType:   legacyLegalQAPromptType,
					SystemPrompt: "qa prompt",
					Enabled:      true,
				},
			},
		}, 0, cprompt.DefaultPromptType),
	}

	promptCfg, usedType, err := handler.getAnswerPrompt(context.Background())
	if err != nil {
		t.Fatalf("getAnswerPrompt() error = %v", err)
	}
	if usedType != legacyLegalQAPromptType {
		t.Fatalf("expected usedType=%q, got %q", legacyLegalQAPromptType, usedType)
	}
	if promptCfg.SystemPrompt != "qa prompt" {
		t.Fatalf("expected qa prompt, got %q", promptCfg.SystemPrompt)
	}
}
