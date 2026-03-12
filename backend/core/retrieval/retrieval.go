package retrieval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
	"github.com/khiemnd777/legal_api/observability"
)

type Service struct {
	Store  *infra.Store
	Qdrant *infra.QdrantClient
	Embed  Embedder
	Logger *slog.Logger

	cfgMu       sync.RWMutex
	cfgCache    runtimeConfig
	cfgLoadedAt time.Time
	cfgReady    bool
	cfgTTL      time.Duration
}

type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float64, error)
}

type SearchOptions struct {
	TopK            int
	Domain          string
	DocType         string
	EffectiveStatus string
	DocumentNumber  string
	ArticleNumber   string
}

type QueryUnderstandingResult struct {
	OriginalQuery   string                 `json:"original_query"`
	NormalizedQuery string                 `json:"normalized_query"`
	LegalDomain     string                 `json:"legal_domain"`
	LegalTopic      string                 `json:"legal_topic"`
	Intent          string                 `json:"intent"`
	Entities        map[string]interface{} `json:"entities"`
	Filters         map[string]interface{} `json:"filters"`
}

type RetrievalPlan struct {
	QueryText          string                 `json:"query_text"`
	Filters            map[string]interface{} `json:"filters"`
	PreferredDocTypes  []string               `json:"preferred_doc_types"`
	TopK               int                    `json:"top_k"`
	ExpandAdjacent     bool                   `json:"expand_adjacent"`
	AdjacentWindow     int                    `json:"adjacent_window"`
	Rerank             bool                   `json:"rerank"`
	CandidatePoolLimit int                    `json:"candidate_pool_limit"`
}

type RetrievalCandidate struct {
	Chunk       domain.Chunk
	Metadata    map[string]interface{}
	VectorScore float64
	FinalScore  float64
	ChunkID     string
}

type RerankedChunk struct {
	ChunkID      string  `json:"chunk_id"`
	VectorScore  float64 `json:"vector_score"`
	LexicalScore float64 `json:"lexical_score"`
	MetaScore    float64 `json:"meta_score"`
	ArticleScore float64 `json:"article_score"`
	FinalScore   float64 `json:"final_score"`
}

type ContextAssemblyResult struct {
	ChunkIDs       []string `json:"chunk_ids"`
	ChunkCount     int      `json:"chunk_count"`
	DroppedByLimit int      `json:"dropped_by_limit"`
	TotalChars     int      `json:"total_chars"`
}

type Result struct {
	ChunkID    string
	Text       string
	CitationID string
	VersionID  string
	ChunkIndex int
	Score      float64
	Metadata   map[string]interface{}
	IsAdjacent bool
}

type runtimeConfig struct {
	DefaultTopK            int
	DefaultEffectiveStatus string
	RerankEnabled          bool
	RerankWeights          rerankWeights
	AdjacentChunkEnabled   bool
	AdjacentChunkWindow    int
	MaxContextChunks       int
	MaxContextChars        int
	CandidateMultiplier    int
	PreferredDocTypes      []string
	DomainDefaults         map[string]domainRuntimeDefault
}

type domainRuntimeDefault struct {
	TopK              int
	PreferredDocTypes []string
}

type rerankWeights struct {
	Vector   float64
	Keyword  float64
	Metadata float64
	Article  float64
}

type observabilityEvent struct {
	OriginalQuery            string                 `json:"original_query"`
	NormalizedQuery          string                 `json:"normalized_query"`
	LegalDomain              string                 `json:"legal_domain"`
	LegalTopic               string                 `json:"legal_topic"`
	Intent                   string                 `json:"intent"`
	AppliedFilters           map[string]interface{} `json:"applied_filters"`
	TopK                     int                    `json:"top_k"`
	InitialVectorHits        []string               `json:"initial_vector_hits"`
	RerankedResults          []RerankedChunk        `json:"reranked_results"`
	FinalSelectedChunkIDs    []string               `json:"final_selected_chunk_ids"`
	AdjacentExpandedChunkIDs []string               `json:"adjacent_expanded_chunk_ids"`
	PromptContextChunkCount  int                    `json:"prompt_context_chunk_count"`
	RetrievalLatencyMS       int64                  `json:"retrieval_latency_ms"`
	RerankLatencyMS          int64                  `json:"rerank_latency_ms"`
}

