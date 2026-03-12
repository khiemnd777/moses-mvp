package auth

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type contextKey string

const identityContextKey contextKey = "auth.identity"

func RequireAuth(jwtManager *JWTManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := strings.TrimSpace(c.Get("Authorization"))
		if raw == "" || !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
			return unauthorized(c)
		}
		token := strings.TrimSpace(raw[7:])
		if token == "" {
			return unauthorized(c)
		}
		identity, err := jwtManager.ParseAndValidate(token)
		if err != nil {
			return unauthorized(c)
		}
		c.Locals(string(identityContextKey), identity)
		userCtx := context.WithValue(c.UserContext(), identityContextKey, identity)
		c.SetUserContext(userCtx)
		return c.Next()
	}
}

func GetIdentity(c *fiber.Ctx) (Identity, bool) {
	value := c.Locals(string(identityContextKey))
	identity, ok := value.(Identity)
	return identity, ok
}

func GetIdentityFromContext(ctx context.Context) (Identity, bool) {
	value := ctx.Value(identityContextKey)
	identity, ok := value.(Identity)
	return identity, ok
}

func unauthorized(c *fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    "unauthorized",
			"message": "invalid or expired token",
		},
	})
}
