package retrieval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"

	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
)

type Service struct {
	Store  *infra.Store
	Qdrant *infra.QdrantClient
	Embed  Embedder
}

type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float64, error)
}

type Result struct {
	ChunkID    string
	Text       string
	CitationID string
	VersionID  string
	ChunkIndex int
	Score      float64
	Metadata   map[string]interface{}
}

func (s *Service) Search(ctx context.Context, query string, topK int) ([]Result, error) {
	vectors, err := s.Embed.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	matches, err := s.Qdrant.Search(ctx, vectors[0], topK)
	if err != nil {
		return nil, err
	}
	chunkIDs := make([]string, 0, len(matches))
	for _, match := range matches {
		if match.ChunkID != "" {
			chunkIDs = append(chunkIDs, match.ChunkID)
		}
	}
	chunks, err := s.Store.GetChunksByIDs(ctx, chunkIDs)
	if err != nil {
		return nil, err
	}
	chunkByID := make(map[string]domain.Chunk, len(chunks))
	for _, chunk := range chunks {
		chunkByID[chunk.ID] = chunk
	}
	results := make([]Result, 0, len(matches))
	for _, match := range matches {
		c, ok := chunkByID[match.ChunkID]
		if !ok {
			continue
		}
		results = append(results, Result{
			ChunkID:    c.ID,
			Text:       c.Text,
			VersionID:  c.DocumentVersionID,
			ChunkIndex: c.Index,
			CitationID: citationID(c.DocumentVersionID, c.Index, c.Text),
			Score:      match.Score,
			Metadata:   decodeMetadata(c.MetadataJSON),
		})
	}
	return results, nil
}

func decodeMetadata(raw []byte) map[string]interface{} {
	if len(raw) == 0 {
		return map[string]interface{}{}
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]interface{}{}
	}
	return out
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
