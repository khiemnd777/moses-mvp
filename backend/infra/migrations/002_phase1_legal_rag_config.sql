CREATE TABLE IF NOT EXISTS ai_guard_policies (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  min_retrieved_chunks INT NOT NULL DEFAULT 1,
  min_similarity_score DOUBLE PRECISION NOT NULL DEFAULT 0.7,
  on_empty_retrieval TEXT NOT NULL DEFAULT 'refuse',
  on_low_confidence TEXT NOT NULL DEFAULT 'ask_clarification',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT ai_guard_policies_empty_action_chk CHECK (on_empty_retrieval IN ('refuse', 'fallback_llm', 'ask_clarification')),
  CONSTRAINT ai_guard_policies_low_conf_action_chk CHECK (on_low_confidence IN ('refuse', 'fallback_llm', 'ask_clarification')),
  CONSTRAINT ai_guard_policies_min_chunks_chk CHECK (min_retrieved_chunks >= 0),
  CONSTRAINT ai_guard_policies_min_score_chk CHECK (min_similarity_score >= 0 AND min_similarity_score <= 1)
);

CREATE INDEX IF NOT EXISTS idx_ai_guard_policies_enabled ON ai_guard_policies(enabled);

CREATE TABLE IF NOT EXISTS ai_prompts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE,
  prompt_type TEXT NOT NULL,
  system_prompt TEXT NOT NULL,
  temperature DOUBLE PRECISION NOT NULL DEFAULT 0.2,
  max_tokens INT NOT NULL DEFAULT 1200,
  retry INT NOT NULL DEFAULT 2,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT ai_prompts_temperature_chk CHECK (temperature >= 0 AND temperature <= 2),
  CONSTRAINT ai_prompts_max_tokens_chk CHECK (max_tokens > 0),
  CONSTRAINT ai_prompts_retry_chk CHECK (retry >= 0)
);

CREATE INDEX IF NOT EXISTS idx_ai_prompts_prompt_type_enabled ON ai_prompts(prompt_type, enabled);
