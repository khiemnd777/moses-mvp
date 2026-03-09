package repository

import (
	"context"

	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
)

type PromptRepository struct {
	Store *infra.Store
}

func NewPromptRepository(store *infra.Store) *PromptRepository {
	return &PromptRepository{Store: store}
}

func (r *PromptRepository) List(ctx context.Context) ([]domain.AIPrompt, error) {
	rows, err := r.Store.DB.QueryContext(ctx, `
SELECT id, name, prompt_type, system_prompt, temperature, max_tokens, retry, enabled, created_at, updated_at
FROM ai_prompts
ORDER BY updated_at DESC, created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.AIPrompt{}
	for rows.Next() {
		var item domain.AIPrompt
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.PromptType,
			&item.SystemPrompt,
			&item.Temperature,
			&item.MaxTokens,
			&item.Retry,
			&item.Enabled,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PromptRepository) GetByID(ctx context.Context, id string) (domain.AIPrompt, error) {
	return r.getByIDWithExec(ctx, r.Store.DB, id)
}

func (r *PromptRepository) getByIDWithExec(ctx context.Context, exec dbtx, id string) (domain.AIPrompt, error) {
	var item domain.AIPrompt
	err := exec.QueryRowContext(ctx, `
SELECT id, name, prompt_type, system_prompt, temperature, max_tokens, retry, enabled, created_at, updated_at
FROM ai_prompts
WHERE id = $1
`, id).Scan(
		&item.ID,
		&item.Name,
		&item.PromptType,
		&item.SystemPrompt,
		&item.Temperature,
		&item.MaxTokens,
		&item.Retry,
		&item.Enabled,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}

func (r *PromptRepository) Create(ctx context.Context, exec dbtx, item domain.AIPrompt) (domain.AIPrompt, error) {
	var created domain.AIPrompt
	err := exec.QueryRowContext(ctx, `
INSERT INTO ai_prompts (
	name, prompt_type, system_prompt, temperature, max_tokens, retry, enabled
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, name, prompt_type, system_prompt, temperature, max_tokens, retry, enabled, created_at, updated_at
`,
		item.Name,
		item.PromptType,
		item.SystemPrompt,
		item.Temperature,
		item.MaxTokens,
		item.Retry,
		item.Enabled,
	).Scan(
		&created.ID,
		&created.Name,
		&created.PromptType,
		&created.SystemPrompt,
		&created.Temperature,
		&created.MaxTokens,
		&created.Retry,
		&created.Enabled,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	return created, err
}

func (r *PromptRepository) Update(ctx context.Context, exec dbtx, id string, item domain.AIPrompt) (domain.AIPrompt, error) {
	var updated domain.AIPrompt
	err := exec.QueryRowContext(ctx, `
UPDATE ai_prompts
SET
	name = $2,
	prompt_type = $3,
	system_prompt = $4,
	temperature = $5,
	max_tokens = $6,
	retry = $7,
	enabled = $8,
	updated_at = NOW()
WHERE id = $1
RETURNING id, name, prompt_type, system_prompt, temperature, max_tokens, retry, enabled, created_at, updated_at
`,
		id,
		item.Name,
		item.PromptType,
		item.SystemPrompt,
		item.Temperature,
		item.MaxTokens,
		item.Retry,
		item.Enabled,
	).Scan(
		&updated.ID,
		&updated.Name,
		&updated.PromptType,
		&updated.SystemPrompt,
		&updated.Temperature,
		&updated.MaxTokens,
		&updated.Retry,
		&updated.Enabled,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)
	return updated, err
}

func (r *PromptRepository) Delete(ctx context.Context, id string) (bool, error) {
	res, err := r.Store.DB.ExecContext(ctx, `DELETE FROM ai_prompts WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *PromptRepository) DisableOthersInPromptType(ctx context.Context, exec dbtx, exceptID, promptType string) error {
	_, err := exec.ExecContext(ctx, `
UPDATE ai_prompts
SET enabled = FALSE, updated_at = NOW()
WHERE id <> $1 AND prompt_type = $2 AND enabled = TRUE
`, exceptID, promptType)
	return err
}