func (s *Service) Search(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	started := time.Now()
	cfg := s.loadRuntimeConfig(ctx)
	qu := UnderstandQuery(query)
	plan := BuildRetrievalPlan(qu, opts, cfg)

	vectors, err := s.Embed.Embed(ctx, []string{plan.QueryText})
	if err != nil {
		return nil, err
	}
	qdrantFilter := buildQdrantFilter(plan.Filters, plan.PreferredDocTypes)
	matches, err := s.Qdrant.Search(ctx, vectors[0], plan.CandidatePoolLimit, qdrantFilter)
	if err != nil {
		return nil, err
	}

	initialHitIDs := make([]string, 0, len(matches))
	chunkIDs := make([]string, 0, len(matches))
	for _, m := range matches {
		if m.ChunkID == "" {
			continue
		}
		initialHitIDs = append(initialHitIDs, m.ChunkID)
		chunkIDs = append(chunkIDs, m.ChunkID)
	}

	chunks, err := s.Store.GetChunksByIDs(ctx, chunkIDs)
	if err != nil {
		return nil, err
	}
	chunkByID := make(map[string]domain.Chunk, len(chunks))
	for _, chunk := range chunks {
		chunkByID[chunk.ID] = chunk
	}

	candidates := make([]RetrievalCandidate, 0, len(matches))
	for _, match := range matches {
		c, ok := chunkByID[match.ChunkID]
		if !ok {
			continue
		}
		candidates = append(candidates, RetrievalCandidate{
			Chunk:       c,
			ChunkID:     c.ID,
			Metadata:    decodeMetadata(c.MetadataJSON),
			VectorScore: match.Score,
			FinalScore:  match.Score,
		})
	}

	rerankedTrace := []RerankedChunk{}
	rerankLatency := time.Duration(0)
	if plan.Rerank && len(candidates) > 1 {
		rStart := time.Now()
		rerankedTrace = rerankCandidates(candidates, qu, plan, cfg)
		rerankLatency = time.Since(rStart)
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].FinalScore > candidates[j].FinalScore
		})
	}

	if len(candidates) > plan.TopK {
		candidates = candidates[:plan.TopK]
	}

	selected := make([]Result, 0, len(candidates))
	for _, c := range candidates {
		selected = append(selected, toResult(c, false))
	}

	if plan.ExpandAdjacent && plan.AdjacentWindow > 0 && len(selected) > 0 {
		selected, err = s.expandAdjacent(ctx, selected, plan.AdjacentWindow)
		if err != nil {
			return nil, err
		}
	}

	limited, assembly := applyContextLimits(selected, cfg.MaxContextChunks, cfg.MaxContextChars)
	event := observabilityEvent{
		OriginalQuery:            qu.OriginalQuery,
		NormalizedQuery:          qu.NormalizedQuery,
		LegalDomain:              qu.LegalDomain,
		LegalTopic:               qu.LegalTopic,
		Intent:                   qu.Intent,
		AppliedFilters:           plan.Filters,
		TopK:                     plan.TopK,
		InitialVectorHits:        initialHitIDs,
		RerankedResults:          rerankedTrace,
		FinalSelectedChunkIDs:    pickResultChunkIDs(limited, false),
		AdjacentExpandedChunkIDs: pickResultChunkIDs(limited, true),
		PromptContextChunkCount:  assembly.ChunkCount,
		RetrievalLatencyMS:       time.Since(started).Milliseconds(),
		RerankLatencyMS:          rerankLatency.Milliseconds(),
	}
	if recorder := observability.RecorderFromContext(ctx); recorder != nil {
		recorder.OnRetrieval(qu.NormalizedQuery, plan.Filters, pickResultChunkIDs(limited, false))
	}
	observability.LogInfo(ctx, s.logger(), "retrieval", "retrieval completed", map[string]interface{}{
		"original_query":              event.OriginalQuery,
		"normalized_query":            event.NormalizedQuery,
		"legal_domain":                event.LegalDomain,
		"legal_topic":                 event.LegalTopic,
		"intent":                      event.Intent,
		"applied_filters":             event.AppliedFilters,
		"top_k":                       event.TopK,
		"initial_vector_hits":         event.InitialVectorHits,
		"reranked_results":            event.RerankedResults,
		"final_selected_chunk_ids":    event.FinalSelectedChunkIDs,
		"adjacent_expanded_chunk_ids": event.AdjacentExpandedChunkIDs,
		"prompt_context_chunk_count":  event.PromptContextChunkCount,
		"retrieval_latency_ms":        event.RetrievalLatencyMS,
		"rerank_latency_ms":           event.RerankLatencyMS,
	})

	return limited, nil
}

