DROP TABLE IF EXISTS ai_retrieval_configs;

CREATE TABLE IF NOT EXISTS ai_retrieval_configs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,

  default_top_k INT NOT NULL DEFAULT 5,

  rerank_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  rerank_vector_weight DOUBLE PRECISION NOT NULL DEFAULT 0.55,
  rerank_keyword_weight DOUBLE PRECISION NOT NULL DEFAULT 0.25,
  rerank_metadata_weight DOUBLE PRECISION NOT NULL DEFAULT 0.15,
  rerank_article_weight DOUBLE PRECISION NOT NULL DEFAULT 0.05,

  adjacent_chunk_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  adjacent_chunk_window INT NOT NULL DEFAULT 1,

  max_context_chunks INT NOT NULL DEFAULT 12,
  max_context_chars INT NOT NULL DEFAULT 12000,

  default_effective_status TEXT NOT NULL DEFAULT 'active',

  preferred_doc_types_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  legal_domain_defaults_json JSONB NOT NULL DEFAULT '{}'::jsonb,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT ai_retrieval_cfg_top_k_chk CHECK (default_top_k >= 1 AND default_top_k <= 20),
  CONSTRAINT ai_retrieval_cfg_vector_w_chk CHECK (rerank_vector_weight >= 0 AND rerank_vector_weight <= 1),
  CONSTRAINT ai_retrieval_cfg_keyword_w_chk CHECK (rerank_keyword_weight >= 0 AND rerank_keyword_weight <= 1),
  CONSTRAINT ai_retrieval_cfg_metadata_w_chk CHECK (rerank_metadata_weight >= 0 AND rerank_metadata_weight <= 1),
  CONSTRAINT ai_retrieval_cfg_article_w_chk CHECK (rerank_article_weight >= 0 AND rerank_article_weight <= 1),
  CONSTRAINT ai_retrieval_cfg_window_chk CHECK (adjacent_chunk_window >= 0 AND adjacent_chunk_window <= 3),
  CONSTRAINT ai_retrieval_cfg_ctx_chunks_chk CHECK (max_context_chunks >= 1 AND max_context_chunks <= 20),
  CONSTRAINT ai_retrieval_cfg_ctx_chars_chk CHECK (max_context_chars >= 1000 AND max_context_chars <= 200000),
  CONSTRAINT ai_retrieval_cfg_weight_sum_chk CHECK (
    (rerank_vector_weight + rerank_keyword_weight + rerank_metadata_weight + rerank_article_weight) <= 1
  )
);

CREATE INDEX IF NOT EXISTS idx_ai_retrieval_configs_updated_at
ON ai_retrieval_configs(updated_at DESC, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_retrieval_configs_single_enabled
ON ai_retrieval_configs ((enabled))
WHERE enabled = TRUE;
