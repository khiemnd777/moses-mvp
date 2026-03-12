package config

import "testing"

func TestLoadSuccessWithDefaults(t *testing.T) {
	t.Setenv("JWT_SECRET", "jwt-secret")
	t.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "admin-pass")
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ServerPort != "8080" {
		t.Fatalf("expected default port 8080, got %q", cfg.ServerPort)
	}
	if cfg.ConfigPath != "config/config.yaml" {
		t.Fatalf("expected default config path, got %q", cfg.ConfigPath)
	}
	if cfg.DatabaseURL != "" {
		t.Fatalf("expected empty DATABASE_URL, got %q", cfg.DatabaseURL)
	}
}

func TestLoadFailsWhenJWTSecretMissing(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	t.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "admin-pass")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "missing required environment variable JWT_SECRET" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadFailsWhenAdminBootstrapPasswordMissing(t *testing.T) {
	t.Setenv("JWT_SECRET", "jwt-secret")
	t.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "missing required environment variable ADMIN_BOOTSTRAP_PASSWORD" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadFailsWhenPortInvalid(t *testing.T) {
	t.Setenv("JWT_SECRET", "jwt-secret")
	t.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "admin-pass")
	t.Setenv("PORT", "abc")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}
