package admin

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/admin/service"
	"github.com/khiemnd777/legal_api/domain"
)

type TelegramBotController interface {
	StartTelegramBot(ctx context.Context, id string) (domain.TelegramBot, error)
	StopTelegramBot(ctx context.Context, id string) (domain.TelegramBot, error)
	StopTelegramBotIfRunning(id string)
}

type TelegramBotHandler struct {
	Service    *service.TelegramBotService
	Controller TelegramBotController
}

type telegramBotRequest struct {
	Name                   string  `json:"name"`
	Token                  string  `json:"token"`
	DefaultTone            string  `json:"default_tone"`
	DefaultTopK            int     `json:"default_top_k"`
	DefaultEffectiveStatus string  `json:"default_effective_status"`
	DefaultDomain          string  `json:"default_domain"`
	DefaultDocType         string  `json:"default_doc_type"`
	AllowedChatIDs         []int64 `json:"allowed_chat_ids"`
	WelcomeMessage         string  `json:"welcome_message"`
	StartAfterSave         bool    `json:"start_after_save"`
}

func NewTelegramBotHandler(svc *service.TelegramBotService, controller TelegramBotController) *TelegramBotHandler {
	return &TelegramBotHandler{Service: svc, Controller: controller}
}

func (h *TelegramBotHandler) List(c *fiber.Ctx) error {
	items, err := h.Service.List(c.Context())
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to list telegram bots", err.Error())
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *TelegramBotHandler) Get(c *fiber.Ctx) error {
	item, err := h.Service.Get(c.Context(), c.Params("id"))
	if err != nil {
		if errors.Is(err, service.ErrTelegramBotNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "telegram bot not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to load telegram bot", err.Error())
	}
	return c.JSON(item)
}

func (h *TelegramBotHandler) Create(c *fiber.Ctx) error {
	var req telegramBotRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json", err.Error())
	}
	created, err := h.Service.Create(c.Context(), toTelegramBotInput(req))
	if err != nil {
		if errors.Is(err, service.ErrInvalidTelegramBot) {
			return respondError(c, fiber.StatusBadRequest, "validation", "invalid telegram bot payload", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to create telegram bot", err.Error())
	}
	if req.StartAfterSave && h.Controller != nil {
		started, startErr := h.Controller.StartTelegramBot(c.UserContext(), created.ID)
		if startErr != nil {
			return respondError(c, fiber.StatusBadGateway, "telegram_error", "failed to start telegram bot", startErr.Error())
		}
		return c.Status(fiber.StatusCreated).JSON(started)
	}
	return c.Status(fiber.StatusCreated).JSON(created)
}

func (h *TelegramBotHandler) Update(c *fiber.Ctx) error {
	var req telegramBotRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json", err.Error())
	}
	shouldStopAfterUpdate := h.Controller != nil && req.Token != ""
	if shouldStopAfterUpdate {
		h.Controller.StopTelegramBotIfRunning(c.Params("id"))
	}
	updated, err := h.Service.Update(c.Context(), c.Params("id"), toTelegramBotInput(req))
	if err != nil {
		if errors.Is(err, service.ErrInvalidTelegramBot) {
			return respondError(c, fiber.StatusBadRequest, "validation", "invalid telegram bot payload", nil)
		}
		if errors.Is(err, service.ErrTelegramBotNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "telegram bot not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to update telegram bot", err.Error())
	}
	if req.StartAfterSave && h.Controller != nil {
		started, startErr := h.Controller.StartTelegramBot(c.UserContext(), updated.ID)
		if startErr != nil {
			return respondError(c, fiber.StatusBadGateway, "telegram_error", "failed to start telegram bot", startErr.Error())
		}
		return c.JSON(started)
	}
	if shouldStopAfterUpdate {
		stopped, stopErr := h.Controller.StopTelegramBot(c.UserContext(), updated.ID)
		if stopErr != nil {
			return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to stop telegram bot after token update", stopErr.Error())
		}
		return c.JSON(stopped)
	}
	return c.JSON(updated)
}

func (h *TelegramBotHandler) Start(c *fiber.Ctx) error {
	if h.Controller == nil {
		return respondError(c, fiber.StatusServiceUnavailable, "runtime_unavailable", "telegram runtime is not available", nil)
	}
	item, err := h.Controller.StartTelegramBot(c.UserContext(), c.Params("id"))
	if err != nil {
		if errors.Is(err, service.ErrTelegramBotNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "telegram bot not found", nil)
		}
		return respondError(c, fiber.StatusBadGateway, "telegram_error", "failed to start telegram bot", err.Error())
	}
	return c.JSON(item)
}

func (h *TelegramBotHandler) Stop(c *fiber.Ctx) error {
	if h.Controller == nil {
		return respondError(c, fiber.StatusServiceUnavailable, "runtime_unavailable", "telegram runtime is not available", nil)
	}
	item, err := h.Controller.StopTelegramBot(c.UserContext(), c.Params("id"))
	if err != nil {
		if errors.Is(err, service.ErrTelegramBotNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "telegram bot not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to stop telegram bot", err.Error())
	}
	return c.JSON(item)
}

func (h *TelegramBotHandler) Delete(c *fiber.Ctx) error {
	if h.Controller != nil {
		h.Controller.StopTelegramBotIfRunning(c.Params("id"))
	}
	if err := h.Service.Delete(c.Context(), c.Params("id")); err != nil {
		if errors.Is(err, service.ErrTelegramBotNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "telegram bot not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to delete telegram bot", err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func toTelegramBotInput(req telegramBotRequest) service.TelegramBotInput {
	return service.TelegramBotInput{
		Name:                   req.Name,
		Token:                  req.Token,
		DefaultTone:            req.DefaultTone,
		DefaultTopK:            req.DefaultTopK,
		DefaultEffectiveStatus: req.DefaultEffectiveStatus,
		DefaultDomain:          req.DefaultDomain,
		DefaultDocType:         req.DefaultDocType,
		AllowedChatIDs:         req.AllowedChatIDs,
		WelcomeMessage:         req.WelcomeMessage,
	}
}
