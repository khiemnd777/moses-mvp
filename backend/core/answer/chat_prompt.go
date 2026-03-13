package answer

import (
	"strconv"
	"strings"
)

type ConversationMessage struct {
	Role    string
	Content string
}

type PromptBuildOptions struct {
	MaxInputTokens   int
	ReservedTokens   int
	MaxHistoryTurns  int
	MaxSourceChars   int
	MaxQuestionChars int
}

func (s *Service) BuildConversationMessages(history []ConversationMessage, question string, sources []Source, opts PromptBuildOptions) []message {
	systemParts := []string{strings.TrimSpace(s.SystemPrompt), strings.TrimSpace(s.Tone)}
	systemPrompt := strings.TrimSpace(strings.Join(systemParts, "\n\n"))
	if systemPrompt == "" {
		systemPrompt = "You are a Vietnamese legal assistant."
	}

	maxInputTokens := opts.MaxInputTokens
	if maxInputTokens <= 0 {
		maxInputTokens = 6000
	}
	reservedTokens := opts.ReservedTokens
	if reservedTokens <= 0 {
		reservedTokens = max(1200, s.MaxTokens)
	}
	inputBudgetChars := estimateCharBudget(maxInputTokens - reservedTokens)
	if inputBudgetChars <= 0 {
		inputBudgetChars = 12000
	}

	historyLimit := opts.MaxHistoryTurns
	if historyLimit <= 0 {
		historyLimit = 12
	}

	question = trimToChars(strings.TrimSpace(question), opts.MaxQuestionChars)
	contextBlock := buildContextBlock(sources, opts.MaxSourceChars)
	currentPrompt := strings.TrimSpace(strings.Join([]string{
		"Retrieved Legal Context:\n" + contextBlock,
		"Current User Question:\n" + question,
		"Instructions:\nAnswer in Vietnamese. Use only the retrieved legal context for legal claims. If context is missing, say so clearly. Cite the legal basis in natural language and keep the response structured.",
	}, "\n\n"))

	msgs := []message{{Role: "system", Content: systemPrompt}}
	usedChars := len(systemPrompt) + len(currentPrompt)

	selectedHistory := make([]ConversationMessage, 0, min(historyLimit, len(history)))
	for i := len(history) - 1; i >= 0 && len(selectedHistory) < historyLimit; i-- {
		item := history[i]
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		content = trimToChars(content, 1500)
		projected := usedChars + len(content)
		if projected > inputBudgetChars && len(selectedHistory) > 0 {
			break
		}
		selectedHistory = append(selectedHistory, ConversationMessage{Role: item.Role, Content: content})
		usedChars = projected
	}

	for i := len(selectedHistory) - 1; i >= 0; i-- {
		msgs = append(msgs, message{
			Role:    normalizeRole(selectedHistory[i].Role),
			Content: selectedHistory[i].Content,
		})
	}
	msgs = append(msgs, message{Role: "user", Content: currentPrompt})
	return msgs
}

func (s *Service) PromptSnapshotWithHistory(history []ConversationMessage, question string, sources []Source, opts PromptBuildOptions) string {
	msgs := s.BuildConversationMessages(history, question, sources, opts)
	parts := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		parts = append(parts, strings.ToUpper(msg.Role)+":\n"+strings.TrimSpace(msg.Content))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func buildContextBlock(sources []Source, maxChars int) string {
	if maxChars <= 0 {
		maxChars = 9000
	}
	var b strings.Builder
	used := 0
	for idx, src := range sources {
		block := formatSourceBlock(idx+1, src)
		if block == "" {
			continue
		}
		if used+len(block) > maxChars && used > 0 {
			break
		}
		b.WriteString(block)
		b.WriteString("\n\n")
		used += len(block) + 2
	}
	return strings.TrimSpace(b.String())
}

func formatSourceBlock(index int, src Source) string {
	var b strings.Builder
	b.WriteString("[Source ")
	b.WriteString(strconv.Itoa(index))
	b.WriteString("]\n")
	if src.Citation.DocumentTitle != "" {
		b.WriteString("Document Title: ")
		b.WriteString(src.Citation.DocumentTitle)
		b.WriteString("\n")
	}
	if src.Citation.LawName != "" {
		b.WriteString("Law Name: ")
		b.WriteString(src.Citation.LawName)
		b.WriteString("\n")
	}
	if src.Citation.Chapter != "" {
		b.WriteString("Chapter: ")
		b.WriteString(src.Citation.Chapter)
		b.WriteString("\n")
	}
	if src.Citation.Article != "" {
		b.WriteString("Article: ")
		b.WriteString(src.Citation.Article)
		b.WriteString("\n")
	}
	if src.Citation.Clause != "" {
		b.WriteString("Clause: ")
		b.WriteString(src.Citation.Clause)
		b.WriteString("\n")
	}
	if src.Citation.ChunkID != "" {
		b.WriteString("Chunk ID: ")
		b.WriteString(src.Citation.ChunkID)
		b.WriteString("\n")
	}
	if src.Citation.CitationLabel != "" {
		b.WriteString("Citation Label: ")
		b.WriteString(src.Citation.CitationLabel)
		b.WriteString("\n")
	}
	b.WriteString("Excerpt:\n")
	b.WriteString(strings.TrimSpace(src.Text))
	return strings.TrimSpace(b.String())
}

func estimateCharBudget(tokens int) int {
	if tokens <= 0 {
		return 0
	}
	return tokens * 4
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

func normalizeRole(role string) string {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "assistant", "system":
		return strings.TrimSpace(strings.ToLower(role))
	default:
		return "user"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
