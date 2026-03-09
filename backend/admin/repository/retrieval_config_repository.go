package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
)

type RetrievalConfigRepository struct {
	Store *infra.Store
}

func NewRetrievalConfigRepository(store *infra.Store) *RetrievalConfigRepository {
	return &RetrievalConfigRepository{Store: store}
}

func (r *RetrievalConfigRepository) List(ctx context.Context) ([]domain.AIRetrievalConfig, error) {
	rows, err := r.Store.DB.QueryContext(ctx, `
SELECT id, name, enabled, default_top_k,
       rerank_enabled, rerank_vector_weight, rerank_keyword_weight, rerank_metadata_weight, rerank_article_weight,
       adjacent_chunk_enabled, adjacent_chunk_window,
       max_context_chunks, max_context_chars,
       default_effective_status, preferred_doc_types_json, legal_domain_defaults_json,
       created_at, updated_at
FROM ai_retrieval_configs
ORDER BY updated_at DESC, created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.AIRetrievalConfig{}
	for rows.Next() {
		item, err := scanRetrievalConfig(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *RetrievalConfigRepository) GetByID(ctx context.Context, id string) (domain.AIRetrievalConfig, error) {
	return r.getByIDWithExec(ctx, r.Store.DB, id)
}

func (r *RetrievalConfigRepository) getByIDWithExec(ctx context.Context, exec dbtx, id string) (domain.AIRetrievalConfig, error) {
	row := exec.QueryRowContext(ctx, `
SELECT id, name, enabled, default_top_k,
       rerank_enabled, rerank_vector_weight, rerank_keyword_weight, rerank_metadata_weight, rerank_article_weight,
       adjacent_chunk_enabled, adjacent_chunk_window,
       max_context_chunks, max_context_chars,
       default_effective_status, preferred_doc_types_json, legal_domain_defaults_json,
       created_at, updated_at
FROM ai_retrieval_configs
WHERE id = $1
`, id)
	return scanRetrievalConfig(row.Scan)
}

func (r *RetrievalConfigRepository) Create(ctx context.Context, exec dbtx, item domain.AIRetrievalConfig) (domain.AIRetrievalConfig, error) {
	preferredDocTypes, _ := json.Marshal(item.PreferredDocTypes)
	legalDefaults, _ := json.Marshal(item.LegalDomainDefaultsJSON)
	row := exec.QueryRowContext(ctx, `
INSERT INTO ai_retrieval_configs (
	name, enabled, default_top_k,
	rerank_enabled, rerank_vector_weight, rerank_keyword_weight, rerank_metadata_weight, rerank_article_weight,
	adjacent_chunk_enabled, adjacent_chunk_window,
	max_context_chunks, max_context_chars,
	default_effective_status, preferred_doc_types_json, legal_domain_defaults_json
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
RETURNING id, name, enabled, default_top_k,
       rerank_enabled, rerank_vector_weight, rerank_keyword_weight, rerank_metadata_weight, rerank_article_weight,
       adjacent_chunk_enabled, adjacent_chunk_window,
       max_context_chunks, max_context_chars,
       default_effective_status, preferred_doc_types_json, legal_domain_defaults_json,
       created_at, updated_at
`,
		item.Name,
		item.Enabled,
		item.DefaultTopK,
		item.RerankEnabled,
		item.RerankVectorWeight,
		item.RerankKeywordWeight,
		item.RerankMetadataWeight,
		item.RerankArticleWeight,
		item.AdjacentChunkEnabled,
		item.AdjacentChunkWindow,
		item.MaxContextChunks,
		item.MaxContextChars,
		item.DefaultEffectiveStatus,
		preferredDocTypes,
		legalDefaults,
	)
	return scanRetrievalConfig(row.Scan)
}

func (r *RetrievalConfigRepository) Update(ctx context.Context, exec dbtx, id string, item domain.AIRetrievalConfig) (domain.AIRetrievalConfig, error) {
	preferredDocTypes, _ := json.Marshal(item.PreferredDocTypes)
	legalDefaults, _ := json.Marshal(item.LegalDomainDefaultsJSON)
	row := exec.QueryRowContext(ctx, `
UPDATE ai_retrieval_configs
SET name = $2,
    enabled = $3,
    default_top_k = $4,
    rerank_enabled = $5,
    rerank_vector_weight = $6,
    rerank_keyword_weight = $7,
    rerank_metadata_weight = $8,
    rerank_article_weight = $9,
    adjacent_chunk_enabled = $10,
    adjacent_chunk_window = $11,
    max_context_chunks = $12,
    max_context_chars = $13,
    default_effective_status = $14,
    preferred_doc_types_json = $15,
    legal_domain_defaults_json = $16,
    updated_at = NOW()
WHERE id = $1
RETURNING id, name, enabled, default_top_k,
       rerank_enabled, rerank_vector_weight, rerank_keyword_weight, rerank_metadata_weight, rerank_article_weight,
       adjacent_chunk_enabled, adjacent_chunk_window,
       max_context_chunks, max_context_chars,
       default_effective_status, preferred_doc_types_json, legal_domain_defaults_json,
       created_at, updated_at
`,
		id,
		item.Name,
		item.Enabled,
		item.DefaultTopK,
		item.RerankEnabled,
		item.RerankVectorWeight,
		item.RerankKeywordWeight,
		item.RerankMetadataWeight,
		item.RerankArticleWeight,
		item.AdjacentChunkEnabled,
		item.AdjacentChunkWindow,
		item.MaxContextChunks,
		item.MaxContextChars,
		item.DefaultEffectiveStatus,
		preferredDocTypes,
		legalDefaults,
	)
	return scanRetrievalConfig(row.Scan)
}

func (r *RetrievalConfigRepository) Delete(ctx context.Context, id string) (bool, error) {
	res, err := r.Store.DB.ExecContext(ctx, `DELETE FROM ai_retrieval_configs WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *RetrievalConfigRepository) DisableOthers(ctx context.Context, exec dbtx, exceptID string) error {
	_, err := exec.ExecContext(ctx, `
UPDATE ai_retrieval_configs
SET enabled = FALSE, updated_at = NOW()
WHERE id <> $1 AND enabled = TRUE
`, exceptID)
	return err
}

func (r *RetrievalConfigRepository) SetEnabled(ctx context.Context, exec dbtx, id string, enabled bool) (domain.AIRetrievalConfig, error) {
	row := exec.QueryRowContext(ctx, `
UPDATE ai_retrieval_configs
SET enabled = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING id, name, enabled, default_top_k,
       rerank_enabled, rerank_vector_weight, rerank_keyword_weight, rerank_metadata_weight, rerank_article_weight,
       adjacent_chunk_enabled, adjacent_chunk_window,
       max_context_chunks, max_context_chars,
       default_effective_status, preferred_doc_types_json, legal_domain_defaults_json,
       created_at, updated_at
`, id, enabled)
	return scanRetrievalConfig(row.Scan)
}

type scannerFn func(dest ...interface{}) error

func scanRetrievalConfig(scan scannerFn) (domain.AIRetrievalConfig, error) {
	var item domain.AIRetrievalConfig
	var preferredDocTypesRaw []byte
	var legalDefaultsRaw []byte
	err := scan(
		&item.ID,
		&item.Name,
		&item.Enabled,
		&item.DefaultTopK,
		&item.RerankEnabled,
		&item.RerankVectorWeight,
		&item.RerankKeywordWeight,
		&item.RerankMetadataWeight,
		&item.RerankArticleWeight,
		&item.AdjacentChunkEnabled,
		&item.AdjacentChunkWindow,
		&item.MaxContextChunks,
		&item.MaxContextChars,
		&item.DefaultEffectiveStatus,
		&preferredDocTypesRaw,
		&legalDefaultsRaw,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return domain.AIRetrievalConfig{}, err
	}
	item.PreferredDocTypes = []string{}
	item.LegalDomainDefaultsJSON = map[string]interface{}{}
	if len(preferredDocTypesRaw) > 0 {
		_ = json.Unmarshal(preferredDocTypesRaw, &item.PreferredDocTypes)
	}
	if len(legalDefaultsRaw) > 0 {
		_ = json.Unmarshal(legalDefaultsRaw, &item.LegalDomainDefaultsJSON)
	}
	if item.PreferredDocTypes == nil {
		item.PreferredDocTypes = []string{}
	}
	if item.LegalDomainDefaultsJSON == nil {
		item.LegalDomainDefaultsJSON = map[string]interface{}{}
	}
	return item, nil
}

var _ dbtx = (*sql.DB)(nil)
