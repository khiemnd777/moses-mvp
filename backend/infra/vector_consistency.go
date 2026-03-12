package infra

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/lib/pq"
)

type VectorScanMode string

const (
	VectorScanQuick VectorScanMode = "quick"
	VectorScanFull  VectorScanMode = "full"
)

type VectorConsistencyOptions struct {
	Mode            VectorScanMode
	ChunkBatchSize  int
	VectorBatchSize int
	MaxChunks       int
	MaxVectors      int
	SampleLimit     int
}

type VectorConsistencyReport struct {
	Mode string

	CollectionDimension int
	ExpectedDimension   int
	DimensionMismatch   bool

	ChunkCount  int64
	VectorCount int64

	ScannedChunkCount  int
	ScannedVectorCount int

	MissingVectorCount            int
	OrphanVectorCount             int
	EmbeddingDimensionMismatchCnt int

	MissingVectorChunkIDs         []string
	OrphanVectorPointIDs          []string
	EmbeddingDimensionMismatchIDs []string

	ChunkVectorCountMismatch bool
	Bounded                  bool
}

func CheckVectorConsistency(ctx context.Context, store *Store, qdrant *QdrantClient, expectedDimension int) (VectorConsistencyReport, error) {
	return CheckVectorConsistencyWithOptions(ctx, store, qdrant, expectedDimension, VectorConsistencyOptions{Mode: VectorScanFull})
}

