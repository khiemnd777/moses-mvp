package observability

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type AnswerTrace struct {
	ID                    string          `json:"id"`
	TraceID               string          `json:"trace_id"`
	Mode                  string          `json:"mode"`
	UserQuery             string          `json:"user_query"`
	NormalizedQuery       string          `json:"normalized_query"`
	RetrievalFiltersJSON  json.RawMessage `json:"retrieval_filters_json"`
	RetrievedChunkIDsJSON json.RawMessage `json:"retrieved_chunk_ids_json"`
	PromptSnapshot        string          `json:"prompt_snapshot"`
	ModelName             string          `json:"model_name"`
	Temperature           float64         `json:"temperature"`
	MaxTokens             int             `json:"max_tokens"`
	Retry                 int             `json:"retry"`
	ResponseText          string          `json:"response_text"`
	StreamCompleted       bool            `json:"stream_completed"`
	LatencyMS             int             `json:"latency_ms"`
	ErrorMessage          string          `json:"error_message"`
	CreatedAt             time.Time       `json:"created_at"`
}

type TraceRecord struct {
	TraceID               string
	Mode                  string
	UserQuery             string
	NormalizedQuery       string
	RetrievalFiltersJSON  []byte
	RetrievedChunkIDsJSON []byte
	PromptSnapshot        string
	ModelName             string
	Temperature           float64
	MaxTokens             int
	Retry                 int
	ResponseText          string
	StreamCompleted       bool
	LatencyMS             int
	ErrorMessage          string
}

type TraceRepository interface {
	Create(ctx context.Context, record TraceRecord) (string, error)
	Update(ctx context.Context, traceID string, record TraceRecord) error
	List(ctx context.Context, limit int) ([]AnswerTrace, error)
	GetByTraceID(ctx context.Context, traceID string) (AnswerTrace, error)
}

type SQLTraceRepository struct {
	db *sql.DB
}

func NewSQLTraceRepository(db *sql.DB) *SQLTraceRepository {
	return &SQLTraceRepository{db: db}
}

func (r *SQLTraceRepository) Create(ctx context.Context, record TraceRecord) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
INSERT INTO ai_answer_traces (
	trace_id, mode, user_query, normalized_query, retrieval_filters_json,
	retrieved_chunk_ids_json, prompt_snapshot, model_name, temperature,
	max_tokens, retry, response_text, stream_completed, latency_ms, error_message
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
RETURNING id
`,
		record.TraceID,
		record.Mode,
		record.UserQuery,
		nullString(record.NormalizedQuery),
		nullJSON(record.RetrievalFiltersJSON),
		nullJSON(record.RetrievedChunkIDsJSON),
		nullString(record.PromptSnapshot),
		nullString(record.ModelName),
		record.Temperature,
		record.MaxTokens,
		record.Retry,
		nullString(record.ResponseText),
		record.StreamCompleted,
		record.LatencyMS,
		nullString(record.ErrorMessage),
	).Scan(&id)
	return id, err
}

func (r *SQLTraceRepository) Update(ctx context.Context, traceID string, record TraceRecord) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE ai_answer_traces
SET mode = $2,
	user_query = $3,
	normalized_query = $4,
	retrieval_filters_json = $5,
	retrieved_chunk_ids_json = $6,
	prompt_snapshot = $7,
	model_name = $8,
	temperature = $9,
	max_tokens = $10,
	retry = $11,
	response_text = $12,
	stream_completed = $13,
	latency_ms = $14,
	error_message = $15
WHERE trace_id = $1
`,
		traceID,
		record.Mode,
		record.UserQuery,
		nullString(record.NormalizedQuery),
		nullJSON(record.RetrievalFiltersJSON),
		nullJSON(record.RetrievedChunkIDsJSON),
		nullString(record.PromptSnapshot),
		nullString(record.ModelName),
		record.Temperature,
		record.MaxTokens,
		record.Retry,
		nullString(record.ResponseText),
		record.StreamCompleted,
		record.LatencyMS,
		nullString(record.ErrorMessage),
	)
	return err
}

func (r *SQLTraceRepository) List(ctx context.Context, limit int) ([]AnswerTrace, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, trace_id, mode, user_query, normalized_query, retrieval_filters_json,
       retrieved_chunk_ids_json, prompt_snapshot, model_name, temperature,
       max_tokens, retry, response_text, stream_completed, latency_ms,
       error_message, created_at
FROM ai_answer_traces
ORDER BY created_at DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AnswerTrace
	for rows.Next() {
		var item AnswerTrace
		if err := rows.Scan(
			&item.ID,
			&item.TraceID,
			&item.Mode,
			&item.UserQuery,
			&item.NormalizedQuery,
			&item.RetrievalFiltersJSON,
			&item.RetrievedChunkIDsJSON,
			&item.PromptSnapshot,
			&item.ModelName,
			&item.Temperature,
			&item.MaxTokens,
			&item.Retry,
			&item.ResponseText,
			&item.StreamCompleted,
			&item.LatencyMS,
			&item.ErrorMessage,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *SQLTraceRepository) GetByTraceID(ctx context.Context, traceID string) (AnswerTrace, error) {
	var item AnswerTrace
	err := r.db.QueryRowContext(ctx, `
SELECT id, trace_id, mode, user_query, normalized_query, retrieval_filters_json,
       retrieved_chunk_ids_json, prompt_snapshot, model_name, temperature,
       max_tokens, retry, response_text, stream_completed, latency_ms,
       error_message, created_at
FROM ai_answer_traces
WHERE trace_id = $1
`, traceID).Scan(
		&item.ID,
		&item.TraceID,
		&item.Mode,
		&item.UserQuery,
		&item.NormalizedQuery,
		&item.RetrievalFiltersJSON,
		&item.RetrievedChunkIDsJSON,
		&item.PromptSnapshot,
		&item.ModelName,
		&item.Temperature,
		&item.MaxTokens,
		&item.Retry,
		&item.ResponseText,
		&item.StreamCompleted,
		&item.LatencyMS,
		&item.ErrorMessage,
		&item.CreatedAt,
	)
	return item, err
}

func nullString(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

func nullJSON(v []byte) interface{} {
	if len(v) == 0 {
		return nil
	}
	return v
}
