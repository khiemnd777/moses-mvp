CREATE TABLE IF NOT EXISTS document_uploads (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title TEXT NOT NULL,
  file_name TEXT NOT NULL,
  content_type TEXT,
  storage_path TEXT NOT NULL UNIQUE,
  file_size_bytes BIGINT NOT NULL DEFAULT 0,
  sha256 TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'uploaded',
  analysis_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  error_message TEXT,
  document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
  document_asset_id UUID REFERENCES document_assets(id) ON DELETE SET NULL,
  document_version_id UUID REFERENCES document_versions(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE document_uploads ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';
ALTER TABLE document_uploads ADD COLUMN IF NOT EXISTS file_size_bytes BIGINT NOT NULL DEFAULT 0;
ALTER TABLE document_uploads ADD COLUMN IF NOT EXISTS sha256 TEXT NOT NULL DEFAULT '';
ALTER TABLE document_uploads ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'uploaded';
ALTER TABLE document_uploads ADD COLUMN IF NOT EXISTS analysis_json JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE document_uploads ADD COLUMN IF NOT EXISTS error_message TEXT;
ALTER TABLE document_uploads ADD COLUMN IF NOT EXISTS document_id UUID REFERENCES documents(id) ON DELETE SET NULL;
ALTER TABLE document_uploads ADD COLUMN IF NOT EXISTS document_asset_id UUID REFERENCES document_assets(id) ON DELETE SET NULL;
ALTER TABLE document_uploads ADD COLUMN IF NOT EXISTS document_version_id UUID REFERENCES document_versions(id) ON DELETE SET NULL;
ALTER TABLE document_uploads ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

DO $$
BEGIN
  ALTER TABLE document_uploads
    ADD CONSTRAINT document_uploads_status_check
    CHECK (status IN (
      'uploaded',
      'extracting',
      'classified',
      'profile_resolved',
      'indexing',
      'validating',
      'ready',
      'extract_failed',
      'classification_low_confidence',
      'needs_review',
      'validation_failed',
      'rejected',
      'archived'
    ));
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_document_uploads_status ON document_uploads(status);
CREATE INDEX IF NOT EXISTS idx_document_uploads_created ON document_uploads(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_document_uploads_sha256 ON document_uploads(sha256);
