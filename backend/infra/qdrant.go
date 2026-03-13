package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	qdrantMaxRetries              = 3
	qdrantBaseBackoff             = 200 * time.Millisecond
	qdrantDeleteMaxAnyValues      = 1000
	qdrantMaxEstimatedDeleteScope = 20000
	qdrantDefaultUpsertMaxBytes   = 30 * 1024 * 1024
)

var allowedDeleteFilterFields = map[string]struct{}{
	"document_version_id": {},
	"chunk_id":            {},
	"document_id":         {},
}

type QdrantClient struct {
	URL                   string
	Collection            string
	HTTP                  *http.Client
	Logger                *slog.Logger
	UpsertPayloadMaxBytes int
}

func NewQdrantClient(url, collection string) *QdrantClient {
	return &QdrantClient{
		URL:                   url,
		Collection:            collection,
		HTTP:                  &http.Client{Timeout: 30 * time.Second},
		Logger:                slog.Default(),
		UpsertPayloadMaxBytes: qdrantDefaultUpsertMaxBytes,
	}
}

func (c *QdrantClient) logger() *slog.Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return slog.Default()
}

type createCollectionRequest struct {
	Vectors struct {
		Size     int    `json:"size"`
		Distance string `json:"distance"`
	} `json:"vectors"`
}

type CollectionInfo struct {
	VectorSize int
	Distance   string
}

func (c *QdrantClient) EnsureCollection(ctx context.Context, vectorSize int) error {
	payload := createCollectionRequest{}
	payload.Vectors.Size = vectorSize
	payload.Vectors.Distance = "Cosine"
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.URL+"/collections/"+c.Collection, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant ensure collection failed: status=%s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}
	return c.ValidateCollectionDimension(ctx, vectorSize)
}

func (c *QdrantClient) ValidateCollectionDimension(ctx context.Context, expected int) error {
	info, err := c.GetCollectionInfo(ctx)
	if err != nil {
		return err
	}
	if info.VectorSize != expected {
		return fmt.Errorf("qdrant collection dimension mismatch: collection=%s expected=%d actual=%d", c.Collection, expected, info.VectorSize)
	}
	return nil
}

func (c *QdrantClient) GetCollectionInfo(ctx context.Context) (CollectionInfo, error) {
	info, err := c.GetCollectionDetails(ctx, c.Collection)
	if err != nil {
		return CollectionInfo{}, err
	}
	return CollectionInfo{
		VectorSize: info.VectorSize,
		Distance:   info.Distance,
	}, nil
}

type CollectionListItem struct {
	Name string `json:"name"`
}

type CollectionDetails struct {
	Name                string                 `json:"name"`
	Status              string                 `json:"status,omitempty"`
	PointsCount         *int64                 `json:"points_count,omitempty"`
	VectorsCount        *int64                 `json:"vectors_count,omitempty"`
	IndexedVectorsCount *int64                 `json:"indexed_vectors_count,omitempty"`
	VectorSize          int                    `json:"vector_size,omitempty"`
	Distance            string                 `json:"distance,omitempty"`
	PayloadSchema       map[string]interface{} `json:"payload_schema,omitempty"`
}

func (c *QdrantClient) ListCollections(ctx context.Context) ([]CollectionListItem, error) {
	var out struct {
		Result struct {
			Collections []CollectionListItem `json:"collections"`
		} `json:"result"`
	}
	if err := c.doWithRetry(ctx, "list_collections", "_meta", func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL+"/collections", nil)
		if err != nil {
			return err
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return qdrantHTTPError{Op: "list_collections", StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
		}
		return json.NewDecoder(resp.Body).Decode(&out)
	}); err != nil {
		return nil, err
	}
	return out.Result.Collections, nil
}

func (c *QdrantClient) GetCollectionDetails(ctx context.Context, collection string) (CollectionDetails, error) {
	if strings.TrimSpace(collection) == "" {
		collection = c.Collection
	}
	var out struct {
		Result struct {
			Status              string                 `json:"status"`
			PointsCount         *int64                 `json:"points_count"`
			VectorsCount        *int64                 `json:"vectors_count"`
			IndexedVectorsCount *int64                 `json:"indexed_vectors_count"`
			PayloadSchema       map[string]interface{} `json:"payload_schema"`
			Config              struct {
				Params struct {
					Vectors json.RawMessage `json:"vectors"`
				} `json:"params"`
			} `json:"config"`
		} `json:"result"`
	}
	if err := c.doWithRetry(ctx, "get_collection", collection, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL+"/collections/"+collection, nil)
		if err != nil {
			return err
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return qdrantHTTPError{Op: "get_collection", StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
		}
		return json.NewDecoder(resp.Body).Decode(&out)
	}); err != nil {
		return CollectionDetails{}, err
	}
	size, distance, err := extractVectorConfig(out.Result.Config.Params.Vectors)
	if err != nil {
		return CollectionDetails{}, err
	}
	return CollectionDetails{
		Name:                collection,
		Status:              out.Result.Status,
		PointsCount:         out.Result.PointsCount,
		VectorsCount:        out.Result.VectorsCount,
		IndexedVectorsCount: out.Result.IndexedVectorsCount,
		VectorSize:          size,
		Distance:            distance,
		PayloadSchema:       out.Result.PayloadSchema,
	}, nil
}

