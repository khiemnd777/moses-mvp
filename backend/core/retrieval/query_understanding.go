package retrieval

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/core/schema"
	"github.com/khiemnd777/legal_api/domain"
)

type QueryDebugResult struct {
	Analysis          QueryUnderstandingResult `json:"analysis"`
	RequestedFilters  SearchOptions            `json:"requested_filters"`
	AppliedFilters    map[string]interface{}   `json:"applied_filters"`
	PreferredDocTypes []string                 `json:"preferred_doc_types"`
	FallbackStages    []FallbackStage          `json:"fallback_stages"`
	Results           []Result                 `json:"results"`
}

type FallbackStage struct {
	Attempt         int      `json:"attempt"`
	LegalDomain     []string `json:"legal_domain,omitempty"`
	DocumentType    []string `json:"document_type,omitempty"`
	EffectiveStatus []string `json:"effective_status,omitempty"`
	Reason          string   `json:"reason,omitempty"`
	HitCount        int      `json:"hit_count"`
}

type docTypeQueryProfile struct {
	Code              string
	FormHash          string
	CanonicalTerms    []string
	SynonymGroups     []schema.SynonymGroup
	QuerySignals      []string
	IntentRules       []schema.IntentRule
	DomainTopicRules  []schema.DomainTopicRule
	LegalSignals      []string
	FollowUpMarkers   []string
	PreferredDocTypes []string
	RoutingPriority   int
}

type synonymRule struct {
	Canonical   string
	Alias       string
	DocTypeCode string
	FormHash    string
}

type queryUnderstandingIndex struct {
	Profiles map[string]docTypeQueryProfile
	Synonyms []synonymRule
}

func (s *Service) AnalyzeQuery(ctx context.Context, query string) QueryUnderstandingResult {
	index := s.loadQueryUnderstandingIndex(ctx)
	return analyzeQueryWithIndex(query, index)
}

func (s *Service) BuildFollowUpSearchQuery(ctx context.Context, history []answer.ConversationMessage, currentQuery string) string {
	index := s.loadQueryUnderstandingIndex(ctx)
	return buildFollowUpSearchQueryWithIndex(index, history, currentQuery)
}

func (s *Service) HasLegalSignal(ctx context.Context, query string) bool {
	index := s.loadQueryUnderstandingIndex(ctx)
	analysis := analyzeQueryWithIndex(query, index)
	return containsLegalSignal(index, analysis.CanonicalQuery)
}

func (s *Service) loadQueryUnderstandingIndex(ctx context.Context) queryUnderstandingIndex {
	ttl := s.queryTTL
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	s.queryMu.RLock()
	if s.queryReady && time.Since(s.queryLoadedAt) <= ttl {
		cached := s.queryCache
		s.queryMu.RUnlock()
		return cached
	}
	s.queryMu.RUnlock()

	index := queryUnderstandingIndex{Profiles: map[string]docTypeQueryProfile{}}
	docTypes, err := s.Store.ListDocTypes(ctx)
	if err == nil {
		index = buildQueryUnderstandingIndex(docTypes)
	}

	s.queryMu.Lock()
	s.queryCache = index
	s.queryLoadedAt = time.Now()
	s.queryReady = true
	s.queryMu.Unlock()
	return index
}

