package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

func CleanupVectorsByDocumentVersion(ctx context.Context, store *Store, qdrant *QdrantClient, documentVersionID string) error {
	if store == nil || qdrant == nil {
		return fmt.Errorf("vector cleanup dependencies missing")
	}
	filter := Filter{Must: []FieldCondition{{Key: "document_version_id", Match: FieldMatch{Value: documentVersionID}}}}
	return qdrant.DeleteByFilter(ctx, qdrant.Collection, filter)
}

func CleanupVectorsByDocument(ctx context.Context, store *Store, qdrant *QdrantClient, documentID string) error {
	if store == nil || qdrant == nil {
		return fmt.Errorf("vector cleanup dependencies missing")
	}
	versionIDs, err := store.ListDocumentVersionIDsByDocument(ctx, documentID)
	if err != nil {
		return err
	}
	for _, versionID := range versionIDs {
		if err := CleanupVectorsByDocumentVersion(ctx, store, qdrant, versionID); err != nil {
			return err
		}
	}
	return nil
}

func RebuildVectorsForVersion(ctx context.Context, store *Store, qdrant *QdrantClient, documentVersionID string, batchSize int) error {
	if store == nil || qdrant == nil {
		return fmt.Errorf("vector rebuild dependencies missing")
	}
	if batchSize <= 0 {
		batchSize = 128
	}
	logger := qdrant.logger().With(
		slog.String("collection", qdrant.Collection),
		slog.String("document_version_id", documentVersionID),
	)
	started := time.Now()
	logger.Info("vector_rebuild_started")

	after := -1
	totalUpserted := 0
	for {
		rows, err := store.ListChunkVectorsByVersion(ctx, documentVersionID, after, batchSize)
		if err != nil {
			logger.Error("vector_rebuild_failed", slog.String("error", err.Error()), slog.Int("upserted", totalUpserted))
			return err
		}
		if len(rows) == 0 {
			break
		}
		points := make([]PointInput, 0, len(rows))
		for _, row := range rows {
			var embedding []float64
			if err := json.Unmarshal(row.EmbeddingJSON, &embedding); err != nil {
				logger.Error("vector_rebuild_failed", slog.String("error", err.Error()), slog.String("chunk_id", row.ID))
				return fmt.Errorf("decode chunk embedding %s: %w", row.ID, err)
			}
			if len(embedding) == 0 {
				continue
			}
			payload := map[string]interface{}{
				"chunk_id":            row.ID,
				"document_version_id": row.DocumentVersionID,
				"chunk_index":         row.Index,
			}
			var meta map[string]interface{}
			if len(row.MetadataJSON) > 0 {
				if err := json.Unmarshal(row.MetadataJSON, &meta); err == nil {
					for _, key := range []string{"document_id", "legal_domain", "document_type", "effective_status", "document_number", "article_number"} {
						if v, ok := meta[key]; ok {
							payload[key] = v
						}
					}
				}
			}
			points = append(points, PointInput{ID: vectorPointID(row.DocumentVersionID, row.Index), Vector: embedding, Payload: payload})
		}
		if len(points) > 0 {
			if err := qdrant.Upsert(ctx, points); err != nil {
				logger.Error("vector_rebuild_failed", slog.String("error", err.Error()), slog.Int("upserted", totalUpserted))
				return err
			}
			totalUpserted += len(points)
		}
		after = rows[len(rows)-1].Index
	}
	logger.Info("vector_rebuild_completed", slog.Int("upserted", totalUpserted), slog.Duration("duration", time.Since(started)))
	return nil
}
