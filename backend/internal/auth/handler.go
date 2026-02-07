package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/config"
	"github.com/JoshBaneyCS/stocks-web/backend/internal/models"
)

// Handler holds auth-related HTTP handlers.
type Handler struct {
	pool *pgxpool.Pool
	cfg  *config.Config
}

// NewHandler creates a new auth handler.
func NewHandler(pool *pgxpool.Pool, cfg *config.Config) *Handler {
	return &Handler{pool: pool, cfg: cfg}
}

// Signup handles POST /api/auth/signup.
// Requires a valid, active referral code.
func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req models.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.ReferralCode = strings.TrimSpace(req.ReferralCode)

	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" || req.ReferralCode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "all fields are required: email, password, first_name, last_name, referral_code"})
		return
	}
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
		return
	}
	if !strings.Contains(req.Email, "@") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email address"})
		return
	}

	ctx := r.Context()

	// Begin transaction
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		slog.Error("signup: begin tx", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	defer tx.Rollback(ctx)

	// Validate referral code (with row lock)
	var codeID int
	var codeStatus string
	var usageLimit *int
	var usedCount int
	err = tx.QueryRow(ctx,
		`SELECT id, status, usage_limit, used_count
		 FROM referral_codes
		 WHERE code = $1
		 FOR UPDATE`,
		req.ReferralCode,
	).Scan(&codeID, &codeStatus, &usageLimit, &usedCount)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid referral code"})
		return
	}
	if err != nil {
		slog.Error("signup: query referral code", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if codeStatus != "active" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "referral code is no longer active"})
		return
	}
	if usageLimit != nil && usedCount >= *usageLimit {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "referral code has reached its usage limit"})
		return
	}

	// Check for existing user
	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, req.Email).Scan(&exists)
	if err != nil {
		slog.Error("signup: check existing user", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if exists {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
		return
	}

	// Hash password
	hash, err := HashPassword(req.Password)
	if err != nil {
		slog.Error("signup: hash password", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	// Insert user
	var user models.User
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, first_name, last_name, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, NOW(), NOW())
		 RETURNING id, email, first_name, last_name, created_at, updated_at`,
		req.Email, hash, req.FirstName, req.LastName,
	).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		slog.Error("signup: insert user", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	// Increment referral code usage
	_, err = tx.Exec(ctx, `UPDATE referral_codes SET used_count = used_count + 1 WHERE id = $1`, codeID)
	if err != nil {
		slog.Error("signup: increment referral", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		slog.Error("signup: commit tx", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	// Generate tokens
	accessToken, expiresAt, err := GenerateAccessToken(user.ID, user.Email, h.cfg)
	if err != nil {
		slog.Error("signup: generate access token", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	refreshToken, err := h.createRefreshToken(ctx, user.ID)
	if err != nil {
		slog.Error("signup: generate refresh token", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	// Set httpOnly cookies
	h.setAuthCookies(w, accessToken, refreshToken, expiresAt)

	slog.Info("user signed up", "user_id", user.ID, "email", user.Email)
	writeJSON(w, http.StatusCreated, models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         user.ToPublic(),
	})
}

// Login handles POST /api/auth/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
		return
	}

	ctx := r.Context()

	var user models.User
	err := h.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, first_name, last_name, created_at, updated_at
		 FROM users WHERE email = $1`,
		req.Email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.FirstName, &user.LastName, &user.CreatedAt, &user.UpdatedAt)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}
	if err != nil {
		slog.Error("login: query user", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	valid, err := VerifyPassword(req.Password, user.PasswordHash)
	if err != nil {
		slog.Error("login: verify password", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if !valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}

	accessToken, expiresAt, err := GenerateAccessToken(user.ID, user.Email, h.cfg)
	if err != nil {
		slog.Error("login: generate access token", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	refreshToken, err := h.createRefreshToken(ctx, user.ID)
	if err != nil {
		slog.Error("login: generate refresh token", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	h.setAuthCookies(w, accessToken, refreshToken, expiresAt)

	slog.Info("user logged in", "user_id", user.ID, "email", user.Email)
	writeJSON(w, http.StatusOK, models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         user.ToPublic(),
	})
}

// Logout handles POST /api/auth/logout.
// Invalidates refresh token and clears cookies.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// Try to delete the refresh token from DB
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		tokenHash := hashToken(cookie.Value)
		_, _ = h.pool.Exec(r.Context(), `DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash)
	}

	// Clear cookies
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// Me handles GET /api/auth/me (requires auth middleware).
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var user models.User
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, email, first_name, last_name, created_at, updated_at
		 FROM users WHERE id = $1`,
		userID,
	).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		slog.Error("me: query user", "error", err, "user_id", userID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, user.ToPublic())
}

// CreateReferralCode handles POST /api/admin/referral-codes (admin-only).
func (h *Handler) CreateReferralCode(w http.ResponseWriter, r *http.Request) {
	var req models.CreateReferralCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	req.Code = strings.TrimSpace(req.Code)
	if req.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "code is required"})
		return
	}

	var code models.ReferralCode
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO referral_codes (code, status, usage_limit, used_count, created_at)
		 VALUES ($1, 'active', $2, 0, NOW())
		 RETURNING id, code, status, usage_limit, used_count, created_at`,
		req.Code, req.UsageLimit,
	).Scan(&code.ID, &code.Code, &code.Status, &code.UsageLimit, &code.UsedCount, &code.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "referral code already exists"})
			return
		}
		slog.Error("create referral code", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	slog.Info("referral code created", "code", code.Code, "usage_limit", code.UsageLimit)
	writeJSON(w, http.StatusCreated, code)
}

// ─── Helpers ─────────────────────────────────────────────────────────

// createRefreshToken generates a random refresh token, stores its hash, and returns the raw token.
func (h *Handler) createRefreshToken(ctx context.Context, userID int) (string, error) {
	// Generate 32 random bytes
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	token := hex.EncodeToString(raw)
	tokenHash := hashToken(token)
	expiresAt := time.Now().Add(h.cfg.RefreshTokenExpiry)

	// Clean up old tokens for this user (keep max 5)
	_, _ = h.pool.Exec(ctx,
		`DELETE FROM refresh_tokens
		 WHERE user_id = $1
		   AND id NOT IN (
		     SELECT id FROM refresh_tokens
		     WHERE user_id = $1
		     ORDER BY created_at DESC
		     LIMIT 4
		   )`,
		userID,
	)

	// Insert new token
	_, err := h.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at)
		 VALUES ($1, $2, $3, NOW())`,
		userID, tokenHash, expiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("insert refresh token: %w", err)
	}

	return token, nil
}

// setAuthCookies sets httpOnly secure cookies for both tokens.
func (h *Handler) setAuthCookies(w http.ResponseWriter, accessToken, refreshToken string, accessExpiry time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		Expires:  accessExpiry,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		Expires:  time.Now().Add(h.cfg.RefreshTokenExpiry),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// hashToken returns the SHA-256 hex digest of a token string.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
