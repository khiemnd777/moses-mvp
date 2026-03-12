package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/khiemnd777/legal_api/core/embedding"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
)

const (
	DefaultSearchDebugTopK = 10
	MaxSearchDebugTopK     = 50
	DefaultReindexAllLimit = 500
	MaxReindexAllLimit     = 2000
	MaxQueryTextLength     = 2000
	MaxFilterValues        = 20
	MaxFilterValueLength   = 256
)

var (
	collectionNameRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	validIngestStatuses = map[string]struct{}{
		"":               {},
		"queued":         {},
		"pending":        {},
		"processing":     {},
		"done":           {},
		"failed":         {},
		"never_ingested": {},
	}
)

var (
	ErrInvalidCollectionName = errors.New("invalid collection name")
	ErrInvalidTopK           = errors.New("invalid top_k")
	ErrInvalidQueryLength    = errors.New("invalid query_text length")
	ErrMissingQueryText      = errors.New("query_text is required")
	ErrInvalidSearchFilters  = errors.New("invalid metadata filters")
	ErrInvalidDeleteRequest  = errors.New("invalid delete request")
	ErrInvalidReindexScope   = errors.New("invalid reindex scope")
)

type qdrantControlStore interface {
	GetChunksByIDs(ctx context.Context, ids []string) ([]domain.Chunk, error)
	GetDocumentVersion(ctx context.Context, id string) (domain.DocumentVersion, error)
	GetDocument(ctx context.Context, id string) (domain.Document, error)
	ListDocumentVersionIDsByDocument(ctx context.Context, documentID string) ([]string, error)
	EnqueueIngestJob(ctx context.Context, documentVersionID string) (domain.IngestJob, bool, error)
	ListDocumentVersionIDsForReindex(ctx context.Context, scope infra.ReindexScopeQuery) ([]string, error)
}

type qdrantControlClient interface {
	GetCollectionInfo(ctx context.Context) (infra.CollectionInfo, error)
	ListCollections(ctx context.Context) ([]infra.CollectionListItem, error)
	GetCollectionDetails(ctx context.Context, collection string) (infra.CollectionDetails, error)
	SearchInCollection(ctx context.Context, collection string, vector []float64, limit int, filter *infra.SearchFilter) ([]infra.SearchResult, error)
	CountPoints(ctx context.Context, collection string, filter *infra.Filter) (int64, bool, error)
	DeleteByFilter(ctx context.Context, collection string, filter infra.Filter) error
}

type embeddingClient interface {
	Embed(ctx context.Context, inputs []string) ([][]float64, error)
}

type vectorHealthChecker func(ctx context.Context, mode infra.VectorScanMode, chunkBatchSize, vectorBatchSize, maxChunks, maxVectors int, maxDuration time.Duration) (infra.VectorConsistencyReport, error)

type QdrantControlPlaneService struct {
	Store             qdrantControlStore
	Qdrant            qdrantControlClient
	Embedder          embeddingClient
	Logger            *slog.Logger
	expectedDimension int
	defaultCollection string
	checkVectorHealth vectorHealthChecker
}

type SearchDebugInput struct {
	QueryText           string
	TopK                int
	Collection          string
	IncludePayload      bool
	IncludeChunkPreview bool
	Filters             SearchDebugFilter
}

type SearchDebugFilter struct {
	LegalDomain     []string
	DocumentType    []string
	EffectiveStatus []string
	DocumentNumber  []string
	ArticleNumber   []string
}

type SearchDebugHit struct {
	Rank    int
	PointID string
	Score   float64
	Payload map[string]interface{}
	Chunk   *domain.Chunk
}

type SearchDebugResult struct {
	QueryHash     string
	Collection    string
	TopK          int
	FilterSummary string
	Hits          []SearchDebugHit
}

type DeleteByFilterInput struct {
	Collection string
	Filter     infra.Filter
	Confirm    bool
	DryRun     bool
}

type DeleteByFilterResult struct {
	Collection     string
	FilterSummary  string
	EstimatedScope *int64
	ScopeEstimated bool
}

type ReindexDocumentInput struct {
	DocumentID        string
	DocumentVersionID string
}

type ReindexAllInput struct {
	Confirm     bool
	DocTypeCode string
	Collection  string
	Status      string
	Limit       int
	Reason      string
}

