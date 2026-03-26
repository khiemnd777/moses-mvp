package retrieval

import (
	"strings"

	"github.com/khiemnd777/legal_api/core/answer"
)

const followUpContextLimit = 240

var followUpMarkers = []string{
	"cam on",
	"cảm ơn",
	"hoi them",
	"hỏi thêm",
	"them nua",
	"thêm nữa",
	"tiep theo",
	"tiếp theo",
	"truong hop nay",
	"trường hợp này",
	"van de nay",
	"vấn đề này",
	"viec nay",
	"việc này",
	"noi tren",
	"nói trên",
}

// BuildFollowUpSearchQuery expands a follow-up question with the latest user
// turn so retrieval has the conversational topic context without introducing
// an extra LLM rewrite pass.
func BuildFollowUpSearchQuery(history []answer.ConversationMessage, currentQuery string) string {
	current := strings.TrimSpace(currentQuery)
	if current == "" {
		return ""
	}
	if !shouldAugmentWithHistory(current) {
		return current
	}

	priorUser := lastUserMessage(history)
	if priorUser == "" {
		return current
	}

	priorUser = trimToChars(strings.TrimSpace(priorUser), followUpContextLimit)
	if priorUser == "" {
		return current
	}
	if normalizeQuery(priorUser) == normalizeQuery(current) {
		return current
	}

	return strings.TrimSpace(priorUser + " " + current)
}

func shouldAugmentWithHistory(currentQuery string) bool {
	normalized := normalizeQuery(currentQuery)
	if normalized == "" {
		return false
	}
	for _, marker := range followUpMarkers {
		if strings.Contains(normalized, normalizeQuery(marker)) {
			return true
		}
	}
	return len(strings.Fields(normalized)) <= 8
}

func lastUserMessage(history []answer.ConversationMessage) string {
	for i := len(history) - 1; i >= 0; i-- {
		if strings.TrimSpace(strings.ToLower(history[i].Role)) != "user" {
			continue
		}
		return strings.TrimSpace(history[i].Content)
	}
	return ""
}

func trimToChars(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return strings.TrimSpace(value[:limit-3]) + "..."
}
