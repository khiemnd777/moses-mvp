package infra

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/khiemnd777/legal_api/domain"

	"github.com/lib/pq"
)

type Store struct {
	DB *sql.DB
}

type ChunkVectorRow struct {
	ID                string
	DocumentVersionID string
	Index             int
	Text              string
	MetadataJSON      []byte
	EmbeddingJSON     []byte
}

type ReindexScopeQuery struct {
	DocTypeCode string
	Status      string
	Limit       int
}

type jobErrorMeta struct {
	Attempt int    `json:"attempt"`
	Message string `json:"message,omitempty"`
}

func NewStore(db *sql.DB) *Store {
	return &Store{DB: db}
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM users`).Scan(&count)
	return count, err
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
	var user domain.User
	query := `SELECT id, username, password_hash, role, created_at FROM users WHERE username = $1`
	err := s.DB.QueryRowContext(ctx, query, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt)
	return user, err
}

func (s *Store) CreateUser(ctx context.Context, user domain.User) error {
	return s.DB.QueryRowContext(
		ctx,
		`INSERT INTO users (id, username, password_hash, role) VALUES ($1, $2, $3, $4) RETURNING created_at`,
		user.ID,
		user.Username,
		user.PasswordHash,
		user.Role,
	).Scan(&user.CreatedAt)
}

func (s *Store) CreateDocType(ctx context.Context, code, name string, formJSON []byte, formHash string) (string, error) {
	var id string
	query := `INSERT INTO doc_types (code, name, form_json, form_hash) VALUES ($1,$2,$3,$4) RETURNING id`
	err := s.DB.QueryRowContext(ctx, query, code, name, formJSON, formHash).Scan(&id)
	return id, err
}

func (s *Store) ListDocTypes(ctx context.Context) ([]domain.DocType, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, code, name, form_json, form_hash, created_at, updated_at FROM doc_types ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.DocType
	for rows.Next() {
		var d domain.DocType
		if err := rows.Scan(&d.ID, &d.Code, &d.Name, &d.FormJSON, &d.FormHash, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) UpdateDocTypeForm(ctx context.Context, id string, formJSON []byte, formHash string) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE doc_types SET form_json = $1, form_hash = $2, updated_at = NOW() WHERE id = $3`, formJSON, formHash, id)
	if err != nil {
		return err
	}
	count, _ := res.RowsAffected()
	if count == 0 {
		return errors.New("doc_type not found")
	}
	return nil
}

