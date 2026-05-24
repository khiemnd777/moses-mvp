package infra

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/khiemnd777/legal_api/core/schema"
	"github.com/khiemnd777/legal_api/domain"

	"github.com/lib/pq"
)

type Store struct {
	DB *sql.DB
}

var ErrInvalidDocumentUploadAction = errors.New("invalid document upload action")

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

func (s *Store) EnsureAuthSchema(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL,
  must_change_password BOOLEAN NOT NULL DEFAULT TRUE,
  password_changed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE TABLE IF NOT EXISTS auth_refresh_sessions (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  replaced_by_hash TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE auth_refresh_sessions ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;
ALTER TABLE auth_refresh_sessions ADD COLUMN IF NOT EXISTS replaced_by_hash TEXT;
ALTER TABLE auth_refresh_sessions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
CREATE INDEX IF NOT EXISTS idx_auth_refresh_sessions_user_id ON auth_refresh_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_refresh_sessions_expires_at ON auth_refresh_sessions(expires_at);
`)
	return err
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM users`).Scan(&count)
	return count, err
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
	var user domain.User
	query := `SELECT id, username, password_hash, role, must_change_password, password_changed_at, created_at FROM users WHERE username = $1`
	err := s.DB.QueryRowContext(ctx, query, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.MustChangePassword, &user.PasswordChangedAt, &user.CreatedAt)
	return user, err
}

func (s *Store) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	var user domain.User
	query := `SELECT id, username, password_hash, role, must_change_password, password_changed_at, created_at FROM users WHERE id = $1`
	err := s.DB.QueryRowContext(ctx, query, id).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.MustChangePassword, &user.PasswordChangedAt, &user.CreatedAt)
	return user, err
}

func (s *Store) CreateUser(ctx context.Context, user domain.User) error {
	return s.DB.QueryRowContext(
		ctx,
		`INSERT INTO users (id, username, password_hash, role, must_change_password, password_changed_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING created_at`,
		user.ID,
		user.Username,
		user.PasswordHash,
		user.Role,
		user.MustChangePassword,
		user.PasswordChangedAt,
	).Scan(&user.CreatedAt)
}

