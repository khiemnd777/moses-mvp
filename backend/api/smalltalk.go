package api

import (
	"strings"

	"github.com/khiemnd777/legal_api/core/guard"
	"github.com/khiemnd777/legal_api/core/retrieval"
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
	legalSignalKeywords = []string{
		"luat", "phap luat", "phap ly", "quy dinh", "thu tuc", "ho so", "dieu", "khoan",
		"nghi dinh", "thong tu", "quyet dinh", "hop dong", "tranh chap", "toa an", "khoi kien",
		"ly hon", "ket hon", "hon nhan", "dat dai", "thua ke", "tai san", "cap duong",
		"bao hiem", "thue", "hinh su", "dan su", "lao dong", "hanh chinh",
	}
)

func detectSmallTalkDecision(content string) (guardDecision, bool) {
	normalized := retrieval.UnderstandQuery(content).NormalizedQuery
	if normalized == "" || containsLegalSignal(normalized) {
		return guardDecision{}, false
	}
	if _, ok := smallTalkExactPhrases[normalized]; ok {
		return smallTalkDecision(defaultGreetingReply), true
	}
	if isGreetingTokenSequence(normalized) {
		return smallTalkDecision(defaultGreetingReply), true
	}
	return guardDecision{}, false
}

func smallTalkDecision(message string) guardDecision {
	return guardDecision{
		Decision:   guard.Decision("SMALLTALK"),
		PromptType: smallTalkPromptType,
		Message:    message,
	}
}

func containsLegalSignal(normalized string) bool {
	for _, keyword := range legalSignalKeywords {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}
	return false
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
