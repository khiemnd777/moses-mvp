package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CONFIG_PATH", "config/config.yaml")
	t.Setenv("SERVER_HOST", "0.0.0.0")
	t.Setenv("JWT_SECRET", "jwt-secret")
	t.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "admin-pass")
	t.Setenv("PORT", "8080")
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_USER", "legal")
	t.Setenv("POSTGRES_PASSWORD", "legal")
	t.Setenv("POSTGRES_DB", "legal_rag")
	t.Setenv("POSTGRES_SSLMODE", "disable")
	t.Setenv("QDRANT_HOST", "localhost")
	t.Setenv("QDRANT_PORT", "6333")
	t.Setenv("QDRANT_COLLECTION", "legal_chunks")
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("OPENAI_EMBEDDINGS_MODEL", "text-embedding-3-small")
	t.Setenv("OPENAI_CHAT_MODEL", "gpt-4.1-mini")
	t.Setenv("STORAGE_ROOT_DIR", "./data/uploads")
	t.Setenv("INGEST_DEFAULT_SEGMENTER", "free_text")
	t.Setenv("INGEST_CHUNK_SIZE", "800")
	t.Setenv("INGEST_CHUNK_OVERLAP", "100")
	t.Setenv("GUARD_PROMPT_PATH", "config/prompts/guard.yaml")
	t.Setenv("TONE_DEFAULT_PROMPT_PATH", "config/prompts/tone_default.yaml")
	t.Setenv("TONE_ACADEMIC_PROMPT_PATH", "config/prompts/tone_academic.yaml")
	t.Setenv("TONE_PROCEDURE_PROMPT_PATH", "config/prompts/tone_procedure.yaml")
	t.Setenv("VECTOR_REPAIR_ENABLED", "true")
	t.Setenv("VECTOR_REPAIR_INTERVAL", "30s")
	t.Setenv("VECTOR_REPAIR_MAX_TASKS_PER_PASS", "20")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")
}

func TestLoadSuccessWithDefaults(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("DATABASE_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ConfigPath != "config/config.yaml" {
		t.Fatalf("expected default config path, got %q", cfg.ConfigPath)
	}
	if cfg.ServerPort != "8080" {
		t.Fatalf("expected default port 8080, got %q", cfg.ServerPort)
	}
	if cfg.DatabaseURL != "postgres://legal:legal@localhost:5433/legal_rag?sslmode=disable" {
		t.Fatalf("expected Postgres DSN built from env, got %q", cfg.DatabaseURL)
	}
	if cfg.IngestChunkSize != 800 {
		t.Fatalf("expected default chunk size 800, got %d", cfg.IngestChunkSize)
	}
	if cfg.VectorRepair.Interval != 30*time.Second {
		t.Fatalf("expected default repair interval 30s, got %s", cfg.VectorRepair.Interval)
	}
	if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "http://localhost:5173" {
		t.Fatalf("expected default CORS origin localhost:5173, got %#v", cfg.CORSAllowedOrigins)
	}
}