func (s *Store) CountDocumentsByDocType(ctx context.Context, docTypeID string) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE doc_type_id = $1`, docTypeID).Scan(&count)
	return count, err
}

func (s *Store) DeleteDocType(ctx context.Context, id string) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM doc_types WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) GetDocType(ctx context.Context, id string) (domain.DocType, error) {
	var d domain.DocType
	query := `SELECT id, code, name, form_json, form_hash, created_at, updated_at FROM doc_types WHERE id = $1`
	err := s.DB.QueryRowContext(ctx, query, id).Scan(&d.ID, &d.Code, &d.Name, &d.FormJSON, &d.FormHash, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (s *Store) GetDocTypeByCode(ctx context.Context, code string) (domain.DocType, error) {
	var d domain.DocType
	query := `SELECT id, code, name, form_json, form_hash, created_at, updated_at FROM doc_types WHERE code = $1`
	err := s.DB.QueryRowContext(ctx, query, code).Scan(&d.ID, &d.Code, &d.Name, &d.FormJSON, &d.FormHash, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (s *Store) CreateDocument(ctx context.Context, docTypeID, title string) (string, error) {
	var id string
	err := s.DB.QueryRowContext(ctx, `INSERT INTO documents (doc_type_id, title) VALUES ($1,$2) RETURNING id`, docTypeID, title).Scan(&id)
	return id, err
}

func (s *Store) GetDocument(ctx context.Context, id string) (domain.Document, error) {
	var d domain.Document
	query := `SELECT d.id, d.doc_type_id, dt.code, d.title, d.created_at, d.updated_at
		FROM documents d
		JOIN doc_types dt ON dt.id = d.doc_type_id
		WHERE d.id = $1`
	err := s.DB.QueryRowContext(ctx, query, id).Scan(&d.ID, &d.DocTypeID, &d.DocTypeCode, &d.Title, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (s *Store) ListDocuments(ctx context.Context) ([]domain.Document, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT d.id, d.doc_type_id, dt.code, d.title, d.created_at, d.updated_at
		FROM documents d
		JOIN doc_types dt ON dt.id = d.doc_type_id
		ORDER BY d.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Document
	for rows.Next() {
		var d domain.Document
		if err := rows.Scan(&d.ID, &d.DocTypeID, &d.DocTypeCode, &d.Title, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) DeleteDocument(ctx context.Context, id string) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM ingest_jobs WHERE document_version_id IN (SELECT id FROM document_versions WHERE document_id=$1)`, id); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE document_version_id IN (SELECT id FROM document_versions WHERE document_id=$1)`, id); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM document_versions WHERE document_id=$1`, id); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM document_assets WHERE document_id=$1`, id); err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) EnqueueDeleteVectorsRepair(ctx context.Context, collection, documentID, documentVersionID string, filter Filter) (bool, error) {
	payload := VectorRepairPayload{
		DocumentID:        documentID,
		DocumentVersionID: documentVersionID,
		Filter:            &filter,
	}
	taskKey := repairTaskKey("delete_vectors_by_filter", collection, payload)
	return s.EnqueueVectorRepairTask(ctx, taskKey, "delete_vectors_by_filter", collection, payload)
}

func (s *Store) EnqueueRebuildVectorsRepair(ctx context.Context, collection, documentVersionID string) (bool, error) {
	payload := VectorRepairPayload{DocumentVersionID: documentVersionID}
	taskKey := repairTaskKey("rebuild_vectors_for_version", collection, payload)
	return s.EnqueueVectorRepairTask(ctx, taskKey, "rebuild_vectors_for_version", collection, payload)
}

func (s *Store) ListDocumentVersionIDsByDocument(ctx context.Context, documentID string) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM document_versions WHERE document_id = $1`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) ListDocumentAssetPaths(ctx context.Context, documentID string) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT storage_path FROM document_assets WHERE document_id=$1`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		out = append(out, path)
	}
	return out, rows.Err()
}

func (s *Store) ListDocumentAssets(ctx context.Context, documentID string) ([]domain.DocumentAssetWithVersions, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT
	a.id, a.document_id, a.file_name, a.content_type, a.storage_path, a.created_at,
	COALESCE(array_agg(v.version ORDER BY v.version) FILTER (WHERE v.id IS NOT NULL), '{}') AS versions
FROM document_assets a
LEFT JOIN document_versions v ON v.asset_id = a.id
WHERE a.document_id = $1
GROUP BY a.id, a.document_id, a.file_name, a.content_type, a.storage_path, a.created_at
ORDER BY a.created_at ASC
`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.DocumentAssetWithVersions
	for rows.Next() {
		var a domain.DocumentAssetWithVersions
		var versions pq.Int64Array
		if err := rows.Scan(&a.ID, &a.DocumentID, &a.FileName, &a.ContentType, &a.StoragePath, &a.CreatedAt, &versions); err != nil {
			return nil, err
		}
		a.Versions = make([]int, 0, len(versions))
		for _, v := range versions {
			a.Versions = append(a.Versions, int(v))
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) CreateDocumentAsset(ctx context.Context, documentID, fileName, contentType, storagePath string) (string, error) {
	var id string
	query := `INSERT INTO document_assets (document_id, file_name, content_type, storage_path) VALUES ($1,$2,$3,$4) RETURNING id`
	err := s.DB.QueryRowContext(ctx, query, documentID, fileName, contentType, storagePath).Scan(&id)
	return id, err
}

func (s *Store) GetDocumentAsset(ctx context.Context, id string) (domain.DocumentAsset, error) {
	var a domain.DocumentAsset
	query := `SELECT id, document_id, file_name, content_type, storage_path, created_at FROM document_assets WHERE id=$1`
	err := s.DB.QueryRowContext(ctx, query, id).Scan(&a.ID, &a.DocumentID, &a.FileName, &a.ContentType, &a.StoragePath, &a.CreatedAt)
	return a, err
}

func (s *Store) CreateDocumentVersion(ctx context.Context, documentID, assetID string) (string, error) {
	var id string
	query := `INSERT INTO document_versions (document_id, asset_id, version) VALUES ($1,$2,(SELECT COALESCE(MAX(version),0)+1 FROM document_versions WHERE document_id=$1)) RETURNING id`
	err := s.DB.QueryRowContext(ctx, query, documentID, assetID).Scan(&id)
	return id, err
}

func (s *Store) GetDocumentVersion(ctx context.Context, id string) (domain.DocumentVersion, error) {
	var v domain.DocumentVersion
	query := `SELECT id, document_id, asset_id, version, created_at FROM document_versions WHERE id=$1`
	err := s.DB.QueryRowContext(ctx, query, id).Scan(&v.ID, &v.DocumentID, &v.AssetID, &v.Version, &v.CreatedAt)
	return v, err
}

func (s *Store) DeleteDocumentVersion(ctx context.Context, id string) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM ingest_jobs WHERE document_version_id = $1`, id); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE document_version_id = $1`, id); err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM document_versions WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) CreateIngestJob(ctx context.Context, documentVersionID string) (string, error) {
	var id string
	query := `INSERT INTO ingest_jobs (document_version_id, status) VALUES ($1,'queued') RETURNING id`
	err := s.DB.QueryRowContext(ctx, query, documentVersionID).Scan(&id)
	return id, err
}

