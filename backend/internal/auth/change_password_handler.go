package auth

import (
	"errors"

	"github.com/gofiber/fiber/v2"
)

func (h *Handlers) ChangePassword(c *fiber.Ctx) error {
	identity, ok := GetIdentity(c)
	if !ok {
		return unauthorized(c)
	}

	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json")
	}
	if req.OldPassword == "" || req.NewPassword == "" {
		return respondError(c, fiber.StatusBadRequest, "validation", "old_password and new_password are required")
	}

	err := h.service.ChangePassword(c.UserContext(), identity.UserID, req.OldPassword, req.NewPassword)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return respondError(c, fiber.StatusUnauthorized, "unauthorized", "invalid old password")
		}
		if errors.Is(err, ErrWeakPassword) {
			return respondError(c, fiber.StatusBadRequest, "validation", "new_password must be at least 8 characters")
		}
		return respondError(c, fiber.StatusInternalServerError, "internal_error", "failed to update password")
	}

	resp, _, err := h.service.issueLoginResponse(Identity{
		UserID:   identity.UserID,
		Username: identity.Username,
		Role:     identity.Role,
	}, false)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "internal_error", "failed to issue session")
	}
	refreshToken, refreshExpiresAt, err := h.service.CreateSession(c.UserContext(), identity)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "internal_error", "failed to create session")
	}
	setRefreshTokenCookie(c, refreshToken, refreshExpiresAt)
	return c.JSON(fiber.Map{
		"status":               "password_updated",
		"access_token":         resp.AccessToken,
		"expires_in":           resp.ExpiresIn,
		"must_change_password": false,
	})
}