func UnderstandQuery(query string) QueryUnderstandingResult {
	normalized := normalizeQuery(query)
	result := QueryUnderstandingResult{
		OriginalQuery:   query,
		NormalizedQuery: normalized,
		Entities:        map[string]interface{}{},
		Filters:         map[string]interface{}{},
	}

	if strings.Contains(normalized, "ly hon") {
		result.LegalDomain = "marriage_family"
		result.LegalTopic = "divorce"
		result.Intent = "legal_procedure_advice"
	}
	if result.LegalDomain == "" && strings.Contains(normalized, "hop dong") {
		result.LegalDomain = "civil"
		result.LegalTopic = "contract"
		result.Intent = "legal_rights_obligations"
	}
	if result.Intent == "" {
		if strings.Contains(normalized, "thu tuc") || strings.Contains(normalized, "ho so") {
			result.Intent = "legal_procedure_advice"
		} else {
			result.Intent = "legal_basis_lookup"
		}
	}

	if year := extractYear(normalized, `\b(19\d{2}|20\d{2})\b`); year > 0 {
		result.Entities["year"] = year
	}
	if n := extractInt(normalized, `(\d+)\s*con`); n > 0 {
		result.Entities["children_count"] = n
	}
	if strings.Contains(normalized, "nha") {
		result.Entities["property_type"] = "house"
	}
	if strings.Contains(normalized, "dieu ") {
		if v := extractString(normalized, `dieu\s+([0-9]+)`); v != "" {
			result.Entities["article_number"] = v
			result.Filters["article_number"] = v
		}
	}

	if result.LegalDomain != "" {
		result.Filters["legal_domain"] = result.LegalDomain
	}
	return result
}

func BuildRetrievalPlan(qu QueryUnderstandingResult, opts SearchOptions, cfg runtimeConfig) RetrievalPlan {
	topK := opts.TopK
	if topK <= 0 {
		topK = cfg.DefaultTopK
	}
	if topK <= 0 {
		topK = 5
	}

	queryText := qu.NormalizedQuery
	if queryText == "" {
		queryText = normalizeQuery(qu.OriginalQuery)
	}
	if queryText == "" {
		queryText = strings.TrimSpace(qu.OriginalQuery)
	}

	filters := copyMap(qu.Filters)
	if v := strings.TrimSpace(strings.ToLower(opts.Domain)); v != "" {
		filters["legal_domain"] = v
	}
	if v := strings.TrimSpace(strings.ToLower(opts.DocType)); v != "" {
		filters["document_type"] = v
	}
	if v := strings.TrimSpace(strings.ToLower(opts.EffectiveStatus)); v != "" {
		filters["effective_status"] = v
	}
	if v := strings.TrimSpace(opts.DocumentNumber); v != "" {
		filters["document_number"] = v
	}
	if v := strings.TrimSpace(opts.ArticleNumber); v != "" {
		filters["article_number"] = v
	}
	if pickString(filters, "effective_status") == "" {
		filters["effective_status"] = cfg.DefaultEffectiveStatus
	}

	preferred := append([]string{}, cfg.PreferredDocTypes...)
	if domainName := pickString(filters, "legal_domain"); domainName != "" {
		if domainCfg, ok := cfg.DomainDefaults[domainName]; ok {
			if domainCfg.TopK > 0 {
				topK = domainCfg.TopK
			}
			if len(domainCfg.PreferredDocTypes) > 0 {
				preferred = domainCfg.PreferredDocTypes
			}
		}
	}
	if v := pickString(filters, "document_type"); v != "" {
		preferred = []string{v}
	}

	return RetrievalPlan{
		QueryText:          queryText,
		Filters:            filters,
		PreferredDocTypes:  dedupeStrings(preferred),
		TopK:               topK,
		ExpandAdjacent:     cfg.AdjacentChunkEnabled && cfg.AdjacentChunkWindow > 0,
		AdjacentWindow:     max(0, cfg.AdjacentChunkWindow),
		Rerank:             cfg.RerankEnabled,
		CandidatePoolLimit: max(topK, topK*max(1, cfg.CandidateMultiplier)),
	}
}

