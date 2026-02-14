package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type apiKeyContextKey string

const apiKeyIDKey apiKeyContextKey = "api_key_id"

// RequireAPIKey returns middleware that validates API keys from the request.
// Keys are extracted from (in order):
//  1. X-API-Key header
//  2. Authorization: Bearer sk_... header
//  3. api_key query parameter
func RequireAPIKey(authDB *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keyStr := extractAPIKey(r)
			if keyStr == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"API key required. Pass via X-API-Key header, Authorization: Bearer sk_..., or api_key query param"}`))
				return
			}

			keyHash := HashAPIKey(keyStr)

			var keyUserID string
			var keyID int64
			var expiresAt *time.Time

			err := authDB.QueryRow(r.Context(), `
				SELECT id, user_id, expires_at
				FROM api_keys
				WHERE key_hash = $1 AND is_active = true
			`, keyHash).Scan(&keyID, &keyUserID, &expiresAt)

			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"invalid API key"}`))
				return
			}

			if expiresAt != nil && time.Now().After(*expiresAt) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"API key has expired"}`))
				return
			}

			// Update last_used_at asynchronously
			go func() {
				_, _ = authDB.Exec(context.Background(),
					`UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`, keyID)
			}()

			// Inject user ID and key ID into context
			ctx := context.WithValue(r.Context(), userIDKey, keyUserID)
			ctx = context.WithValue(ctx, apiKeyIDKey, keyID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// APIKeyRateLimit returns middleware that applies per-key rate limiting.
func APIKeyRateLimit(defaultRate int) func(http.Handler) http.Handler {
	rl := &rateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     float64(defaultRate) / 60.0,
		capacity: float64(defaultRate),
	}

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.cleanup()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keyID, ok := r.Context().Value(apiKeyIDKey).(int64)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			key := fmt.Sprintf("apikey:%d", keyID)
			if !rl.allow(key) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"API rate limit exceeded, try again later"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractAPIKey(r *http.Request) string {
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer sk_") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return r.URL.Query().Get("api_key")
}

// HashAPIKey returns the SHA-256 hex digest of an API key.
func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
