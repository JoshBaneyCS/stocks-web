package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	Port string

	// Database
	DatabaseURL string
	DBHost      string // extracted for logging only (no secrets)
	DBPoolMax   int

	// Auth
	JWTSecret          string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration

	// Admin
	AdminSecret string

	// CORS
	CORSOrigin string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	cfg := &Config{
		Port:               envOrDefault("PORT", "8080"),
		DatabaseURL:        envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/stock-data?sslmode=disable"),
		DBPoolMax:          envOrDefaultInt("DB_POOL_MAX", 20),
		JWTSecret:          envOrDefault("JWT_SECRET", "change-me-in-production"),
		AccessTokenExpiry:  envOrDefaultDuration("ACCESS_TOKEN_EXPIRY", 15*time.Minute),
		RefreshTokenExpiry: envOrDefaultDuration("REFRESH_TOKEN_EXPIRY", 7*24*time.Hour),
		AdminSecret:        envOrDefault("ADMIN_SECRET", ""),
		CORSOrigin:         envOrDefault("CORS_ORIGIN", "https://stocks.baneynet.net"),
	}

	// Extract host for safe logging
	cfg.DBHost = extractHost(cfg.DatabaseURL)

	return cfg
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envOrDefaultDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

// extractHost pulls just the host portion from a Postgres URL for safe logging.
func extractHost(dsn string) string {
	// Naive extraction: find @ and next / or :
	start := 0
	for i, c := range dsn {
		if c == '@' {
			start = i + 1
			break
		}
	}
	if start == 0 {
		return "unknown"
	}
	end := len(dsn)
	for i := start; i < len(dsn); i++ {
		if dsn[i] == ':' || dsn[i] == '/' || dsn[i] == '?' {
			end = i
			break
		}
	}
	if start >= end {
		return "unknown"
	}
	return dsn[start:end]
}
