package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/khiemnd777/legal_api/domain"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidRefreshToken = errors.New("invalid refresh token")

type Service struct {
	store           UserStore
	jwt             *JWTManager
	refreshTokenTTL time.Duration
	nowFn           func() time.Time
}

func NewService(store UserStore, cfg Config) *Service {
	return &Service{
		store:           store,
		jwt:             NewJWTManager(cfg.Secret, cfg.Issuer, cfg.TokenTTL),
		refreshTokenTTL: cfg.RefreshTokenTTL,
		nowFn:           time.Now,
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
	loginResp, _, err := s.issueLoginResponse(identity, user.MustChangePassword)
	if err != nil {
		return LoginResponse{}, Identity{}, err
	}
	return loginResp, identity, nil
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
	changedAt := s.nowFn().UTC()
	if err := s.store.UpdateUserPassword(ctx, userID, string(newHash), changedAt); err != nil {
		return err
	}
	return s.store.RevokeAllRefreshSessionsByUserID(ctx, userID, changedAt)
}

func (s *Service) CreateSession(ctx context.Context, identity Identity) (string, time.Time, error) {
	rawToken, tokenHash, err := generateRefreshToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := s.nowFn().UTC().Add(s.refreshTokenTTL)
	err = s.store.CreateRefreshSession(ctx, domain.RefreshSession{
		ID:        uuid.NewString(),
		UserID:    identity.UserID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return rawToken, expiresAt, nil
}

func (s *Service) RefreshSession(ctx context.Context, refreshToken string) (RefreshResponse, string, time.Time, error) {
	tokenHash := hashRefreshToken(refreshToken)
	session, err := s.store.GetRefreshSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RefreshResponse{}, "", time.Time{}, ErrInvalidRefreshToken
		}
		return RefreshResponse{}, "", time.Time{}, err
	}
	now := s.nowFn().UTC()
	if session.RevokedAt != nil || !session.ExpiresAt.After(now) {
		return RefreshResponse{}, "", time.Time{}, ErrInvalidRefreshToken
	}
	user, err := s.store.GetUserByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RefreshResponse{}, "", time.Time{}, ErrInvalidRefreshToken
		}
		return RefreshResponse{}, "", time.Time{}, err
	}
	identity := Identity{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
	}
	resp, _, err := s.issueRefreshResponse(identity, user.MustChangePassword)
	if err != nil {
		return RefreshResponse{}, "", time.Time{}, err
	}

	nextToken, nextHash, err := generateRefreshToken()
	if err != nil {
		return RefreshResponse{}, "", time.Time{}, err
	}
	nextExpiresAt := now.Add(s.refreshTokenTTL)
	if err := s.store.RotateRefreshSession(ctx, session.ID, tokenHash, nextHash, nextExpiresAt, now); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RefreshResponse{}, "", time.Time{}, ErrInvalidRefreshToken
		}
		return RefreshResponse{}, "", time.Time{}, err
	}
	return resp, nextToken, nextExpiresAt, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := hashRefreshToken(refreshToken)
	if tokenHash == "" {
		return nil
	}
	err := s.store.RevokeRefreshSessionByTokenHash(ctx, tokenHash, s.nowFn().UTC())
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

func (s *Service) issueLoginResponse(identity Identity, mustChangePassword bool) (LoginResponse, time.Time, error) {
	accessToken, expiresAt, err := s.jwt.GenerateToken(identity)
	if err != nil {
		return LoginResponse{}, time.Time{}, err
	}
	return LoginResponse{
		AccessToken:        accessToken,
		ExpiresIn:          int(time.Until(expiresAt).Seconds()),
		MustChangePassword: mustChangePassword,
	}, expiresAt, nil
}

func (s *Service) issueRefreshResponse(identity Identity, mustChangePassword bool) (RefreshResponse, time.Time, error) {
	accessToken, expiresAt, err := s.jwt.GenerateToken(identity)
	if err != nil {
		return RefreshResponse{}, time.Time{}, err
	}
	return RefreshResponse{
		AccessToken:        accessToken,
		ExpiresIn:          int(time.Until(expiresAt).Seconds()),
		MustChangePassword: mustChangePassword,
	}, expiresAt, nil
}

func generateRefreshToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashRefreshToken(raw), nil
}

func hashRefreshToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