func (s *Store) EnqueueIngestJob(ctx context.Context, documentVersionID string) (domain.IngestJob, bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.IngestJob{}, false, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, documentVersionID); err != nil {
		return domain.IngestJob{}, false, err
	}

	var existing domain.IngestJob
	row := tx.QueryRowContext(ctx, `
SELECT id, document_version_id, status, error_message, created_at, updated_at
FROM ingest_jobs
WHERE document_version_id = $1
  AND status IN ('pending', 'processing', 'queued', 'running')
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE
`, documentVersionID)
	switch err := row.Scan(&existing.ID, &existing.DocumentVersionID, &existing.Status, &existing.ErrorMessage, &existing.CreatedAt, &existing.UpdatedAt); err {
	case nil:
		if err := tx.Commit(); err != nil {
			return domain.IngestJob{}, false, err
		}
		return existing, false, nil
	case sql.ErrNoRows:
	default:
		return domain.IngestJob{}, false, err
	}

	var created domain.IngestJob
	err = tx.QueryRowContext(ctx, `
INSERT INTO ingest_jobs (document_version_id, status, error_message)
VALUES ($1, 'pending', NULL)
RETURNING id, document_version_id, status, error_message, created_at, updated_at
`, documentVersionID).Scan(
		&created.ID,
		&created.DocumentVersionID,
		&created.Status,
		&created.ErrorMessage,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		return domain.IngestJob{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return domain.IngestJob{}, false, err
	}
	return created, true, nil
}