func extractVectorSize(raw json.RawMessage) (int, error) {
	size, _, err := extractVectorConfig(raw)
	return size, err
}

func extractVectorConfig(raw json.RawMessage) (int, string, error) {
	if len(raw) == 0 {
		return 0, "", errors.New("qdrant collection vectors config missing")
	}
	var single struct {
		Size     int    `json:"size"`
		Distance string `json:"distance"`
	}
	if err := json.Unmarshal(raw, &single); err == nil && single.Size > 0 {
		return single.Size, single.Distance, nil
	}
	var named map[string]struct {
		Size     int    `json:"size"`
		Distance string `json:"distance"`
	}
	if err := json.Unmarshal(raw, &named); err == nil {
		for _, cfg := range named {
			if cfg.Size > 0 {
				return cfg.Size, cfg.Distance, nil
			}
		}
	}
	return 0, "", fmt.Errorf("unable to parse qdrant vector size from config: %s", string(raw))
}

type PointInput struct {
	ID      string                 `json:"id"`
	Vector  []float64              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

type upsertRequest struct {
	Points []PointInput `json:"points"`
}

func (c *QdrantClient) Upsert(ctx context.Context, points []PointInput) error {
	batches, err := splitUpsertPoints(points, c.upsertPayloadLimit())
	if err != nil {
		return err
	}
	for _, batch := range batches {
		payload := upsertRequest{Points: batch}
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if err := c.doWithRetry(ctx, "upsert", c.Collection, func() error {
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.URL+"/collections/"+c.Collection+"/points?wait=true", bytes.NewReader(b))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := c.HTTP.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 300 {
				body, _ := io.ReadAll(resp.Body)
				return qdrantHTTPError{Op: "upsert", StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (c *QdrantClient) upsertPayloadLimit() int {
	if c.UpsertPayloadMaxBytes > 0 {
		return c.UpsertPayloadMaxBytes
	}
	return qdrantDefaultUpsertMaxBytes
}

func splitUpsertPoints(points []PointInput, maxPayloadBytes int) ([][]PointInput, error) {
	if len(points) == 0 {
		return nil, nil
	}
	if maxPayloadBytes <= 16 {
		return nil, fmt.Errorf("invalid upsert payload size limit: %d", maxPayloadBytes)
	}

	const envelopeOverhead = 16 // conservative for {"points":[ ... ]}
	maxPointsBytes := maxPayloadBytes - envelopeOverhead

	batches := make([][]PointInput, 0, len(points)/128+1)
	current := make([]PointInput, 0, 128)
	currentBytes := 0
	for i, point := range points {
		pb, err := json.Marshal(point)
		if err != nil {
			return nil, err
		}
		pointBytes := len(pb)
		if pointBytes > maxPointsBytes {
			return nil, fmt.Errorf("single point exceeds qdrant upsert payload limit: index=%d size=%d limit=%d", i, pointBytes+envelopeOverhead, maxPayloadBytes)
		}

		// Account for the comma separator between points in the array.
		needed := pointBytes
		if len(current) > 0 {
			needed++
		}
		if currentBytes+needed > maxPointsBytes {
			batches = append(batches, current)
			current = make([]PointInput, 0, 128)
			currentBytes = 0
			needed = pointBytes
		}
		current = append(current, point)
		currentBytes += needed
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches, nil
}

type searchRequest struct {
	Vector      []float64 `json:"vector"`
	Limit       int       `json:"limit"`
	WithPayload bool      `json:"with_payload"`
	Filter      *Filter   `json:"filter,omitempty"`
}

type searchResponse struct {
	Result []struct {
		ID      string                 `json:"id"`
		Score   float64                `json:"score"`
		Payload map[string]interface{} `json:"payload"`
	} `json:"result"`
}

type SearchResult struct {
	ID      string
	ChunkID string
	Score   float64
	Payload map[string]interface{}
}

type SearchFilter struct {
	LegalDomain     []string
	DocumentType    []string
	EffectiveStatus []string
	DocumentNumber  []string
	ArticleNumber   []string
}

type Filter struct {
	Must []FieldCondition `json:"must,omitempty"`
}

type FieldCondition struct {
	Key   string     `json:"key"`
	Match FieldMatch `json:"match"`
}

type FieldMatch struct {
	Value interface{} `json:"value,omitempty"`
	Any   []string    `json:"any,omitempty"`
}

func (c *QdrantClient) Search(ctx context.Context, vector []float64, limit int, filter *SearchFilter) ([]SearchResult, error) {
	return c.SearchInCollection(ctx, c.Collection, vector, limit, filter)
}

func (c *QdrantClient) SearchInCollection(ctx context.Context, collection string, vector []float64, limit int, filter *SearchFilter) ([]SearchResult, error) {
	if strings.TrimSpace(collection) == "" {
		collection = c.Collection
	}
	payload := searchRequest{Vector: vector, Limit: limit, WithPayload: true}
	if qf := toQFilter(filter); qf != nil {
		payload.Filter = qf
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var out searchResponse
	if err := c.doWithRetry(ctx, "search", collection, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/collections/"+collection+"/points/search", bytes.NewReader(b))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return qdrantHTTPError{Op: "search", StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
		}
		var decoded searchResponse
		if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
			return err
		}
		out = decoded
		return nil
	}); err != nil {
		return nil, err
	}
	results := make([]SearchResult, 0, len(out.Result))
	for _, r := range out.Result {
		chunkID, _ := r.Payload["chunk_id"].(string)
		results = append(results, SearchResult{ID: r.ID, ChunkID: chunkID, Score: r.Score, Payload: r.Payload})
	}
	return results, nil
}

func toQFilter(filter *SearchFilter) *Filter {
	if filter == nil {
		return nil
	}
	must := make([]FieldCondition, 0, 5)
	appendFilter := func(key string, values []string) {
		switch len(values) {
		case 0:
			return
		case 1:
			must = append(must, FieldCondition{Key: key, Match: FieldMatch{Value: values[0]}})
		default:
			must = append(must, FieldCondition{Key: key, Match: FieldMatch{Any: values}})
		}
	}
	appendFilter("legal_domain", filter.LegalDomain)
	appendFilter("document_type", filter.DocumentType)
	appendFilter("effective_status", filter.EffectiveStatus)
	appendFilter("document_number", filter.DocumentNumber)
	appendFilter("article_number", filter.ArticleNumber)
	if len(must) == 0 {
		return nil
	}
	return &Filter{Must: must}
}

type getPointsRequest struct {
	IDs         []string `json:"ids"`
	WithPayload bool     `json:"with_payload"`
	WithVector  bool     `json:"with_vector"`
}

type getPointsResponse struct {
	Result []struct {
		ID      string                 `json:"id"`
		Payload map[string]interface{} `json:"payload"`
	} `json:"result"`
}

func (c *QdrantClient) GetPayloadByPointID(ctx context.Context, pointID string) (map[string]interface{}, bool, error) {
	payload := getPointsRequest{IDs: []string{pointID}, WithPayload: true, WithVector: false}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, false, err
	}
	var out getPointsResponse
	if err := c.doWithRetry(ctx, "get_payload_by_point_id", c.Collection, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/collections/"+c.Collection+"/points", bytes.NewReader(b))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return qdrantHTTPError{Op: "get_payload_by_point_id", StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
		}
		var decoded getPointsResponse
		if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
			return err
		}
		out = decoded
		return nil
	}); err != nil {
		return nil, false, err
	}
	if len(out.Result) == 0 {
		return nil, false, nil
	}
	return out.Result[0].Payload, true, nil
}

func (c *QdrantClient) GetExistingPointIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	if len(ids) == 0 {
		return map[string]struct{}{}, nil
	}
	payload := getPointsRequest{IDs: ids, WithPayload: false, WithVector: false}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var out getPointsResponse
	if err := c.doWithRetry(ctx, "get_points", c.Collection, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/collections/"+c.Collection+"/points", bytes.NewReader(b))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return qdrantHTTPError{Op: "get_points", StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
		}
		var decoded getPointsResponse
		if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
			return err
		}
		out = decoded
		return nil
	}); err != nil {
		return nil, err
	}
	existing := make(map[string]struct{}, len(out.Result))
	for _, item := range out.Result {
		existing[item.ID] = struct{}{}
	}
	return existing, nil
}