func rerankCandidates(candidates []RetrievalCandidate, qu QueryUnderstandingResult, plan RetrievalPlan, cfg runtimeConfig) []RerankedChunk {
	trace := make([]RerankedChunk, 0, len(candidates))
	queryTokens := tokenize(qu.NormalizedQuery)
	for i := range candidates {
		lexical := lexicalOverlapScore(queryTokens, tokenize(normalizeQuery(candidates[i].Chunk.Text)))
		metaScore := metadataMatchScore(candidates[i].Metadata, plan.Filters)
		articleScore := articleBonus(candidates[i].Metadata, qu)
		finalScore := cfg.RerankWeights.Vector*candidates[i].VectorScore +
			cfg.RerankWeights.Keyword*lexical +
			cfg.RerankWeights.Metadata*metaScore +
			cfg.RerankWeights.Article*articleScore
		candidates[i].FinalScore = finalScore
		trace = append(trace, RerankedChunk{
			ChunkID:      candidates[i].ChunkID,
			VectorScore:  candidates[i].VectorScore,
			LexicalScore: lexical,
			MetaScore:    metaScore,
			ArticleScore: articleScore,
			FinalScore:   finalScore,
		})
	}
	sort.SliceStable(trace, func(i, j int) bool {
		return trace[i].FinalScore > trace[j].FinalScore
	})
	return trace
}

func (s *Service) expandAdjacent(ctx context.Context, selected []Result, window int) ([]Result, error) {
	byVersion := map[string]map[int]struct{}{}
	for _, r := range selected {
		if _, ok := byVersion[r.VersionID]; !ok {
			byVersion[r.VersionID] = map[int]struct{}{}
		}
		for idx := r.ChunkIndex - window; idx <= r.ChunkIndex+window; idx++ {
			if idx >= 0 {
				byVersion[r.VersionID][idx] = struct{}{}
			}
		}
	}

	adjacentByID := map[string]Result{}
	for versionID, idxSet := range byVersion {
		idxs := make([]int, 0, len(idxSet))
		for idx := range idxSet {
			idxs = append(idxs, idx)
		}
		chunks, err := s.Store.GetChunksByVersionAndIndexes(ctx, versionID, idxs)
		if err != nil {
			return nil, err
		}
		for _, c := range chunks {
			adjacentByID[c.ID] = Result{
				ChunkID:    c.ID,
				Text:       c.Text,
				VersionID:  c.DocumentVersionID,
				ChunkIndex: c.Index,
				CitationID: citationID(c.DocumentVersionID, c.Index, c.Text),
				Metadata:   decodeMetadata(c.MetadataJSON),
				IsAdjacent: true,
			}
		}
	}

	ordered := make([]Result, 0, len(adjacentByID))
	seen := map[string]struct{}{}
	for _, base := range selected {
		neighbors := make([]Result, 0, 2*window+1)
		for _, r := range adjacentByID {
			if r.VersionID != base.VersionID {
				continue
			}
			if r.ChunkIndex < base.ChunkIndex-window || r.ChunkIndex > base.ChunkIndex+window {
				continue
			}
			neighbors = append(neighbors, r)
		}
		sort.SliceStable(neighbors, func(i, j int) bool {
			return neighbors[i].ChunkIndex < neighbors[j].ChunkIndex
		})
		for _, n := range neighbors {
			if _, ok := seen[n.ChunkID]; ok {
				continue
			}
			if n.ChunkID == base.ChunkID {
				n.IsAdjacent = false
				n.Score = base.Score
			}
			seen[n.ChunkID] = struct{}{}
			ordered = append(ordered, n)
		}
	}
	return ordered, nil
}

