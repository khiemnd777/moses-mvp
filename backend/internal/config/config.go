package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type VectorRepairConfig struct {
	Enabled         bool
	Interval        time.Duration
	MaxTasksPerPass int
}

type Config struct {
	ConfigPath             string
	ServerHost             string
	ServerPort             string
	DatabaseURL            string
	QdrantURL              string
	QdrantCollection       string
	OpenAIAPIKey           string
	OpenAIEmbeddingsModel  string
	OpenAIChatModel        string
	StorageRootDir         string
	IngestDefaultSegmenter string
	IngestChunkSize        int
	IngestChunkOverlap     int
	GuardPromptPath        string
	ToneDefaultPath        string
	ToneAcademicPath       string
	ToneProcedurePath      string
	CORSAllowedOrigins     []string
	JWTSecret              string
	AdminBootstrapPassword string
	VectorRepair           VectorRepairConfig
}

func Load() (*Config, error) {
	loadDotEnvIfPresent()

	chunkSize, err := getRequiredEnvInt("INGEST_CHUNK_SIZE")
	if err != nil {
		return nil, err
	}
	chunkOverlap, err := getRequiredEnvInt("INGEST_CHUNK_OVERLAP")
	if err != nil {
		return nil, err
	}
	repairInterval, err := getRequiredEnvDuration("VECTOR_REPAIR_INTERVAL")
	if err != nil {
		return nil, err
	}
	repairMaxTasks, err := getRequiredEnvInt("VECTOR_REPAIR_MAX_TASKS_PER_PASS")
	if err != nil {
		return nil, err
	}
	repairEnabled, err := getRequiredEnvBool("VECTOR_REPAIR_ENABLED")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		ConfigPath:             strings.TrimSpace(os.Getenv("CONFIG_PATH")),
		ServerHost:             strings.TrimSpace(os.Getenv("SERVER_HOST")),
		ServerPort:             strings.TrimSpace(os.Getenv("PORT")),
		DatabaseURL:            databaseURL(),
		QdrantURL:              qdrantURL(),
		QdrantCollection:       strings.TrimSpace(os.Getenv("QDRANT_COLLECTION")),
		OpenAIAPIKey:           strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		OpenAIEmbeddingsModel:  strings.TrimSpace(os.Getenv("OPENAI_EMBEDDINGS_MODEL")),
		OpenAIChatModel:        strings.TrimSpace(os.Getenv("OPENAI_CHAT_MODEL")),
		StorageRootDir:         strings.TrimSpace(os.Getenv("STORAGE_ROOT_DIR")),
		IngestDefaultSegmenter: strings.TrimSpace(os.Getenv("INGEST_DEFAULT_SEGMENTER")),
		GuardPromptPath:        strings.TrimSpace(os.Getenv("GUARD_PROMPT_PATH")),
		ToneDefaultPath:        strings.TrimSpace(os.Getenv("TONE_DEFAULT_PROMPT_PATH")),
		ToneAcademicPath:       strings.TrimSpace(os.Getenv("TONE_ACADEMIC_PROMPT_PATH")),
		ToneProcedurePath:      strings.TrimSpace(os.Getenv("TONE_PROCEDURE_PROMPT_PATH")),
		CORSAllowedOrigins:     parseRequiredCSVEnv("CORS_ALLOWED_ORIGINS"),
		JWTSecret:              strings.TrimSpace(os.Getenv("JWT_SECRET")),
		AdminBootstrapPassword: strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_PASSWORD")),
		IngestChunkSize:        chunkSize,
		IngestChunkOverlap:     chunkOverlap,
		VectorRepair: VectorRepairConfig{
			Enabled:         repairEnabled,
			Interval:        repairInterval,
			MaxTasksPerPass: repairMaxTasks,
		},
	}

	if cfg.ConfigPath == "" {
		return nil, fmt.Errorf("missing required environment variable CONFIG_PATH")
	}
	if cfg.ServerHost == "" {
		return nil, fmt.Errorf("missing required environment variable SERVER_HOST")
	}
	if cfg.ServerPort == "" {
		return nil, fmt.Errorf("missing required environment variable PORT")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("missing required environment variable JWT_SECRET")
	}
	if cfg.AdminBootstrapPassword == "" {
		return nil, fmt.Errorf("missing required environment variable ADMIN_BOOTSTRAP_PASSWORD")
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("missing required Postgres connection configuration")
	}
	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("missing required environment variable OPENAI_API_KEY")
	}
	if cfg.QdrantURL == "" {
		return nil, fmt.Errorf("missing required Qdrant connection configuration")
	}
	if cfg.QdrantCollection == "" {
		return nil, fmt.Errorf("missing required environment variable QDRANT_COLLECTION")
	}
	if cfg.OpenAIEmbeddingsModel == "" {
		return nil, fmt.Errorf("missing required environment variable OPENAI_EMBEDDINGS_MODEL")
	}
	if cfg.OpenAIChatModel == "" {
		return nil, fmt.Errorf("missing required environment variable OPENAI_CHAT_MODEL")
	}
	if cfg.StorageRootDir == "" {
		return nil, fmt.Errorf("missing required environment variable STORAGE_ROOT_DIR")
	}
	if cfg.IngestDefaultSegmenter == "" {
		return nil, fmt.Errorf("missing required environment variable INGEST_DEFAULT_SEGMENTER")
	}
	if cfg.GuardPromptPath == "" {
		return nil, fmt.Errorf("missing required environment variable GUARD_PROMPT_PATH")
	}
	if cfg.ToneDefaultPath == "" {
		return nil, fmt.Errorf("missing required environment variable TONE_DEFAULT_PROMPT_PATH")
	}
	if cfg.ToneAcademicPath == "" {
		return nil, fmt.Errorf("missing required environment variable TONE_ACADEMIC_PROMPT_PATH")
	}
	if cfg.ToneProcedurePath == "" {
		return nil, fmt.Errorf("missing required environment variable TONE_PROCEDURE_PROMPT_PATH")
	}
	if _, err := strconv.Atoi(cfg.ServerPort); err != nil {
		return nil, fmt.Errorf("invalid PORT value %q: must be a valid integer", cfg.ServerPort)
	}

	return cfg, nil
}

func RenderAppConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.ConfigPath), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	content := fmt.Sprintf(`server:
  host: %q
  port: %s
postgres:
  dsn: %q
qdrant:
  url: %q
  collection: %q
openai:
  api_key: %q
  embeddings_model: %q
  chat_model: %q
storage:
  root_dir: %q
ingest:
  default_segmenter: %q
  chunk_size: %d
  chunk_overlap: %d
prompts:
  guard: %q
  tone_default: %q
  tone_academic: %q
  tone_procedure: %q
vector_repair:
  enabled: %t
  interval: %s
  max_tasks_per_pass: %d
`,
		cfg.ServerHost,
		cfg.ServerPort,
		cfg.DatabaseURL,
		cfg.QdrantURL,
		cfg.QdrantCollection,
		cfg.OpenAIAPIKey,
		cfg.OpenAIEmbeddingsModel,
		cfg.OpenAIChatModel,
		cfg.StorageRootDir,
		cfg.IngestDefaultSegmenter,
		cfg.IngestChunkSize,
		cfg.IngestChunkOverlap,
		cfg.GuardPromptPath,
		cfg.ToneDefaultPath,
		cfg.ToneAcademicPath,
		cfg.ToneProcedurePath,
		cfg.VectorRepair.Enabled,
		cfg.VectorRepair.Interval,
		cfg.VectorRepair.MaxTasksPerPass,
	)

	if err := os.WriteFile(cfg.ConfigPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write rendered config: %w", err)
	}
	return nil
}

func databaseURL() string {
	if value := strings.TrimSpace(os.Getenv("DATABASE_URL")); value != "" {
		return value
	}
	host := strings.TrimSpace(os.Getenv("POSTGRES_HOST"))
	port := strings.TrimSpace(os.Getenv("POSTGRES_PORT"))
	user := strings.TrimSpace(os.Getenv("POSTGRES_USER"))
	password := strings.TrimSpace(os.Getenv("POSTGRES_PASSWORD"))
	db := strings.TrimSpace(os.Getenv("POSTGRES_DB"))
	sslmode := strings.TrimSpace(os.Getenv("POSTGRES_SSLMODE"))
	if host == "" || port == "" || user == "" || password == "" || db == "" || sslmode == "" {
		return ""
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		url.QueryEscape(user),
		url.QueryEscape(password),
		host,
		port,
		db,
		url.QueryEscape(sslmode),
	)
}

func qdrantURL() string {
	host := strings.TrimSpace(os.Getenv("QDRANT_HOST"))
	port := strings.TrimSpace(os.Getenv("QDRANT_PORT"))
	if host == "" || port == "" {
		return ""
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

func getRequiredEnvInt(key string) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return 0, fmt.Errorf("missing required environment variable %s", key)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: must be a valid integer", key, value)
	}
	return parsed, nil
}

func getRequiredEnvBool(key string) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return false, fmt.Errorf("missing required environment variable %s", key)
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid %s value %q: must be true or false", key, value)
	}
	return parsed, nil
}

func getRequiredEnvDuration(key string) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return 0, fmt.Errorf("missing required environment variable %s", key)
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", key, value, err)
	}
	return parsed, nil
}

func parseRequiredCSVEnv(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		origins = append(origins, trimmed)
	}
	return origins
}

func loadDotEnvIfPresent() {
	path := filepath.Clean(".env")
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		_ = os.Setenv(key, value)
	}
}