type deletePointsRequest struct {
	Points []string `json:"points"`
}

func (c *QdrantClient) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	payload := deletePointsRequest{Points: ids}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.doDeletePayloadWithRetry(ctx, "delete_points", c.Collection, b)
}

type deleteFilterRequest struct {
	Filter *Filter `json:"filter"`
}

func (c *QdrantClient) DeleteByFilter(ctx context.Context, collection string, filter Filter) error {
	if collection == "" {
		return errors.New("qdrant delete by filter requires collection")
	}
	if err := validateDeleteFilter(filter); err != nil {
		return err
	}
	estimatedCount, estimated, err := c.CountPoints(ctx, collection, &filter)
	if err != nil {
		c.logger().Warn("qdrant_delete_by_filter_estimate_failed",
			slog.String("collection", collection),
			slog.String("error", err.Error()),
		)
	}
	if err == nil && !isStrongDeleteScope(filter) && estimatedCount > qdrantMaxEstimatedDeleteScope {
		return fmt.Errorf("qdrant delete by filter rejected broad scope: estimated_count=%d limit=%d", estimatedCount, qdrantMaxEstimatedDeleteScope)
	}

	payload := deleteFilterRequest{Filter: &filter}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	started := time.Now()
	logger := c.logger().With(
		slog.String("collection", collection),
		slog.String("filter_summary", summarizeFilter(filter)),
		slog.Bool("scope_estimated", estimated),
		slog.Int64("estimated_count", estimatedCount),
	)
	logger.Info("qdrant_delete_by_filter_started")
	if err := c.doDeletePayloadWithRetry(ctx, "delete_by_filter", collection, b); err != nil {
		logger.Error("qdrant_delete_by_filter_failed", slog.String("error", err.Error()), slog.Duration("duration", time.Since(started)))
		return err
	}
	logger.Info("qdrant_delete_by_filter_completed", slog.Duration("duration", time.Since(started)))
	return nil
}

