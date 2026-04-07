package retrieval

import "github.com/khiemnd777/legal_api/core/answer"

const followUpContextLimit = 240

// BuildFollowUpSearchQuery expands a follow-up question with the latest user
// turn so retrieval has the conversational topic context without introducing
// an extra LLM rewrite pass.
func BuildFollowUpSearchQuery(history []answer.ConversationMessage, currentQuery string) string {
	return buildFollowUpSearchQueryWithIndex(queryUnderstandingIndex{Profiles: map[string]docTypeQueryProfile{}}, history, currentQuery)
}
