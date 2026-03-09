package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type QdrantClient struct {
	URL        string
	Collection string
	HTTP       *http.Client
}

func NewQdrantClient(url, collection string) *QdrantClient {
	return &QdrantClient{URL: url, Collection: collection, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

type createCollectionRequest struct {
	Vectors struct {
		Size     int    `json:"size"`
		Distance string `json:"distance"`
	} `json:"vectors"`
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
	if resp.StatusCode >= 300 && resp.StatusCode != 409 {
		return errors.New("qdrant ensure collection failed")
	}
	return nil
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
	payload := upsertRequest{Points: points}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
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
		return errors.New("qdrant upsert failed")
	}
	return nil
}

type searchRequest struct {
	Vector      []float64 `json:"vector"`
	Limit       int       `json:"limit"`
	WithPayload bool      `json:"with_payload"`
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

func (c *QdrantClient) Search(ctx context.Context, vector []float64, limit int) ([]SearchResult, error) {
	payload := searchRequest{Vector: vector, Limit: limit, WithPayload: true}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/collections/"+c.Collection+"/points/search", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, errors.New("qdrant search failed")
	}
	var out searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	results := make([]SearchResult, 0, len(out.Result))
	for _, r := range out.Result {
		chunkID, _ := r.Payload["chunk_id"].(string)
		results = append(results, SearchResult{
			ID:      r.ID,
			ChunkID: chunkID,
			Score:   r.Score,
			Payload: r.Payload,
		})
	}
	return results, nil
}

type getPointsRequest struct {
	IDs         []string `json:"ids"`
	WithPayload bool     `json:"with_payload"`
	WithVectors bool     `json:"with_vectors"`
}

type getPointsResponse struct {
	Result []struct {
		ID      string                 `json:"id"`
		Payload map[string]interface{} `json:"payload"`
	} `json:"result"`
}

func (c *QdrantClient) GetPayloadByPointID(ctx context.Context, pointID string) (map[string]interface{}, bool, error) {
	payload := getPointsRequest{
		IDs:         []string{pointID},
		WithPayload: true,
		WithVectors: false,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/collections/"+c.Collection+"/points", bytes.NewReader(b))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, false, errors.New("qdrant point lookup failed")
	}
	var out getPointsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, false, err
	}
	if len(out.Result) == 0 {
		return nil, false, nil
	}
	return out.Result[0].Payload, true, nil
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/collections/"+c.Collection+"/points/delete?wait=true", bytes.NewReader(b))
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
		return errors.New("qdrant delete failed")
	}
	return nil
}
