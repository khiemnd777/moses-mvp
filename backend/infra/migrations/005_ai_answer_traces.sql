CREATE TABLE IF NOT EXISTS ai_answer_traces (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  trace_id TEXT NOT NULL UNIQUE,
  mode TEXT NOT NULL CHECK (mode IN ('answer', 'stream')),
  user_query TEXT NOT NULL,
  normalized_query TEXT,
  retrieval_filters_json JSONB,
  retrieved_chunk_ids_json JSONB,
  prompt_snapshot TEXT,
  model_name TEXT,
  temperature DOUBLE PRECISION,
  max_tokens INT,
  retry INT,
  response_text TEXT,
  stream_completed BOOLEAN NOT NULL DEFAULT FALSE,
  latency_ms INT,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_answer_traces_created_at
ON ai_answer_traces(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_answer_traces_trace_id
ON ai_answer_traces(trace_id);
