package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/auth"
)

// APIKeysHandler handles API key management endpoints.
type APIKeysHandler struct {
	AuthDB *pgxpool.Pool
}

// NewAPIKeysHandler creates a new APIKeysHandler.
func NewAPIKeysHandler(authDB *pgxpool.Pool) *APIKeysHandler {
	return &APIKeysHandler{AuthDB: authDB}
}

type apiKeyResponse struct {
	ID         int64      `json:"id"`
	KeyPrefix  string     `json:"key_prefix"`
	Name       string     `json:"name"`
	RateLimit  int        `json:"rate_limit"`
	IsActive   bool       `json:"is_active"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

type createAPIKeyRequest struct {
	Name      string  `json:"name"`
	RateLimit *int    `json:"rate_limit,omitempty"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

type createAPIKeyResponse struct {
	APIKey   apiKeyResponse `json:"api_key"`
	PlainKey string         `json:"key"`
}

// Create generates a new API key for the authenticated user.
// POST /api/api-keys
func (h *APIKeysHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req createAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	rateLimit := 60
	if req.RateLimit != nil && *req.RateLimit > 0 {
		rateLimit = *req.RateLimit
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid expires_at, use RFC3339 format")
			return
		}
		expiresAt = &t
	}

	// Generate key: sk_live_ + 32 random hex chars
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		slog.Error("failed to generate random bytes", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	plainKey := "sk_live_" + hex.EncodeToString(randomBytes)
	keyPrefix := plainKey[:16]
	keyHash := auth.HashAPIKey(plainKey)

	ctx := r.Context()
	var keyID int64
	var createdAt time.Time
	err := h.AuthDB.QueryRow(ctx, `
		INSERT INTO api_keys (user_id, key_prefix, key_hash, name, rate_limit, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`, userID, keyPrefix, keyHash, req.Name, rateLimit, expiresAt).Scan(&keyID, &createdAt)
	if err != nil {
		slog.Error("failed to create API key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := createAPIKeyResponse{
		APIKey: apiKeyResponse{
			ID:        keyID,
			KeyPrefix: keyPrefix,
			Name:      req.Name,
			RateLimit: rateLimit,
			IsActive:  true,
			ExpiresAt: expiresAt,
			CreatedAt: createdAt,
		},
		PlainKey: plainKey,
	}

	writeJSON(w, http.StatusCreated, resp)
}

// List returns all API keys for the authenticated user (prefix only).
// GET /api/api-keys
func (h *APIKeysHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	rows, err := h.AuthDB.Query(r.Context(), `
		SELECT id, key_prefix, name, rate_limit, is_active, last_used_at, expires_at, created_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		slog.Error("failed to list API keys", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	keys := make([]apiKeyResponse, 0)
	for rows.Next() {
		var k apiKeyResponse
		if err := rows.Scan(
			&k.ID, &k.KeyPrefix, &k.Name, &k.RateLimit,
			&k.IsActive, &k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt,
		); err != nil {
			slog.Error("failed to scan API key", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		slog.Error("row iteration error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, keys)
}

// Revoke deactivates an API key.
// DELETE /api/api-keys/{id}
func (h *APIKeysHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	idStr := chi.URLParam(r, "id")
	keyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid key ID")
		return
	}

	result, err := h.AuthDB.Exec(r.Context(), `
		UPDATE api_keys SET is_active = false, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
	`, keyID, userID)
	if err != nil {
		slog.Error("failed to revoke API key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if result.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "API key not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "API key revoked"})
}
