package api

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/core/retrieval"
	"github.com/khiemnd777/legal_api/domain"
)

const legalGuardPromptType = "legal_guard"

const (
	emptyRetrievalRefuseMessage = "Không tìm thấy căn cứ pháp lý phù hợp trong hệ thống."
	lowConfidenceRefuseMessage  = "Nguồn trích dẫn hiện tại chưa đạt độ tin cậy để đưa ra kết luận pháp lý."
	askClarificationMessage     = "Chưa đủ căn cứ pháp lý rõ ràng. Vui lòng bổ sung tình huống, văn bản, hoặc điều khoản cần tra cứu."
)

type runtimeAnswerConfig struct {
	Prompt domain.AIPrompt
	Policy domain.AIGuardPolicy
	Tone   string
}

type guardDecision struct {
	Allow   bool
	Message string
}

func (h *Handler) loadRuntimeAnswerConfig(ctx context.Context, toneKey string) (runtimeAnswerConfig, error) {
	promptCfg, err := h.Store.GetActiveAIPromptByType(ctx, legalGuardPromptType)
	if err != nil {
		if err == sql.ErrNoRows {
			return runtimeAnswerConfig{}, fmt.Errorf("no active ai prompt config for prompt_type=%s", legalGuardPromptType)
		}
		return runtimeAnswerConfig{}, err
	}

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

	return runtimeAnswerConfig{
		Prompt: promptCfg,
		Policy: guardCfg,
		Tone:   tone,
	}, nil
}

func evaluateGuardPolicy(policy domain.AIGuardPolicy, results []retrieval.Result) guardDecision {
	chunkCount := len(results)
	maxScore := 0.0
	for _, result := range results {
		if result.Score > maxScore {
			maxScore = result.Score
		}
	}

	if chunkCount < policy.MinRetrievedChunks {
		return applyGuardAction(policy.OnEmptyRetrieval, emptyRetrievalRefuseMessage)
	}

	if maxScore < policy.MinSimilarityScore {
		return applyGuardAction(policy.OnLowConfidence, lowConfidenceRefuseMessage)
	}

	return guardDecision{Allow: true}
}

func applyGuardAction(action string, refuseMessage string) guardDecision {
	switch strings.TrimSpace(action) {
	case "fallback_llm":
		return guardDecision{Allow: true}
	case "ask_clarification":
		return guardDecision{Allow: false, Message: askClarificationMessage}
	case "refuse":
		fallthrough
	default:
		return guardDecision{Allow: false, Message: refuseMessage}
	}
}

func buildAnswerSources(results []retrieval.Result) []answer.Source {
	sources := make([]answer.Source, 0, len(results))
	for _, r := range results {
		sources = append(sources, answer.Source{
			Text: r.Text,
			Citation: answer.Citation{
				ID:             r.ChunkID,
				DocumentTitle:  pickString(r.Metadata, "document_title", "title", "doc_title"),
				DocumentNumber: pickString(r.Metadata, "document_number", "number", "doc_number", "doc_code"),
				Article:        pickString(r.Metadata, "article", "article_number", "dieu"),
				Clause:         pickString(r.Metadata, "clause", "clause_number", "khoan"),
				Year:           pickInt(r.Metadata, "year", "document_year", "nam"),
				Excerpt:        excerptText(r.Text, 320),
				URL:            pickString(r.Metadata, "url", "document_url", "source_url"),
			},
		})
	}
	return sources
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
