package api

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/core/guard"
	"github.com/khiemnd777/legal_api/core/retrieval"
	"github.com/khiemnd777/legal_api/domain"
)

const (
	legalAnswerPromptType        = "legal_answer"
	legalRefusalPromptType       = "legal_refusal"
	legalClarificationPromptType = "legal_clarification"
	legacyLegalQAPromptType      = "legal_qa"
)

const (
	defaultRefusalMessage       = "Không đủ căn cứ pháp lý trong dữ liệu truy xuất để đưa ra kết luận."
	defaultClarificationMessage = "Chưa đủ căn cứ pháp lý rõ ràng. Vui lòng bổ sung tình huống, văn bản, hoặc điều khoản cần tra cứu."
	defaultValidationRefusal    = "Câu trả lời không hợp lệ vì có trích dẫn vượt ngoài nguồn truy xuất. Vui lòng cung cấp thêm dữ kiện pháp lý."
)

type runtimeAnswerConfig struct {
	Policy domain.AIGuardPolicy
	Tone   string
}

type guardDecision struct {
	Decision   guard.Decision
	PromptType string
	Message    string
}

func (d guardDecision) Allow() bool {
	return d.Decision == guard.DecisionAllow
}

type retrievalDiagnostics struct {
	RetrievedChunks int
	MaxSimilarity   float64
}

func (h *Handler) loadRuntimeAnswerConfig(ctx context.Context, toneKey string) (runtimeAnswerConfig, error) {
	h.runtimeCfgMu.RLock()
	if h.runtimeCfgReady && time.Since(h.runtimeCfgLoadedAt) <= h.runtimeCfgTTL {
		cached := h.runtimeCfg
		h.runtimeCfgMu.RUnlock()
		if v, ok := h.Tones[toneKey]; ok {
			cached.Tone = v
		} else {
			cached.Tone = h.Tones[defaultToneKey]
		}
		return cached, nil
	}
	h.runtimeCfgMu.RUnlock()

	guardCfg, err := h.Store.GetActiveAIGuardPolicy(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return runtimeAnswerConfig{}, fmt.Errorf("no active ai guard policy")
		}
		return runtimeAnswerConfig{}, err
	}

	tone := h.Tones[defaultToneKey]
	if v, ok := h.Tones[toneKey]; ok {
		tone = v
	}

	out := runtimeAnswerConfig{
		Policy: guardCfg,
		Tone:   tone,
	}
	h.runtimeCfgMu.Lock()
	h.runtimeCfg = out
	h.runtimeCfgReady = true
	h.runtimeCfgLoadedAt = time.Now()
	h.runtimeCfgMu.Unlock()
	return out, nil
}

func (h *Handler) InvalidateRuntimeAnswerConfigCache() {
	h.runtimeCfgMu.Lock()
	h.runtimeCfgReady = false
	h.runtimeCfgLoadedAt = time.Time{}
	h.runtimeCfgMu.Unlock()
	if h.PromptRouter != nil {
		h.PromptRouter.Invalidate()
	}
}

func computeRetrievalDiagnostics(results []retrieval.Result) retrievalDiagnostics {
	diag := retrievalDiagnostics{RetrievedChunks: len(results)}
	for _, result := range results {
		if result.Score > diag.MaxSimilarity {
			diag.MaxSimilarity = result.Score
		}
	}
	return diag
}

func (h *Handler) evaluateGuardPolicy(ctx context.Context, policy domain.AIGuardPolicy, results []retrieval.Result) (guardDecision, retrievalDiagnostics, error) {
	if h.GuardEngine == nil {
		return guardDecision{}, retrievalDiagnostics{}, fmt.Errorf("guard engine is not configured")
	}
	diag := computeRetrievalDiagnostics(results)
	decision := h.GuardEngine.Decide(guard.RetrievalResult{
		RetrievedChunks: diag.RetrievedChunks,
		MaxSimilarity:   diag.MaxSimilarity,
	}, policy)

	switch decision {
	case guard.DecisionAllow:
		return guardDecision{Decision: decision, PromptType: legalAnswerPromptType}, diag, nil
	case guard.DecisionAskClarification:
		promptCfg, usedType, found, err := h.getRuntimePromptExact(ctx, legalClarificationPromptType)
		if err != nil {
			return guardDecision{}, diag, err
		}
		message := defaultClarificationMessage
		if found {
			message = sanitizeGuardMessage(promptCfg.SystemPrompt, defaultClarificationMessage)
		}
		return guardDecision{Decision: decision, PromptType: coalescePromptType(usedType, legalClarificationPromptType), Message: message}, diag, nil
	case guard.DecisionRefuse:
		fallthrough
	default:
		promptCfg, usedType, found, err := h.getRuntimePromptExact(ctx, legalRefusalPromptType)
		if err != nil {
			return guardDecision{}, diag, err
		}
		message := defaultRefusalMessage
		if found {
			message = sanitizeGuardMessage(promptCfg.SystemPrompt, defaultRefusalMessage)
		}
		return guardDecision{Decision: guard.DecisionRefuse, PromptType: coalescePromptType(usedType, legalRefusalPromptType), Message: message}, diag, nil
	}
}

