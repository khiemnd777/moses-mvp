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
		if strings.EqualFold(strings.TrimSpace(policy.OnEmptyRetrieval), "refuse") {
			return DecisionRefuse
		}
	}
	if result.MaxSimilarity < policy.MinSimilarityScore {
		switch strings.TrimSpace(strings.ToLower(policy.OnLowConfidence)) {
		case "ask_clarification":
			return DecisionAskClarification
		case "refuse":
			return DecisionRefuse
		}
	}
	return DecisionAllow
}
