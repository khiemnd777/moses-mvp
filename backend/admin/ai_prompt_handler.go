package admin

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/admin/service"
	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/core/retrieval"
	"github.com/khiemnd777/legal_api/domain"
)

type PromptHandler struct {
	Service         *service.PromptService
	Retriever       *retrieval.Service
	AnswerClient    *answer.Client
	Tone            string
	OnConfigChanged func()
}

func NewPromptHandler(
	svc *service.PromptService,
	retriever *retrieval.Service,
	answerClient *answer.Client,
	defaultTone string,
	onConfigChanged func(),
) *PromptHandler {
	return &PromptHandler{
		Service:         svc,
		Retriever:       retriever,
		AnswerClient:    answerClient,
		Tone:            defaultTone,
		OnConfigChanged: onConfigChanged,
	}
}

func (h *PromptHandler) List(c *fiber.Ctx) error {
	items, err := h.Service.List(c.Context())
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to list prompts", err.Error())
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *PromptHandler) Get(c *fiber.Ctx) error {
	item, err := h.Service.Get(c.Context(), c.Params("id"))
	if err != nil {
		if errors.Is(err, service.ErrPromptNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "prompt not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to load prompt", err.Error())
	}
	return c.JSON(item)
}

func (h *PromptHandler) Create(c *fiber.Ctx) error {
	var req domain.AIPrompt
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json", err.Error())
	}
	created, err := h.Service.Create(c.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidPrompt) {
			return respondError(c, fiber.StatusBadRequest, "validation", "invalid prompt payload", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to create prompt", err.Error())
	}
	h.notifyChanged()
	return c.Status(fiber.StatusCreated).JSON(created)
}

func (h *PromptHandler) Update(c *fiber.Ctx) error {
	var req domain.AIPrompt
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json", err.Error())
	}
	updated, err := h.Service.Update(c.Context(), c.Params("id"), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidPrompt) {
			return respondError(c, fiber.StatusBadRequest, "validation", "invalid prompt payload", nil)
		}
		if errors.Is(err, service.ErrPromptNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "prompt not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to update prompt", err.Error())
	}
	h.notifyChanged()
	return c.JSON(updated)
}

func (h *PromptHandler) Delete(c *fiber.Ctx) error {
	if err := h.Service.Delete(c.Context(), c.Params("id")); err != nil {
		if errors.Is(err, service.ErrPromptNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "prompt not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to delete prompt", err.Error())
	}
	h.notifyChanged()
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *PromptHandler) Test(c *fiber.Ctx) error {
	var req struct {
		PromptID string `json:"prompt_id"`
		Query    string `json:"query"`
		TopK     int    `json:"top_k"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json", err.Error())
	}
	if req.PromptID == "" || req.Query == "" {
		return respondError(c, fiber.StatusBadRequest, "validation", "prompt_id and query are required", nil)
	}
	prompt, err := h.Service.Get(c.Context(), req.PromptID)
	if err != nil {
		if errors.Is(err, service.ErrPromptNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "prompt not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to load prompt", err.Error())
	}
	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}
	results, err := h.Retriever.Search(c.Context(), req.Query, retrieval.SearchOptions{TopK: topK})
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "search_error", "failed to search", err.Error())
	}
	sources := make([]answer.Source, 0, len(results))
	for _, r := range results {
		sources = append(sources, answer.Source{Text: r.Text, Citation: answer.Citation{ID: r.ChunkID, Excerpt: r.Text}})
	}
	ansSvc := &answer.Service{
		Client:       h.AnswerClient,
		SystemPrompt: prompt.SystemPrompt,
		Tone:         h.Tone,
		Temperature:  prompt.Temperature,
		MaxTokens:    prompt.MaxTokens,
		Retry:        prompt.Retry,
	}
	resp, err := ansSvc.Generate(c.Context(), req.Query, sources)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "answer_error", "failed to generate answer", err.Error())
	}
	citations := make([]answer.Citation, 0, len(sources))
	for _, source := range sources {
		citations = append(citations, source.Citation)
	}
	return c.JSON(fiber.Map{"answer": resp, "citations": citations})
}

func (h *PromptHandler) notifyChanged() {
	if h.OnConfigChanged != nil {
		h.OnConfigChanged()
	}
}