func (s *Store) ListIngestJobs(ctx context.Context) ([]domain.IngestJob, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, document_version_id, status, error_message, created_at, updated_at FROM ingest_jobs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.IngestJob
	for rows.Next() {
		var j domain.IngestJob
		if err := rows.Scan(&j.ID, &j.DocumentVersionID, &j.Status, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Store) DeleteIngestJob(ctx context.Context, id string) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM ingest_jobs WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) GetQueuedJobs(ctx context.Context, limit int) ([]domain.IngestJob, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, document_version_id, status, error_message, created_at, updated_at FROM ingest_jobs WHERE status='queued' ORDER BY created_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.IngestJob
	for rows.Next() {
		var j domain.IngestJob
		if err := rows.Scan(&j.ID, &j.DocumentVersionID, &j.Status, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Store) UpdateJobStatus(ctx context.Context, id, status, errorMessage string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE ingest_jobs SET status=$1, error_message=$2, updated_at=NOW() WHERE id=$3`, status, errorMessage, id)
	return err
}

func (s *Store) ClaimNextIngestJob(ctx context.Context) (domain.IngestJob, bool, error) {
	var job domain.IngestJob
	err := s.DB.QueryRowContext(ctx, `
WITH candidate AS (
	SELECT id
	FROM ingest_jobs
	WHERE status IN ('pending', 'queued')
	ORDER BY created_at ASC
	FOR UPDATE SKIP LOCKED
	LIMIT 1
)
UPDATE ingest_jobs j
SET status = 'processing',
    updated_at = NOW()
FROM candidate
WHERE j.id = candidate.id
RETURNING j.id, j.document_version_id, j.status, j.error_message, j.created_at, j.updated_at
`).Scan(&job.ID, &job.DocumentVersionID, &job.Status, &job.ErrorMessage, &job.CreatedAt, &job.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.IngestJob{}, false, nil
	}
	if err != nil {
		return domain.IngestJob{}, false, err
	}
	return job, true, nil
}

func (s *Store) TouchJob(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE ingest_jobs SET updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *Store) MarkJobCompleted(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `
UPDATE ingest_jobs
SET status = 'completed',
    error_message = NULL,
    updated_at = NOW()
WHERE id = $1
`, id)
	return err
}

func (s *Store) MarkJobFailed(ctx context.Context, id string, attempt int, message string) error {
	meta, err := encodeJobErrorMeta(jobErrorMeta{Attempt: attempt, Message: message})
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `
UPDATE ingest_jobs
SET status = 'failed',
    error_message = $1,
    updated_at = NOW()
WHERE id = $2
`, meta, id)
	return err
}

func (s *Store) ResetStaleProcessingJobs(ctx context.Context, staleBefore time.Time) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `
UPDATE ingest_jobs
SET status = 'pending',
    updated_at = NOW()
WHERE status IN ('processing', 'running')
  AND updated_at < $1
`, staleBefore)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) ListFailedIngestJobs(ctx context.Context, limit int) ([]domain.IngestJob, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT id, document_version_id, status, error_message, created_at, updated_at
FROM ingest_jobs
WHERE status = 'failed'
ORDER BY updated_at ASC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.IngestJob
	for rows.Next() {
		var job domain.IngestJob
		if err := rows.Scan(&job.ID, &job.DocumentVersionID, &job.Status, &job.ErrorMessage, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

func (s *Store) RequeueJob(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `
UPDATE ingest_jobs
SET status = 'pending',
    updated_at = NOW()
WHERE id = $1
  AND status = 'failed'
`, id)
	return err
}

func (s *Store) GetDocumentVersionBundle(ctx context.Context, id string) (domain.DocumentVersion, domain.Document, domain.DocumentAsset, domain.DocType, error) {
	var v domain.DocumentVersion
	var d domain.Document
	var a domain.DocumentAsset
	var t domain.DocType
	query := `
SELECT v.id, v.document_id, v.asset_id, v.version, v.created_at,
       d.id, d.doc_type_id, d.title, d.created_at, d.updated_at,
       a.id, a.document_id, a.file_name, a.content_type, a.storage_path, a.created_at,
       dt.id, dt.code, dt.name, dt.form_json, dt.form_hash, dt.created_at, dt.updated_at
FROM document_versions v
JOIN documents d ON d.id = v.document_id
JOIN document_assets a ON a.id = v.asset_id
JOIN doc_types dt ON dt.id = d.doc_type_id
WHERE v.id = $1`
	err := s.DB.QueryRowContext(ctx, query, id).Scan(
		&v.ID, &v.DocumentID, &v.AssetID, &v.Version, &v.CreatedAt,
		&d.ID, &d.DocTypeID, &d.Title, &d.CreatedAt, &d.UpdatedAt,
		&a.ID, &a.DocumentID, &a.FileName, &a.ContentType, &a.StoragePath, &a.CreatedAt,
		&t.ID, &t.Code, &t.Name, &t.FormJSON, &t.FormHash, &t.CreatedAt, &t.UpdatedAt,
	)
	return v, d, a, t, err
}

func (s *Store) InsertChunk(ctx context.Context, chunk domain.Chunk) (string, error) {
	var id string
	err := s.DB.QueryRowContext(ctx, `INSERT INTO chunks (document_version_id, idx, text, metadata_json, embedding_json) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		chunk.DocumentVersionID, chunk.Index, chunk.Text, chunk.MetadataJSON, chunk.EmbeddingJSON).Scan(&id)
	return id, err
}

func (s *Store) ReplaceChunks(ctx context.Context, documentVersionID string, chunks []domain.Chunk) ([]domain.Chunk, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE document_version_id = $1`, documentVersionID); err != nil {
		return nil, err
	}

	inserted := make([]domain.Chunk, 0, len(chunks))
	for _, chunk := range chunks {
		var id string
		if chunk.ID == "" {
			err := tx.QueryRowContext(ctx, `
INSERT INTO chunks (document_version_id, idx, text, metadata_json, embedding_json)
VALUES ($1, $2, $3, $4, $5)
RETURNING id
`, chunk.DocumentVersionID, chunk.Index, chunk.Text, chunk.MetadataJSON, chunk.EmbeddingJSON).Scan(&id)
			if err != nil {
				return nil, err
			}
		} else {
			err := tx.QueryRowContext(ctx, `
INSERT INTO chunks (id, document_version_id, idx, text, metadata_json, embedding_json)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id
`, chunk.ID, chunk.DocumentVersionID, chunk.Index, chunk.Text, chunk.MetadataJSON, chunk.EmbeddingJSON).Scan(&id)
			if err != nil {
				return nil, err
			}
		}
		chunk.ID = id
		inserted = append(inserted, chunk)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return inserted, nil
}

func (s *Store) DeleteChunksByVersion(ctx context.Context, documentVersionID string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM chunks WHERE document_version_id = $1`, documentVersionID)
	return err
}

func (s *Store) ListChunkIDsByVersion(ctx context.Context, documentVersionID string) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM chunks WHERE document_version_id = $1`, documentVersionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) ListChunkVectorsByVersion(ctx context.Context, documentVersionID string, afterIdx, limit int) ([]ChunkVectorRow, error) {
	if limit <= 0 {
		limit = 128
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT id, document_version_id, idx, text, metadata_json, embedding_json
FROM chunks
WHERE document_version_id = $1 AND idx > $2
ORDER BY idx ASC
LIMIT $3
`, documentVersionID, afterIdx, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ChunkVectorRow, 0, limit)
	for rows.Next() {
		var row ChunkVectorRow
		if err := rows.Scan(&row.ID, &row.DocumentVersionID, &row.Index, &row.Text, &row.MetadataJSON, &row.EmbeddingJSON); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ListChunkVectorRefsAfterID(ctx context.Context, afterID string, limit int) ([]ChunkVectorRow, error) {
	if limit <= 0 {
		limit = 256
	}
	var (
		rows *sql.Rows
		err  error
	)
	if afterID == "" {
		rows, err = s.DB.QueryContext(ctx, `
SELECT id, document_version_id, idx, COALESCE(embedding_json, '[]'::jsonb), COALESCE(metadata_json, '{}'::jsonb)
FROM chunks
ORDER BY id ASC
LIMIT $1
`, limit)
	} else {
		rows, err = s.DB.QueryContext(ctx, `
SELECT id, document_version_id, idx, COALESCE(embedding_json, '[]'::jsonb), COALESCE(metadata_json, '{}'::jsonb)
FROM chunks
WHERE id > $1::uuid
ORDER BY id ASC
LIMIT $2
`, afterID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ChunkVectorRow, 0, limit)
	for rows.Next() {
		var row ChunkVectorRow
		if err := rows.Scan(&row.ID, &row.DocumentVersionID, &row.Index, &row.EmbeddingJSON, &row.MetadataJSON); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) CountChunksByVersion(ctx context.Context, documentVersionID string) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM chunks
WHERE document_version_id = $1
`, documentVersionID).Scan(&count)
	return count, err
}

func (s *Store) GetChunksByIDs(ctx context.Context, ids []string) ([]domain.Chunk, error) {
	if len(ids) == 0 {
		return []domain.Chunk{}, nil
	}
	query := `SELECT id, document_version_id, idx, text, metadata_json, embedding_json, created_at FROM chunks WHERE id = ANY($1)`
	rows, err := s.DB.QueryContext(ctx, query, pqArray(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Chunk
	for rows.Next() {
		var c domain.Chunk
		if err := rows.Scan(&c.ID, &c.DocumentVersionID, &c.Index, &c.Text, &c.MetadataJSON, &c.EmbeddingJSON, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetChunksByVersionAndIndexes(ctx context.Context, documentVersionID string, indexes []int) ([]domain.Chunk, error) {
	if len(indexes) == 0 {
		return []domain.Chunk{}, nil
	}
	query := `
SELECT id, document_version_id, idx, text, metadata_json, embedding_json, created_at
FROM chunks
WHERE document_version_id = $1
  AND idx = ANY($2)
ORDER BY idx ASC
`
	int64Indexes := make([]int64, 0, len(indexes))
	for _, idx := range indexes {
		int64Indexes = append(int64Indexes, int64(idx))
	}
	rows, err := s.DB.QueryContext(ctx, query, documentVersionID, pq.Array(int64Indexes))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Chunk, 0, len(indexes))
	for rows.Next() {
		var c domain.Chunk
		if err := rows.Scan(&c.ID, &c.DocumentVersionID, &c.Index, &c.Text, &c.MetadataJSON, &c.EmbeddingJSON, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) ListDocumentVersionIDsForReindex(ctx context.Context, scope ReindexScopeQuery) ([]string, error) {
	limit := scope.Limit
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT dv.id
FROM document_versions dv
JOIN documents d ON d.id = dv.document_id
JOIN doc_types dt ON dt.id = d.doc_type_id
LEFT JOIN LATERAL (
	SELECT ij.status
	FROM ingest_jobs ij
	WHERE ij.document_version_id = dv.id
	ORDER BY ij.created_at DESC
	LIMIT 1
) latest ON TRUE
WHERE ($1 = '' OR dt.code = $1)
  AND ($2 = '' OR COALESCE(latest.status, 'never_ingested') = $2)
ORDER BY dv.created_at DESC
LIMIT $3
`, scope.DocTypeCode, scope.Status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) LogQuery(ctx context.Context, q string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO query_logs (query) VALUES ($1)`, q)
	return err
}

func (s *Store) LogAnswer(ctx context.Context, q, a string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO answer_logs (query, answer) VALUES ($1,$2)`, q, a)
	return err
}

func (s *Store) GetActiveAIGuardPolicy(ctx context.Context) (domain.AIGuardPolicy, error) {
	var policy domain.AIGuardPolicy
	query := `
SELECT id, name, enabled, min_retrieved_chunks, min_similarity_score, on_empty_retrieval, on_low_confidence, created_at, updated_at
FROM ai_guard_policies
WHERE enabled = TRUE
ORDER BY updated_at DESC, created_at DESC
LIMIT 1
`
	err := s.DB.QueryRowContext(ctx, query).Scan(
		&policy.ID,
		&policy.Name,
		&policy.Enabled,
		&policy.MinRetrievedChunks,
		&policy.MinSimilarityScore,
		&policy.OnEmptyRetrieval,
		&policy.OnLowConfidence,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)
	return policy, err
}

func (s *Store) GetActiveAIPromptByType(ctx context.Context, promptType string) (domain.AIPrompt, error) {
	var prompt domain.AIPrompt
	query := `
SELECT id, name, prompt_type, system_prompt, temperature, max_tokens, retry, enabled, created_at, updated_at
FROM ai_prompts
WHERE enabled = TRUE AND prompt_type = $1
ORDER BY updated_at DESC, created_at DESC
LIMIT 1
`
	err := s.DB.QueryRowContext(ctx, query, promptType).Scan(
		&prompt.ID,
		&prompt.Name,
		&prompt.PromptType,
		&prompt.SystemPrompt,
		&prompt.Temperature,
		&prompt.MaxTokens,
		&prompt.Retry,
		&prompt.Enabled,
		&prompt.CreatedAt,
		&prompt.UpdatedAt,
	)
	return prompt, err
}

func (s *Store) GetActiveAIRetrievalConfig(ctx context.Context) (domain.AIRetrievalConfig, error) {
	var cfg domain.AIRetrievalConfig
	var preferredRaw []byte
	var legalDomainRaw []byte
	query := `
SELECT id, name, enabled, default_top_k,
       rerank_enabled, rerank_vector_weight, rerank_keyword_weight, rerank_metadata_weight, rerank_article_weight,
       adjacent_chunk_enabled, adjacent_chunk_window,
       max_context_chunks, max_context_chars,
       default_effective_status, preferred_doc_types_json, legal_domain_defaults_json,
       created_at, updated_at
FROM ai_retrieval_configs
WHERE enabled = TRUE
ORDER BY updated_at DESC, created_at DESC
LIMIT 1
`
	err := s.DB.QueryRowContext(ctx, query).Scan(
		&cfg.ID,
		&cfg.Name,
		&cfg.Enabled,
		&cfg.DefaultTopK,
		&cfg.RerankEnabled,
		&cfg.RerankVectorWeight,
		&cfg.RerankKeywordWeight,
		&cfg.RerankMetadataWeight,
		&cfg.RerankArticleWeight,
		&cfg.AdjacentChunkEnabled,
		&cfg.AdjacentChunkWindow,
		&cfg.MaxContextChunks,
		&cfg.MaxContextChars,
		&cfg.DefaultEffectiveStatus,
		&preferredRaw,
		&legalDomainRaw,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
	)
	if err != nil {
		return cfg, err
	}
	if len(preferredRaw) > 0 {
		_ = json.Unmarshal(preferredRaw, &cfg.PreferredDocTypes)
	}
	if len(legalDomainRaw) > 0 {
		_ = json.Unmarshal(legalDomainRaw, &cfg.LegalDomainDefaultsJSON)
	}
	if cfg.PreferredDocTypes == nil {
		cfg.PreferredDocTypes = []string{}
	}
	if cfg.LegalDomainDefaultsJSON == nil {
		cfg.LegalDomainDefaultsJSON = map[string]interface{}{}
	}
	return cfg, nil
}

func (s *Store) SetChunkEmbedding(ctx context.Context, chunkID string, embedding []float64) error {
	b, err := json.Marshal(embedding)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE chunks SET embedding_json=$1 WHERE id=$2`, b, chunkID)
	return err
}

func (s *Store) TouchDocumentVersion(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE document_versions SET updated_at = NOW() WHERE id=$1`, id)
	return err
}

