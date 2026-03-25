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
	users           map[string]domain.User
	refreshSessions map[string]domain.RefreshSession
}

func newFakeUserStore(t *testing.T) *fakeUserStore {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := domain.User{
		ID:                 "f6f42987-61ab-4be1-ae31-3fbe2273ab8b",
		Username:           "admin",
		PasswordHash:       string(hash),
		Role:               "admin",
		MustChangePassword: false,
		CreatedAt:          time.Now().UTC(),
	}
	return &fakeUserStore{
		users: map[string]domain.User{
			user.Username: user,
		},
		refreshSessions: make(map[string]domain.RefreshSession),
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

func (f *fakeUserStore) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	for _, user := range f.users {
		if user.ID == id {
			return user, nil
		}
	}
	return domain.User{}, sql.ErrNoRows
}

func (f *fakeUserStore) CreateUser(ctx context.Context, user domain.User) error {
	if _, exists := f.users[user.Username]; exists {
		return errors.New("duplicate username")
	}
	f.users[user.Username] = user
	return nil
}

func (f *fakeUserStore) UpdateUserPassword(ctx context.Context, userID, passwordHash string, changedAt time.Time) error {
	for username, user := range f.users {
		if user.ID == userID {
			user.PasswordHash = passwordHash
			user.MustChangePassword = false
			user.PasswordChangedAt = &changedAt
			f.users[username] = user
			return nil
		}
	}
	return sql.ErrNoRows
}

func (f *fakeUserStore) CreateRefreshSession(ctx context.Context, session domain.RefreshSession) error {
	session.CreatedAt = time.Now().UTC()
	session.UpdatedAt = session.CreatedAt
	f.refreshSessions[session.TokenHash] = session
	return nil
}

func (f *fakeUserStore) GetRefreshSessionByTokenHash(ctx context.Context, tokenHash string) (domain.RefreshSession, error) {
	session, ok := f.refreshSessions[tokenHash]
	if !ok {
		return domain.RefreshSession{}, sql.ErrNoRows
	}
	return session, nil
}

func (f *fakeUserStore) RotateRefreshSession(ctx context.Context, sessionID, currentTokenHash, nextTokenHash string, expiresAt, rotatedAt time.Time) error {
	session, ok := f.refreshSessions[currentTokenHash]
	if !ok || session.ID != sessionID || session.RevokedAt != nil {
		return sql.ErrNoRows
	}
	delete(f.refreshSessions, currentTokenHash)
	session.TokenHash = nextTokenHash
	session.ExpiresAt = expiresAt
	session.ReplacedByHash = &nextTokenHash
	session.UpdatedAt = rotatedAt
	f.refreshSessions[nextTokenHash] = session
	return nil
}

func (f *fakeUserStore) RevokeRefreshSessionByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	session, ok := f.refreshSessions[tokenHash]
	if !ok {
		return sql.ErrNoRows
	}
	session.RevokedAt = &revokedAt
	session.UpdatedAt = revokedAt
	f.refreshSessions[tokenHash] = session
	return nil
}

func (f *fakeUserStore) RevokeAllRefreshSessionsByUserID(ctx context.Context, userID string, revokedAt time.Time) error {
	for tokenHash, session := range f.refreshSessions {
		if session.UserID == userID {
			session.RevokedAt = &revokedAt
			session.UpdatedAt = revokedAt
			f.refreshSessions[tokenHash] = session
		}
	}
	return nil
}

func setupAuthApp(t *testing.T, secret string) *fiber.App {
	t.Helper()
	store := newFakeUserStore(t)
	return setupAuthAppWithStore(t, store, secret)
}

func setupAuthAppWithStore(t *testing.T, store *fakeUserStore, secret string) *fiber.App {
	t.Helper()
	service := NewService(store, Config{
		Secret:          secret,
		Issuer:          "test_issuer",
		TokenTTL:        time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	})
	handlers := NewHandlers(service, NewLoginRateLimiter(5, time.Minute))
	requireAuth := RequireAuth(service.JWTManager(), store)

	app := fiber.New()
	app.Post("/auth/login", handlers.Login)
	app.Post("/auth/refresh", handlers.Refresh)
	app.Post("/auth/logout", handlers.Logout)
	app.Get("/auth/me", requireAuth, handlers.Me)
	app.Post("/auth/change-password", requireAuth, handlers.ChangePassword)
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
	if len(resp.Cookies()) == 0 {
		t.Fatalf("expected refresh token cookie")
	}
	if loginResp.ExpiresIn <= 0 {
		t.Fatalf("expected positive expires_in")
	}
	if loginResp.MustChangePassword {
		t.Fatalf("expected must_change_password=false")
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

	var mePayload map[string]any
	if err := json.NewDecoder(meResp.Body).Decode(&mePayload); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if mePayload["must_change_password"] != false {
		t.Fatalf("expected must_change_password=false in /auth/me")
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

func TestProtectedRouteBlockedWhenPasswordChangeRequired(t *testing.T) {
	store := newFakeUserStore(t)
	user := store.users["admin"]
	user.MustChangePassword = true
	store.users["admin"] = user

	service := NewService(store, Config{
		Secret:          "test-secret",
		Issuer:          "test_issuer",
		TokenTTL:        time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	})
	handlers := NewHandlers(service, NewLoginRateLimiter(5, time.Minute))
	requireAuth := RequireAuth(service.JWTManager(), store)

	app := fiber.New()
	app.Post("/auth/login", handlers.Login)
	app.Get("/playground/ping", requireAuth, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})

	reqBody := map[string]string{"username": "admin", "password": "password"}
	body, _ := json.Marshal(reqBody)
	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq, -1)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer loginResp.Body.Close()

	var tokenResp LoginResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&tokenResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if !tokenResp.MustChangePassword {
		t.Fatalf("expected must_change_password=true")
	}

	protectedReq := httptest.NewRequest(http.MethodGet, "/playground/ping", nil)
	protectedReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	protectedResp, err := app.Test(protectedReq, -1)
	if err != nil {
		t.Fatalf("protected request failed: %v", err)
	}
	defer protectedResp.Body.Close()
	if protectedResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", protectedResp.StatusCode)
	}
}