func NewQdrantControlPlaneService(store *infra.Store, qdrant *infra.QdrantClient, embedder *embedding.Client, logger *slog.Logger) *QdrantControlPlaneService {
	expected := 0
	defaultCollection := ""
	if qdrant != nil {
		defaultCollection = strings.TrimSpace(qdrant.Collection)
	}
	if embedder != nil {
		if dim, err := embedding.ExpectedDimensions(embedder.Model); err == nil {
			expected = dim
		}
	}
	return NewQdrantControlPlaneServiceWithDeps(store, qdrant, embedder, defaultCollection, expected, logger, func(ctx context.Context, mode infra.VectorScanMode, chunkBatchSize, vectorBatchSize, maxChunks, maxVectors int, maxDuration time.Duration) (infra.VectorConsistencyReport, error) {
		opts := infra.VectorConsistencyOptions{
			Mode:            mode,
			ChunkBatchSize:  chunkBatchSize,
			VectorBatchSize: vectorBatchSize,
			MaxChunks:       maxChunks,
			MaxVectors:      maxVectors,
			MaxDuration:     maxDuration,
		}
		return infra.CheckVectorConsistencyWithOptions(ctx, store, qdrant, expected, opts)
	})
}

func NewQdrantControlPlaneServiceWithDeps(
	store qdrantControlStore,
	qdrant qdrantControlClient,
	embedder embeddingClient,
	defaultCollection string,
	expectedDimension int,
	logger *slog.Logger,
	checker vectorHealthChecker,
) *QdrantControlPlaneService {
	return &QdrantControlPlaneService{
		Store:             store,
		Qdrant:            qdrant,
		Embedder:          embedder,
		Logger:            logger,
		expectedDimension: expectedDimension,
		defaultCollection: strings.TrimSpace(defaultCollection),
		checkVectorHealth: checker,
	}
}

func (s *QdrantControlPlaneService) ExpectedDimension(ctx context.Context) int {
	if s.expectedDimension > 0 {
		return s.expectedDimension
	}
	if s.Qdrant == nil {
		return 0
	}
	info, err := s.Qdrant.GetCollectionInfo(ctx)
	if err != nil {
		return 0
	}
	return info.VectorSize
}

func (s *QdrantControlPlaneService) ResolveCollection(name string) (string, error) {
	collection := strings.TrimSpace(name)
	if collection == "" {
		collection = s.defaultCollection
	}
	if !collectionNameRegex.MatchString(collection) {
		return "", ErrInvalidCollectionName
	}
	return collection, nil
}

func (s *QdrantControlPlaneService) ListCollections(ctx context.Context) ([]infra.CollectionDetails, error) {
	items, err := s.Qdrant.ListCollections(ctx)
	if err != nil {
		return nil, err
	}
	details := make([]infra.CollectionDetails, 0, len(items))
	for _, item := range items {
		detail, err := s.Qdrant.GetCollectionDetails(ctx, item.Name)
		if err != nil {
			return nil, err
		}
		details = append(details, detail)
	}
	sort.SliceStable(details, func(i, j int) bool { return details[i].Name < details[j].Name })
	return details, nil
}

func (s *QdrantControlPlaneService) GetCollection(ctx context.Context, name string) (infra.CollectionDetails, bool, error) {
	collection, err := s.ResolveCollection(name)
	if err != nil {
		return infra.CollectionDetails{}, false, err
	}
	detail, err := s.Qdrant.GetCollectionDetails(ctx, collection)
	if err != nil {
		if infra.IsQdrantNotFoundError(err) {
			return infra.CollectionDetails{Name: collection}, false, nil
		}
		return infra.CollectionDetails{}, false, err
	}
	return detail, true, nil
}

func (s *QdrantControlPlaneService) SearchDebug(ctx context.Context, in SearchDebugInput) (SearchDebugResult, error) {
	queryText := strings.TrimSpace(in.QueryText)
	if queryText == "" {
		return SearchDebugResult{}, ErrMissingQueryText
	}
	if len([]rune(queryText)) > MaxQueryTextLength {
		return SearchDebugResult{}, ErrInvalidQueryLength
	}
	if err := validateSearchDebugFilters(in.Filters); err != nil {
		return SearchDebugResult{}, err
	}
	topK := in.TopK
	if topK <= 0 {
		topK = DefaultSearchDebugTopK
	}
	if topK < 1 || topK > MaxSearchDebugTopK {
		return SearchDebugResult{}, ErrInvalidTopK
	}
	collection, err := s.ResolveCollection(in.Collection)
	if err != nil {
		return SearchDebugResult{}, err
	}
	if s.Embedder == nil {
		return SearchDebugResult{}, errors.New("embedder is not configured")
	}
	vectors, err := s.Embedder.Embed(ctx, []string{queryText})
	if err != nil {
		return SearchDebugResult{}, err
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return SearchDebugResult{}, errors.New("embedding model returned empty vector")
	}
	matches, err := s.Qdrant.SearchInCollection(ctx, collection, vectors[0], topK, toSearchFilter(in.Filters))
	if err != nil {
		return SearchDebugResult{}, err
	}
	chunkIDs := make([]string, 0, len(matches))
	for _, m := range matches {
		if strings.TrimSpace(m.ChunkID) != "" {
			chunkIDs = append(chunkIDs, m.ChunkID)
		}
	}
	chunkMap := map[string]domain.Chunk{}
	if len(chunkIDs) > 0 {
		chunks, err := s.Store.GetChunksByIDs(ctx, chunkIDs)
		if err != nil {
			return SearchDebugResult{}, err
		}
		for _, chunk := range chunks {
			chunkMap[chunk.ID] = chunk
		}
	}
	hits := make([]SearchDebugHit, 0, len(matches))
	for idx, m := range matches {
		hit := SearchDebugHit{
			Rank:    idx + 1,
			PointID: m.ID,
			Score:   m.Score,
		}
		if in.IncludePayload {
			hit.Payload = m.Payload
		}
		if chunk, ok := chunkMap[m.ChunkID]; ok {
			cp := chunk
			if !in.IncludeChunkPreview {
				cp.Text = ""
			}
			hit.Chunk = &cp
		}
		hits = append(hits, hit)
	}
	return SearchDebugResult{
		QueryHash:     queryHash(queryText),
		Collection:    collection,
		TopK:          topK,
		FilterSummary: summarizeSearchFilter(in.Filters),
		Hits:          hits,
	}, nil
}