func (s *Store) EnsureDocTypeSeed(ctx context.Context) error {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM doc_types WHERE code='legal_normative'`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	seed := map[string]any{
		"version":         1,
		"doc_type":        map[string]any{"code": "legal_normative", "name": "Legal Normative"},
		"segment_rules":   map[string]any{"strategy": "legal_article", "hierarchy": "article", "normalization": "basic"},
		"metadata_schema": map[string]any{"fields": []map[string]any{{"name": "title", "type": "string"}, {"name": "date", "type": "date"}}},
		"mapping_rules": []map[string]any{
			{"field": "title", "regex": "^Title\\s*:\\s*(.+)$", "group": 1},
			{"field": "date", "regex": "^Date\\s*:\\s*(.+)$", "group": 1},
		},
		"reindex_policy": map[string]any{"on_content_change": true, "on_form_change": true},
	}
	b, _ := json.Marshal(seed)
	_, err = s.DB.ExecContext(ctx, `INSERT INTO doc_types (code, name, form_json, form_hash) VALUES ('legal_normative','Legal Normative',$1,'seed')`, b)
	return err
}

func (s *Store) EnsureAIConfigSeed(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO ai_guard_policies (
	name,
	enabled,
	min_retrieved_chunks,
	min_similarity_score,
	on_empty_retrieval,
	on_low_confidence
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (name) DO NOTHING
`,
		"default_legal_guard_policy",
		true,
		1,
		0.7,
		"refuse",
		"ask_clarification",
	)
	if err != nil {
		return err
	}

	_, err = s.DB.ExecContext(ctx, `
INSERT INTO ai_prompts (
	name,
	prompt_type,
	system_prompt,
	temperature,
	max_tokens,
	retry,
	enabled
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (name) DO NOTHING
`,
		"legal_guard_prompt",
		"legal_guard",
		"You are a Vietnamese legal assistant.\nUse ONLY the provided sources.\nNever invent legal provisions.\nIf sources are insufficient, say that no legal basis was found.\nCite legal provisions in human-readable format.",
		0.2,
		1200,
		2,
		true,
	)
	if err != nil {
		return err
	}

	preferredDocTypes, _ := json.Marshal([]string{"law", "resolution", "decree"})
	legalDomainDefaults, _ := json.Marshal(map[string]interface{}{
		"marriage_family": map[string]interface{}{
			"top_k":               6,
			"preferred_doc_types": []string{"law", "resolution"},
		},
		"criminal_law": map[string]interface{}{
			"top_k": 8,
		},
	})
	_, err = s.DB.ExecContext(ctx, `
INSERT INTO ai_retrieval_configs (
	name,
	enabled,
	default_top_k,
	rerank_enabled,
	rerank_vector_weight,
	rerank_keyword_weight,
	rerank_metadata_weight,
	rerank_article_weight,
	adjacent_chunk_enabled,
	adjacent_chunk_window,
	max_context_chunks,
	max_context_chars,
	default_effective_status,
	preferred_doc_types_json,
	legal_domain_defaults_json
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT (name) DO NOTHING
`,
		"default_legal_retrieval_config",
		true,
		5,
		true,
		0.55,
		0.25,
		0.15,
		0.05,
		true,
		1,
		12,
		12000,
		"active",
		preferredDocTypes,
		legalDomainDefaults,
	)
	return err
}

