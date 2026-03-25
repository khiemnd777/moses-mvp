package api

import (
	"bufio"
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/core/retrieval"
	"github.com/khiemnd777/legal_api/observability"
)

type streamState struct {
	writer     *sseWriter
	messageID  string
	model      string
	tokenSent  bool
	lastErr    error
	sendErrors bool
	builder    strings.Builder
}

func (s *streamState) OnToken(delta string) error {
	s.tokenSent = true
	s.builder.WriteString(delta)
	return s.writer.writeEvent("token", fiber.Map{"delta": delta})
}

func (s *streamState) OnCitations(citations []answer.Citation) error {
	return s.writer.writeEvent("citations", citations)
}

func (s *streamState) OnError(err error) {
	s.lastErr = err
	if !s.sendErrors {
		return
	}
	_ = s.writer.writeEvent("error", fiber.Map{"code": "LLM_ERROR", "message": "Upstream error"})
}

func (s *streamState) OnDone() {
	_ = s.writer.writeEvent("done", fiber.Map{"ok": true})
}

func (h *Handler) AnswerStream(c *fiber.Ctx) error {
	started := time.Now()
	var req answerRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid_request", "invalid json", err.Error())
	}
	question, filters := normalizeAnswerRequest(req, h.Tones)
	if question == "" {
		return respondError(c, 400, "validation", "question is required", nil)
	}
	ctx := c.UserContext()
	ctx, traceSvc, traceID, err := h.startAnswerTrace(ctx, "stream", question)
	if err != nil {
		return respondError(c, 500, "trace_error", "failed to create answer trace", err.Error())
	}
	_ = h.Store.LogQuery(ctx, question)
	runtimeCfg, err := h.loadRuntimeAnswerConfig(ctx, filters.Tone)
	if err != nil {
		traceSvc.OnError(err, traceLatency(started))
		return respondError(c, 500, "config_error", "failed to load answer runtime config", err.Error())
	}
	results, err := h.Retriever.Search(ctx, question, retrieval.SearchOptions{
		TopK:            filters.TopK,
		Domain:          filters.Domain,
		DocType:         filters.DocType,
		EffectiveStatus: filters.EffectiveStatus,
		DocumentNumber:  filters.DocumentNumber,
		ArticleNumber:   filters.ArticleNumber,
	})
	if err != nil {
		traceSvc.OnError(err, traceLatency(started))
		return respondError(c, 500, "search_error", "failed to search", err.Error())
	}
	decision, diag, err := h.evaluateGuardPolicy(ctx, runtimeCfg.Policy, results)
	if err != nil {
		traceSvc.OnError(err, traceLatency(started))
		return respondError(c, 500, "config_error", "failed to evaluate guard decision", err.Error())
	}
	promptTypeUsed := decision.PromptType
	if decision.Allow() {
		if _, usedPromptType, routeErr := h.getAnswerPrompt(ctx); routeErr == nil {
			promptTypeUsed = usedPromptType
		}
	}
	traceSvc.OnRetrieval(retrieval.UnderstandQuery(question).NormalizedQuery, map[string]interface{}{
		"legal_domain":     filters.Domain,
		"document_type":    filters.DocType,
		"effective_status": filters.EffectiveStatus,
		"document_number":  filters.DocumentNumber,
		"article_number":   filters.ArticleNumber,
		"retrieved_chunks": diag.RetrievedChunks,
		"max_similarity":   diag.MaxSimilarity,
		"guard_decision":   string(decision.Decision),
		"prompt_type_used": promptTypeUsed,
		"retrieval": fiber.Map{
			"chunks":         diag.RetrievedChunks,
			"max_similarity": diag.MaxSimilarity,
		},
		"guard": fiber.Map{
			"decision": string(decision.Decision),
		},
		"prompt": fiber.Map{
			"type": promptTypeUsed,
		},
	}, traceChunkIDs(results))
	if !decision.Allow() {
		traceSvc.OnResponse(decision.Message, true, traceLatency(started))
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("X-Accel-Buffering", "no")
		c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
			writer := newSSEWriter(w)
			_ = writer.writeEvent("meta", fiber.Map{"trace_id": traceID})
			_ = writer.writeEvent("token", fiber.Map{"delta": decision.Message})
			_ = writer.writeEvent("citations", []answer.Citation{})
			_ = writer.writeEvent("done", fiber.Map{"ok": true})
		})
		return nil
	}
	promptCfg, _, err := h.getAnswerPrompt(ctx)
	if err != nil {
		traceSvc.OnError(err, traceLatency(started))
		return respondError(c, 500, "config_error", "failed to route legal answer prompt", err.Error())
	}
	sources := buildAnswerSources(results)
	ansSvc := &answer.Service{
		Client:       h.AnswerCli,
		SystemPrompt: promptCfg.SystemPrompt,
		Tone:         runtimeCfg.Tone,
		Temperature:  promptCfg.Temperature,
		MaxTokens:    promptCfg.MaxTokens,
		Retry:        promptCfg.Retry,
	}
	traceSvc.OnPromptSnapshot(ansSvc.PromptSnapshot(question, sources))
	traceSvc.OnLLMCall(h.AnswerCli.Model, promptCfg.Temperature, promptCfg.MaxTokens, promptCfg.Retry)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	messageID := uuid.NewString()
	model := h.AnswerCli.Model
	createdAt := time.Now().UTC().Format(time.RFC3339)

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		streamCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		reqCtx := c.Context()
		if reqCtx != nil && reqCtx.Done() != nil {
			go func() {
				<-reqCtx.Done()
				cancel()
			}()
		}

		writer := newSSEWriter(w)
		state := &streamState{writer: writer, messageID: messageID, model: model}

		_ = writer.writeEvent("meta", fiber.Map{
			"message_id": messageID,
			"model":      model,
			"created_at": createdAt,
			"trace_id":   traceID,
		})

		observability.LogInfo(streamCtx, h.Logger, "stream", "stream start", map[string]interface{}{
			"message_id": messageID,
			"model":      model,
		})

		finalAnswer, err := ansSvc.Generate(streamCtx, question, sources)
		if err != nil {
			traceSvc.OnError(err, traceLatency(started))
			observability.LogError(streamCtx, h.Logger, "stream", "stream error", map[string]interface{}{
				"message_id": messageID,
				"error":      err.Error(),
			})
			return
		}
		finalAnswer, finalCitations, _, validationErr := h.validateGeneratedLegalAnswer(streamCtx, finalAnswer, sources)
		if validationErr != nil {
			traceSvc.OnError(validationErr, traceLatency(started))
			observability.LogError(streamCtx, h.Logger, "stream", "stream validation error", map[string]interface{}{
				"message_id": messageID,
				"error":      validationErr.Error(),
			})
			return
		}

		_ = state.OnToken(finalAnswer)
		_ = state.OnCitations(finalCitations)
		state.OnDone()
		traceSvc.OnResponse(finalAnswer, true, traceLatency(started))
		observability.LogInfo(streamCtx, h.Logger, "stream", "stream end", map[string]interface{}{
			"message_id": messageID,
		})
	})

	return nil
}