func (s *Store) UpdateUserPassword(ctx context.Context, userID, passwordHash string, changedAt time.Time) error {
	result, err := s.DB.ExecContext(
		ctx,
		`UPDATE users SET password_hash = $1, must_change_password = FALSE, password_changed_at = $2 WHERE id = $3`,
		passwordHash,
		changedAt,
		userID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) CreateRefreshSession(ctx context.Context, session domain.RefreshSession) error {
	return s.DB.QueryRowContext(
		ctx,
		`INSERT INTO auth_refresh_sessions (id, user_id, token_hash, expires_at, revoked_at, replaced_by_hash)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING created_at, updated_at`,
		session.ID,
		session.UserID,
		session.TokenHash,
		session.ExpiresAt,
		session.RevokedAt,
		session.ReplacedByHash,
	).Scan(&session.CreatedAt, &session.UpdatedAt)
}

func (s *Store) GetRefreshSessionByTokenHash(ctx context.Context, tokenHash string) (domain.RefreshSession, error) {
	var session domain.RefreshSession
	err := s.DB.QueryRowContext(
		ctx,
		`SELECT id, user_id, token_hash, expires_at, revoked_at, replaced_by_hash, created_at, updated_at
		 FROM auth_refresh_sessions
		 WHERE token_hash = $1`,
		tokenHash,
	).Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.ExpiresAt,
		&session.RevokedAt,
		&session.ReplacedByHash,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	return session, err
}

func (s *Store) RotateRefreshSession(ctx context.Context, sessionID, currentTokenHash, nextTokenHash string, expiresAt, rotatedAt time.Time) error {
	result, err := s.DB.ExecContext(
		ctx,
		`UPDATE auth_refresh_sessions
		 SET token_hash = $1,
		     expires_at = $2,
		     replaced_by_hash = $1,
		     updated_at = $3
		 WHERE id = $4
		   AND token_hash = $5
		   AND revoked_at IS NULL`,
		nextTokenHash,
		expiresAt,
		rotatedAt,
		sessionID,
		currentTokenHash,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) RevokeRefreshSessionByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	result, err := s.DB.ExecContext(
		ctx,
		`UPDATE auth_refresh_sessions
		 SET revoked_at = COALESCE(revoked_at, $2),
		     updated_at = $2
		 WHERE token_hash = $1`,
		tokenHash,
		revokedAt,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) RevokeAllRefreshSessionsByUserID(ctx context.Context, userID string, revokedAt time.Time) error {
	_, err := s.DB.ExecContext(
		ctx,
		`UPDATE auth_refresh_sessions
		 SET revoked_at = COALESCE(revoked_at, $2),
		     updated_at = $2
		 WHERE user_id = $1
		   AND revoked_at IS NULL`,
		userID,
		revokedAt,
	)
	return err
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

func (s *Store) CreateDocumentUpload(ctx context.Context, input domain.DocumentUploadInput) (domain.DocumentUpload, error) {
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "uploaded"
	}
	analysisJSON := input.AnalysisJSON
	if len(analysisJSON) == 0 {
		analysisJSON = []byte(`{}`)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.DocumentUpload{}, err
	}
	defer tx.Rollback()

	var upload domain.DocumentUpload
	query := `
INSERT INTO document_uploads (
	title,
	file_name,
	content_type,
	storage_path,
	file_size_bytes,
	sha256,
	status,
	analysis_json
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
RETURNING id, title, file_name, content_type, storage_path, file_size_bytes, sha256, status, analysis_json,
	error_message, document_id, document_asset_id, document_version_id, created_at, updated_at`
	row := tx.QueryRowContext(
		ctx,
		query,
		input.Title,
		input.FileName,
		input.ContentType,
		input.StoragePath,
		input.FileSizeBytes,
		input.SHA256,
		status,
		analysisJSON,
	)
	if err := scanDocumentUpload(row, &upload); err != nil {
		return domain.DocumentUpload{}, err
	}
	for _, eventInput := range input.Events {
		if eventInput.UploadID == nil {
			uploadID := upload.ID
			eventInput.UploadID = &uploadID
		}
		if eventInput.FileName == "" {
			eventInput.FileName = upload.FileName
		}
		if eventInput.ContentType == "" {
			eventInput.ContentType = upload.ContentType
		}
		if eventInput.FileSizeBytes == 0 {
			eventInput.FileSizeBytes = upload.FileSizeBytes
		}
		if eventInput.SHA256 == "" {
			eventInput.SHA256 = upload.SHA256
		}
		event, err := insertDocumentUploadEvent(ctx, tx, eventInput)
		if err != nil {
			return domain.DocumentUpload{}, err
		}
		upload.Events = append(upload.Events, event)
	}
	if err := tx.Commit(); err != nil {
		return domain.DocumentUpload{}, err
	}
	return upload, nil
}

func (s *Store) CreateDocumentUploadEvent(ctx context.Context, input domain.DocumentUploadEventInput) (domain.DocumentUploadEvent, error) {
	return insertDocumentUploadEvent(ctx, s.DB, input)
}

func (s *Store) GetDocumentUpload(ctx context.Context, id string) (domain.DocumentUpload, error) {
	var upload domain.DocumentUpload
	row := s.DB.QueryRowContext(ctx, `
SELECT id, title, file_name, content_type, storage_path, file_size_bytes, sha256, status, analysis_json,
	error_message, document_id, document_asset_id, document_version_id, created_at, updated_at
FROM document_uploads
WHERE id = $1`, id)
	if err := scanDocumentUpload(row, &upload); err != nil {
		return domain.DocumentUpload{}, err
	}
	uploads := []domain.DocumentUpload{upload}
	if err := s.attachDocumentUploadEvents(ctx, uploads); err != nil {
		return domain.DocumentUpload{}, err
	}
	return uploads[0], nil
}

func (s *Store) ListDocumentUploads(ctx context.Context, limit int) ([]domain.DocumentUpload, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.DB.QueryContext(ctx, `
SELECT id, title, file_name, content_type, storage_path, file_size_bytes, sha256, status, analysis_json,
	error_message, document_id, document_asset_id, document_version_id, created_at, updated_at
FROM document_uploads
ORDER BY created_at DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.DocumentUpload, 0)
	for rows.Next() {
		var upload domain.DocumentUpload
		if err := scanDocumentUpload(rows, &upload); err != nil {
			return nil, err
		}
		out = append(out, upload)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachDocumentUploadEvents(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) GetPipelineHealth(ctx context.Context, opts domain.PipelineHealthOptions) (domain.PipelineHealth, error) {
	generatedAt := time.Now().UTC()
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	recentSince := opts.RecentSince
	if recentSince.IsZero() {
		recentSince = generatedAt.Add(-24 * time.Hour)
	}
	staleBefore := opts.StaleBefore
	if staleBefore.IsZero() {
		staleBefore = generatedAt.Add(-15 * time.Minute)
	}

	uploadCounts, err := s.pipelineUploadStatusCounts(ctx)
	if err != nil {
		return domain.PipelineHealth{}, err
	}
	jobCounts, err := s.pipelineJobStatusCounts(ctx)
	if err != nil {
		return domain.PipelineHealth{}, err
	}
	stageCounts, err := s.pipelineStageStatusCounts(ctx, recentSince)
	if err != nil {
		return domain.PipelineHealth{}, err
	}
	security, err := s.pipelineSecurityStats(ctx, recentSince)
	if err != nil {
		return domain.PipelineHealth{}, err
	}
	latency, err := s.pipelineLatencyStats(ctx, recentSince)
	if err != nil {
		return domain.PipelineHealth{}, err
	}
	staleUploads, err := s.pipelineStaleUploads(ctx, staleBefore, generatedAt, limit)
	if err != nil {
		return domain.PipelineHealth{}, err
	}
	recentIssues, err := s.pipelineRecentIssues(ctx, recentSince, generatedAt, limit)
	if err != nil {
		return domain.PipelineHealth{}, err
	}

	summary := summarizePipelineHealth(uploadCounts, jobCounts, security, staleUploads, recentIssues)
	alerts, severity := evaluatePipelineHealth(summary, latency)
	return domain.PipelineHealth{
		GeneratedAt:       generatedAt,
		RecentSince:       recentSince,
		StaleBefore:       staleBefore,
		Severity:          severity,
		Alerts:            alerts,
		Summary:           summary,
		UploadStatusCount: uploadCounts,
		JobStatusCount:    jobCounts,
		StageStatusCount:  stageCounts,
		Security:          security,
		Latency:           latency,
		StaleUploads:      staleUploads,
		RecentIssues:      recentIssues,
	}, nil
}

func (s *Store) pipelineUploadStatusCounts(ctx context.Context) ([]domain.PipelineStatusCount, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT status, COUNT(*)
FROM document_uploads
GROUP BY status
ORDER BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var counts []domain.PipelineStatusCount
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts = append(counts, domain.PipelineStatusCount{Status: status, Count: int(count)})
	}
	return counts, rows.Err()
}

func (s *Store) pipelineJobStatusCounts(ctx context.Context) ([]domain.PipelineStatusCount, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT status, COUNT(*)
FROM ingest_jobs
GROUP BY status
ORDER BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var counts []domain.PipelineStatusCount
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts = append(counts, domain.PipelineStatusCount{Status: status, Count: int(count)})
	}
	return counts, rows.Err()
}

func (s *Store) pipelineStageStatusCounts(ctx context.Context, recentSince time.Time) ([]domain.PipelineStageStatusCount, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT stage, status, COUNT(*)
FROM document_upload_events
WHERE created_at >= $1
GROUP BY stage, status
ORDER BY stage, status`, recentSince)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var counts []domain.PipelineStageStatusCount
	for rows.Next() {
		var stage, status string
		var count int64
		if err := rows.Scan(&stage, &status, &count); err != nil {
			return nil, err
		}
		counts = append(counts, domain.PipelineStageStatusCount{Stage: stage, Status: status, Count: int(count)})
	}
	return counts, rows.Err()
}

func (s *Store) pipelineSecurityStats(ctx context.Context, recentSince time.Time) (domain.PipelineSecurityStats, error) {
	var passed, blocked, unavailable int64
	err := s.DB.QueryRowContext(ctx, `
SELECT
	COUNT(*) FILTER (WHERE status = 'done') AS passed,
	COUNT(*) FILTER (WHERE status IN ('blocked', 'failed', 'error', 'rejected')) AS blocked,
	COUNT(*) FILTER (WHERE status = 'unavailable') AS unavailable
FROM document_upload_events
WHERE stage = 'security'
  AND created_at >= $1`, recentSince).Scan(&passed, &blocked, &unavailable)
	if err != nil {
		return domain.PipelineSecurityStats{}, err
	}
	return domain.PipelineSecurityStats{
		Passed:      int(passed),
		Blocked:     int(blocked),
		Unavailable: int(unavailable),
	}, nil
}

func (s *Store) pipelineLatencyStats(ctx context.Context, recentSince time.Time) (domain.PipelineLatencyStats, error) {
	var completed int64
	var avg, p50, p95, max sql.NullFloat64
	err := s.DB.QueryRowContext(ctx, `
SELECT
	COUNT(*),
	AVG(EXTRACT(EPOCH FROM (updated_at - created_at))),
	percentile_cont(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (updated_at - created_at))),
	percentile_cont(0.95) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (updated_at - created_at))),
	MAX(EXTRACT(EPOCH FROM (updated_at - created_at)))
FROM document_uploads
WHERE status IN ('ready', 'published')
  AND updated_at >= $1`, recentSince).Scan(&completed, &avg, &p50, &p95, &max)
	if err != nil {
		return domain.PipelineLatencyStats{}, err
	}
	return domain.PipelineLatencyStats{
		CompletedCount: int(completed),
		AverageSeconds: pipelineNullFloat(avg),
		P50Seconds:     pipelineNullFloat(p50),
		P95Seconds:     pipelineNullFloat(p95),
		MaxSeconds:     pipelineNullFloat(max),
	}, nil
}

func (s *Store) pipelineStaleUploads(ctx context.Context, staleBefore, generatedAt time.Time, limit int) ([]domain.PipelineHealthIssue, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT id, title, file_name, status, COALESCE(error_message, ''), created_at, updated_at
FROM document_uploads
WHERE status = ANY($1::text[])
  AND updated_at < $2
ORDER BY updated_at ASC
LIMIT $3`, pq.Array(pipelineProcessingUploadStatuses()), staleBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	issues := make([]domain.PipelineHealthIssue, 0)
	for rows.Next() {
		var issue domain.PipelineHealthIssue
		var uploadID string
		if err := rows.Scan(
			&uploadID,
			&issue.Title,
			&issue.FileName,
			&issue.Status,
			&issue.ErrorMessage,
			&issue.CreatedAt,
			&issue.UpdatedAt,
		); err != nil {
			return nil, err
		}
		issue.UploadID = &uploadID
		issue.Stage, issue.EventStatus, _ = documentUploadStatusEventShape(issue.Status)
		issue.Message = "Upload has not advanced before the stale threshold"
		issue.AgeSeconds = pipelineIssueAgeSeconds(generatedAt, issue.UpdatedAt)
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

func (s *Store) pipelineRecentIssues(ctx context.Context, recentSince, generatedAt time.Time, limit int) ([]domain.PipelineHealthIssue, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT
	e.upload_id,
	COALESCE(NULLIF(u.title, ''), e.file_name, '') AS title,
	COALESCE(NULLIF(u.file_name, ''), e.file_name, '') AS file_name,
	COALESCE(u.status, '') AS upload_status,
	COALESCE(u.error_message, '') AS error_message,
	e.stage,
	e.status,
	e.message,
	COALESCE(u.created_at, e.created_at) AS created_at,
	COALESCE(u.updated_at, e.created_at) AS updated_at,
	e.created_at AS event_created_at
FROM document_upload_events e
LEFT JOIN document_uploads u ON u.id = e.upload_id
WHERE e.created_at >= $1
  AND (
	e.event_type IN ('stage_error', 'stage_review')
	OR e.status IN ('blocked', 'warning', 'failed', 'error', 'unavailable', 'rejected')
	OR COALESCE(u.status, '') = ANY($2::text[])
  )
ORDER BY e.created_at DESC
LIMIT $3`, recentSince, pq.Array(pipelineReviewOrFailureUploadStatuses()), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	issues := make([]domain.PipelineHealthIssue, 0)
	for rows.Next() {
		var issue domain.PipelineHealthIssue
		var uploadID sql.NullString
		var eventCreatedAt time.Time
		if err := rows.Scan(
			&uploadID,
			&issue.Title,
			&issue.FileName,
			&issue.Status,
			&issue.ErrorMessage,
			&issue.Stage,
			&issue.EventStatus,
			&issue.Message,
			&issue.CreatedAt,
			&issue.UpdatedAt,
			&eventCreatedAt,
		); err != nil {
			return nil, err
		}
		if uploadID.Valid {
			issue.UploadID = &uploadID.String
		}
		issue.EventCreatedAt = &eventCreatedAt
		issue.AgeSeconds = pipelineIssueAgeSeconds(generatedAt, eventCreatedAt)
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

func summarizePipelineHealth(uploadCounts, jobCounts []domain.PipelineStatusCount, security domain.PipelineSecurityStats, staleUploads, recentIssues []domain.PipelineHealthIssue) domain.PipelineHealthSummary {
	uploadsByStatus := pipelineCountsByStatus(uploadCounts)
	jobsByStatus := pipelineCountsByStatus(jobCounts)
	summary := domain.PipelineHealthSummary{
		StaleUploads:        len(staleUploads),
		RecentIssues:        len(recentIssues),
		SecurityBlocked:     security.Blocked,
		SecurityUnavailable: security.Unavailable,
	}
	for _, item := range uploadCounts {
		summary.TotalUploads += item.Count
	}
	for _, status := range pipelineProcessingUploadStatuses() {
		summary.ProcessingUploads += uploadsByStatus[status]
	}
	for _, status := range []string{"classification_low_confidence", "needs_review", "validation_failed"} {
		summary.ReviewUploads += uploadsByStatus[status]
	}
	for _, status := range []string{"extract_failed", "validation_failed", "rejected"} {
		summary.FailedUploads += uploadsByStatus[status]
	}
	for _, status := range []string{"ready", "published"} {
		summary.PublishedUploads += uploadsByStatus[status]
	}
	for _, status := range []string{"pending", "queued", "processing", "running"} {
		summary.ActiveJobs += jobsByStatus[status]
	}
	summary.FailedJobs = jobsByStatus["failed"]
	return summary
}

func evaluatePipelineHealth(summary domain.PipelineHealthSummary, latency domain.PipelineLatencyStats) ([]domain.PipelineHealthAlert, string) {
	alerts := make([]domain.PipelineHealthAlert, 0)
	add := func(code, severity, message string, value, threshold float64) {
		alerts = append(alerts, domain.PipelineHealthAlert{
			Code:      code,
			Severity:  severity,
			Message:   message,
			Value:     value,
			Threshold: threshold,
		})
	}
	if summary.SecurityUnavailable > 0 {
		add("security_scanner_unavailable", "critical", "Upload security scanner reported unavailable events", float64(summary.SecurityUnavailable), 0)
	}
	if summary.StaleUploads > 0 {
		add("stale_uploads", "critical", "Uploads are stuck beyond the stale threshold", float64(summary.StaleUploads), 0)
	}
	if summary.FailedJobs > 0 {
		add("failed_ingest_jobs", "critical", "Ingest jobs are failed and require operator action", float64(summary.FailedJobs), 0)
	}
	if summary.FailedUploads > 0 {
		add("failed_uploads", "degraded", "Uploads reached failed terminal states", float64(summary.FailedUploads), 0)
	}
	if summary.ReviewUploads > 0 {
		add("review_backlog", "degraded", "Uploads are waiting for operator review", float64(summary.ReviewUploads), 0)
	}
	if summary.SecurityBlocked > 0 {
		add("security_blocked_uploads", "degraded", "Security scan blocked suspicious uploads", float64(summary.SecurityBlocked), 0)
	}
	if summary.ActiveJobs > 10 {
		add("index_backlog_high", "degraded", "Index job backlog is above the expected operating threshold", float64(summary.ActiveJobs), 10)
	}
	if latency.CompletedCount > 0 && latency.P95Seconds > 600 {
		add("publish_latency_high", "degraded", "Pipeline publish latency p95 is above threshold", latency.P95Seconds, 600)
	}

	severity := "ok"
	for _, alert := range alerts {
		if alert.Severity == "critical" {
			return alerts, "critical"
		}
		if alert.Severity == "degraded" {
			severity = "degraded"
		}
	}
	return alerts, severity
}

func pipelineCountsByStatus(counts []domain.PipelineStatusCount) map[string]int {
	out := make(map[string]int, len(counts))
	for _, item := range counts {
		out[item.Status] = item.Count
	}
	return out
}

func pipelineProcessingUploadStatuses() []string {
	return []string{"uploaded", "extracting", "classified", "profile_resolved", "indexing", "validating"}
}

func pipelineReviewOrFailureUploadStatuses() []string {
	return []string{"extract_failed", "classification_low_confidence", "needs_review", "validation_failed", "rejected"}
}

func pipelineNullFloat(value sql.NullFloat64) float64 {
	if !value.Valid {
		return 0
	}
	return value.Float64
}

func pipelineIssueAgeSeconds(now, since time.Time) int64 {
	seconds := int64(now.Sub(since).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}

func (s *Store) ClaimNextDocumentUpload(ctx context.Context) (domain.DocumentUpload, bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.DocumentUpload{}, false, err
	}
	defer tx.Rollback()

	var upload domain.DocumentUpload
	row := tx.QueryRowContext(ctx, `
SELECT id, title, file_name, content_type, storage_path, file_size_bytes, sha256, status, analysis_json,
	error_message, document_id, document_asset_id, document_version_id, created_at, updated_at
FROM document_uploads
WHERE status = 'uploaded'
ORDER BY created_at ASC
FOR UPDATE SKIP LOCKED
LIMIT 1`)
	if err := scanDocumentUpload(row, &upload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.DocumentUpload{}, false, nil
		}
		return domain.DocumentUpload{}, false, err
	}

	row = tx.QueryRowContext(ctx, `
UPDATE document_uploads
SET status = 'extracting', updated_at = NOW()
WHERE id = $1
RETURNING id, title, file_name, content_type, storage_path, file_size_bytes, sha256, status, analysis_json,
	error_message, document_id, document_asset_id, document_version_id, created_at, updated_at`, upload.ID)
	if err := scanDocumentUpload(row, &upload); err != nil {
		return domain.DocumentUpload{}, false, err
	}
	if _, err := insertDocumentUploadEvent(ctx, tx, domain.DocumentUploadEventInput{
		UploadID:      &upload.ID,
		EventType:     "stage_transition",
		Stage:         "extract",
		Status:        "active",
		Message:       "Worker claimed upload for extraction",
		Actor:         "worker",
		FileName:      upload.FileName,
		ContentType:   upload.ContentType,
		FileSizeBytes: upload.FileSizeBytes,
		SHA256:        upload.SHA256,
	}); err != nil {
		return domain.DocumentUpload{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return domain.DocumentUpload{}, false, err
	}
	return upload, true, nil
}

func (s *Store) UpdateDocumentUploadStatus(ctx context.Context, id, status string, analysisJSON []byte, errorMessage *string) error {
	if len(analysisJSON) == 0 {
		analysisJSON = []byte(`{}`)
	}
	var errValue sql.NullString
	if errorMessage != nil {
		errValue = sql.NullString{String: *errorMessage, Valid: true}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var upload domain.DocumentUpload
	row := tx.QueryRowContext(ctx, `
UPDATE document_uploads
SET status = $2,
	analysis_json = $3,
	error_message = $4,
	updated_at = NOW()
WHERE id = $1
RETURNING id, title, file_name, content_type, storage_path, file_size_bytes, sha256, status, analysis_json,
	error_message, document_id, document_asset_id, document_version_id, created_at, updated_at`, id, status, analysisJSON, errValue)
	if err := scanDocumentUpload(row, &upload); err != nil {
		return err
	}
	if eventInput, ok := documentUploadStatusEventInput(upload, status, analysisJSON, errorMessage); ok {
		if _, err := insertDocumentUploadEvent(ctx, tx, eventInput); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) PromoteDocumentUpload(ctx context.Context, upload domain.DocumentUpload, docType domain.DocType, analysisJSON []byte) (domain.DocumentUploadPromotion, error) {
	if len(analysisJSON) == 0 {
		analysisJSON = []byte(`{}`)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	defer tx.Rollback()

	var promotion domain.DocumentUploadPromotion
	if err := tx.QueryRowContext(ctx, `INSERT INTO documents (doc_type_id, title) VALUES ($1,$2) RETURNING id`, docType.ID, upload.Title).Scan(&promotion.DocumentID); err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	if err := tx.QueryRowContext(
		ctx,
		`INSERT INTO document_assets (document_id, file_name, content_type, storage_path) VALUES ($1,$2,$3,$4) RETURNING id`,
		promotion.DocumentID,
		upload.FileName,
		upload.ContentType,
		upload.StoragePath,
	).Scan(&promotion.DocumentAssetID); err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	if err := tx.QueryRowContext(
		ctx,
		`INSERT INTO document_versions (document_id, asset_id, version) VALUES ($1,$2,1) RETURNING id`,
		promotion.DocumentID,
		promotion.DocumentAssetID,
	).Scan(&promotion.DocumentVersionID); err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	if err := tx.QueryRowContext(
		ctx,
		`INSERT INTO ingest_jobs (document_version_id, status) VALUES ($1,'queued') RETURNING id`,
		promotion.DocumentVersionID,
	).Scan(&promotion.IngestJobID); err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	result, err := tx.ExecContext(ctx, `
UPDATE document_uploads
SET status = 'indexing',
	analysis_json = $2,
	error_message = NULL,
	document_id = $3,
	document_asset_id = $4,
	document_version_id = $5,
	updated_at = NOW()
WHERE id = $1`, upload.ID, analysisJSON, promotion.DocumentID, promotion.DocumentAssetID, promotion.DocumentVersionID)
	if err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	if affected == 0 {
		return domain.DocumentUploadPromotion{}, sql.ErrNoRows
	}
	promotionEvidence := mustJSON(map[string]interface{}{
		"doc_type_code":       docType.Code,
		"document_id":         promotion.DocumentID,
		"document_asset_id":   promotion.DocumentAssetID,
		"document_version_id": promotion.DocumentVersionID,
		"ingest_job_id":       promotion.IngestJobID,
		"pipeline_version":    "rag_upload_intake_v1",
		"status_reason":       "profile_resolved",
	})
	for _, eventInput := range []domain.DocumentUploadEventInput{
		{
			UploadID:      &upload.ID,
			EventType:     "stage_transition",
			Stage:         "profile",
			Status:        "done",
			Message:       "Resolved ingestion profile from classified document type",
			EvidenceJSON:  promotionEvidence,
			Actor:         "worker",
			FileName:      upload.FileName,
			ContentType:   upload.ContentType,
			FileSizeBytes: upload.FileSizeBytes,
			SHA256:        upload.SHA256,
		},
		{
			UploadID:      &upload.ID,
			EventType:     "stage_transition",
			Stage:         "document_version",
			Status:        "done",
			Message:       "Created document, asset, and version records",
			EvidenceJSON:  promotionEvidence,
			Actor:         "worker",
			FileName:      upload.FileName,
			ContentType:   upload.ContentType,
			FileSizeBytes: upload.FileSizeBytes,
			SHA256:        upload.SHA256,
		},
		{
			UploadID:      &upload.ID,
			EventType:     "stage_transition",
			Stage:         "embed_index",
			Status:        "active",
			Message:       "Queued ingest job for chunking, embedding, indexing, and validation",
			EvidenceJSON:  promotionEvidence,
			Actor:         "worker",
			FileName:      upload.FileName,
			ContentType:   upload.ContentType,
			FileSizeBytes: upload.FileSizeBytes,
			SHA256:        upload.SHA256,
		},
	} {
		if _, err := insertDocumentUploadEvent(ctx, tx, eventInput); err != nil {
			return domain.DocumentUploadPromotion{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	return promotion, nil
}

func (s *Store) ApproveDocumentUpload(ctx context.Context, id string, docType domain.DocType, analysisJSON []byte, actor, reason string) (domain.DocumentUploadPromotion, error) {
	if len(analysisJSON) == 0 {
		analysisJSON = []byte(`{}`)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	defer tx.Rollback()

	var upload domain.DocumentUpload
	row := tx.QueryRowContext(ctx, `
SELECT id, title, file_name, content_type, storage_path, file_size_bytes, sha256, status, analysis_json,
	error_message, document_id, document_asset_id, document_version_id, created_at, updated_at
FROM document_uploads
WHERE id = $1
FOR UPDATE`, id)
	if err := scanDocumentUpload(row, &upload); err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	if upload.Status != "classification_low_confidence" && upload.Status != "needs_review" {
		return domain.DocumentUploadPromotion{}, ErrInvalidDocumentUploadAction
	}
	if upload.DocumentVersionID != nil {
		return domain.DocumentUploadPromotion{}, ErrInvalidDocumentUploadAction
	}

	var promotion domain.DocumentUploadPromotion
	if err := tx.QueryRowContext(ctx, `INSERT INTO documents (doc_type_id, title) VALUES ($1,$2) RETURNING id`, docType.ID, upload.Title).Scan(&promotion.DocumentID); err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	if err := tx.QueryRowContext(
		ctx,
		`INSERT INTO document_assets (document_id, file_name, content_type, storage_path) VALUES ($1,$2,$3,$4) RETURNING id`,
		promotion.DocumentID,
		upload.FileName,
		upload.ContentType,
		upload.StoragePath,
	).Scan(&promotion.DocumentAssetID); err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	if err := tx.QueryRowContext(
		ctx,
		`INSERT INTO document_versions (document_id, asset_id, version) VALUES ($1,$2,1) RETURNING id`,
		promotion.DocumentID,
		promotion.DocumentAssetID,
	).Scan(&promotion.DocumentVersionID); err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	if err := tx.QueryRowContext(
		ctx,
		`INSERT INTO ingest_jobs (document_version_id, status) VALUES ($1,'queued') RETURNING id`,
		promotion.DocumentVersionID,
	).Scan(&promotion.IngestJobID); err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	result, err := tx.ExecContext(ctx, `
UPDATE document_uploads
SET status = 'indexing',
	analysis_json = $2,
	error_message = NULL,
	document_id = $3,
	document_asset_id = $4,
	document_version_id = $5,
	updated_at = NOW()
WHERE id = $1`, upload.ID, analysisJSON, promotion.DocumentID, promotion.DocumentAssetID, promotion.DocumentVersionID)
	if err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	if affected == 0 {
		return domain.DocumentUploadPromotion{}, sql.ErrNoRows
	}

	evidence := mustJSON(map[string]interface{}{
		"doc_type_code":       docType.Code,
		"document_id":         promotion.DocumentID,
		"document_asset_id":   promotion.DocumentAssetID,
		"document_version_id": promotion.DocumentVersionID,
		"ingest_job_id":       promotion.IngestJobID,
		"pipeline_version":    "rag_upload_intake_v1",
		"reason":              reason,
		"status_reason":       "approved_by_operator",
	})
	for _, eventInput := range []domain.DocumentUploadEventInput{
		{
			UploadID:      &upload.ID,
			EventType:     "review_action",
			Stage:         "classify",
			Status:        "done",
			Message:       "Operator approved document type classification",
			EvidenceJSON:  evidence,
			Actor:         actor,
			FileName:      upload.FileName,
			ContentType:   upload.ContentType,
			FileSizeBytes: upload.FileSizeBytes,
			SHA256:        upload.SHA256,
		},
		{
			UploadID:      &upload.ID,
			EventType:     "stage_transition",
			Stage:         "profile",
			Status:        "done",
			Message:       "Resolved ingestion profile from operator-approved document type",
			EvidenceJSON:  evidence,
			Actor:         "worker",
			FileName:      upload.FileName,
			ContentType:   upload.ContentType,
			FileSizeBytes: upload.FileSizeBytes,
			SHA256:        upload.SHA256,
		},
		{
			UploadID:      &upload.ID,
			EventType:     "stage_transition",
			Stage:         "document_version",
			Status:        "done",
			Message:       "Created document, asset, and version records",
			EvidenceJSON:  evidence,
			Actor:         "worker",
			FileName:      upload.FileName,
			ContentType:   upload.ContentType,
			FileSizeBytes: upload.FileSizeBytes,
			SHA256:        upload.SHA256,
		},
		{
			UploadID:      &upload.ID,
			EventType:     "stage_transition",
			Stage:         "embed_index",
			Status:        "active",
			Message:       "Queued ingest job for chunking, embedding, indexing, and validation",
			EvidenceJSON:  evidence,
			Actor:         "worker",
			FileName:      upload.FileName,
			ContentType:   upload.ContentType,
			FileSizeBytes: upload.FileSizeBytes,
			SHA256:        upload.SHA256,
		},
	} {
		if _, err := insertDocumentUploadEvent(ctx, tx, eventInput); err != nil {
			return domain.DocumentUploadPromotion{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.DocumentUploadPromotion{}, err
	}
	return promotion, nil
}

func (s *Store) ReindexDocumentUpload(ctx context.Context, id, actor, reason string) (domain.IngestJob, bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.IngestJob{}, false, err
	}
	defer tx.Rollback()

	var upload domain.DocumentUpload
	row := tx.QueryRowContext(ctx, `
SELECT id, title, file_name, content_type, storage_path, file_size_bytes, sha256, status, analysis_json,
	error_message, document_id, document_asset_id, document_version_id, created_at, updated_at
FROM document_uploads
WHERE id = $1
FOR UPDATE`, id)
	if err := scanDocumentUpload(row, &upload); err != nil {
		return domain.IngestJob{}, false, err
	}
	if upload.DocumentVersionID == nil || strings.TrimSpace(*upload.DocumentVersionID) == "" {
		return domain.IngestJob{}, false, ErrInvalidDocumentUploadAction
	}
	documentVersionID := *upload.DocumentVersionID
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, documentVersionID); err != nil {
		return domain.IngestJob{}, false, err
	}

	var job domain.IngestJob
	created := false
	existingRow := tx.QueryRowContext(ctx, `
SELECT id, document_version_id, status, error_message, created_at, updated_at
FROM ingest_jobs
WHERE document_version_id = $1
  AND status IN ('pending', 'processing', 'queued', 'running')
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE`, documentVersionID)
	switch err := existingRow.Scan(&job.ID, &job.DocumentVersionID, &job.Status, &job.ErrorMessage, &job.CreatedAt, &job.UpdatedAt); err {
	case nil:
	case sql.ErrNoRows:
		created = true
		if err := tx.QueryRowContext(ctx, `
INSERT INTO ingest_jobs (document_version_id, status, error_message)
VALUES ($1, 'pending', NULL)
RETURNING id, document_version_id, status, error_message, created_at, updated_at`, documentVersionID).Scan(
			&job.ID,
			&job.DocumentVersionID,
			&job.Status,
			&job.ErrorMessage,
			&job.CreatedAt,
			&job.UpdatedAt,
		); err != nil {
			return domain.IngestJob{}, false, err
		}
	default:
		return domain.IngestJob{}, false, err
	}

	result, err := tx.ExecContext(ctx, `
UPDATE document_uploads
SET status = 'indexing',
    error_message = NULL,
    updated_at = NOW()
WHERE id = $1`, upload.ID)
	if err != nil {
		return domain.IngestJob{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.IngestJob{}, false, err
	}
	if affected == 0 {
		return domain.IngestJob{}, false, sql.ErrNoRows
	}
	if _, err := insertDocumentUploadEvent(ctx, tx, domain.DocumentUploadEventInput{
		UploadID:  &upload.ID,
		EventType: "review_action",
		Stage:     "embed_index",
		Status:    "active",
		Message:   "Operator requested reindex",
		EvidenceJSON: mustJSON(map[string]interface{}{
			"created":             created,
			"document_version_id": documentVersionID,
			"ingest_job_id":       job.ID,
			"pipeline_version":    "rag_upload_intake_v1",
			"reason":              reason,
			"status_reason":       "reindex_requested",
		}),
		Actor:         actor,
		FileName:      upload.FileName,
		ContentType:   upload.ContentType,
		FileSizeBytes: upload.FileSizeBytes,
		SHA256:        upload.SHA256,
	}); err != nil {
		return domain.IngestJob{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return domain.IngestJob{}, false, err
	}
	return job, created, nil
}

func (s *Store) UpdateDocumentUploadReviewStatus(ctx context.Context, id, status, action, actor, reason string) (domain.DocumentUpload, error) {
	if status != "rejected" && status != "archived" {
		return domain.DocumentUpload{}, ErrInvalidDocumentUploadAction
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.DocumentUpload{}, err
	}
	defer tx.Rollback()

	var upload domain.DocumentUpload
	var errValue sql.NullString
	if status == "rejected" && strings.TrimSpace(reason) != "" {
		errValue = sql.NullString{String: strings.TrimSpace(reason), Valid: true}
	}
	row := tx.QueryRowContext(ctx, `
UPDATE document_uploads
SET status = $2,
    error_message = $3,
    updated_at = NOW()
WHERE id = $1
  AND status NOT IN ('rejected', 'archived')
RETURNING id, title, file_name, content_type, storage_path, file_size_bytes, sha256, status, analysis_json,
	error_message, document_id, document_asset_id, document_version_id, created_at, updated_at`, id, status, errValue)
	if err := scanDocumentUpload(row, &upload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.DocumentUpload{}, ErrInvalidDocumentUploadAction
		}
		return domain.DocumentUpload{}, err
	}
	if _, err := insertDocumentUploadEvent(ctx, tx, domain.DocumentUploadEventInput{
		UploadID:  &upload.ID,
		EventType: "review_action",
		Stage:     "publish",
		Status:    "blocked",
		Message:   "Operator marked upload as " + status,
		EvidenceJSON: mustJSON(map[string]interface{}{
			"action":           action,
			"pipeline_version": "rag_upload_intake_v1",
			"reason":           reason,
			"status_reason":    status,
		}),
		Actor:         actor,
		FileName:      upload.FileName,
		ContentType:   upload.ContentType,
		FileSizeBytes: upload.FileSizeBytes,
		SHA256:        upload.SHA256,
	}); err != nil {
		return domain.DocumentUpload{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.DocumentUpload{}, err
	}
	uploads := []domain.DocumentUpload{upload}
	if err := s.attachDocumentUploadEvents(ctx, uploads); err != nil {
		return domain.DocumentUpload{}, err
	}
	return uploads[0], nil
}

func (s *Store) ResetStaleDocumentUploads(ctx context.Context, staleBefore time.Time) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `
UPDATE document_uploads
SET status = 'uploaded',
    error_message = NULL,
    updated_at = NOW()
WHERE status = 'extracting'
  AND updated_at < $1
`, staleBefore)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

type documentUploadScanner interface {
	Scan(dest ...any) error
}

func scanDocumentUpload(scanner documentUploadScanner, upload *domain.DocumentUpload) error {
	var (
		contentType       sql.NullString
		errorMessage      sql.NullString
		documentID        sql.NullString
		documentAssetID   sql.NullString
		documentVersionID sql.NullString
	)
	err := scanner.Scan(
		&upload.ID,
		&upload.Title,
		&upload.FileName,
		&contentType,
		&upload.StoragePath,
		&upload.FileSizeBytes,
		&upload.SHA256,
		&upload.Status,
		&upload.AnalysisJSON,
		&errorMessage,
		&documentID,
		&documentAssetID,
		&documentVersionID,
		&upload.CreatedAt,
		&upload.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if contentType.Valid {
		upload.ContentType = contentType.String
	}
	if errorMessage.Valid {
		upload.ErrorMessage = &errorMessage.String
	}
	if documentID.Valid {
		upload.DocumentID = &documentID.String
	}
	if documentAssetID.Valid {
		upload.DocumentAssetID = &documentAssetID.String
	}
	if documentVersionID.Valid {
		upload.DocumentVersionID = &documentVersionID.String
	}
	return nil
}

type documentUploadEventInserter interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type documentUploadEventScanner interface {
	Scan(dest ...any) error
}

func insertDocumentUploadEvent(ctx context.Context, runner documentUploadEventInserter, input domain.DocumentUploadEventInput) (domain.DocumentUploadEvent, error) {
	eventType := strings.TrimSpace(input.EventType)
	if eventType == "" {
		eventType = "stage_transition"
	}
	stage := strings.TrimSpace(input.Stage)
	status := strings.TrimSpace(input.Status)
	message := strings.TrimSpace(input.Message)
	actor := strings.TrimSpace(input.Actor)
	if actor == "" {
		actor = "system"
	}
	evidenceJSON := input.EvidenceJSON
	if len(evidenceJSON) == 0 {
		evidenceJSON = []byte(`{}`)
	}
	var uploadID any
	if input.UploadID != nil && strings.TrimSpace(*input.UploadID) != "" {
		uploadID = strings.TrimSpace(*input.UploadID)
	}

	var event domain.DocumentUploadEvent
	row := runner.QueryRowContext(ctx, `
INSERT INTO document_upload_events (
	upload_id,
	event_type,
	stage,
	status,
	message,
	evidence_json,
	actor,
	file_name,
	content_type,
	file_size_bytes,
	sha256
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
RETURNING id, upload_id, event_type, stage, status, message, evidence_json, actor, file_name, content_type,
	file_size_bytes, sha256, created_at`,
		uploadID,
		eventType,
		stage,
		status,
		message,
		evidenceJSON,
		actor,
		input.FileName,
		input.ContentType,
		input.FileSizeBytes,
		input.SHA256,
	)
	if err := scanDocumentUploadEvent(row, &event); err != nil {
		return domain.DocumentUploadEvent{}, err
	}
	return event, nil
}

func (s *Store) attachDocumentUploadEvents(ctx context.Context, uploads []domain.DocumentUpload) error {
	if len(uploads) == 0 {
		return nil
	}
	ids := make([]string, 0, len(uploads))
	indexByID := make(map[string]int, len(uploads))
	for idx, upload := range uploads {
		if upload.ID == "" {
			continue
		}
		ids = append(ids, upload.ID)
		indexByID[upload.ID] = idx
	}
	if len(ids) == 0 {
		return nil
	}

	rows, err := s.DB.QueryContext(ctx, `
SELECT id, upload_id, event_type, stage, status, message, evidence_json, actor, file_name, content_type,
	file_size_bytes, sha256, created_at
FROM document_upload_events
WHERE upload_id = ANY($1::uuid[])
ORDER BY created_at ASC`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var event domain.DocumentUploadEvent
		if err := scanDocumentUploadEvent(rows, &event); err != nil {
			return err
		}
		if event.UploadID == nil {
			continue
		}
		idx, ok := indexByID[*event.UploadID]
		if !ok {
			continue
		}
		uploads[idx].Events = append(uploads[idx].Events, event)
	}
	return rows.Err()
}

func scanDocumentUploadEvent(scanner documentUploadEventScanner, event *domain.DocumentUploadEvent) error {
	var uploadID sql.NullString
	if err := scanner.Scan(
		&event.ID,
		&uploadID,
		&event.EventType,
		&event.Stage,
		&event.Status,
		&event.Message,
		&event.EvidenceJSON,
		&event.Actor,
		&event.FileName,
		&event.ContentType,
		&event.FileSizeBytes,
		&event.SHA256,
		&event.CreatedAt,
	); err != nil {
		return err
	}
	if uploadID.Valid {
		event.UploadID = &uploadID.String
	}
	return nil
}

func documentUploadStatusEventInput(upload domain.DocumentUpload, status string, analysisJSON []byte, errorMessage *string) (domain.DocumentUploadEventInput, bool) {
	stage, eventStatus, eventType := documentUploadStatusEventShape(status)
	if stage == "" {
		return domain.DocumentUploadEventInput{}, false
	}
	message := defaultDocumentUploadStatusEventMessage(status)
	if errorMessage != nil && strings.TrimSpace(*errorMessage) != "" {
		message = strings.TrimSpace(*errorMessage)
	}
	return domain.DocumentUploadEventInput{
		UploadID:      &upload.ID,
		EventType:     eventType,
		Stage:         stage,
		Status:        eventStatus,
		Message:       message,
		EvidenceJSON:  analysisJSON,
		Actor:         "worker",
		FileName:      upload.FileName,
		ContentType:   upload.ContentType,
		FileSizeBytes: upload.FileSizeBytes,
		SHA256:        upload.SHA256,
	}, true
}

func documentUploadStatusEventShape(status string) (stage, eventStatus, eventType string) {
	switch status {
	case "uploaded":
		return "upload", "done", "stage_transition"
	case "extracting":
		return "extract", "active", "stage_transition"
	case "extract_failed":
		return "extract", "blocked", "stage_error"
	case "classified":
		return "classify", "done", "stage_transition"
	case "classification_low_confidence":
		return "classify", "warning", "stage_review"
	case "profile_resolved":
		return "profile", "done", "stage_transition"
	case "indexing":
		return "embed_index", "active", "stage_transition"
	case "validating":
		return "validate", "active", "stage_transition"
	case "validation_failed":
		return "validate", "blocked", "stage_error"
	case "ready", "published":
		return "publish", "done", "stage_transition"
	case "needs_review":
		return "publish", "warning", "stage_review"
	case "rejected":
		return "publish", "blocked", "review_action"
	case "archived":
		return "publish", "blocked", "review_action"
	default:
		return "", "", ""
	}
}

func defaultDocumentUploadStatusEventMessage(status string) string {
	switch status {
	case "uploaded":
		return "Upload accepted into intake queue"
	case "extracting":
		return "Extracting text from uploaded file"
	case "extract_failed":
		return "Text extraction failed"
	case "classified":
		return "Document type classified"
	case "classification_low_confidence":
		return "Classification confidence is below auto-publish threshold"
	case "profile_resolved":
		return "Ingestion profile resolved"
	case "indexing":
		return "Ingest job queued for chunking and indexing"
	case "validating":
		return "Validating indexed document"
	case "validation_failed":
		return "Ingest validation failed"
	case "ready", "published":
		return "Document published to retrieval index"
	case "needs_review":
		return "Upload requires operator review"
	case "rejected":
		return "Upload rejected by operator"
	case "archived":
		return "Upload archived by operator"
	default:
		return "Upload status changed"
	}
}

func mustJSON(value map[string]interface{}) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		return []byte(`{}`)
	}
	return raw
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
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.IngestJob{}, false, err
	}
	defer tx.Rollback()

	var job domain.IngestJob
	err = tx.QueryRowContext(ctx, `
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
	var upload domain.DocumentUpload
	row := tx.QueryRowContext(ctx, `
SELECT id, title, file_name, content_type, storage_path, file_size_bytes, sha256, status, analysis_json,
	error_message, document_id, document_asset_id, document_version_id, created_at, updated_at
FROM document_uploads
WHERE document_version_id = $1
LIMIT 1`, job.DocumentVersionID)
	if err := scanDocumentUpload(row, &upload); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.IngestJob{}, false, err
	}
	if upload.ID != "" {
		evidence := mustJSON(map[string]interface{}{
			"ingest_job_id":       job.ID,
			"document_version_id": job.DocumentVersionID,
			"pipeline_version":    "rag_upload_intake_v1",
			"status_reason":       "ingest_claimed",
		})
		for _, eventInput := range []domain.DocumentUploadEventInput{
			{
				UploadID:      &upload.ID,
				EventType:     "stage_transition",
				Stage:         "chunk",
				Status:        "active",
				Message:       "Ingest worker started chunk generation",
				EvidenceJSON:  evidence,
				Actor:         "worker",
				FileName:      upload.FileName,
				ContentType:   upload.ContentType,
				FileSizeBytes: upload.FileSizeBytes,
				SHA256:        upload.SHA256,
			},
			{
				UploadID:      &upload.ID,
				EventType:     "stage_transition",
				Stage:         "embed_index",
				Status:        "active",
				Message:       "Ingest worker started embedding and indexing",
				EvidenceJSON:  evidence,
				Actor:         "worker",
				FileName:      upload.FileName,
				ContentType:   upload.ContentType,
				FileSizeBytes: upload.FileSizeBytes,
				SHA256:        upload.SHA256,
			},
		} {
			if _, err := insertDocumentUploadEvent(ctx, tx, eventInput); err != nil {
				return domain.IngestJob{}, false, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.IngestJob{}, false, err
	}
	return job, true, nil
}

func (s *Store) TouchJob(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE ingest_jobs SET updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *Store) MarkJobCompleted(ctx context.Context, id string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var documentVersionID string
	if err := tx.QueryRowContext(ctx, `
UPDATE ingest_jobs
SET status = 'completed',
    error_message = NULL,
    updated_at = NOW()
WHERE id = $1
RETURNING document_version_id`, id).Scan(&documentVersionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	var upload domain.DocumentUpload
	row := tx.QueryRowContext(ctx, `
UPDATE document_uploads
SET status = 'ready',
    error_message = NULL,
    updated_at = NOW()
WHERE document_version_id = $1
  AND status IN ('indexing', 'validating', 'validation_failed')
RETURNING id, title, file_name, content_type, storage_path, file_size_bytes, sha256, status, analysis_json,
	error_message, document_id, document_asset_id, document_version_id, created_at, updated_at`, documentVersionID)
	if err := scanDocumentUpload(row, &upload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return tx.Commit()
		}
		return err
	}
	evidence := mustJSON(map[string]interface{}{
		"ingest_job_id":       id,
		"document_version_id": documentVersionID,
		"pipeline_version":    "rag_upload_intake_v1",
		"status_reason":       "ingest_completed",
	})
	for _, eventInput := range []domain.DocumentUploadEventInput{
		{
			UploadID:      &upload.ID,
			EventType:     "stage_transition",
			Stage:         "chunk",
			Status:        "done",
			Message:       "Chunk rows and vector payloads were generated",
			EvidenceJSON:  evidence,
			Actor:         "worker",
			FileName:      upload.FileName,
			ContentType:   upload.ContentType,
			FileSizeBytes: upload.FileSizeBytes,
			SHA256:        upload.SHA256,
		},
		{
			UploadID:      &upload.ID,
			EventType:     "stage_transition",
			Stage:         "embed_index",
			Status:        "done",
			Message:       "Embeddings indexed successfully",
			EvidenceJSON:  evidence,
			Actor:         "worker",
			FileName:      upload.FileName,
			ContentType:   upload.ContentType,
			FileSizeBytes: upload.FileSizeBytes,
			SHA256:        upload.SHA256,
		},
		{
			UploadID:      &upload.ID,
			EventType:     "stage_transition",
			Stage:         "validate",
			Status:        "done",
			Message:       "Ingest validation completed",
			EvidenceJSON:  evidence,
			Actor:         "worker",
			FileName:      upload.FileName,
			ContentType:   upload.ContentType,
			FileSizeBytes: upload.FileSizeBytes,
			SHA256:        upload.SHA256,
		},
		{
			UploadID:      &upload.ID,
			EventType:     "stage_transition",
			Stage:         "publish",
			Status:        "done",
			Message:       "Document published to retrieval index",
			EvidenceJSON:  evidence,
			Actor:         "worker",
			FileName:      upload.FileName,
			ContentType:   upload.ContentType,
			FileSizeBytes: upload.FileSizeBytes,
			SHA256:        upload.SHA256,
		},
	} {
		if _, err := insertDocumentUploadEvent(ctx, tx, eventInput); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) MarkJobFailed(ctx context.Context, id string, attempt int, message string) error {
	meta, err := encodeJobErrorMeta(jobErrorMeta{Attempt: attempt, Message: message})
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var documentVersionID string
	if err := tx.QueryRowContext(ctx, `
UPDATE ingest_jobs
SET status = 'failed',
    error_message = $1,
    updated_at = NOW()
WHERE id = $2
RETURNING document_version_id`, meta, id).Scan(&documentVersionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	var upload domain.DocumentUpload
	row := tx.QueryRowContext(ctx, `
UPDATE document_uploads
SET status = 'validation_failed',
    error_message = $1,
    updated_at = NOW()
WHERE document_version_id = $2
RETURNING id, title, file_name, content_type, storage_path, file_size_bytes, sha256, status, analysis_json,
	error_message, document_id, document_asset_id, document_version_id, created_at, updated_at`, meta, documentVersionID)
	if err := scanDocumentUpload(row, &upload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return tx.Commit()
		}
		return err
	}
	evidence := mustJSON(map[string]interface{}{
		"attempt":             attempt,
		"ingest_job_id":       id,
		"document_version_id": documentVersionID,
		"pipeline_version":    "rag_upload_intake_v1",
		"status_reason":       "ingest_failed",
	})
	if _, err := insertDocumentUploadEvent(ctx, tx, domain.DocumentUploadEventInput{
		UploadID:      &upload.ID,
		EventType:     "stage_error",
		Stage:         "validate",
		Status:        "blocked",
		Message:       message,
		EvidenceJSON:  evidence,
		Actor:         "worker",
		FileName:      upload.FileName,
		ContentType:   upload.ContentType,
		FileSizeBytes: upload.FileSizeBytes,
		SHA256:        upload.SHA256,
	}); err != nil {
		return err
	}
	return tx.Commit()
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
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var documentVersionID string
	if err := tx.QueryRowContext(ctx, `
UPDATE ingest_jobs
SET status = 'pending',
    updated_at = NOW()
WHERE id = $1
  AND status = 'failed'
RETURNING document_version_id`, id).Scan(&documentVersionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	var upload domain.DocumentUpload
	row := tx.QueryRowContext(ctx, `
UPDATE document_uploads
SET status = 'indexing',
    error_message = NULL,
    updated_at = NOW()
WHERE document_version_id = $1
RETURNING id, title, file_name, content_type, storage_path, file_size_bytes, sha256, status, analysis_json,
	error_message, document_id, document_asset_id, document_version_id, created_at, updated_at`, documentVersionID)
	if err := scanDocumentUpload(row, &upload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return tx.Commit()
		}
		return err
	}
	if _, err := insertDocumentUploadEvent(ctx, tx, domain.DocumentUploadEventInput{
		UploadID:  &upload.ID,
		EventType: "stage_transition",
		Stage:     "embed_index",
		Status:    "active",
		Message:   "Failed ingest job was requeued",
		EvidenceJSON: mustJSON(map[string]interface{}{
			"ingest_job_id":       id,
			"document_version_id": documentVersionID,
			"pipeline_version":    "rag_upload_intake_v1",
			"status_reason":       "job_requeued",
		}),
		Actor:         "worker",
		FileName:      upload.FileName,
		ContentType:   upload.ContentType,
		FileSizeBytes: upload.FileSizeBytes,
		SHA256:        upload.SHA256,
	}); err != nil {
		return err
	}
	return tx.Commit()
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

func (s *Store) CreateConversation(ctx context.Context, title string, userID *string) (domain.Conversation, error) {
	var convo domain.Conversation
	err := s.DB.QueryRowContext(ctx, `
INSERT INTO conversations (title, user_id)
VALUES ($1, $2)
RETURNING id, title, user_id, created_at, updated_at
`, title, userID).Scan(&convo.ID, &convo.Title, &convo.UserID, &convo.CreatedAt, &convo.UpdatedAt)
	return convo, err
}

func (s *Store) ListConversations(ctx context.Context, userID *string) ([]domain.Conversation, error) {
	var (
		rows *sql.Rows
		err  error
	)
	baseQuery := `
SELECT
	c.id,
	c.title,
	c.user_id,
	lm.content AS last_message,
	lm.created_at AS last_message_at,
	COALESCE(mc.message_count, 0) AS message_count,
	c.created_at,
	c.updated_at
FROM conversations c
LEFT JOIN LATERAL (
	SELECT m.content, m.created_at
	FROM messages m
	WHERE m.conversation_id = c.id
	ORDER BY m.created_at DESC
	LIMIT 1
) lm ON TRUE
LEFT JOIN LATERAL (
	SELECT COUNT(1) AS message_count
	FROM messages m
	WHERE m.conversation_id = c.id
) mc ON TRUE
`
	if userID != nil && strings.TrimSpace(*userID) != "" {
		rows, err = s.DB.QueryContext(ctx, baseQuery+`WHERE c.user_id = $1 ORDER BY c.updated_at DESC, c.created_at DESC`, *userID)
	} else {
		rows, err = s.DB.QueryContext(ctx, baseQuery+`ORDER BY c.updated_at DESC, c.created_at DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Conversation, 0)
	for rows.Next() {
		var convo domain.Conversation
		if err := rows.Scan(
			&convo.ID,
			&convo.Title,
			&convo.UserID,
			&convo.LastMessage,
			&convo.LastMessageAt,
			&convo.MessageCount,
			&convo.CreatedAt,
			&convo.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, convo)
	}
	return out, rows.Err()
}

func (s *Store) GetConversation(ctx context.Context, id string) (domain.Conversation, error) {
	var convo domain.Conversation
	err := s.DB.QueryRowContext(ctx, `
SELECT
	c.id,
	c.title,
	c.user_id,
	lm.content AS last_message,
	lm.created_at AS last_message_at,
	COALESCE(mc.message_count, 0) AS message_count,
	c.created_at,
	c.updated_at
FROM conversations c
LEFT JOIN LATERAL (
	SELECT m.content, m.created_at
	FROM messages m
	WHERE m.conversation_id = c.id
	ORDER BY m.created_at DESC
	LIMIT 1
) lm ON TRUE
LEFT JOIN LATERAL (
	SELECT COUNT(1) AS message_count
	FROM messages m
	WHERE m.conversation_id = c.id
) mc ON TRUE
WHERE c.id = $1
`, id).Scan(
		&convo.ID,
		&convo.Title,
		&convo.UserID,
		&convo.LastMessage,
		&convo.LastMessageAt,
		&convo.MessageCount,
		&convo.CreatedAt,
		&convo.UpdatedAt,
	)
	return convo, err
}

func (s *Store) DeleteConversation(ctx context.Context, id string) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM conversations WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) UpdateConversationTitle(ctx context.Context, id, title string) error {
	res, err := s.DB.ExecContext(ctx, `
UPDATE conversations
SET title = $2,
    updated_at = NOW()
WHERE id = $1
`, id, title)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) CreateMessage(ctx context.Context, conversationID, role, content string, citationsJSON []byte, traceID *string) (domain.Message, error) {
	var msg domain.Message
	if len(citationsJSON) == 0 {
		citationsJSON = []byte("[]")
	}
	err := s.DB.QueryRowContext(ctx, `
WITH inserted AS (
	INSERT INTO messages (conversation_id, role, content, citations_json, trace_id)
	VALUES ($1, $2, $3, $4::jsonb, $5)
	RETURNING id, conversation_id, role, content, citations_json, trace_id, created_at
)
UPDATE conversations c
SET updated_at = NOW()
FROM inserted
WHERE c.id = inserted.conversation_id
RETURNING inserted.id, inserted.conversation_id, inserted.role, inserted.content, inserted.citations_json, inserted.trace_id, inserted.created_at
`, conversationID, role, content, citationsJSON, traceID).Scan(
		&msg.ID,
		&msg.ConversationID,
		&msg.Role,
		&msg.Content,
		&msg.CitationsJSON,
		&msg.TraceID,
		&msg.CreatedAt,
	)
	return msg, err
}

func (s *Store) UpdateMessage(ctx context.Context, id, content string, citationsJSON []byte, traceID *string) error {
	if len(citationsJSON) == 0 {
		citationsJSON = []byte("[]")
	}
	res, err := s.DB.ExecContext(ctx, `
UPDATE messages
SET content = $2,
    citations_json = $3::jsonb,
    trace_id = $4
WHERE id = $1
`, id, content, citationsJSON, traceID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ListMessagesByConversation(ctx context.Context, conversationID string) ([]domain.Message, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT id, conversation_id, role, content, citations_json, trace_id, created_at
FROM messages
WHERE conversation_id = $1
ORDER BY created_at ASC, id ASC
`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Message, 0)
	for rows.Next() {
		var msg domain.Message
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &msg.CitationsJSON, &msg.TraceID, &msg.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, rows.Err()
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

func (s *Store) ListEnabledAIPrompts(ctx context.Context) ([]domain.AIPrompt, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT id, name, prompt_type, system_prompt, temperature, max_tokens, retry, enabled, created_at, updated_at
FROM ai_prompts
WHERE enabled = TRUE
ORDER BY prompt_type ASC, updated_at DESC, created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AIPrompt, 0)
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
	form := schema.DocTypeForm{
		Version:       1,
		DocType:       schema.DocType{Code: "legal_normative", Name: "Legal Normative"},
		SegmentRules:  schema.SegmentRules{Strategy: "legal_article", Hierarchy: "article", Normalization: "basic"},
		Metadata:      schema.MetadataSchema{Fields: []schema.MetadataField{{Name: "title", Type: "string"}, {Name: "date", Type: "date"}}},
		MappingRules:  []schema.MappingRule{{Field: "title", Regex: "^Title\\s*:\\s*(.+)$", Group: 1}, {Field: "date", Regex: "^Date\\s*:\\s*(.+)$", Group: 1}},
		ReindexPolicy: schema.ReindexPolicy{OnContentChange: true, OnFormChange: true},
	}
	b, err := json.Marshal(form)
	if err != nil {
		return err
	}
	hash := "seed"
	_, err = s.DB.ExecContext(ctx, `INSERT INTO doc_types (code, name, form_json, form_hash) VALUES ('legal_normative','Legal Normative',$1,$2)`, b, hash)
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
SELECT
	$1,
	CASE
		WHEN EXISTS (SELECT 1 FROM ai_guard_policies WHERE enabled = TRUE) THEN FALSE
		ELSE TRUE
	END,
	$2, $3, $4, $5
ON CONFLICT (name) DO NOTHING
	`,
		"default_legal_guard_policy",
		1,
		0.7,
		"refuse",
		"ask_clarification",
	)
	if err != nil {
		return err
	}

	defaultPrompts := []struct {
		name         string
		promptType   string
		systemPrompt string
		temperature  float64
		maxTokens    int
		retry        int
	}{
		{
			name:         "legal_guard_prompt",
			promptType:   "legal_guard",
			systemPrompt: "You are a Vietnamese legal assistant.\nUse ONLY the provided legal sources.\nNever invent legal provisions, article numbers, or law names.\nIf the sources do not provide sufficient legal basis, clearly say:\n\"Không tìm thấy căn cứ pháp lý rõ ràng trong các nguồn hiện có.\"\nAlways cite legal provisions in human-readable format.\nDo not provide speculative legal advice.",
			temperature:  0.2,
			maxTokens:    1200,
			retry:        2,
		},
		{
			name:         "legal_answer_prompt",
			promptType:   "legal_answer",
			systemPrompt: "You are a Vietnamese legal assistant.\nUse ONLY the provided legal sources.\nNever invent legal provisions, article numbers, or law names.\nIf the retrieved sources are insufficient, state that clearly instead of guessing.\nAlways answer in Vietnamese with this exact structure:\n1. Legal Issue\n2. Applicable Law\n3. Legal Analysis\n4. Conclusion",
			temperature:  0.2,
			maxTokens:    1200,
			retry:        2,
		},
		{
			name:         "legal_refusal_prompt",
			promptType:   "legal_refusal",
			systemPrompt: "Không đủ căn cứ pháp lý trong dữ liệu truy xuất để đưa ra kết luận.",
			temperature:  0,
			maxTokens:    128,
			retry:        0,
		},
		{
			name:         "legal_clarification_prompt",
			promptType:   "legal_clarification",
			systemPrompt: "Chưa đủ căn cứ pháp lý rõ ràng. Vui lòng bổ sung tình huống, văn bản, hoặc điều khoản cần tra cứu.",
			temperature:  0,
			maxTokens:    128,
			retry:        0,
		},
	}
	for _, prompt := range defaultPrompts {
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
SELECT
	$1, $2, $3, $4, $5, $6,
	CASE
		WHEN EXISTS (SELECT 1 FROM ai_prompts WHERE prompt_type = $2 AND enabled = TRUE) THEN FALSE
		ELSE TRUE
	END
ON CONFLICT (name) DO NOTHING
	`,
			prompt.name,
			prompt.promptType,
			prompt.systemPrompt,
			prompt.temperature,
			prompt.maxTokens,
			prompt.retry,
		)
		if err != nil {
			return err
		}
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
SELECT
	$1,
	CASE
		WHEN EXISTS (SELECT 1 FROM ai_retrieval_configs WHERE enabled = TRUE) THEN FALSE
		ELSE TRUE
	END,
	$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14
ON CONFLICT (name) DO NOTHING
	`,
		"default_legal_retrieval_config",
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