func TestChangePasswordFlowUnblocksProtectedRoutes(t *testing.T) {
	store := newFakeUserStore(t)
	user := store.users["admin"]
	user.MustChangePassword = true
	store.users["admin"] = user

	service := NewService(store, Config{
		Secret:          "test-secret",
		Issuer:          "test_issuer",
		TokenTTL:        time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	})
	handlers := NewHandlers(service, NewLoginRateLimiter(5, time.Minute))
	requireAuth := RequireAuth(service.JWTManager(), store)

	app := fiber.New()
	app.Post("/auth/login", handlers.Login)
	app.Post("/auth/refresh", handlers.Refresh)
	app.Post("/auth/logout", handlers.Logout)
	app.Post("/auth/change-password", requireAuth, handlers.ChangePassword)
	app.Get("/playground/ping", requireAuth, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})

	loginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "password"})
	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq, -1)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer loginResp.Body.Close()

	var tokenResp LoginResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&tokenResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	refreshCookie := findCookie(loginResp.Cookies(), refreshTokenCookieName)
	if refreshCookie == nil {
		t.Fatalf("expected refresh token cookie")
	}

	changeBody, _ := json.Marshal(map[string]string{"old_password": "password", "new_password": "newpassword"})
	changeReq := httptest.NewRequest(http.MethodPost, "/auth/change-password", bytes.NewReader(changeBody))
	changeReq.Header.Set("Content-Type", "application/json")
	changeReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	changeResp, err := app.Test(changeReq, -1)
	if err != nil {
		t.Fatalf("change password request failed: %v", err)
	}
	defer changeResp.Body.Close()
	if changeResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", changeResp.StatusCode)
	}

	protectedReq := httptest.NewRequest(http.MethodGet, "/playground/ping", nil)
	protectedReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	protectedResp, err := app.Test(protectedReq, -1)
	if err != nil {
		t.Fatalf("protected request failed: %v", err)
	}
	defer protectedResp.Body.Close()
	if protectedResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", protectedResp.StatusCode)
	}

	oldLoginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "password"})
	oldLoginReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(oldLoginBody))
	oldLoginReq.Header.Set("Content-Type", "application/json")
	oldLoginResp, err := app.Test(oldLoginReq, -1)
	if err != nil {
		t.Fatalf("old login request failed: %v", err)
	}
	defer oldLoginResp.Body.Close()
	if oldLoginResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected old password login to fail with 401, got %d", oldLoginResp.StatusCode)
	}

	refreshReq := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	refreshReq.AddCookie(refreshCookie)
	refreshResp, err := app.Test(refreshReq, -1)
	if err != nil {
		t.Fatalf("refresh request failed: %v", err)
	}
	defer refreshResp.Body.Close()
	if refreshResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected revoked refresh token to fail with 401, got %d", refreshResp.StatusCode)
	}
}

func TestRefreshRotatesSessionAndReturnsNewAccessToken(t *testing.T) {
	app := setupAuthApp(t, "test-secret")

	loginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "password"})
	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq, -1)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer loginResp.Body.Close()

	refreshCookie := findCookie(loginResp.Cookies(), refreshTokenCookieName)
	if refreshCookie == nil {
		t.Fatalf("expected refresh token cookie")
	}

	refreshReq := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	refreshReq.AddCookie(refreshCookie)
	refreshResp, err := app.Test(refreshReq, -1)
	if err != nil {
		t.Fatalf("refresh request failed: %v", err)
	}
	defer refreshResp.Body.Close()
	if refreshResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", refreshResp.StatusCode)
	}

	var refreshed RefreshResponse
	if err := json.NewDecoder(refreshResp.Body).Decode(&refreshed); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	if refreshed.AccessToken == "" {
		t.Fatalf("expected refreshed access token")
	}

	nextRefreshCookie := findCookie(refreshResp.Cookies(), refreshTokenCookieName)
	if nextRefreshCookie == nil || nextRefreshCookie.Value == refreshCookie.Value {
		t.Fatalf("expected rotated refresh token cookie")
	}

	reuseReq := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	reuseReq.AddCookie(refreshCookie)
	reuseResp, err := app.Test(reuseReq, -1)
	if err != nil {
		t.Fatalf("reuse refresh request failed: %v", err)
	}
	defer reuseResp.Body.Close()
	if reuseResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected reused refresh token to fail with 401, got %d", reuseResp.StatusCode)
	}
}

func TestLogoutRevokesRefreshSession(t *testing.T) {
	app := setupAuthApp(t, "test-secret")

	loginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "password"})
	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq, -1)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer loginResp.Body.Close()

	refreshCookie := findCookie(loginResp.Cookies(), refreshTokenCookieName)
	if refreshCookie == nil {
		t.Fatalf("expected refresh token cookie")
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutReq.AddCookie(refreshCookie)
	logoutResp, err := app.Test(logoutReq, -1)
	if err != nil {
		t.Fatalf("logout request failed: %v", err)
	}
	defer logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", logoutResp.StatusCode)
	}

	refreshReq := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	refreshReq.AddCookie(refreshCookie)
	refreshResp, err := app.Test(refreshReq, -1)
	if err != nil {
		t.Fatalf("refresh request failed: %v", err)
	}
	defer refreshResp.Body.Close()
	if refreshResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected logged out refresh token to fail with 401, got %d", refreshResp.StatusCode)
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}
