package auth

import (
	"context"
	"errors"
	"time"

	"github.com/khiemnd777/legal_api/domain"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrWeakPassword = errors.New("weak password")

type Identity struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken        string `json:"access_token"`
	ExpiresIn          int    `json:"expires_in"`
	MustChangePassword bool   `json:"must_change_password"`
}

type RefreshResponse struct {
	AccessToken        string `json:"access_token"`
	ExpiresIn          int    `json:"expires_in"`
	MustChangePassword bool   `json:"must_change_password"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type UserStore interface {
	CountUsers(ctx context.Context) (int, error)
	GetUserByID(ctx context.Context, id string) (domain.User, error)
	GetUserByUsername(ctx context.Context, username string) (domain.User, error)
	CreateUser(ctx context.Context, user domain.User) error
	UpdateUserPassword(ctx context.Context, userID, passwordHash string, changedAt time.Time) error
	CreateRefreshSession(ctx context.Context, session domain.RefreshSession) error
	GetRefreshSessionByTokenHash(ctx context.Context, tokenHash string) (domain.RefreshSession, error)
	RotateRefreshSession(ctx context.Context, sessionID, currentTokenHash, nextTokenHash string, expiresAt, rotatedAt time.Time) error
	RevokeRefreshSessionByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error
	RevokeAllRefreshSessionsByUserID(ctx context.Context, userID string, revokedAt time.Time) error
}

type Config struct {
	Secret          string
	Issuer          string
	TokenTTL        time.Duration
	RefreshTokenTTL time.Duration
}
