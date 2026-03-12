package auth

import (
	"context"
	"errors"
	"time"

	"github.com/khiemnd777/legal_api/domain"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

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
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type UserStore interface {
	CountUsers(ctx context.Context) (int, error)
	GetUserByUsername(ctx context.Context, username string) (domain.User, error)
	CreateUser(ctx context.Context, user domain.User) error
}

type Config struct {
	Secret   string
	Issuer   string
	TokenTTL time.Duration
}
