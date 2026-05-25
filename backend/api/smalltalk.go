package api

import (
	"context"
	"strings"

	"github.com/khiemnd777/legal_api/core/guard"
)

const (
	smallTalkPromptType  = "smalltalk"
	defaultGreetingReply = "Chào bạn, mình có thể giúp tra cứu văn bản, tóm tắt căn cứ và phân tích tình huống pháp lý Việt Nam dựa trên tài liệu đã ingest. Bạn cứ mô tả vụ việc hoặc hỏi văn bản cần kiểm tra."
)

var (
	smallTalkExactPhrases = map[string]struct{}{
		"xin chao":                {},
		"chao":                    {},
		"chao ban":                {},
		"hello":                   {},
		"hi":                      {},
		"hey":                     {},
		"alo":                     {},
		"help":                    {},
		"giup toi voi":            {},
		"ban lam duoc gi":         {},
		"ban co the lam gi":       {},
		"ban giup duoc gi":        {},
		"huong dan su dung":       {},
		"bat dau nhu the nao":     {},
		"toi nen bat dau tu dau":  {},
		"toi can tu van phap ly":  {},
		"toi muon tu van phap ly": {},
		"can tu van phap ly":      {},
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
	if _, ok := smallTalkExactPhrases[normalized]; ok {
		return h.smallTalkDecision(ctx), true, normalized
	}
	if len(analysis.MatchedDocTypes) > 0 || analysis.LegalDomain != "" || analysis.Intent != "" && analysis.Intent != "legal_basis_lookup" || h.Retriever.HasLegalSignal(ctx, content) {
		return guardDecision{}, false, normalized
	}
	if isGreetingTokenSequence(normalized) {
		return h.smallTalkDecision(ctx), true, normalized
	}
	return guardDecision{}, false, normalized
}

func (h *Handler) smallTalkDecision(ctx context.Context) guardDecision {
	message := defaultGreetingReply
	if h != nil {
		if promptCfg, _, found, err := h.getRuntimePromptExact(ctx, smallTalkPromptType); err == nil && found {
			message = sanitizeGuardMessage(promptCfg.SystemPrompt, defaultGreetingReply)
		}
	}
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
