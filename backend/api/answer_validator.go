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
	return h.validateGeneratedLegalAnswerWithMode(ctx, answerText, sources, true)
}

func (h *Handler) validateGeneratedLegalAnswerForStream(ctx context.Context, answerText string, sources []answer.Source) (string, []answer.Citation, bool, error) {
	return h.validateGeneratedLegalAnswerWithMode(ctx, answerText, sources, false)
}

func (h *Handler) validateGeneratedLegalAnswerWithMode(ctx context.Context, answerText string, sources []answer.Source, replaceOnFailure bool) (string, []answer.Citation, bool, error) {
	originalText := answerText
	trimmedText := strings.TrimSpace(answerText)
	if trimmedText == "" {
		if replaceOnFailure {
			refusal, err := h.validationRefusalMessage(ctx)
			return refusal, []answer.Citation{}, false, err
		}
		return originalText, []answer.Citation{}, false, nil
	}

	if h.isTerminalLegalResponse(ctx, trimmedText) {
		return originalText, []answer.Citation{}, true, nil
	}

	citations := validateCitations(citationsFromSources(sources))
	if !hasLegalAnswerStructure(trimmedText) {
		return h.validationFailureResponse(ctx, originalText, replaceOnFailure)
	}
	if !referencesExistInSources(trimmedText, sources) {
		return h.validationFailureResponse(ctx, originalText, replaceOnFailure)
	}
	return originalText, citations, true, nil
}

func (h *Handler) validationRefusalMessage(ctx context.Context) (string, error) {
	promptCfg, _, found, err := h.getRuntimePromptExact(ctx, legalRefusalPromptType)
	if err != nil {
		return defaultValidationRefusal, err
	}
	if !found {
		return defaultValidationRefusal, nil
	}
	return sanitizeGuardMessage(promptCfg.SystemPrompt, defaultValidationRefusal), nil
}

func (h *Handler) validationClarificationMessage(ctx context.Context) (string, error) {
	promptCfg, _, found, err := h.getRuntimePromptExact(ctx, legalClarificationPromptType)
	if err != nil {
		return defaultClarificationMessage, err
	}
	if !found {
		return defaultClarificationMessage, nil
	}
	return sanitizeGuardMessage(promptCfg.SystemPrompt, defaultClarificationMessage), nil
}

func (h *Handler) validationFailureResponse(ctx context.Context, answerText string, replaceOnFailure bool) (string, []answer.Citation, bool, error) {
	if !replaceOnFailure {
		return answerText, []answer.Citation{}, false, nil
	}
	refusal, err := h.validationRefusalMessage(ctx)
	return refusal, []answer.Citation{}, false, err
}

func (h *Handler) isTerminalLegalResponse(ctx context.Context, answerText string) bool {
	normalized := normalizeValidationText(answerText)
	if normalized == "" {
		return false
	}
	candidates := []string{
		defaultRefusalMessage,
		defaultClarificationMessage,
	}
	if refusal, err := h.validationRefusalMessage(ctx); err == nil {
		candidates = append(candidates, refusal)
	}
	if clarification, err := h.validationClarificationMessage(ctx); err == nil {
		candidates = append(candidates, clarification)
	}
	for _, candidate := range candidates {
		if normalizeValidationText(candidate) == normalized {
			return true
		}
	}
	return looksLikeLegalRefusal(normalized) || looksLikeLegalClarification(normalized)
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

func normalizeValidationText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func looksLikeLegalRefusal(normalized string) bool {
	refusalPhrases := []string{
		"không đủ căn cứ pháp lý",
		"khong du can cu phap ly",
		"không tìm thấy căn cứ pháp lý",
		"khong tim thay can cu phap ly",
		"không thể kết luận",
		"khong the ket luan",
		"chưa đủ dữ kiện pháp lý",
		"chua du du kien phap ly",
	}
	for _, phrase := range refusalPhrases {
		if strings.Contains(normalized, normalizeValidationText(phrase)) {
			return true
		}
	}
	return (strings.Contains(normalized, "không đủ") || strings.Contains(normalized, "khong du") ||
		strings.Contains(normalized, "chưa đủ") || strings.Contains(normalized, "chua du")) &&
		(strings.Contains(normalized, "căn cứ pháp lý") || strings.Contains(normalized, "can cu phap ly") ||
			strings.Contains(normalized, "dữ liệu truy xuất") || strings.Contains(normalized, "du lieu truy xuat") ||
			strings.Contains(normalized, "nguồn hiện có") || strings.Contains(normalized, "nguon hien co"))
}

func looksLikeLegalClarification(normalized string) bool {
	clarificationPhrases := []string{
		"vui lòng bổ sung",
		"vui long bo sung",
		"vui lòng cung cấp thêm",
		"vui long cung cap them",
		"cần thêm thông tin",
		"can them thong tin",
		"bổ sung tình huống",
		"bo sung tinh huong",
		"bổ sung dữ kiện",
		"bo sung du kien",
	}
	for _, phrase := range clarificationPhrases {
		if strings.Contains(normalized, normalizeValidationText(phrase)) {
			return true
		}
	}
	return false
}