func validateDeleteFilter(filter Filter) error {
	if len(filter.Must) == 0 {
		return errors.New("qdrant delete by filter rejected empty filter")
	}
	hasWhitelisted := false
	for _, cond := range filter.Must {
		if _, ok := allowedDeleteFilterFields[cond.Key]; !ok {
			return fmt.Errorf("qdrant delete by filter rejected field: %s", cond.Key)
		}
		hasWhitelisted = true
		hasValue := cond.Match.Value != nil
		hasAny := len(cond.Match.Any) > 0
		if !hasValue && !hasAny {
			return fmt.Errorf("qdrant delete by filter requires a value for key: %s", cond.Key)
		}
		if hasValue && hasAny {
			return fmt.Errorf("qdrant delete by filter has ambiguous match for key: %s", cond.Key)
		}
		if s, ok := cond.Match.Value.(string); ok && strings.TrimSpace(s) == "" {
			return fmt.Errorf("qdrant delete by filter rejected empty value for key: %s", cond.Key)
		}
		if len(cond.Match.Any) > qdrantDeleteMaxAnyValues {
			return fmt.Errorf("qdrant delete by filter rejected too many values for key: %s", cond.Key)
		}
	}
	if !hasWhitelisted {
		return errors.New("qdrant delete by filter requires at least one whitelisted field")
	}
	return nil
}

func ValidateDeleteFilter(filter Filter) error {
	return validateDeleteFilter(filter)
}

func isStrongDeleteScope(filter Filter) bool {
	for _, cond := range filter.Must {
		if cond.Key == "document_version_id" || cond.Key == "chunk_id" {
			return true
		}
	}
	return false
}

func summarizeFilter(filter Filter) string {
	parts := make([]string, 0, len(filter.Must))
	for _, cond := range filter.Must {
		if len(cond.Match.Any) > 0 {
			parts = append(parts, fmt.Sprintf("%s:any(%d)", cond.Key, len(cond.Match.Any)))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:value", cond.Key))
	}
	return strings.Join(parts, ",")
}

func SummarizeFilter(filter Filter) string {
	return summarizeFilter(filter)
}

func (c *QdrantClient) doDeletePayloadWithRetry(ctx context.Context, op, collection string, payload []byte) error {
	return c.doWithRetry(ctx, op, collection, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/collections/"+collection+"/points/delete?wait=true", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return qdrantHTTPError{Op: op, StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
		}
		return nil
	})
}

type countRequest struct {
	Filter *Filter `json:"filter,omitempty"`
	Exact  bool    `json:"exact"`
}