func (s *Store) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return s.DB.PingContext(ctx)
}

func pqArray(ids []string) interface{} {
	return pq.Array(ids)
}

func DecodeJobAttempt(job domain.IngestJob) int {
	meta := decodeJobErrorMeta(job.ErrorMessage)
	if meta.Attempt < 0 {
		return 0
	}
	return meta.Attempt
}

func DecodeJobMessage(job domain.IngestJob) string {
	return decodeJobErrorMeta(job.ErrorMessage).Message
}

func encodeJobErrorMeta(meta jobErrorMeta) (string, error) {
	b, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeJobErrorMeta(raw *string) jobErrorMeta {
	if raw == nil || *raw == "" {
		return jobErrorMeta{}
	}
	var meta jobErrorMeta
	if err := json.Unmarshal([]byte(*raw), &meta); err == nil {
		return meta
	}
	return jobErrorMeta{Message: *raw}
}

type WaitPostgresOptions struct {
	MaxRetries int
	Interval   time.Duration
	Timeout    time.Duration
}

func WaitForPostgres(
	ctx context.Context,
	db *sql.DB,
	opts WaitPostgresOptions,
) error {
	if db == nil {
		return errors.New("db is nil")
	}

	if opts.MaxRetries <= 0 {
		opts.MaxRetries = 10
	}
	if opts.Interval <= 0 {
		opts.Interval = 2 * time.Second
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 2 * time.Second
	}

	var lastErr error

	for i := 1; i <= opts.MaxRetries; i++ {
		pingCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		err := db.PingContext(pingCtx)
		cancel()

		if err == nil {
			return nil
		}

		lastErr = err

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(opts.Interval):
		}
	}

	return fmt.Errorf("postgres not ready after %d retries: %w", opts.MaxRetries, lastErr)
}