func applyContextLimits(results []Result, maxChunks, maxChars int) ([]Result, ContextAssemblyResult) {
	if maxChunks <= 0 {
		maxChunks = len(results)
	}
	if maxChars <= 0 {
		maxChars = math.MaxInt32
	}
	out := make([]Result, 0, min(maxChunks, len(results)))
	totalChars := 0
	for _, r := range results {
		if len(out) >= maxChunks {
			break
		}
		if totalChars+len(r.Text) > maxChars {
			break
		}
		out = append(out, r)
		totalChars += len(r.Text)
	}
	return out, ContextAssemblyResult{
		ChunkIDs:       pickResultChunkIDs(out, false),
		ChunkCount:     len(out),
		DroppedByLimit: len(results) - len(out),
		TotalChars:     totalChars,
	}
}

func buildQdrantFilter(filters map[string]interface{}, preferredDocTypes []string) *infra.SearchFilter {
	qf := &infra.SearchFilter{
		LegalDomain:     asStringSlice(filters["legal_domain"]),
		DocumentType:    asStringSlice(filters["document_type"]),
		EffectiveStatus: asStringSlice(filters["effective_status"]),
		DocumentNumber:  asStringSlice(filters["document_number"]),
		ArticleNumber:   asStringSlice(filters["article_number"]),
	}
	if len(qf.DocumentType) == 0 && len(preferredDocTypes) > 0 {
		qf.DocumentType = preferredDocTypes
	}
	if len(qf.LegalDomain)+len(qf.DocumentType)+len(qf.EffectiveStatus)+len(qf.DocumentNumber)+len(qf.ArticleNumber) == 0 {
		return nil
	}
	return qf
}

func toResult(candidate RetrievalCandidate, isAdjacent bool) Result {
	return Result{
		ChunkID:    candidate.Chunk.ID,
		Text:       candidate.Chunk.Text,
		VersionID:  candidate.Chunk.DocumentVersionID,
		ChunkIndex: candidate.Chunk.Index,
		CitationID: citationID(candidate.Chunk.DocumentVersionID, candidate.Chunk.Index, candidate.Chunk.Text),
		Score:      candidate.FinalScore,
		Metadata:   candidate.Metadata,
		IsAdjacent: isAdjacent,
	}
}

func (s *Service) loadRuntimeConfig(ctx context.Context) runtimeConfig {
	ttl := s.cfgTTL
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	s.cfgMu.RLock()
	if s.cfgReady && time.Since(s.cfgLoadedAt) <= ttl {
		cached := s.cfgCache
		s.cfgMu.RUnlock()
		return cached
	}
	s.cfgMu.RUnlock()

	cfg := defaultRuntimeConfig()
	dbCfg, err := s.Store.GetActiveAIRetrievalConfig(ctx)
	if err == nil {
		if dbCfg.DefaultTopK > 0 {
			cfg.DefaultTopK = dbCfg.DefaultTopK
		}
		if v := strings.TrimSpace(dbCfg.DefaultEffectiveStatus); v != "" {
			cfg.DefaultEffectiveStatus = strings.ToLower(v)
		}
		cfg.RerankEnabled = dbCfg.RerankEnabled
		cfg.AdjacentChunkEnabled = dbCfg.AdjacentChunkEnabled
		if dbCfg.AdjacentChunkWindow >= 0 {
			cfg.AdjacentChunkWindow = dbCfg.AdjacentChunkWindow
		}
		if dbCfg.MaxContextChunks > 0 {
			cfg.MaxContextChunks = dbCfg.MaxContextChunks
		}
		if dbCfg.MaxContextChars > 0 {
			cfg.MaxContextChars = dbCfg.MaxContextChars
		}
		cfg.RerankWeights = rerankWeights{
			Vector:   dbCfg.RerankVectorWeight,
			Keyword:  dbCfg.RerankKeywordWeight,
			Metadata: dbCfg.RerankMetadataWeight,
			Article:  dbCfg.RerankArticleWeight,
		}
		cfg.PreferredDocTypes = dedupeStrings(dbCfg.PreferredDocTypes)
		cfg.DomainDefaults = parseDomainDefaults(dbCfg.LegalDomainDefaultsJSON)
	} else {
		s.logger().Warn("use_default_retrieval_config", slog.String("error", err.Error()))
	}

	s.cfgMu.Lock()
	s.cfgCache = cfg
	s.cfgLoadedAt = time.Now()
	s.cfgReady = true
	s.cfgMu.Unlock()
	return cfg
}