func buildQueryUnderstandingIndex(docTypes []domain.DocType) queryUnderstandingIndex {
	index := queryUnderstandingIndex{Profiles: map[string]docTypeQueryProfile{}}
	for _, docType := range docTypes {
		var form schema.DocTypeForm
		if len(docType.FormJSON) == 0 || json.Unmarshal(docType.FormJSON, &form) != nil {
			continue
		}
		profile := docTypeQueryProfile{
			Code:              strings.TrimSpace(docType.Code),
			FormHash:          strings.TrimSpace(docType.FormHash),
			CanonicalTerms:    normalizeTerms(form.QueryProfile.CanonicalTerms),
			SynonymGroups:     form.QueryProfile.SynonymGroups,
			QuerySignals:      normalizeTerms(form.QueryProfile.QuerySignals),
			IntentRules:       form.QueryProfile.IntentRules,
			DomainTopicRules:  form.QueryProfile.DomainTopicRules,
			LegalSignals:      normalizeTerms(form.QueryProfile.LegalSignalRules),
			FollowUpMarkers:   normalizeTerms(form.QueryProfile.FollowUpMarkers),
			PreferredDocTypes: normalizeTerms(form.QueryProfile.PreferredDocTypes),
			RoutingPriority:   form.QueryProfile.RoutingPriority,
		}
		if profile.Code == "" {
			continue
		}
		index.Profiles[profile.Code] = profile
		for _, group := range profile.SynonymGroups {
			canonical := normalizeQuery(group.Canonical)
			if canonical == "" {
				continue
			}
			index.Synonyms = append(index.Synonyms, synonymRule{
				Canonical:   canonical,
				Alias:       canonical,
				DocTypeCode: profile.Code,
				FormHash:    profile.FormHash,
			})
			for _, alias := range group.Aliases {
				alias = normalizeQuery(alias)
				if alias == "" {
					continue
				}
				index.Synonyms = append(index.Synonyms, synonymRule{
					Canonical:   canonical,
					Alias:       alias,
					DocTypeCode: profile.Code,
					FormHash:    profile.FormHash,
				})
			}
		}
	}
	sort.SliceStable(index.Synonyms, func(i, j int) bool {
		if len(index.Synonyms[i].Alias) != len(index.Synonyms[j].Alias) {
			return len(index.Synonyms[i].Alias) > len(index.Synonyms[j].Alias)
		}
		return index.Synonyms[i].Alias < index.Synonyms[j].Alias
	})
	return index
}

func analyzeQueryWithIndex(query string, index queryUnderstandingIndex) QueryUnderstandingResult {
	normalized := normalizeQuery(query)
	result := QueryUnderstandingResult{
		OriginalQuery:      query,
		NormalizedQuery:    normalized,
		CanonicalQuery:     canonicalizeQuery(normalized, index),
		Entities:           map[string]interface{}{},
		Filters:            map[string]interface{}{},
		MatchedDocTypes:    []string{},
		MatchedQueryRules:  []string{},
		QueryProfileHashes: map[string]string{},
	}

	type scoredMatch struct {
		code     string
		score    int
		priority int
	}
	matches := make([]scoredMatch, 0, len(index.Profiles))
	for code, profile := range index.Profiles {
		score := 0
		for _, term := range profile.CanonicalTerms {
			if containsPhrase(result.CanonicalQuery, term) {
				score += 3
				result.MatchedQueryRules = append(result.MatchedQueryRules, "canonical_term:"+code+":"+term)
			}
		}
		for _, signal := range profile.QuerySignals {
			if containsPhrase(result.CanonicalQuery, signal) {
				score += 4
				result.MatchedQueryRules = append(result.MatchedQueryRules, "query_signal:"+code+":"+signal)
			}
		}
		if score <= 0 {
			continue
		}
		matches = append(matches, scoredMatch{code: code, score: score, priority: profile.RoutingPriority})
		result.QueryProfileHashes[code] = profile.FormHash
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if matches[i].priority != matches[j].priority {
			return matches[i].priority > matches[j].priority
		}
		return matches[i].code < matches[j].code
	})
	for _, match := range matches {
		result.MatchedDocTypes = append(result.MatchedDocTypes, match.code)
		if profile, ok := index.Profiles[match.code]; ok {
			result.PreferredDocTypes = dedupeStrings(append(result.PreferredDocTypes, profile.PreferredDocTypes...))
		}
	}

	for _, match := range matches {
		profile := index.Profiles[match.code]
		for _, rule := range profile.DomainTopicRules {
			for _, term := range normalizeTerms(rule.Terms) {
				if containsPhrase(result.CanonicalQuery, term) {
					result.LegalDomain = strings.TrimSpace(rule.LegalDomain)
					result.LegalTopic = strings.TrimSpace(rule.LegalTopic)
					result.MatchedQueryRules = append(result.MatchedQueryRules, "domain_topic:"+match.code+":"+term)
					goto intent
				}
			}
		}
	}

intent:
	for _, match := range matches {
		profile := index.Profiles[match.code]
		for _, rule := range profile.IntentRules {
			for _, term := range normalizeTerms(rule.Terms) {
				if containsPhrase(result.CanonicalQuery, term) {
					result.Intent = strings.TrimSpace(rule.Intent)
					result.MatchedQueryRules = append(result.MatchedQueryRules, "intent:"+match.code+":"+term)
					goto entities
				}
			}
		}
	}

