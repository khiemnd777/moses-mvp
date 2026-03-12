package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/khiemnd777/legal_api/domain"
	"golang.org/x/crypto/bcrypt"
)

const (
	bootstrapAdminUsername = "admin"
	bootstrapAdminRole     = "admin"
)

func BootstrapAdmin(ctx context.Context, store UserStore, bootstrapPassword string, logger *slog.Logger) error {
	count, err := store.CountUsers(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	password := strings.TrimSpace(bootstrapPassword)
	if password == "" {
		return errors.New("missing required environment variable ADMIN_BOOTSTRAP_PASSWORD")
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := store.CreateUser(ctx, domain.User{
		ID:           uuid.NewString(),
		Username:     bootstrapAdminUsername,
		PasswordHash: string(passwordHash),
		Role:         bootstrapAdminRole,
	}); err != nil {
		return err
	}

	if logger != nil {
		logger.Info("Bootstrap admin created")
	}
	return nil
}
