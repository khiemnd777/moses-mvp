package api

import "strings"

const (
	defaultTopK            = 5
	maxTopK                = 20
	defaultToneKey         = "default"
	defaultEffectiveStatus = "active"
)

type ChatFilters struct {
	Tone            string `json:"tone"`
	TopK            int    `json:"topK"`
	EffectiveStatus string `json:"effectiveStatus"`
	Domain          string `json:"domain"`
	DocType         string `json:"docType"`
	DocumentNumber  string `json:"documentNumber"`
	ArticleNumber   string `json:"articleNumber"`
}

type answerRequest struct {
	Question string      `json:"question"`
	Filters  ChatFilters `json:"filters"`

	Query string `json:"query"`
	TopK  int    `json:"top_k"`
	Tone  string `json:"tone"`
}

func normalizeAnswerRequest(req answerRequest, tones map[string]string) (string, ChatFilters) {
	question := strings.TrimSpace(req.Question)
	if question == "" {
		question = strings.TrimSpace(req.Query)
	}

	filters := req.Filters
	if filters.TopK == 0 && req.TopK > 0 {
		filters.TopK = req.TopK
	}
	if filters.Tone == "" && req.Tone != "" {
		filters.Tone = req.Tone
	}

	filters.Domain = strings.TrimSpace(filters.Domain)
	filters.DocType = strings.TrimSpace(filters.DocType)
	filters.DocumentNumber = strings.TrimSpace(filters.DocumentNumber)
	filters.ArticleNumber = strings.TrimSpace(filters.ArticleNumber)
	filters.EffectiveStatus = normalizeEffectiveStatus(filters.EffectiveStatus)
	filters.TopK = normalizeTopK(filters.TopK)
	filters.Tone = normalizeTone(filters.Tone, tones)

	return question, filters
}

func normalizeTopK(topK int) int {
	if topK <= 0 {
		return defaultTopK
	}
	if topK > maxTopK {
		return maxTopK
	}
	return topK
}

func normalizeTone(tone string, tones map[string]string) string {
	tone = strings.TrimSpace(tone)
	if tone == "" {
		return defaultToneKey
	}
	if tones != nil {
		if _, ok := tones[tone]; ok {
			return tone
		}
	}
	return defaultToneKey
}

func normalizeEffectiveStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "archived" {
		return status
	}
	return defaultEffectiveStatus
}
