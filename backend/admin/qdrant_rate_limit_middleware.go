package admin

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type rateWindowLimiter struct {
	mu      sync.Mutex
	windows map[string]*rateWindowState
}

type rateWindowState struct {
	resetAt time.Time
	count   int
}

func newRateWindowLimiter() *rateWindowLimiter {
	return &rateWindowLimiter{
		windows: make(map[string]*rateWindowState),
	}
}

func (l *rateWindowLimiter) allow(key string, limit int, window time.Duration, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	state, ok := l.windows[key]
	if !ok || now.After(state.resetAt) {
		l.windows[key] = &rateWindowState{
			resetAt: now.Add(window),
			count:   1,
		}
		return true, 0
	}
	if state.count < limit {
		state.count++
		return true, 0
	}
	retryAfter := state.resetAt.Sub(now)
	if retryAfter < 0 {
		retryAfter = 0
	}
	return false, retryAfter
}

type qdrantRatePolicy struct {
	Limit  int
	Window time.Duration
}

var adminQdrantLimiter = newRateWindowLimiter()

func qdrantRateLimitMiddleware(policy qdrantRatePolicy) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if policy.Limit <= 0 || policy.Window <= 0 {
			return c.Next()
		}
		key := c.Path() + "|" + adminIdentityKey(c)
		ok, retryAfter := adminQdrantLimiter.allow(key, policy.Limit, policy.Window, time.Now())
		if ok {
			return c.Next()
		}
		seconds := int(math.Ceil(retryAfter.Seconds()))
		if seconds < 1 {
			seconds = 1
		}
		c.Set("Retry-After", strconv.Itoa(seconds))
		return respondError(c, fiber.StatusTooManyRequests, "rate_limited", "rate limit exceeded", fiber.Map{
			"retry_after_seconds": seconds,
		})
	}
}

func adminIdentityKey(c *fiber.Ctx) string {
	if actor := strings.TrimSpace(c.Get("X-Admin-Actor")); actor != "" {
		return "actor:" + actor
	}
	if key := strings.TrimSpace(c.Get("X-Admin-Key")); key != "" {
		return "key:" + key
	}
	auth := strings.TrimSpace(c.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		token := strings.TrimSpace(auth[7:])
		if token != "" {
			return "bearer:" + token
		}
	}
	return "ip:" + c.IP()
}
