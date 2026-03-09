package repository

import (
	"context"
	"database/sql"

	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
)

type GuardPolicyRepository struct {
	Store *infra.Store
}

type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

func NewGuardPolicyRepository(store *infra.Store) *GuardPolicyRepository {
	return &GuardPolicyRepository{Store: store}
}

func (r *GuardPolicyRepository) List(ctx context.Context) ([]domain.AIGuardPolicy, error) {
	rows, err := r.Store.DB.QueryContext(ctx, `
SELECT id, name, enabled, min_retrieved_chunks, min_similarity_score, on_empty_retrieval, on_low_confidence, created_at, updated_at
FROM ai_guard_policies
ORDER BY updated_at DESC, created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.AIGuardPolicy{}
	for rows.Next() {
		var item domain.AIGuardPolicy
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Enabled,
			&item.MinRetrievedChunks,
			&item.MinSimilarityScore,
			&item.OnEmptyRetrieval,
			&item.OnLowConfidence,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *GuardPolicyRepository) GetByID(ctx context.Context, id string) (domain.AIGuardPolicy, error) {
	return r.getByIDWithExec(ctx, r.Store.DB, id)
}

func (r *GuardPolicyRepository) getByIDWithExec(ctx context.Context, exec dbtx, id string) (domain.AIGuardPolicy, error) {
	var item domain.AIGuardPolicy
	err := exec.QueryRowContext(ctx, `
SELECT id, name, enabled, min_retrieved_chunks, min_similarity_score, on_empty_retrieval, on_low_confidence, created_at, updated_at
FROM ai_guard_policies
WHERE id = $1
`, id).Scan(
		&item.ID,
		&item.Name,
		&item.Enabled,
		&item.MinRetrievedChunks,
		&item.MinSimilarityScore,
		&item.OnEmptyRetrieval,
		&item.OnLowConfidence,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}

func (r *GuardPolicyRepository) Create(ctx context.Context, exec dbtx, item domain.AIGuardPolicy) (domain.AIGuardPolicy, error) {
	var created domain.AIGuardPolicy
	err := exec.QueryRowContext(ctx, `
INSERT INTO ai_guard_policies (
	name, enabled, min_retrieved_chunks, min_similarity_score, on_empty_retrieval, on_low_confidence
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, name, enabled, min_retrieved_chunks, min_similarity_score, on_empty_retrieval, on_low_confidence, created_at, updated_at
`,
		item.Name,
		item.Enabled,
		item.MinRetrievedChunks,
		item.MinSimilarityScore,
		item.OnEmptyRetrieval,
		item.OnLowConfidence,
	).Scan(
		&created.ID,
		&created.Name,
		&created.Enabled,
		&created.MinRetrievedChunks,
		&created.MinSimilarityScore,
		&created.OnEmptyRetrieval,
		&created.OnLowConfidence,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	return created, err
}

func (r *GuardPolicyRepository) Update(ctx context.Context, exec dbtx, id string, item domain.AIGuardPolicy) (domain.AIGuardPolicy, error) {
	var updated domain.AIGuardPolicy
	err := exec.QueryRowContext(ctx, `
UPDATE ai_guard_policies
SET
	name = $2,
	enabled = $3,
	min_retrieved_chunks = $4,
	min_similarity_score = $5,
	on_empty_retrieval = $6,
	on_low_confidence = $7,
	updated_at = NOW()
WHERE id = $1
RETURNING id, name, enabled, min_retrieved_chunks, min_similarity_score, on_empty_retrieval, on_low_confidence, created_at, updated_at
`,
		id,
		item.Name,
		item.Enabled,
		item.MinRetrievedChunks,
		item.MinSimilarityScore,
		item.OnEmptyRetrieval,
		item.OnLowConfidence,
	).Scan(
		&updated.ID,
		&updated.Name,
		&updated.Enabled,
		&updated.MinRetrievedChunks,
		&updated.MinSimilarityScore,
		&updated.OnEmptyRetrieval,
		&updated.OnLowConfidence,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)
	return updated, err
}

func (r *GuardPolicyRepository) Delete(ctx context.Context, id string) (bool, error) {
	res, err := r.Store.DB.ExecContext(ctx, `DELETE FROM ai_guard_policies WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *GuardPolicyRepository) DisableOthers(ctx context.Context, exec dbtx, exceptID string) error {
	_, err := exec.ExecContext(ctx, `
UPDATE ai_guard_policies
SET enabled = FALSE, updated_at = NOW()
WHERE id <> $1 AND enabled = TRUE
`, exceptID)
	return err
}