func defaultRuntimeConfig() runtimeConfig {
	return runtimeConfig{
		DefaultTopK:            5,
		DefaultEffectiveStatus: "active",
		RerankEnabled:          true,
		AdjacentChunkEnabled:   true,
		AdjacentChunkWindow:    1,
		MaxContextChunks:       12,
		MaxContextChars:        12000,
		CandidateMultiplier:    3,
		PreferredDocTypes:      []string{"law", "resolution", "decree"},
		DomainDefaults: map[string]domainRuntimeDefault{
			"marriage_family": {TopK: 6, PreferredDocTypes: []string{"law", "resolution"}},
			"criminal_law":    {TopK: 8, PreferredDocTypes: []string{"law", "decree"}},
		},
		RerankWeights: rerankWeights{
			Vector:   0.55,
			Keyword:  0.25,
			Metadata: 0.15,
			Article:  0.05,
		},
	}
}

func (s *Service) InvalidateRuntimeConfigCache() {
	s.cfgMu.Lock()
	s.cfgReady = false
	s.cfgLoadedAt = time.Time{}
	s.cfgCache = runtimeConfig{}
	s.cfgMu.Unlock()
}

func parseDomainDefaults(raw map[string]interface{}) map[string]domainRuntimeDefault {
	out := map[string]domainRuntimeDefault{}
	for domainKey, value := range raw {
		domainKey = strings.TrimSpace(strings.ToLower(domainKey))
		if domainKey == "" {
			continue
		}
		cfgMap, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		item := domainRuntimeDefault{}
		if topK := toInt(cfgMap["top_k"]); topK > 0 {
			item.TopK = topK
		}
		item.PreferredDocTypes = asStringSlice(cfgMap["preferred_doc_types"])
		out[domainKey] = item
	}
	return out
}

func toInt(v interface{}) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err == nil {
			return n
		}
	}
	return 0
}

func metadataMatchScore(meta map[string]interface{}, filters map[string]interface{}) float64 {
	if len(filters) == 0 {
		return 0
	}
	matches := 0.0
	total := 0.0
	for _, key := range []string{"legal_domain", "document_type", "effective_status", "document_number", "article_number"} {
		expected := pickString(filters, key)
		if expected == "" {
			continue
		}
		total++
		actual := pickString(meta, key)
		if strings.EqualFold(strings.TrimSpace(actual), strings.TrimSpace(expected)) {
			matches++
		}
	}
	if total == 0 {
		return 0
	}
	return matches / total
}

func articleBonus(meta map[string]interface{}, qu QueryUnderstandingResult) float64 {
	article := pickString(meta, "article_number", "article", "dieu")
	if article == "" {
		return 0
	}
	if v, ok := qu.Entities["article_number"]; ok {
		if strings.EqualFold(strings.TrimSpace(article), strings.TrimSpace(toString(v))) {
			return 1
		}
	}
	if strings.Contains(qu.NormalizedQuery, "dieu "+strings.TrimSpace(article)) {
		return 1
	}
	return 0
}

func lexicalOverlapScore(queryTokens, textTokens map[string]struct{}) float64 {
	if len(queryTokens) == 0 || len(textTokens) == 0 {
		return 0
	}
	inter := 0
	for token := range queryTokens {
		if _, ok := textTokens[token]; ok {
			inter++
		}
	}
	return float64(inter) / float64(len(queryTokens))
}

func tokenize(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, token := range strings.Fields(s) {
		token = strings.TrimSpace(token)
		if len(token) < 2 {
			continue
		}
		out[token] = struct{}{}
	}
	return out
}

