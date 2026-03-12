package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ConfigPath             string
	ServerPort             string
	DatabaseURL            string
	JWTSecret              string
	AdminBootstrapPassword string
}

func Load() (*Config, error) {
	cfg := &Config{
		ConfigPath:             strings.TrimSpace(getEnv("CONFIG_PATH", "config/config.yaml")),
		ServerPort:             strings.TrimSpace(getEnv("PORT", "8080")),
		DatabaseURL:            strings.TrimSpace(os.Getenv("DATABASE_URL")),
		JWTSecret:              strings.TrimSpace(os.Getenv("JWT_SECRET")),
		AdminBootstrapPassword: strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_PASSWORD")),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("missing required environment variable JWT_SECRET")
	}
	if cfg.AdminBootstrapPassword == "" {
		return nil, fmt.Errorf("missing required environment variable ADMIN_BOOTSTRAP_PASSWORD")
	}
	if _, err := strconv.Atoi(cfg.ServerPort); err != nil {
		return nil, fmt.Errorf("invalid PORT value %q: must be a valid integer", cfg.ServerPort)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
