package admin

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/admin/service"
	"github.com/khiemnd777/legal_api/domain"
)

type RetrievalConfigHandler struct {
	Service         *service.RetrievalConfigService
	OnConfigChanged func()
}

func NewRetrievalConfigHandler(svc *service.RetrievalConfigService, onConfigChanged func()) *RetrievalConfigHandler {
	return &RetrievalConfigHandler{Service: svc, OnConfigChanged: onConfigChanged}
}

func (h *RetrievalConfigHandler) List(c *fiber.Ctx) error {
	items, err := h.Service.List(c.Context())
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to list retrieval configs", err.Error())
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *RetrievalConfigHandler) Get(c *fiber.Ctx) error {
	item, err := h.Service.Get(c.Context(), c.Params("id"))
	if err != nil {
		if errors.Is(err, service.ErrRetrievalConfigNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "retrieval config not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to load retrieval config", err.Error())
	}
	return c.JSON(item)
}

func (h *RetrievalConfigHandler) Create(c *fiber.Ctx) error {
	var req domain.AIRetrievalConfig
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json", err.Error())
	}
	created, err := h.Service.Create(c.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidRetrievalConfig) {
			return respondError(c, fiber.StatusBadRequest, "validation", "invalid retrieval config payload", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to create retrieval config", err.Error())
	}
	h.notifyChanged()
	return c.Status(fiber.StatusCreated).JSON(created)
}

func (h *RetrievalConfigHandler) Update(c *fiber.Ctx) error {
	var req domain.AIRetrievalConfig
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json", err.Error())
	}
	updated, err := h.Service.Update(c.Context(), c.Params("id"), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidRetrievalConfig) {
			return respondError(c, fiber.StatusBadRequest, "validation", "invalid retrieval config payload", nil)
		}
		if errors.Is(err, service.ErrRetrievalConfigNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "retrieval config not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to update retrieval config", err.Error())
	}
	h.notifyChanged()
	return c.JSON(updated)
}

func (h *RetrievalConfigHandler) Delete(c *fiber.Ctx) error {
	if err := h.Service.Delete(c.Context(), c.Params("id")); err != nil {
		if errors.Is(err, service.ErrRetrievalConfigNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "retrieval config not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to delete retrieval config", err.Error())
	}
	h.notifyChanged()
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *RetrievalConfigHandler) Enable(c *fiber.Ctx) error {
	updated, err := h.Service.Enable(c.Context(), c.Params("id"))
	if err != nil {
		if errors.Is(err, service.ErrRetrievalConfigNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "retrieval config not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to enable retrieval config", err.Error())
	}
	h.notifyChanged()
	return c.JSON(updated)
}

func (h *RetrievalConfigHandler) Disable(c *fiber.Ctx) error {
	updated, err := h.Service.Disable(c.Context(), c.Params("id"))
	if err != nil {
		if errors.Is(err, service.ErrRetrievalConfigNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "retrieval config not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to disable retrieval config", err.Error())
	}
	h.notifyChanged()
	return c.JSON(updated)
}

func (h *RetrievalConfigHandler) notifyChanged() {
	if h.OnConfigChanged != nil {
		h.OnConfigChanged()
	}
}
