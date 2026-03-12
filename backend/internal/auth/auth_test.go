package auth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/domain"
	"golang.org/x/crypto/bcrypt"
)

type fakeUserStore struct {
	users map[string]domain.User
}

func newFakeUserStore(t *testing.T) *fakeUserStore {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := domain.User{
		ID:           "f6f42987-61ab-4be1-ae31-3fbe2273ab8b",
		Username:     "admin",
		PasswordHash: string(hash),
		Role:         "admin",
		CreatedAt:    time.Now().UTC(),
	}
	return &fakeUserStore{
		users: map[string]domain.User{
			user.Username: user,
		},
	}
}

func (f *fakeUserStore) CountUsers(ctx context.Context) (int, error) {
	return len(f.users), nil
}

func (f *fakeUserStore) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
	user, ok := f.users[username]
	if !ok {
		return domain.User{}, sql.ErrNoRows
	}
	return user, nil
}

func (f *fakeUserStore) CreateUser(ctx context.Context, user domain.User) error {
	if _, exists := f.users[user.Username]; exists {
		return errors.New("duplicate username")
	}
	f.users[user.Username] = user
	return nil
}

func setupAuthApp(t *testing.T, secret string) *fiber.App {
	t.Helper()
	store := newFakeUserStore(t)
	service := NewService(store, Config{
		Secret:   secret,
		Issuer:   "test_issuer",
		TokenTTL: time.Hour,
	})
	handlers := NewHandlers(service, NewLoginRateLimiter(5, time.Minute))
	requireAuth := RequireAuth(service.JWTManager())

	app := fiber.New()
	app.Post("/auth/login", handlers.Login)
	app.Get("/auth/me", requireAuth, handlers.Me)
	app.Get("/admin/ping", requireAuth, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})
	return app
}

func TestLoginAndMeSuccess(t *testing.T) {
	app := setupAuthApp(t, "test-secret")

	reqBody := map[string]string{"username": "admin", "password": "password"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", resp.StatusCode)
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginResp.AccessToken == "" {
		t.Fatalf("expected access token")
	}
	if loginResp.ExpiresIn <= 0 {
		t.Fatalf("expected positive expires_in")
	}

	meReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	meResp, err := app.Test(meReq, -1)
	if err != nil {
		t.Fatalf("me request failed: %v", err)
	}
	defer meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("expected /auth/me 200, got %d", meResp.StatusCode)
	}
}

func TestWrongPasswordReturnsUnauthorized(t *testing.T) {
	app := setupAuthApp(t, "test-secret")
	reqBody := map[string]string{"username": "admin", "password": "wrong"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestProtectedRouteRejectsUnauthenticated(t *testing.T) {
	app := setupAuthApp(t, "test-secret")
	req := httptest.NewRequest(http.MethodGet, "/admin/ping", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	app := setupAuthApp(t, "test-secret")
	manager := NewJWTManager("test-secret", "test_issuer", -1*time.Second)
	token, _, err := manager.GenerateToken(Identity{
		UserID:   "f6f42987-61ab-4be1-ae31-3fbe2273ab8b",
		Username: "admin",
		Role:     "admin",
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLoginRateLimit(t *testing.T) {
	app := setupAuthApp(t, "test-secret")
	reqBody := map[string]string{"username": "admin", "password": "wrong"}
	body, _ := json.Marshal(reqBody)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d expected 401, got %d", i+1, resp.StatusCode)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
}
