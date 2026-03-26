package guard

import (
	"strings"

	"github.com/khiemnd777/legal_api/domain"
)

type Decision string

const (
	DecisionAllow            Decision = "ALLOW"
	DecisionRefuse           Decision = "REFUSE"
	DecisionAskClarification Decision = "ASK_CLARIFICATION"
	DecisionFallbackLLM      Decision = "FALLBACK_LLM"
)

type RetrievalResult struct {
	RetrievedChunks int
	MaxSimilarity   float64
}

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Decide(result RetrievalResult, policy domain.AIGuardPolicy) Decision {
	if result.RetrievedChunks == 0 {
		return decideByAction(policy.OnEmptyRetrieval)
	}

	if policy.MinRetrievedChunks > 0 && result.RetrievedChunks < policy.MinRetrievedChunks {
		return decideByAction(policy.OnLowConfidence)
	}

	if result.MaxSimilarity < policy.MinSimilarityScore {
		return decideByAction(policy.OnLowConfidence)
	}
	return DecisionAllow
}

func decideByAction(action string) Decision {
	switch strings.TrimSpace(strings.ToLower(action)) {
	case "ask_clarification":
		return DecisionAskClarification
	case "fallback_llm":
		return DecisionFallbackLLM
	case "refuse":
		return DecisionRefuse
	default:
		return DecisionAllow
	}
}
