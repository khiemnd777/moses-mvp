package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type UserClaims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
	nowFn  func() time.Time
}

func NewJWTManager(secret, issuer string, ttl time.Duration) *JWTManager {
	secret = strings.TrimSpace(secret)
	return &JWTManager{
		secret: []byte(secret),
		issuer: issuer,
		ttl:    ttl,
		nowFn:  time.Now,
	}
}

func (m *JWTManager) GenerateToken(identity Identity) (string, time.Time, error) {
	now := m.nowFn().UTC()
	exp := now.Add(m.ttl)
	claims := UserClaims{
		Username: identity.Username,
		Role:     identity.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   identity.UserID,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

func (m *JWTManager) ParseAndValidate(tokenRaw string) (Identity, error) {
	token, err := jwt.ParseWithClaims(tokenRaw, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return Identity{}, ErrInvalidToken
	}
	claims, ok := token.Claims.(*UserClaims)
	if !ok {
		return Identity{}, ErrInvalidToken
	}
	if claims.Issuer != m.issuer {
		return Identity{}, ErrInvalidToken
	}
	if claims.ExpiresAt == nil || claims.ExpiresAt.Time.Before(m.nowFn().UTC()) {
		return Identity{}, ErrInvalidToken
	}
	if claims.Subject == "" || claims.Username == "" || claims.Role == "" {
		return Identity{}, ErrInvalidToken
	}
	return Identity{
		UserID:   claims.Subject,
		Username: claims.Username,
		Role:     claims.Role,
	}, nil
}
