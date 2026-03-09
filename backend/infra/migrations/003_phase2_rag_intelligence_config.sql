CREATE TABLE IF NOT EXISTS ai_retrieval_configs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  default_top_k INT NOT NULL DEFAULT 5,
  rerank_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  rerank_weights JSONB NOT NULL DEFAULT '{"vector":0.55,"keyword":0.25,"metadata":0.15,"article":0.05}'::jsonb,
  adjacent_chunk_window INT NOT NULL DEFAULT 1,
  max_context_chunks INT NOT NULL DEFAULT 12,
  max_context_chars INT NOT NULL DEFAULT 12000,
  candidate_multiplier INT NOT NULL DEFAULT 3,
  metadata_filter_defaults JSONB NOT NULL DEFAULT '{"effective_status":"active"}'::jsonb,
  preferred_doc_types_by_domain JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT ai_retrieval_configs_default_top_k_chk CHECK (default_top_k > 0),
  CONSTRAINT ai_retrieval_configs_adjacent_window_chk CHECK (adjacent_chunk_window >= 0),
  CONSTRAINT ai_retrieval_configs_max_chunks_chk CHECK (max_context_chunks > 0),
  CONSTRAINT ai_retrieval_configs_max_chars_chk CHECK (max_context_chars > 0),
  CONSTRAINT ai_retrieval_configs_candidate_multiplier_chk CHECK (candidate_multiplier > 0)
);

CREATE INDEX IF NOT EXISTS idx_ai_retrieval_configs_enabled ON ai_retrieval_configs(enabled);
