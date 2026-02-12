package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds all configuration values for the server.
type Config struct {
	DatabaseURL        string
	Port               string
	JWTSecret          string
	RefreshSecret      string
	AdminSecret        string
	CORSOrigin         string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
}

// Load reads configuration from environment variables.
// It panics if any required variable is missing.
func Load() *Config {
	cfg := &Config{
		DatabaseURL:   requireEnv("DATABASE_URL"),
		JWTSecret:     requireEnv("JWT_SECRET"),
		RefreshSecret: requireEnv("REFRESH_SECRET"),
		AdminSecret:   requireEnv("ADMIN_SECRET"),
		Port:          getEnvOrDefault("PORT", "8080"),
		CORSOrigin:    getEnvOrDefault("CORS_ORIGIN", "*"),
	}

	var err error
	cfg.AccessTokenExpiry, err = parseDuration(getEnvOrDefault("ACCESS_TOKEN_EXPIRY", "15m"))
	if err != nil {
		panic(fmt.Sprintf("invalid ACCESS_TOKEN_EXPIRY: %v", err))
	}

	cfg.RefreshTokenExpiry, err = parseDuration(getEnvOrDefault("REFRESH_TOKEN_EXPIRY", "7d"))
	if err != nil {
		panic(fmt.Sprintf("invalid REFRESH_TOKEN_EXPIRY: %v", err))
	}

	return cfg
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return val
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// parseDuration parses duration strings like "15m", "1h", "7d".
// Extends Go's time.ParseDuration to support "d" suffix for days.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(numStr, "%d", &days); err != nil {
			return 0, fmt.Errorf("cannot parse %q as duration: %w", s, err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
