package api

import (
	"context"
	"regexp"
	"sort"
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

	citations := deriveSupportingCitations(trimmedText, sources)
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

func deriveSupportingCitations(answerText string, sources []answer.Source) []answer.Citation {
	if len(sources) == 0 {
		return []answer.Citation{}
	}

	normalizedAnswer := normalizeValidationText(answerText)
	if normalizedAnswer == "" || isNegativeFindingAnswer(normalizedAnswer) {
		return []answer.Citation{}
	}

	documentMentions := extractDocumentMentions(normalizedAnswer, sources)
	referencedArticles := extractArticleRefs(answerText)
	if len(referencedArticles) == 0 && len(documentMentions) == 0 {
		return []answer.Citation{}
	}

	candidates := citationsFromSources(sources)
	filtered := make([]answer.Citation, 0, len(candidates))
	for _, citation := range candidates {
		if citationSupportsAnswer(citation, referencedArticles, documentMentions) {
			filtered = append(filtered, citation)
		}
	}
	return validateCitations(filtered)
}

func extractDocumentMentions(normalizedAnswer string, sources []answer.Source) map[string]struct{} {
	out := map[string]struct{}{}
	for _, src := range sources {
		for _, candidate := range citationDocumentKeys(src.Citation) {
			if candidate == "" {
				continue
			}
			if strings.Contains(normalizedAnswer, candidate) {
				out[candidate] = struct{}{}
			}
		}
	}
	return out
}

func citationSupportsAnswer(citation answer.Citation, referencedArticles map[string]struct{}, documentMentions map[string]struct{}) bool {
	hasDocumentMentions := len(documentMentions) > 0
	hasArticleRefs := len(referencedArticles) > 0

	citationArticle := normalizeArticleNumber(citation.Article)
	articleMatch := !hasArticleRefs
	if hasArticleRefs {
		_, articleMatch = referencedArticles[citationArticle]
	}

	documentMatch := !hasDocumentMentions
	if hasDocumentMentions {
		for _, key := range citationDocumentKeys(citation) {
			if _, ok := documentMentions[key]; ok {
				documentMatch = true
				break
			}
		}
	}

	switch {
	case hasArticleRefs && hasDocumentMentions:
		return articleMatch && documentMatch
	case hasArticleRefs:
		return articleMatch
	case hasDocumentMentions:
		return documentMatch
	default:
		return false
	}
}

func citationDocumentKeys(c answer.Citation) []string {
	keys := make([]string, 0, 4)
	for _, candidate := range []string{c.LawName, c.DocumentTitle, c.DocumentNumber, c.DocumentType} {
		normalized := normalizeValidationText(candidate)
		if normalized != "" {
			keys = append(keys, normalized)
		}
	}
	sort.Strings(keys)
	out := keys[:0]
	seen := map[string]struct{}{}
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func isNegativeFindingAnswer(normalizedAnswer string) bool {
	phrases := []string{
		"không có quy định cụ thể",
		"khong co quy dinh cu the",
		"không có quy định nào",
		"khong co quy dinh nao",
		"không đề cập trực tiếp",
		"khong de cap truc tiep",
		"không đề cập gián tiếp",
		"khong de cap gian tiep",
		"không tìm thấy căn cứ",
		"khong tim thay can cu",
		"không thể xác định cụ thể",
		"khong the xac dinh cu the",
		"không thể kết luận",
		"khong the ket luan",
		"chưa đủ căn cứ pháp lý",
		"chua du can cu phap ly",
	}
	for _, phrase := range phrases {
		if strings.Contains(normalizedAnswer, normalizeValidationText(phrase)) {
			return true
		}
	}
	return false
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