func (c *QdrantClient) CountPoints(ctx context.Context, collection string, filter *Filter) (int64, bool, error) {
	if collection == "" {
		collection = c.Collection
	}
	payload := countRequest{Filter: filter, Exact: false}
	b, err := json.Marshal(payload)
	if err != nil {
		return 0, false, err
	}
	var out struct {
		Result struct {
			Count int64 `json:"count"`
		} `json:"result"`
	}
	if err := c.doWithRetry(ctx, "count", collection, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/collections/"+collection+"/points/count", bytes.NewReader(b))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return qdrantHTTPError{Op: "count", StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
		}
		return json.NewDecoder(resp.Body).Decode(&out)
	}); err != nil {
		return 0, false, err
	}
	return out.Result.Count, true, nil
}

type scrollRequest struct {
	Filter      *Filter     `json:"filter,omitempty"`
	Limit       int         `json:"limit"`
	Offset      interface{} `json:"offset,omitempty"`
	WithPayload bool        `json:"with_payload"`
	WithVector  bool        `json:"with_vector"`
}

type scrollResponse struct {
	Result struct {
		Points []struct {
			ID      string                 `json:"id"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"points"`
		NextPageOffset interface{} `json:"next_page_offset"`
	} `json:"result"`
}

type PointPayload struct {
	ID      string
	Payload map[string]interface{}
}

func (c *QdrantClient) IteratePointPayloads(ctx context.Context, collection string, filter *Filter, batchSize int, maxPoints int, fn func([]PointPayload) error) (int, error) {
	if collection == "" {
		collection = c.Collection
	}
	if batchSize <= 0 {
		batchSize = 256
	}
	offset := interface{}(nil)
	scanned := 0
	for {
		if maxPoints > 0 && scanned >= maxPoints {
			return scanned, nil
		}
		limit := batchSize
		if maxPoints > 0 && scanned+limit > maxPoints {
			limit = maxPoints - scanned
		}
		payload := scrollRequest{Filter: filter, Limit: limit, Offset: offset, WithPayload: true, WithVector: false}
		b, err := json.Marshal(payload)
		if err != nil {
			return scanned, err
		}
		var respBody scrollResponse
		if err := c.doWithRetry(ctx, "scroll", collection, func() error {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/collections/"+collection+"/points/scroll", bytes.NewReader(b))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := c.HTTP.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 300 {
				body, _ := io.ReadAll(resp.Body)
				return qdrantHTTPError{Op: "scroll", StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
			}
			return json.NewDecoder(resp.Body).Decode(&respBody)
		}); err != nil {
			return scanned, err
		}
		batch := make([]PointPayload, 0, len(respBody.Result.Points))
		for _, point := range respBody.Result.Points {
			batch = append(batch, PointPayload{ID: point.ID, Payload: point.Payload})
		}
		if len(batch) == 0 {
			return scanned, nil
		}
		if fn != nil {
			if err := fn(batch); err != nil {
				return scanned, err
			}
		}
		scanned += len(batch)
		if respBody.Result.NextPageOffset == nil {
			return scanned, nil
		}
		offset = respBody.Result.NextPageOffset
	}
}

func (c *QdrantClient) ListPointPayloads(ctx context.Context, filter *Filter, limit int) ([]PointPayload, error) {
	out := make([]PointPayload, 0)
	_, err := c.IteratePointPayloads(ctx, c.Collection, filter, limit, 0, func(batch []PointPayload) error {
		out = append(out, batch...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *QdrantClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL+"/collections", nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return errors.New("qdrant health check failed: " + string(body))
	}
	return nil
}

type qdrantHTTPError struct {
	Op         string
	StatusCode int
	Body       string
}

func IsQdrantNotFoundError(err error) bool {
	var httpErr qdrantHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == http.StatusNotFound
	}
	return false
}

func (e qdrantHTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("qdrant %s failed: status=%d", e.Op, e.StatusCode)
	}
	return fmt.Sprintf("qdrant %s failed: status=%d body=%s", e.Op, e.StatusCode, e.Body)
}

func (c *QdrantClient) doWithRetry(ctx context.Context, op, collection string, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= qdrantMaxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err
		if !isTransientQdrantError(err) || attempt == qdrantMaxRetries {
			return err
		}
		delay := backoffDelay(attempt)
		c.logger().Warn("qdrant_operation_retry",
			slog.String("operation", op),
			slog.String("collection", collection),
			slog.Int("attempt", attempt+1),
			slog.Duration("delay", delay),
			slog.String("error", err.Error()),
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("qdrant %s failed", op)
}

func backoffDelay(attempt int) time.Duration {
	base := float64(qdrantBaseBackoff) * math.Pow(2, float64(attempt))
	jitterFactor := 0.8 + rand.Float64()*0.4
	return time.Duration(base * jitterFactor)
}

func isTransientQdrantError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var httpErr qdrantHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode >= 500
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return false
}