func CheckVectorConsistencyWithOptions(ctx context.Context, store *Store, qdrant *QdrantClient, expectedDimension int, opts VectorConsistencyOptions) (VectorConsistencyReport, error) {
	if store == nil || qdrant == nil {
		return VectorConsistencyReport{}, fmt.Errorf("vector consistency check dependencies missing")
	}
	opts = normalizeScanOptions(opts)
	report := VectorConsistencyReport{ExpectedDimension: expectedDimension, Mode: string(opts.Mode)}

	logger := qdrant.logger().With(
		slog.String("mode", string(opts.Mode)),
		slog.String("collection", qdrant.Collection),
	)
	started := time.Now()
	logger.Info("vector_consistency_scan_started",
		slog.Int("chunk_batch_size", opts.ChunkBatchSize),
		slog.Int("vector_batch_size", opts.VectorBatchSize),
		slog.Int("max_chunks", opts.MaxChunks),
		slog.Int("max_vectors", opts.MaxVectors),
	)

	info, err := qdrant.GetCollectionInfo(ctx)
	if err != nil {
		return report, err
	}
	report.CollectionDimension = info.VectorSize
	report.DimensionMismatch = info.VectorSize != expectedDimension

	if err := store.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM chunks`).Scan(&report.ChunkCount); err != nil {
		return report, err
	}
	if vectorCount, estimated, err := qdrant.CountPoints(ctx, qdrant.Collection, nil); err == nil {
		report.VectorCount = vectorCount
		_ = estimated
	} else {
		logger.Warn("vector_consistency_vector_count_failed", slog.String("error", err.Error()))
	}
	report.ChunkVectorCountMismatch = report.ChunkCount != report.VectorCount

	afterChunkID := ""
	for {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if opts.MaxChunks > 0 && report.ScannedChunkCount >= opts.MaxChunks {
			report.Bounded = true
			break
		}
		batchLimit := opts.ChunkBatchSize
		if opts.MaxChunks > 0 && report.ScannedChunkCount+batchLimit > opts.MaxChunks {
			batchLimit = opts.MaxChunks - report.ScannedChunkCount
		}
		rows, err := store.ListChunkVectorRefsAfterID(ctx, afterChunkID, batchLimit)
		if err != nil {
			return report, err
		}
		if len(rows) == 0 {
			break
		}
		afterChunkID = rows[len(rows)-1].ID
		report.ScannedChunkCount += len(rows)

		pointIDs := make([]string, 0, len(rows))
		chunkIDByPointID := make(map[string]string, len(rows))
		for _, row := range rows {
			pid := vectorPointID(row.DocumentVersionID, row.Index)
			pointIDs = append(pointIDs, pid)
			chunkIDByPointID[pid] = row.ID
			dim, err := embeddingDimension(row.EmbeddingJSON)
			if err == nil && dim > 0 && dim != expectedDimension {
				report.EmbeddingDimensionMismatchCnt++
				if len(report.EmbeddingDimensionMismatchIDs) < opts.SampleLimit {
					report.EmbeddingDimensionMismatchIDs = append(report.EmbeddingDimensionMismatchIDs, row.ID)
				}
			}
		}
		existing, err := qdrant.GetExistingPointIDs(ctx, pointIDs)
		if err != nil {
			return report, err
		}
		for _, pid := range pointIDs {
			if _, ok := existing[pid]; ok {
				continue
			}
			report.MissingVectorCount++
			if len(report.MissingVectorChunkIDs) < opts.SampleLimit {
				report.MissingVectorChunkIDs = append(report.MissingVectorChunkIDs, chunkIDByPointID[pid])
			}
		}
	}

	maxVectors := opts.MaxVectors
	if maxVectors < 0 {
		maxVectors = 0
	}
	_, err = qdrant.IteratePointPayloads(ctx, qdrant.Collection, nil, opts.VectorBatchSize, maxVectors, func(batch []PointPayload) error {
		report.ScannedVectorCount += len(batch)
		chunkIDs := make([]string, 0, len(batch))
		pointIDs := make([]string, 0, len(batch))
		for _, point := range batch {
			pointIDs = append(pointIDs, point.ID)
			if chunkID, _ := point.Payload["chunk_id"].(string); chunkID != "" {
				chunkIDs = append(chunkIDs, chunkID)
			}
		}
		existingChunkIDs, err := lookupChunkIDSet(ctx, store.DB, chunkIDs)
		if err != nil {
			return err
		}
		for i, point := range batch {
			chunkID, _ := point.Payload["chunk_id"].(string)
			if chunkID == "" {
				report.OrphanVectorCount++
				if len(report.OrphanVectorPointIDs) < opts.SampleLimit {
					report.OrphanVectorPointIDs = append(report.OrphanVectorPointIDs, pointIDs[i])
				}
				continue
			}
			if _, ok := existingChunkIDs[chunkID]; !ok {
				report.OrphanVectorCount++
				if len(report.OrphanVectorPointIDs) < opts.SampleLimit {
					report.OrphanVectorPointIDs = append(report.OrphanVectorPointIDs, pointIDs[i])
				}
			}
		}
		return nil
	})
	if err != nil {
		return report, err
	}
	if opts.MaxVectors > 0 && report.ScannedVectorCount >= opts.MaxVectors {
		report.Bounded = true
	}

	logger.Info("vector_consistency_scan_completed",
		slog.Int64("chunk_count", report.ChunkCount),
		slog.Int64("vector_count", report.VectorCount),
		slog.Bool("count_mismatch", report.ChunkVectorCountMismatch),
		slog.Bool("collection_dimension_mismatch", report.DimensionMismatch),
		slog.Int("missing_vector_count", report.MissingVectorCount),
		slog.Int("orphan_vector_count", report.OrphanVectorCount),
		slog.Int("embedding_dimension_mismatch_count", report.EmbeddingDimensionMismatchCnt),
		slog.Int("scanned_chunks", report.ScannedChunkCount),
		slog.Int("scanned_vectors", report.ScannedVectorCount),
		slog.Bool("bounded", report.Bounded),
		slog.Duration("duration", time.Since(started)),
	)
	return report, nil
}

func normalizeScanOptions(opts VectorConsistencyOptions) VectorConsistencyOptions {
	if opts.Mode == "" {
		opts.Mode = VectorScanQuick
	}
	if opts.ChunkBatchSize <= 0 {
		opts.ChunkBatchSize = 256
	}
	if opts.VectorBatchSize <= 0 {
		opts.VectorBatchSize = 256
	}
	if opts.SampleLimit <= 0 {
		opts.SampleLimit = 50
	}
	switch opts.Mode {
	case VectorScanQuick:
		if opts.MaxChunks <= 0 {
			opts.MaxChunks = 1000
		}
		if opts.MaxVectors <= 0 {
			opts.MaxVectors = 1000
		}
	case VectorScanFull:
		if opts.MaxChunks < 0 {
			opts.MaxChunks = 0
		}
		if opts.MaxVectors < 0 {
			opts.MaxVectors = 0
		}
	}
	return opts
}

func embeddingDimension(raw []byte) (int, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return 0, err
	}
	return len(arr), nil
}

func lookupChunkIDSet(ctx context.Context, db *sql.DB, ids []string) (map[string]struct{}, error) {
	out := make(map[string]struct{}, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := db.QueryContext(ctx, `SELECT id FROM chunks WHERE id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}
