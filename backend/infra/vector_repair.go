package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

const (
	VectorRepairStatusPending   = "pending"
	VectorRepairStatusRunning   = "running"
	VectorRepairStatusCompleted = "completed"
)

type VectorRepairTask struct {
	ID          string
	TaskKey     string
	TaskType    string
	Collection  string
	PayloadJSON []byte
	Status      string
	Attempt     int
	LastError   *string
	NextRunAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type VectorRepairPayload struct {
	DocumentID        string  `json:"document_id,omitempty"`
	DocumentVersionID string  `json:"document_version_id,omitempty"`
	Filter            *Filter `json:"filter,omitempty"`
}

func (s *Store) EnsureVectorRepairSchema(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS vector_repair_tasks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_key TEXT NOT NULL UNIQUE,
  task_type TEXT NOT NULL,
  collection TEXT NOT NULL,
  payload_json JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INT NOT NULL DEFAULT 0,
  last_error TEXT,
  next_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_vector_repair_due ON vector_repair_tasks(status, next_run_at);
`)
	return err
}

func (s *Store) EnqueueVectorRepairTask(ctx context.Context, taskKey, taskType, collection string, payload VectorRepairPayload) (bool, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}
	res, err := s.DB.ExecContext(ctx, `
INSERT INTO vector_repair_tasks(task_key, task_type, collection, payload_json, status, attempt_count, next_run_at, updated_at)
VALUES($1, $2, $3, $4, 'pending', 0, NOW(), NOW())
ON CONFLICT (task_key)
DO UPDATE SET
  status = CASE WHEN vector_repair_tasks.status = 'completed' THEN vector_repair_tasks.status ELSE 'pending' END,
  payload_json = EXCLUDED.payload_json,
  collection = EXCLUDED.collection,
  updated_at = NOW(),
  next_run_at = CASE WHEN vector_repair_tasks.status = 'completed' THEN vector_repair_tasks.next_run_at ELSE NOW() END
`, taskKey, taskType, collection, b)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (s *Store) ClaimDueVectorRepairTasks(ctx context.Context, limit int) ([]VectorRepairTask, error) {
	if limit <= 0 {
		limit = 20
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
SELECT id, task_key, task_type, collection, payload_json, status, attempt_count, last_error, next_run_at, created_at, updated_at
FROM vector_repair_tasks
WHERE status = 'pending' AND next_run_at <= NOW()
ORDER BY next_run_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]VectorRepairTask, 0, limit)
	for rows.Next() {
		var task VectorRepairTask
		if err := rows.Scan(&task.ID, &task.TaskKey, &task.TaskType, &task.Collection, &task.PayloadJSON, &task.Status, &task.Attempt, &task.LastError, &task.NextRunAt, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, task := range out {
		if _, err := tx.ExecContext(ctx, `UPDATE vector_repair_tasks SET status = 'running', updated_at = NOW() WHERE id = $1`, task.ID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) CompleteVectorRepairTask(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `
UPDATE vector_repair_tasks
SET status='completed', attempt_count=attempt_count+1, last_error=NULL, updated_at=NOW()
WHERE id = $1
`, id)
	return err
}

func (s *Store) RetryVectorRepairTask(ctx context.Context, id string, reason string, nextRunAt time.Time) error {
	_, err := s.DB.ExecContext(ctx, `
UPDATE vector_repair_tasks
SET status='pending', attempt_count=attempt_count+1, last_error=$2, next_run_at=$3, updated_at=NOW()
WHERE id = $1
`, id, reason, nextRunAt)
	return err
}

func RunVectorRepairPass(ctx context.Context, logger *slog.Logger, store *Store, qdrant *QdrantClient, limit int) (int, error) {
	if store == nil || qdrant == nil {
		return 0, fmt.Errorf("vector repair dependencies missing")
	}
	started := time.Now()
	tasks, err := store.ClaimDueVectorRepairTasks(ctx, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, task := range tasks {
		processed++
		log := logger.With(
			slog.String("task_id", task.ID),
			slog.String("task_key", task.TaskKey),
			slog.String("task_type", task.TaskType),
			slog.String("collection", task.Collection),
			slog.Int("attempt", task.Attempt+1),
		)
		log.Info("vector_repair_started")

		var payload VectorRepairPayload
		if err := json.Unmarshal(task.PayloadJSON, &payload); err != nil {
			next := time.Now().Add(repairBackoff(task.Attempt + 1))
			_ = store.RetryVectorRepairTask(ctx, task.ID, "invalid repair payload: "+err.Error(), next)
			log.Error("vector_repair_failed", slog.String("error", err.Error()), slog.Time("next_run_at", next))
			continue
		}

		repairErr := runRepairTask(ctx, store, qdrant, task, payload)
		if repairErr != nil {
			next := time.Now().Add(repairBackoff(task.Attempt + 1))
			_ = store.RetryVectorRepairTask(ctx, task.ID, repairErr.Error(), next)
			log.Error("vector_repair_failed", slog.String("error", repairErr.Error()), slog.Time("next_run_at", next))
			continue
		}
		if err := store.CompleteVectorRepairTask(ctx, task.ID); err != nil {
			log.Error("vector_repair_complete_mark_failed", slog.String("error", err.Error()))
			continue
		}
		log.Info("vector_repair_completed")
	}
	logger.Info("vector_repair_pass_completed",
		slog.Int("tasks_processed", processed),
		slog.Duration("duration", time.Since(started)),
	)
	return processed, nil
}

func runRepairTask(ctx context.Context, store *Store, qdrant *QdrantClient, task VectorRepairTask, payload VectorRepairPayload) error {
	switch task.TaskType {
	case "delete_vectors_by_filter":
		if payload.Filter == nil {
			return fmt.Errorf("missing filter for delete_vectors_by_filter")
		}
		return qdrant.DeleteByFilter(ctx, task.Collection, *payload.Filter)
	case "rebuild_vectors_for_version":
		if payload.DocumentVersionID == "" {
			return fmt.Errorf("missing document_version_id for rebuild_vectors_for_version")
		}
		return RebuildVectorsForVersion(ctx, store, qdrant, payload.DocumentVersionID, 128)
	default:
		return fmt.Errorf("unsupported vector repair task type: %s", task.TaskType)
	}
}

func repairBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 6 {
		attempt = 6
	}
	return time.Duration(1<<uint(attempt-1)) * time.Minute
}

func repairTaskKey(taskType string, collection string, payload VectorRepairPayload) string {
	if payload.DocumentVersionID != "" {
		return fmt.Sprintf("%s:%s:%s", taskType, collection, payload.DocumentVersionID)
	}
	if payload.DocumentID != "" {
		return fmt.Sprintf("%s:%s:%s", taskType, collection, payload.DocumentID)
	}
	return fmt.Sprintf("%s:%s:generic", taskType, collection)
}

func EnqueueDeleteVectorsRepair(ctx context.Context, logger *slog.Logger, store *Store, qdrant *QdrantClient, documentID, documentVersionID string, filter Filter) error {
	if store == nil || qdrant == nil {
		return fmt.Errorf("repair enqueue dependencies missing")
	}
	payload := VectorRepairPayload{
		DocumentID:        documentID,
		DocumentVersionID: documentVersionID,
		Filter:            &filter,
	}
	key := repairTaskKey("delete_vectors_by_filter", qdrant.Collection, payload)
	enqueued, err := store.EnqueueVectorRepairTask(ctx, key, "delete_vectors_by_filter", qdrant.Collection, payload)
	if err != nil {
		return err
	}
	if logger != nil {
		logger.Warn("vector_repair_enqueued",
			slog.String("task_key", key),
			slog.String("task_type", "delete_vectors_by_filter"),
			slog.String("collection", qdrant.Collection),
			slog.String("document_id", documentID),
			slog.String("document_version_id", documentVersionID),
			slog.Bool("enqueued", enqueued),
		)
	}
	return nil
}

func EnqueueRebuildVectorsRepair(ctx context.Context, logger *slog.Logger, store *Store, qdrant *QdrantClient, documentVersionID string) error {
	if store == nil || qdrant == nil {
		return fmt.Errorf("repair enqueue dependencies missing")
	}
	payload := VectorRepairPayload{DocumentVersionID: documentVersionID}
	key := repairTaskKey("rebuild_vectors_for_version", qdrant.Collection, payload)
	enqueued, err := store.EnqueueVectorRepairTask(ctx, key, "rebuild_vectors_for_version", qdrant.Collection, payload)
	if err != nil {
		return err
	}
	if logger != nil {
		logger.Warn("vector_repair_enqueued",
			slog.String("task_key", key),
			slog.String("task_type", "rebuild_vectors_for_version"),
			slog.String("collection", qdrant.Collection),
			slog.String("document_version_id", documentVersionID),
			slog.Bool("enqueued", enqueued),
		)
	}
	return nil
}
