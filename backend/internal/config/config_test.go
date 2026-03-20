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
	if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "http://localhost:5173" {
		t.Fatalf("expected default CORS origin localhost:5173, got %#v", cfg.CORSAllowedOrigins)
	}
}

func TestLoadParsesCORSAllowedOrigins(t *testing.T) {
	t.Setenv("JWT_SECRET", "jwt-secret")
	t.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "admin-pass")
	t.Setenv("CORS_ALLOWED_ORIGINS", " http://localhost:5173, https://ai.dailyturning.com ,https://staging.dailyturning.com ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	expected := []string{
		"http://localhost:5173",
		"https://ai.dailyturning.com",
		"https://staging.dailyturning.com",
	}
	if len(cfg.CORSAllowedOrigins) != len(expected) {
		t.Fatalf("expected %d CORS origins, got %#v", len(expected), cfg.CORSAllowedOrigins)
	}
	for i, want := range expected {
		if cfg.CORSAllowedOrigins[i] != want {
			t.Fatalf("expected CORS origin %d to be %q, got %q", i, want, cfg.CORSAllowedOrigins[i])
		}
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
