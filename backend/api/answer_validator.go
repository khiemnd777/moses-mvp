package api

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/khiemnd777/legal_api/core/answer"
)

var (
	reArticleRef = regexp.MustCompile(`(?i)(?:điều|dieu)\s+([0-9]+[a-z]?)`)
)

func (h *Handler) validateGeneratedLegalAnswer(ctx context.Context, answerText string, sources []answer.Source) (string, []answer.Citation, bool, error) {
	citations := validateCitations(citationsFromSources(sources))
	if !hasLegalAnswerStructure(answerText) {
		refusal, err := h.validationRefusalMessage(ctx)
		return refusal, []answer.Citation{}, false, err
	}
	if !referencesExistInSources(answerText, sources) {
		refusal, err := h.validationRefusalMessage(ctx)
		return refusal, []answer.Citation{}, false, err
	}
	return answerText, citations, true, nil
}

func (h *Handler) validationRefusalMessage(ctx context.Context) (string, error) {
	promptCfg, _, err := h.getRuntimePrompt(ctx, legalRefusalPromptType)
	if err != nil {
		return defaultValidationRefusal, err
	}
	msg := strings.TrimSpace(promptCfg.SystemPrompt)
	if msg == "" {
		return defaultValidationRefusal, nil
	}
	return msg, nil
}

func hasLegalAnswerStructure(value string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return false
	}
	return strings.Contains(trimmed, "1.") &&
		strings.Contains(trimmed, "2.") &&
		strings.Contains(trimmed, "3.") &&
		strings.Contains(trimmed, "4.")
}

func referencesExistInSources(answerText string, sources []answer.Source) bool {
	referencedArticles := extractArticleRefs(answerText)
	if len(referencedArticles) == 0 {
		return true
	}
	allowed := collectAllowedArticles(sources)
	for article := range referencedArticles {
		if _, ok := allowed[article]; !ok {
			return false
		}
	}
	return true
}

func collectAllowedArticles(sources []answer.Source) map[string]struct{} {
	out := map[string]struct{}{}
	for _, src := range sources {
		article := normalizeArticleNumber(src.Citation.Article)
		if article != "" {
			out[article] = struct{}{}
		}
		for articleFromText := range extractArticleRefs(src.Text) {
			out[articleFromText] = struct{}{}
		}
	}
	return out
}

func extractArticleRefs(value string) map[string]struct{} {
	out := map[string]struct{}{}
	matches := reArticleRef.FindAllStringSubmatch(value, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		normalized := normalizeArticleNumber(match[1])
		if normalized == "" {
			continue
		}
		out[normalized] = struct{}{}
	}
	return out
}

func normalizeArticleNumber(value string) string {
	v := strings.TrimSpace(strings.ToLower(value))
	if v == "" {
		return ""
	}
	if i, err := strconv.Atoi(v); err == nil {
		return strconv.Itoa(i)
	}
	return v
}