func (h *Handler) getRuntimePrompt(ctx context.Context, promptType string) (domain.AIPrompt, string, error) {
	if h.PromptRouter == nil {
		return domain.AIPrompt{}, "", fmt.Errorf("prompt router is not configured")
	}
	return h.PromptRouter.GetPrompt(ctx, promptType)
}

func (h *Handler) getRuntimePromptExact(ctx context.Context, promptType string) (domain.AIPrompt, string, bool, error) {
	if h.PromptRouter == nil {
		return domain.AIPrompt{}, "", false, fmt.Errorf("prompt router is not configured")
	}
	prompt, ok, err := h.PromptRouter.GetPromptExact(ctx, promptType)
	if err != nil {
		return domain.AIPrompt{}, "", false, err
	}
	if !ok {
		return domain.AIPrompt{}, "", false, nil
	}
	return prompt, promptType, true, nil
}

func (h *Handler) getAnswerPrompt(ctx context.Context) (domain.AIPrompt, string, error) {
	for _, promptType := range []string{legalAnswerPromptType, legacyLegalQAPromptType} {
		promptCfg, usedType, found, err := h.getRuntimePromptExact(ctx, promptType)
		if err != nil {
			return domain.AIPrompt{}, "", err
		}
		if found {
			return promptCfg, usedType, nil
		}
	}
	return domain.AIPrompt{}, "", fmt.Errorf("missing prompt for types=%q,%q", legalAnswerPromptType, legacyLegalQAPromptType)
}

func coalescePromptType(current, fallback string) string {
	current = strings.TrimSpace(current)
	if current != "" {
		return current
	}
	return fallback
}

func sanitizeGuardMessage(systemPrompt, fallback string) string {
	message := strings.TrimSpace(systemPrompt)
	if message == "" {
		return fallback
	}
	lower := strings.ToLower(message)
	if len(message) > 220 ||
		strings.Contains(message, "\n") ||
		strings.Contains(lower, "you are ") ||
		strings.Contains(lower, "use only") ||
		strings.Contains(lower, "never invent") ||
		strings.Contains(lower, "do not") ||
		strings.Contains(lower, "always cite") ||
		strings.Contains(lower, "rules:") ||
		strings.Contains(lower, "examples:") {
		return fallback
	}
	return message
}

func buildAnswerSources(results []retrieval.Result) []answer.Source {
	sources := make([]answer.Source, 0, len(results))
	for _, r := range results {
		citation := answer.Citation{
			ID:               r.ChunkID,
			DocumentTitle:    pickString(r.Metadata, "document_title", "title", "doc_title"),
			DocumentNumber:   pickString(r.Metadata, "document_number", "number", "doc_number", "doc_code"),
			DocumentType:     pickString(r.Metadata, "document_type", "doc_type"),
			IssuingAuthority: pickString(r.Metadata, "issuing_authority", "authority", "co_quan_ban_hanh"),
			EffectiveStatus:  pickString(r.Metadata, "effective_status", "status", "hieu_luc"),
			Article:          pickString(r.Metadata, "article", "article_number", "dieu"),
			Clause:           pickString(r.Metadata, "clause", "clause_number", "khoan"),
			Year:             pickInt(r.Metadata, "year", "document_year", "signed_year", "nam"),
			Excerpt:          excerptText(r.Text, 320),
			URL:              pickString(r.Metadata, "url", "document_url", "source_url"),
		}
		citation.CitationLabel = buildCitationLabel(citation)
		sources = append(sources, answer.Source{
			Text:     r.Text,
			Citation: citation,
		})
	}
	return sources
}

func buildCitationLabel(c answer.Citation) string {
	parts := make([]string, 0, 4)
	if c.Article != "" {
		parts = append(parts, "Dieu "+strings.TrimSpace(c.Article))
	}
	if c.DocumentTitle != "" {
		parts = append(parts, strings.TrimSpace(c.DocumentTitle))
	} else if c.DocumentNumber != "" {
		parts = append(parts, "Van ban "+strings.TrimSpace(c.DocumentNumber))
	}
	if c.Year > 0 {
		parts = append(parts, strconv.Itoa(c.Year))
	}
	return strings.Join(parts, " ")
}

func citationsFromSources(sources []answer.Source) []answer.Citation {
	citations := make([]answer.Citation, 0, len(sources))
	for _, source := range sources {
		citation := source.Citation
		if citation.Excerpt == "" {
			citation.Excerpt = excerptText(source.Text, 320)
		}
		citations = append(citations, citation)
	}
	return citations
}

func pickString(meta map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		v, ok := meta[key]
		if !ok || v == nil {
			continue
		}
		switch value := v.(type) {
		case string:
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				return trimmed
			}
		case fmt.Stringer:
			trimmed := strings.TrimSpace(value.String())
			if trimmed != "" {
				return trimmed
			}
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64)
		case int:
			return strconv.Itoa(value)
		}
	}
	return ""
}

func pickInt(meta map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		v, ok := meta[key]
		if !ok || v == nil {
			continue
		}
		switch value := v.(type) {
		case float64:
			return int(value)
		case int:
			return value
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}

func excerptText(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if limit <= 0 || len(trimmed) <= limit {
		return trimmed
	}
	if limit <= 3 {
		return trimmed[:limit]
	}
	return strings.TrimSpace(trimmed[:limit-3]) + "..."
}