entities:
	if year := extractYear(result.CanonicalQuery, `\b(19\d{2}|20\d{2})\b`); year > 0 {
		result.Entities["year"] = year
	}
	if n := extractInt(result.CanonicalQuery, `(\d+)\s*con`); n > 0 {
		result.Entities["children_count"] = n
	}
	if containsPhrase(result.CanonicalQuery, "nha") {
		result.Entities["property_type"] = "house"
	}
	if containsPhrase(result.CanonicalQuery, "dieu ") {
		if v := extractString(result.CanonicalQuery, `dieu\s+([0-9]+)`); v != "" {
			result.Entities["article_number"] = v
			result.Filters["article_number"] = v
		}
	}
	if result.LegalDomain != "" {
		result.Filters["legal_domain"] = result.LegalDomain
	}
	if result.Intent == "" && containsPhrase(result.CanonicalQuery, "thu tuc") {
		result.Intent = "legal_procedure_advice"
	}
	if result.Intent == "" {
		result.Intent = "legal_basis_lookup"
	}
	return result
}

func buildFollowUpSearchQueryWithIndex(index queryUnderstandingIndex, history []answer.ConversationMessage, currentQuery string) string {
	current := strings.TrimSpace(currentQuery)
	if current == "" {
		return ""
	}
	if !shouldAugmentWithHistory(index, current) {
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
	currentAnalysis := analyzeQueryWithIndex(current, index)
	priorAnalysis := analyzeQueryWithIndex(priorUser, index)
	if priorAnalysis.CanonicalQuery == currentAnalysis.CanonicalQuery {
		return current
	}
	return strings.TrimSpace(priorUser + " " + current)
}

func shouldAugmentWithHistory(index queryUnderstandingIndex, currentQuery string) bool {
	analysis := analyzeQueryWithIndex(currentQuery, index)
	if analysis.CanonicalQuery == "" {
		return false
	}
	for _, profile := range index.Profiles {
		for _, marker := range profile.FollowUpMarkers {
			if containsPhrase(analysis.CanonicalQuery, marker) {
				return true
			}
		}
	}
	return len(strings.Fields(analysis.CanonicalQuery)) <= 8
}

func containsLegalSignal(index queryUnderstandingIndex, normalized string) bool {
	for _, profile := range index.Profiles {
		for _, signal := range profile.LegalSignals {
			if containsPhrase(normalized, signal) {
				return true
			}
		}
	}
	return false
}

func normalizeTerms(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := normalizeQuery(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func canonicalizeQuery(normalized string, index queryUnderstandingIndex) string {
	if normalized == "" {
		return ""
	}
	out := normalized
	matchedDocTypes := map[string]struct{}{}
	hashes := map[string]string{}
	for _, rule := range index.Synonyms {
		if rule.Alias == "" || rule.Canonical == "" {
			continue
		}
		if containsPhrase(out, rule.Alias) {
			out = replacePhrase(out, rule.Alias, rule.Canonical)
			matchedDocTypes[rule.DocTypeCode] = struct{}{}
			hashes[rule.DocTypeCode] = rule.FormHash
		}
	}
	_ = matchedDocTypes
	_ = hashes
	return strings.Join(strings.Fields(out), " ")
}

func containsPhrase(text, phrase string) bool {
	text = strings.TrimSpace(text)
	phrase = strings.TrimSpace(phrase)
	if text == "" || phrase == "" {
		return false
	}
	paddedText := " " + text + " "
	paddedPhrase := " " + phrase + " "
	return strings.Contains(paddedText, paddedPhrase)
}

func replacePhrase(text, phrase, replacement string) string {
	if phrase == "" || replacement == "" {
		return text
	}
	paddedText := " " + strings.TrimSpace(text) + " "
	paddedPhrase := " " + strings.TrimSpace(phrase) + " "
	paddedReplacement := " " + strings.TrimSpace(replacement) + " "
	for strings.Contains(paddedText, paddedPhrase) {
		paddedText = strings.ReplaceAll(paddedText, paddedPhrase, paddedReplacement)
	}
	return strings.Join(strings.Fields(paddedText), " ")
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
