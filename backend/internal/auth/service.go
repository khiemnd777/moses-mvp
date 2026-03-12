package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	store UserStore
	jwt   *JWTManager
}

func NewService(store UserStore, cfg Config) *Service {
	return &Service{
		store: store,
		jwt:   NewJWTManager(cfg.Secret, cfg.Issuer, cfg.TokenTTL),
	}
}

func (s *Service) JWTManager() *JWTManager {
	return s.jwt
}

func (s *Service) Authenticate(ctx context.Context, username, password string) (LoginResponse, Identity, error) {
	user, err := s.store.GetUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginResponse{}, Identity{}, ErrInvalidCredentials
		}
		return LoginResponse{}, Identity{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return LoginResponse{}, Identity{}, ErrInvalidCredentials
	}
	identity := Identity{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
	}
	token, _, err := s.jwt.GenerateToken(identity)
	if err != nil {
		return LoginResponse{}, Identity{}, err
	}
	return LoginResponse{
		AccessToken: token,
		ExpiresIn:   int(s.jwt.ttl.Seconds()),
	}, identity, nil
}
