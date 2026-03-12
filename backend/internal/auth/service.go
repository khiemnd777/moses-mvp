package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/khiemnd777/legal_api/domain"
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
		AccessToken:        token,
		ExpiresIn:          int(s.jwt.ttl.Seconds()),
		MustChangePassword: user.MustChangePassword,
	}, identity, nil
}

func (s *Service) GetUserByID(ctx context.Context, userID string) (domain.User, error) {
	return s.store.GetUserByID(ctx, userID)
}

func (s *Service) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)) != nil {
		return ErrInvalidCredentials
	}
	if len(newPassword) < 8 {
		return ErrWeakPassword
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.store.UpdateUserPassword(ctx, userID, string(newHash), time.Now().UTC())
}
