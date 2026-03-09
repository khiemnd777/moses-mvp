package api

import (
	"bufio"
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/khiemnd777/legal_api/core/answer"
)

type streamState struct {
	writer     *sseWriter
	messageID  string
	model      string
	tokenSent  bool
	lastErr    error
	sendErrors bool
}

func (s *streamState) OnToken(delta string) error {
	s.tokenSent = true
	return s.writer.writeEvent("token", fiber.Map{"delta": delta})
}

func (s *streamState) OnCitations(citations []answer.Citation) error {
	return s.writer.writeEvent("citations", fiber.Map{"citations": citations})
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
	var req answerRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid_request", "invalid json", err.Error())
	}
	question, filters := normalizeAnswerRequest(req, h.Tones)
	if question == "" {
		return respondError(c, 400, "validation", "question is required", nil)
	}
	ctx := c.Context()
	_ = h.Store.LogQuery(ctx, question)
	results, err := h.Retriever.Search(ctx, question, filters.TopK)
	if err != nil {
		return respondError(c, 500, "search_error", "failed to search", err.Error())
	}
	sources := make([]answer.Source, 0, len(results))
	for _, r := range results {
		sources = append(sources, answer.Source{Text: r.Text, CitationID: r.CitationID})
	}
	oneTone := h.Tones[defaultToneKey]
	if v, ok := h.Tones[filters.Tone]; ok {
		oneTone = v
	}
	ansSvc := &answer.Service{Client: h.AnswerCli, Guard: h.Guard, Tone: oneTone}

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

		if done := c.Context().Done(); done != nil {
			go func() {
				<-done
				cancel()
			}()
		}

		writer := newSSEWriter(w)
		state := &streamState{writer: writer, messageID: messageID, model: model}

		_ = writer.writeEvent("meta", fiber.Map{
			"message_id": messageID,
			"model":      model,
			"created_at": createdAt,
		})

		h.Logger.Info("stream start", "message_id", messageID, "model", model)

		state.sendErrors = false
		err := ansSvc.Stream(streamCtx, question, sources, state)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				h.Logger.Info("stream abort", "message_id", messageID)
				return
			}
			if state.tokenSent {
				state.sendErrors = true
				state.OnError(err)
				h.Logger.Error("stream error", "message_id", messageID, "error", err)
				return
			}
			ans, genErr := ansSvc.Generate(streamCtx, question, sources)
			if genErr != nil {
				state.sendErrors = true
				state.OnError(genErr)
				h.Logger.Error("stream fallback error", "message_id", messageID, "error", genErr)
				return
			}
			_ = state.OnToken(ans)
			_ = state.OnCitations(mapCitations(sources))
			state.OnDone()
			h.Logger.Info("stream end", "message_id", messageID, "fallback", true)
			return
		}
		h.Logger.Info("stream end", "message_id", messageID, "fallback", false)
	})

	return nil
}

func mapCitations(sources []answer.Source) []answer.Citation {
	citations := make([]answer.Citation, 0, len(sources))
	for _, s := range sources {
		citations = append(citations, answer.Citation{Text: s.Text, CitationID: s.CitationID})
	}
	return citations
}
