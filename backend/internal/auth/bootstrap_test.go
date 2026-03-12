package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/khiemnd777/legal_api/domain"
	"golang.org/x/crypto/bcrypt"
)

func TestBootstrapAdminCreatesAdminWhenUsersTableEmpty(t *testing.T) {
	store := &fakeUserStore{users: map[string]domain.User{}}

	if err := BootstrapAdmin(context.Background(), store, "super-secret", nil); err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}

	user, ok := store.users["admin"]
	if !ok {
		t.Fatalf("expected bootstrap admin user to be created")
	}
	if user.Role != "admin" {
		t.Fatalf("expected role admin, got %q", user.Role)
	}
	if user.ID == "" {
		t.Fatalf("expected generated user id")
	}
	if user.PasswordHash == "" {
		t.Fatalf("expected hashed password")
	}
	if user.PasswordHash == "super-secret" {
		t.Fatalf("password should be hashed")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("super-secret")); err != nil {
		t.Fatalf("password hash mismatch: %v", err)
	}
}

func TestBootstrapAdminNoOpWhenUsersExist(t *testing.T) {
	store := newFakeUserStore(t)

	if err := BootstrapAdmin(context.Background(), store, "", nil); err != nil {
		t.Fatalf("bootstrap admin should no-op when users exist: %v", err)
	}

	if len(store.users) != 1 {
		t.Fatalf("expected no new users, got %d", len(store.users))
	}
}

func TestBootstrapAdminErrorsWhenPasswordMissingAndUsersTableEmpty(t *testing.T) {
	store := &fakeUserStore{users: map[string]domain.User{}}

	err := BootstrapAdmin(context.Background(), store, "   ", nil)
	if err == nil {
		t.Fatalf("expected error when bootstrap password is missing")
	}
	if !strings.Contains(err.Error(), "missing required environment variable ADMIN_BOOTSTRAP_PASSWORD") {
		t.Fatalf("unexpected error: %v", err)
	}
}