func normalizeQuery(q string) string {
	q = strings.ToLower(strings.TrimSpace(q))
	replacer := strings.NewReplacer(
		"à", "a", "á", "a", "ạ", "a", "ả", "a", "ã", "a",
		"â", "a", "ầ", "a", "ấ", "a", "ậ", "a", "ẩ", "a", "ẫ", "a",
		"ă", "a", "ằ", "a", "ắ", "a", "ặ", "a", "ẳ", "a", "ẵ", "a",
		"è", "e", "é", "e", "ẹ", "e", "ẻ", "e", "ẽ", "e",
		"ê", "e", "ề", "e", "ế", "e", "ệ", "e", "ể", "e", "ễ", "e",
		"ì", "i", "í", "i", "ị", "i", "ỉ", "i", "ĩ", "i",
		"ò", "o", "ó", "o", "ọ", "o", "ỏ", "o", "õ", "o",
		"ô", "o", "ồ", "o", "ố", "o", "ộ", "o", "ổ", "o", "ỗ", "o",
		"ơ", "o", "ờ", "o", "ớ", "o", "ợ", "o", "ở", "o", "ỡ", "o",
		"ù", "u", "ú", "u", "ụ", "u", "ủ", "u", "ũ", "u",
		"ư", "u", "ừ", "u", "ứ", "u", "ự", "u", "ử", "u", "ữ", "u",
		"ỳ", "y", "ý", "y", "ỵ", "y", "ỷ", "y", "ỹ", "y",
		"đ", "d",
	)
	q = replacer.Replace(q)
	re := regexp.MustCompile(`[^a-z0-9\s]+`)
	q = re.ReplaceAllString(q, " ")
	return strings.Join(strings.Fields(q), " ")
}

func extractYear(text, pattern string) int { return extractInt(text, pattern) }

func extractInt(text, pattern string) int {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(m[1]))
	if err != nil {
		return 0
	}
	return n
}

func extractString(text, pattern string) string {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func copyMap(in map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func asStringSlice(v interface{}) []string {
	switch value := v.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return []string{strings.TrimSpace(value)}
	case []string:
		return dedupeStrings(value)
	case []interface{}:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if s := strings.TrimSpace(toString(item)); s != "" {
				out = append(out, s)
			}
		}
		return dedupeStrings(out)
	default:
		s := strings.TrimSpace(toString(value))
		if s == "" {
			return nil
		}
		return []string{s}
	}
}

func pickString(meta map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		v, ok := meta[key]
		if !ok || v == nil {
			continue
		}
		s := strings.TrimSpace(toString(v))
		if s != "" {
			return s
		}
	}
	return ""
}

func toString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return t.String()
	default:
		return ""
	}
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		v := strings.TrimSpace(strings.ToLower(raw))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func pickResultChunkIDs(results []Result, onlyAdjacent bool) []string {
	out := []string{}
	for _, r := range results {
		if onlyAdjacent && !r.IsAdjacent {
			continue
		}
		if !onlyAdjacent && r.IsAdjacent {
			continue
		}
		out = append(out, r.ChunkID)
	}
	return out
}

func decodeMetadata(raw []byte) map[string]interface{} {
	if len(raw) == 0 {
		return map[string]interface{}{}
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]interface{}{}
	}
	flat := map[string]interface{}{}
	for k, v := range out {
		flat[k] = v
	}
	for _, key := range []string{"document", "structural", "system"} {
		rawSection, ok := out[key]
		if !ok || rawSection == nil {
			continue
		}
		section, ok := rawSection.(map[string]interface{})
		if !ok {
			continue
		}
		for k, v := range section {
			if _, exists := flat[k]; !exists {
				flat[k] = v
			}
		}
	}
	return flat
}

func citationID(versionID string, idx int, text string) string {
	h := sha256.Sum256([]byte(versionID + ":" + strconv.Itoa(idx) + ":" + text))
	return hex.EncodeToString(h[:])
}

func ToDomainChunks(results []Result) []domain.Chunk {
	out := make([]domain.Chunk, 0, len(results))
	for _, r := range results {
		out = append(out, domain.Chunk{ID: r.ChunkID, DocumentVersionID: r.VersionID, Index: r.ChunkIndex, Text: r.Text})
	}
	return out
}

func (s *Service) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
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