func TestLoadParsesCORSAllowedOrigins(t *testing.T) {
	setRequiredEnv(t)
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

func TestLoadPrefersDatabaseURLOverride(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("DATABASE_URL", "postgres://override")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DatabaseURL != "postgres://override" {
		t.Fatalf("expected DATABASE_URL override, got %q", cfg.DatabaseURL)
	}
}

func TestLoadReadsDotEnvWhenEnvironmentIsEmpty(t *testing.T) {
	tempDir := t.TempDir()
	content := "" +
		"CONFIG_PATH=config/config.yaml\n" +
		"SERVER_HOST=0.0.0.0\n" +
		"JWT_SECRET=jwt-from-file\n" +
		"ADMIN_BOOTSTRAP_PASSWORD=admin-from-file\n" +
		"PORT=8080\n" +
		"POSTGRES_HOST=localhost\n" +
		"POSTGRES_PORT=5433\n" +
		"POSTGRES_USER=legal\n" +
		"POSTGRES_PASSWORD=legal\n" +
		"POSTGRES_DB=legal_rag\n" +
		"POSTGRES_SSLMODE=disable\n" +
		"QDRANT_COLLECTION=legal_chunks\n" +
		"OPENAI_API_KEY=file-openai-key\n" +
		"QDRANT_HOST=qdrant\n" +
		"QDRANT_PORT=6333\n" +
		"OPENAI_EMBEDDINGS_MODEL=text-embedding-3-small\n" +
		"OPENAI_CHAT_MODEL=gpt-4.1-mini\n" +
		"STORAGE_ROOT_DIR=./data/uploads\n" +
		"INGEST_DEFAULT_SEGMENTER=free_text\n" +
		"INGEST_CHUNK_SIZE=800\n" +
		"INGEST_CHUNK_OVERLAP=100\n" +
		"GUARD_PROMPT_PATH=config/prompts/guard.yaml\n" +
		"TONE_DEFAULT_PROMPT_PATH=config/prompts/tone_default.yaml\n" +
		"TONE_ACADEMIC_PROMPT_PATH=config/prompts/tone_academic.yaml\n" +
		"TONE_PROCEDURE_PROMPT_PATH=config/prompts/tone_procedure.yaml\n" +
		"VECTOR_REPAIR_ENABLED=true\n" +
		"VECTOR_REPAIR_INTERVAL=30s\n" +
		"VECTOR_REPAIR_MAX_TASKS_PER_PASS=20\n" +
		"CORS_ALLOWED_ORIGINS=http://localhost:5173\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte(content), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.JWTSecret != "jwt-from-file" {
		t.Fatalf("expected JWT secret from .env, got %q", cfg.JWTSecret)
	}
	if cfg.DatabaseURL != "postgres://legal:legal@localhost:5433/legal_rag?sslmode=disable" {
		t.Fatalf("expected Postgres DSN from .env components, got %q", cfg.DatabaseURL)
	}
}

func TestRenderAppConfigWritesDerivedConfigYAML(t *testing.T) {
	cfg := &Config{
		ConfigPath:             filepath.Join(t.TempDir(), "config", "config.yaml"),
		ServerHost:             "0.0.0.0",
		ServerPort:             "8080",
		DatabaseURL:            "postgres://legal",
		QdrantURL:              "http://qdrant:6333",
		QdrantCollection:       "legal_chunks",
		OpenAIAPIKey:           "key",
		OpenAIEmbeddingsModel:  "text-embedding-3-small",
		OpenAIChatModel:        "gpt-4.1-mini",
		StorageRootDir:         "/app/data/uploads",
		IngestDefaultSegmenter: "free_text",
		IngestChunkSize:        800,
		IngestChunkOverlap:     100,
		GuardPromptPath:        "config/prompts/guard.yaml",
		ToneDefaultPath:        "config/prompts/tone_default.yaml",
		ToneAcademicPath:       "config/prompts/tone_academic.yaml",
		ToneProcedurePath:      "config/prompts/tone_procedure.yaml",
		VectorRepair: VectorRepairConfig{
			Enabled:         true,
			Interval:        30 * time.Second,
			MaxTasksPerPass: 20,
		},
	}

	if err := RenderAppConfig(cfg); err != nil {
		t.Fatalf("render config: %v", err)
	}

	data, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		t.Fatalf("read rendered config: %v", err)
	}
	text := string(data)
	for _, fragment := range []string{
		`dsn: "postgres://legal"`,
		`url: "http://qdrant:6333"`,
		`root_dir: "/app/data/uploads"`,
		`interval: 30s`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("rendered config missing %q:\n%s", fragment, text)
		}
	}
}

func TestLoadFailsWhenJWTSecretMissing(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	t.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "admin-pass")
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_USER", "legal")
	t.Setenv("POSTGRES_PASSWORD", "legal")
	t.Setenv("POSTGRES_DB", "legal_rag")
	t.Setenv("POSTGRES_SSLMODE", "disable")
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

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
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_USER", "legal")
	t.Setenv("POSTGRES_PASSWORD", "legal")
	t.Setenv("POSTGRES_DB", "legal_rag")
	t.Setenv("POSTGRES_SSLMODE", "disable")
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "missing required environment variable ADMIN_BOOTSTRAP_PASSWORD" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadFailsWhenPortInvalid(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("PORT", "abc")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadFailsWhenRepairIntervalInvalid(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("VECTOR_REPAIR_INTERVAL", "tomorrow")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}
