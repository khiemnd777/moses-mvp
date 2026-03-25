package auth

import (
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

const refreshTokenCookieName = "refresh_token"

type Handlers struct {
	service *Service
	limiter *LoginRateLimiter
}

func NewHandlers(service *Service, limiter *LoginRateLimiter) *Handlers {
	return &Handlers{service: service, limiter: limiter}
}

func (h *Handlers) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid_request", "invalid json")
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		return respondError(c, fiber.StatusBadRequest, "validation", "username and password are required")
	}
	if !h.limiter.Allow(c.IP()) {
		return respondError(c, fiber.StatusTooManyRequests, "too_many_requests", "too many login attempts, try again later")
	}
	resp, identity, err := h.service.Authenticate(c.UserContext(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return respondError(c, fiber.StatusUnauthorized, "unauthorized", "invalid username or password")
		}
		return respondError(c, fiber.StatusInternalServerError, "internal_error", "failed to authenticate")
	}
	refreshToken, refreshExpiresAt, err := h.service.CreateSession(c.UserContext(), identity)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "internal_error", "failed to create session")
	}
	setRefreshTokenCookie(c, refreshToken, refreshExpiresAt)
	return c.JSON(resp)
}

func (h *Handlers) Refresh(c *fiber.Ctx) error {
	refreshToken := strings.TrimSpace(c.Cookies(refreshTokenCookieName))
	if refreshToken == "" {
		clearRefreshTokenCookie(c)
		return unauthorized(c)
	}
	resp, nextRefreshToken, refreshExpiresAt, err := h.service.RefreshSession(c.UserContext(), refreshToken)
	if err != nil {
		clearRefreshTokenCookie(c)
		if errors.Is(err, ErrInvalidRefreshToken) {
			return unauthorized(c)
		}
		return respondError(c, fiber.StatusInternalServerError, "internal_error", "failed to refresh session")
	}
	setRefreshTokenCookie(c, nextRefreshToken, refreshExpiresAt)
	return c.JSON(resp)
}

func (h *Handlers) Logout(c *fiber.Ctx) error {
	refreshToken := strings.TrimSpace(c.Cookies(refreshTokenCookieName))
	if refreshToken != "" {
		if err := h.service.Logout(c.UserContext(), refreshToken); err != nil {
			return respondError(c, fiber.StatusInternalServerError, "internal_error", "failed to logout")
		}
	}
	clearRefreshTokenCookie(c)
	return c.JSON(fiber.Map{"status": "logged_out"})
}

func (h *Handlers) Me(c *fiber.Ctx) error {
	identity, ok := GetIdentity(c)
	if !ok {
		return unauthorized(c)
	}
	user, err := h.service.GetUserByID(c.UserContext(), identity.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return unauthorized(c)
		}
		return respondError(c, fiber.StatusInternalServerError, "internal_error", "failed to load user")
	}
	return c.JSON(fiber.Map{
		"id":                   identity.UserID,
		"username":             identity.Username,
		"role":                 identity.Role,
		"must_change_password": user.MustChangePassword,
	})
}

func respondError(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    code,
			"message": message,
		},
	})
}

func setRefreshTokenCookie(c *fiber.Ctx, token string, expiresAt time.Time) {
	c.Cookie(&fiber.Cookie{
		Name:     refreshTokenCookieName,
		Value:    token,
		HTTPOnly: true,
		Secure:   isSecureRequest(c),
		SameSite: sameSiteMode(c),
		Path:     "/auth",
		Expires:  expiresAt,
	})
}

func clearRefreshTokenCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     refreshTokenCookieName,
		Value:    "",
		HTTPOnly: true,
		Secure:   isSecureRequest(c),
		SameSite: sameSiteMode(c),
		Path:     "/auth",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func isSecureRequest(c *fiber.Ctx) bool {
	if strings.EqualFold(c.Protocol(), "https") {
		return true
	}
	return strings.EqualFold(c.Get("X-Forwarded-Proto"), "https")
}

func sameSiteMode(c *fiber.Ctx) string {
	if isSecureRequest(c) {
		return "None"
	}
	return "Lax"
}

type LoginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
	nowFn    func() time.Time
}

func NewLoginRateLimiter(limit int, window time.Duration) *LoginRateLimiter {
	return &LoginRateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
		nowFn:    time.Now,
	}
}

func (l *LoginRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.nowFn()
	cutoff := now.Add(-l.window)
	records := l.attempts[ip]
	kept := records[:0]
	for _, at := range records {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) >= l.limit {
		l.attempts[ip] = kept
		return false
	}
	l.attempts[ip] = append(kept, now)
	return true
}
