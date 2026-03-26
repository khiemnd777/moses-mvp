package guard

import (
	"testing"

	"github.com/khiemnd777/legal_api/domain"
)

func TestEngineDecideUsesMinRetrievedChunksBeforeSimilarity(t *testing.T) {
	engine := NewEngine()
	policy := domain.AIGuardPolicy{
		MinRetrievedChunks: 2,
		MinSimilarityScore: 0.9,
		OnLowConfidence:    "ask_clarification",
	}

	got := engine.Decide(RetrievalResult{
		RetrievedChunks: 1,
		MaxSimilarity:   0.95,
	}, policy)

	if got != DecisionAskClarification {
		t.Fatalf("expected ask clarification when retrieved chunks are below threshold, got %q", got)
	}
}

func TestEngineDecideSupportsFallbackLLM(t *testing.T) {
	engine := NewEngine()
	policy := domain.AIGuardPolicy{
		MinRetrievedChunks: 1,
		MinSimilarityScore: 0.8,
		OnLowConfidence:    "fallback_llm",
	}

	got := engine.Decide(RetrievalResult{
		RetrievedChunks: 1,
		MaxSimilarity:   0.2,
	}, policy)

	if got != DecisionFallbackLLM {
		t.Fatalf("expected fallback_llm decision, got %q", got)
	}
}

func TestEngineDecideFallsBackToAllowForUnknownAction(t *testing.T) {
	engine := NewEngine()
	policy := domain.AIGuardPolicy{
		MinRetrievedChunks: 1,
		MinSimilarityScore: 0.8,
		OnLowConfidence:    "something_else",
	}

	got := engine.Decide(RetrievalResult{
		RetrievedChunks: 1,
		MaxSimilarity:   0.2,
	}, policy)

	if got != DecisionAllow {
		t.Fatalf("expected allow for unknown action, got %q", got)
	}
}

