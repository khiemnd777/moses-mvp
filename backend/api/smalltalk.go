package api

import (
	"context"
	"strings"

	"github.com/khiemnd777/legal_api/core/guard"
)

const (
	smallTalkPromptType  = "smalltalk"
	defaultGreetingReply = "Chào bạn! Nếu bạn có câu hỏi pháp lý cụ thể, vui lòng gửi tình huống hoặc điều khoản cần tra cứu."
)

var (
	smallTalkExactPhrases = map[string]struct{}{
		"xin chao": {},
		"chao":     {},
		"chao ban": {},
		"hello":    {},
		"hi":       {},
		"hey":      {},
		"alo":      {},
	}
	smallTalkAllowedTokens = map[string]struct{}{
		"xin": {}, "chao": {}, "ban": {}, "anh": {}, "chi": {}, "em": {}, "ad": {}, "admin": {},
		"hello": {}, "hi": {}, "hey": {}, "alo": {}, "a": {}, "oi": {}, "nhe": {}, "nha": {},
	}
)

func (h *Handler) detectSmallTalkDecision(ctx context.Context, content string) (guardDecision, bool, string) {
	analysis := h.Retriever.AnalyzeQuery(ctx, content)
	normalized := analysis.NormalizedQuery
	if normalized == "" {
		return guardDecision{}, false, normalized
	}
	if len(analysis.MatchedDocTypes) > 0 || analysis.LegalDomain != "" || analysis.Intent != "" && analysis.Intent != "legal_basis_lookup" || h.Retriever.HasLegalSignal(ctx, content) {
		return guardDecision{}, false, normalized
	}
	if _, ok := smallTalkExactPhrases[normalized]; ok {
		return smallTalkDecision(defaultGreetingReply), true, normalized
	}
	if isGreetingTokenSequence(normalized) {
		return smallTalkDecision(defaultGreetingReply), true, normalized
	}
	return guardDecision{}, false, normalized
}

func smallTalkDecision(message string) guardDecision {
	return guardDecision{
		Decision:   guard.Decision("SMALLTALK"),
		PromptType: smallTalkPromptType,
		Message:    message,
	}
}

func isGreetingTokenSequence(normalized string) bool {
	tokens := strings.Fields(normalized)
	if len(tokens) == 0 || len(tokens) > 5 {
		return false
	}
	hasGreetingWord := false
	for _, token := range tokens {
		if _, ok := smallTalkAllowedTokens[token]; !ok {
			return false
		}
		switch token {
		case "chao", "hello", "hi", "hey", "alo":
			hasGreetingWord = true
		}
	}
	return hasGreetingWord
}
