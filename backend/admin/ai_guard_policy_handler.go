package admin

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/admin/service"
	"github.com/khiemnd777/legal_api/domain"
)

type GuardPolicyHandler struct {
	Service         *service.GuardPolicyService
	OnConfigChanged func()
}

func NewGuardPolicyHandler(svc *service.GuardPolicyService, onConfigChanged func()) *GuardPolicyHandler {
	return &GuardPolicyHandler{Service: svc, OnConfigChanged: onConfigChanged}
}

func (h *GuardPolicyHandler) List(c *fiber.Ctx) error {
	items, err := h.Service.List(c.Context())
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to list guard policies", err.Error())
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *GuardPolicyHandler) Get(c *fiber.Ctx) error {
	item, err := h.Service.Get(c.Context(), c.Params("id"))
	if err != nil {
		if errors.Is(err, service.ErrGuardPolicyNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "guard policy not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to load guard policy", err.Error())
	}
	return c.JSON(item)
}

func (h *GuardPolicyHandler) Create(c *fiber.Ctx) error {
	var req domain.AIGuardPolicy
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json", err.Error())
	}
	created, err := h.Service.Create(c.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidGuardPolicy) {
			return respondError(c, fiber.StatusBadRequest, "validation", "invalid guard policy payload", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to create guard policy", err.Error())
	}
	h.notifyChanged()
	return c.Status(fiber.StatusCreated).JSON(created)
}

func (h *GuardPolicyHandler) Update(c *fiber.Ctx) error {
	var req domain.AIGuardPolicy
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json", err.Error())
	}
	updated, err := h.Service.Update(c.Context(), c.Params("id"), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidGuardPolicy) {
			return respondError(c, fiber.StatusBadRequest, "validation", "invalid guard policy payload", nil)
		}
		if errors.Is(err, service.ErrGuardPolicyNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "guard policy not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to update guard policy", err.Error())
	}
	h.notifyChanged()
	return c.JSON(updated)
}

func (h *GuardPolicyHandler) Delete(c *fiber.Ctx) error {
	if err := h.Service.Delete(c.Context(), c.Params("id")); err != nil {
		if errors.Is(err, service.ErrGuardPolicyNotFound) {
			return respondError(c, fiber.StatusNotFound, "not_found", "guard policy not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to delete guard policy", err.Error())
	}
	h.notifyChanged()
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *GuardPolicyHandler) notifyChanged() {
	if h.OnConfigChanged != nil {
		h.OnConfigChanged()
	}
}