func (s *QdrantControlPlaneService) VectorHealth(ctx context.Context, mode infra.VectorScanMode, chunkBatchSize, vectorBatchSize, maxChunks, maxVectors int, maxDuration time.Duration) (infra.VectorConsistencyReport, error) {
	if s.checkVectorHealth == nil {
		return infra.VectorConsistencyReport{}, errors.New("vector health checker is not configured")
	}
	return s.checkVectorHealth(ctx, mode, chunkBatchSize, vectorBatchSize, maxChunks, maxVectors, maxDuration)
}

func (s *QdrantControlPlaneService) DeleteByFilter(ctx context.Context, in DeleteByFilterInput) (DeleteByFilterResult, error) {
	if !in.DryRun && !in.Confirm {
		return DeleteByFilterResult{}, ErrInvalidDeleteRequest
	}
	if in.DryRun && in.Confirm {
		return DeleteByFilterResult{}, ErrInvalidDeleteRequest
	}
	collection, err := s.ResolveCollection(in.Collection)
	if err != nil {
		return DeleteByFilterResult{}, err
	}
	if err := infra.ValidateDeleteFilter(in.Filter); err != nil {
		return DeleteByFilterResult{}, err
	}

	estimatedCount, estimated, err := s.Qdrant.CountPoints(ctx, collection, &in.Filter)
	if err != nil {
		return DeleteByFilterResult{}, err
	}
	result := DeleteByFilterResult{
		Collection:     collection,
		FilterSummary:  infra.SummarizeFilter(in.Filter),
		ScopeEstimated: estimated,
	}
	if estimated {
		result.EstimatedScope = &estimatedCount
	}
	if in.DryRun {
		return result, nil
	}
	if err := s.Qdrant.DeleteByFilter(ctx, collection, in.Filter); err != nil {
		return DeleteByFilterResult{}, err
	}
	return result, nil
}

func (s *QdrantControlPlaneService) EnqueueReindexDocument(ctx context.Context, in ReindexDocumentInput) ([]domain.IngestJob, []bool, error) {
	documentID := strings.TrimSpace(in.DocumentID)
	documentVersionID := strings.TrimSpace(in.DocumentVersionID)
	if documentID == "" && documentVersionID == "" {
		return nil, nil, ErrInvalidReindexScope
	}
	if documentID != "" && documentVersionID != "" {
		return nil, nil, ErrInvalidReindexScope
	}
	versionIDs := []string{}
	if documentVersionID != "" {
		if _, err := s.Store.GetDocumentVersion(ctx, documentVersionID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil, ErrInvalidReindexScope
			}
			return nil, nil, err
		}
		versionIDs = append(versionIDs, documentVersionID)
	} else {
		if _, err := s.Store.GetDocument(ctx, documentID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil, ErrInvalidReindexScope
			}
			return nil, nil, err
		}
		ids, err := s.Store.ListDocumentVersionIDsByDocument(ctx, documentID)
		if err != nil {
			return nil, nil, err
		}
		versionIDs = ids
	}
	jobs := make([]domain.IngestJob, 0, len(versionIDs))
	createdFlags := make([]bool, 0, len(versionIDs))
	for _, versionID := range versionIDs {
		job, created, err := s.Store.EnqueueIngestJob(ctx, versionID)
		if err != nil {
			return nil, nil, err
		}
		jobs = append(jobs, job)
		createdFlags = append(createdFlags, created)
	}
	return jobs, createdFlags, nil
}

