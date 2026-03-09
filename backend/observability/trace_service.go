package observability

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

type Recorder interface {
	OnRetrieval(normalizedQuery string, filters map[string]interface{}, chunkIDs []string)
	OnPromptSnapshot(snapshot string)
	OnLLMCall(modelName string, temperature float64, maxTokens, retry int)
	OnResponse(responseText string, streamCompleted bool, latency time.Duration)
	OnError(err error, latency time.Duration)
}

type TraceService struct {
	repo    TraceRepository
	logger  *slog.Logger
	traceID string
	mode    string
	started time.Time

	mu     sync.Mutex
	record TraceRecord
}

func NewTraceService(ctx context.Context, repo TraceRepository, logger *slog.Logger, traceID, mode, userQuery string) (*TraceService, error) {
	svc := &TraceService{
		repo:    repo,
		logger:  logger,
		traceID: traceID,
		mode:    mode,
		started: time.Now(),
		record: TraceRecord{
			TraceID:   traceID,
			Mode:      mode,
			UserQuery: userQuery,
		},
	}
	if repo != nil {
		if _, err := repo.Create(ctx, svc.record); err != nil {
			return nil, err
		}
	}
	LogInfo(ctx, logger, "trace", "answer trace created", map[string]interface{}{
		"mode":       mode,
		"user_query": userQuery,
	})
	return svc, nil
}

func (s *TraceService) OnRetrieval(normalizedQuery string, filters map[string]interface{}, chunkIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record.NormalizedQuery = normalizedQuery
	s.record.RetrievalFiltersJSON = mustJSON(filters)
	s.record.RetrievedChunkIDsJSON = mustJSON(chunkIDs)
	s.persistLocked(context.Background(), "retrieval", "retrieval trace updated", map[string]interface{}{
		"normalized_query": normalizedQuery,
		"chunk_count":      len(chunkIDs),
	})
}

func (s *TraceService) OnPromptSnapshot(snapshot string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record.PromptSnapshot = snapshot
	s.persistLocked(context.Background(), "prompt", "prompt snapshot stored", map[string]interface{}{
		"snapshot_length": len(snapshot),
	})
}

func (s *TraceService) OnLLMCall(modelName string, temperature float64, maxTokens, retry int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record.ModelName = modelName
	s.record.Temperature = temperature
	s.record.MaxTokens = maxTokens
	s.record.Retry = retry
	s.persistLocked(context.Background(), "llm", "llm call metadata stored", map[string]interface{}{
		"model_name":  modelName,
		"temperature": temperature,
		"max_tokens":  maxTokens,
		"retry":       retry,
	})
}

func (s *TraceService) OnResponse(responseText string, streamCompleted bool, latency time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record.ResponseText = responseText
	s.record.StreamCompleted = streamCompleted
	s.record.LatencyMS = int(latency.Milliseconds())
	s.persistLocked(context.Background(), "response", "answer trace completed", map[string]interface{}{
		"stream_completed": streamCompleted,
		"latency_ms":       s.record.LatencyMS,
		"response_length":  len(responseText),
	})
}

func (s *TraceService) OnError(err error, latency time.Duration) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record.ErrorMessage = err.Error()
	if latency > 0 {
		s.record.LatencyMS = int(latency.Milliseconds())
	}
	s.persistLocked(context.Background(), "trace", "answer trace failed", map[string]interface{}{
		"error":      err.Error(),
		"latency_ms": s.record.LatencyMS,
	})
}

func (s *TraceService) persistLocked(ctx context.Context, component, message string, metadata map[string]interface{}) {
	if s.repo != nil {
		_ = s.repo.Update(ctx, s.traceID, s.record)
	}
	ctx = WithTraceID(ctx, s.traceID)
	LogInfo(ctx, s.logger, component, message, metadata)
}

func mustJSON(v interface{}) []byte {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
