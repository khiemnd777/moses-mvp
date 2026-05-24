CREATE TABLE IF NOT EXISTS document_upload_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  upload_id UUID REFERENCES document_uploads(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL DEFAULT 'stage_transition',
  stage TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  evidence_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  actor TEXT NOT NULL DEFAULT 'system',
  file_name TEXT NOT NULL DEFAULT '',
  content_type TEXT NOT NULL DEFAULT '',
  file_size_bytes BIGINT NOT NULL DEFAULT 0,
  sha256 TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE document_upload_events ADD COLUMN IF NOT EXISTS upload_id UUID REFERENCES document_uploads(id) ON DELETE CASCADE;
ALTER TABLE document_upload_events ADD COLUMN IF NOT EXISTS event_type TEXT NOT NULL DEFAULT 'stage_transition';
ALTER TABLE document_upload_events ADD COLUMN IF NOT EXISTS stage TEXT NOT NULL DEFAULT '';
ALTER TABLE document_upload_events ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT '';
ALTER TABLE document_upload_events ADD COLUMN IF NOT EXISTS message TEXT NOT NULL DEFAULT '';
ALTER TABLE document_upload_events ADD COLUMN IF NOT EXISTS evidence_json JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE document_upload_events ADD COLUMN IF NOT EXISTS actor TEXT NOT NULL DEFAULT 'system';
ALTER TABLE document_upload_events ADD COLUMN IF NOT EXISTS file_name TEXT NOT NULL DEFAULT '';
ALTER TABLE document_upload_events ADD COLUMN IF NOT EXISTS content_type TEXT NOT NULL DEFAULT '';
ALTER TABLE document_upload_events ADD COLUMN IF NOT EXISTS file_size_bytes BIGINT NOT NULL DEFAULT 0;
ALTER TABLE document_upload_events ADD COLUMN IF NOT EXISTS sha256 TEXT NOT NULL DEFAULT '';
ALTER TABLE document_upload_events ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_document_upload_events_upload_created ON document_upload_events(upload_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_document_upload_events_stage_created ON document_upload_events(stage, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_document_upload_events_sha_created ON document_upload_events(sha256, created_at DESC);