func (s *QdrantControlPlaneService) EnqueueReindexAll(ctx context.Context, in ReindexAllInput) ([]domain.IngestJob, []bool, error) {
	if !in.Confirm {
		return nil, nil, ErrInvalidReindexScope
	}
	if strings.TrimSpace(in.Reason) == "" {
		return nil, nil, ErrInvalidReindexScope
	}
	if _, ok := validIngestStatuses[in.Status]; !ok {
		return nil, nil, ErrInvalidReindexScope
	}
	if strings.TrimSpace(in.Collection) != "" && strings.TrimSpace(in.Collection) != s.defaultCollection {
		return nil, nil, ErrInvalidReindexScope
	}
	limit := in.Limit
	if limit <= 0 {
		limit = DefaultReindexAllLimit
	}
	if limit > MaxReindexAllLimit {
		limit = MaxReindexAllLimit
	}
	versionIDs, err := s.Store.ListDocumentVersionIDsForReindex(ctx, infra.ReindexScopeQuery{
		DocTypeCode: strings.TrimSpace(in.DocTypeCode),
		Status:      strings.TrimSpace(in.Status),
		Limit:       limit,
	})
	if err != nil {
		return nil, nil, err
	}
	jobs := make([]domain.IngestJob, 0, len(versionIDs))
	createdFlags := make([]bool, 0, len(versionIDs))
	for _, versionID := range versionIDs {
		job, created, err := s.Store.EnqueueIngestJob(ctx, versionID)
		if err != nil {
			return nil, nil, err
		}
		jobs = append(jobs, job)
		createdFlags = append(createdFlags, created)
	}
	return jobs, createdFlags, nil
}

func toSearchFilter(in SearchDebugFilter) *infra.SearchFilter {
	clean := func(items []string) []string {
		out := make([]string, 0, len(items))
		seen := map[string]struct{}{}
		for _, item := range items {
			v := strings.TrimSpace(item)
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
	filter := &infra.SearchFilter{
		LegalDomain:     clean(in.LegalDomain),
		DocumentType:    clean(in.DocumentType),
		EffectiveStatus: clean(in.EffectiveStatus),
		DocumentNumber:  clean(in.DocumentNumber),
		ArticleNumber:   clean(in.ArticleNumber),
	}
	if len(filter.LegalDomain) == 0 &&
		len(filter.DocumentType) == 0 &&
		len(filter.EffectiveStatus) == 0 &&
		len(filter.DocumentNumber) == 0 &&
		len(filter.ArticleNumber) == 0 {
		return nil
	}
	return filter
}

func validateSearchDebugFilters(in SearchDebugFilter) error {
	validate := func(values []string) error {
		if len(values) > MaxFilterValues {
			return ErrInvalidSearchFilters
		}
		for _, item := range values {
			v := strings.TrimSpace(item)
			if v == "" {
				continue
			}
			if len([]rune(v)) > MaxFilterValueLength {
				return ErrInvalidSearchFilters
			}
		}
		return nil
	}
	if err := validate(in.LegalDomain); err != nil {
		return err
	}
	if err := validate(in.DocumentType); err != nil {
		return err
	}
	if err := validate(in.EffectiveStatus); err != nil {
		return err
	}
	if err := validate(in.DocumentNumber); err != nil {
		return err
	}
	if err := validate(in.ArticleNumber); err != nil {
		return err
	}
	return nil
}

func summarizeSearchFilter(in SearchDebugFilter) string {
	parts := make([]string, 0, 5)
	appendPart := func(key string, values []string) {
		n := len(values)
		if n == 0 {
			return
		}
		parts = append(parts, fmt.Sprintf("%s:%d", key, n))
	}
	appendPart("legal_domain", in.LegalDomain)
	appendPart("document_type", in.DocumentType)
	appendPart("effective_status", in.EffectiveStatus)
	appendPart("document_number", in.DocumentNumber)
	appendPart("article_number", in.ArticleNumber)
	if len(parts) == 0 {
		return "none"
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func queryHash(query string) string {
	sum := sha256.Sum256([]byte(query))
	return hex.EncodeToString(sum[:8])
}

func BuildPayloadSummary(payloadSchema map[string]interface{}) []map[string]string {
	if len(payloadSchema) == 0 {
		return nil
	}
	keys := make([]string, 0, len(payloadSchema))
	for k := range payloadSchema {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]map[string]string, 0, len(keys))
	for _, key := range keys {
		v := payloadSchema[key]
		typ := ""
		if m, ok := v.(map[string]interface{}); ok {
			if dt, ok := m["data_type"].(string); ok {
				typ = dt
			}
		}
		if typ == "" {
			b, _ := json.Marshal(v)
			typ = string(b)
		}
		out = append(out, map[string]string{"key": key, "type": typ})
	}
	return out
}

func TruncateText(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max] + "..."
}
