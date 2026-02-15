package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/config"
	"github.com/JoshBaneyCS/stocks-web/backend/internal/models"
)

// Handler handles all authentication-related HTTP endpoints.
type Handler struct {
	DB  *pgxpool.Pool
	Cfg *config.Config
}

// NewHandler creates a new auth Handler.
func NewHandler(db *pgxpool.Pool, cfg *config.Config) *Handler {
	return &Handler{DB: db, Cfg: cfg}
}

// Signup handles user registration with referral code validation.
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

	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email, password, first_name, and last_name are required"})
		return
	}
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
		return
	}
	if req.ReferralCode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "referral_code is required"})
		return
	}

	ctx := r.Context()

	// Start a transaction for referral code check + user creation
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		slog.Error("failed to begin transaction", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	defer tx.Rollback(ctx)

	// Validate referral code
	var codeActive bool
	var usageLimit *int
	var usedCount int
	err = tx.QueryRow(ctx,
		`SELECT is_active, usage_limit, used_count FROM referral_codes WHERE code = $1`,
		req.ReferralCode,
	).Scan(&codeActive, &usageLimit, &usedCount)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid referral code"})
		} else {
			slog.Error("failed to query referral code", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		return
	}
	if !codeActive {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "referral code is no longer active"})
		return
	}
	if usageLimit != nil && usedCount >= *usageLimit {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "referral code has reached its usage limit"})
		return
	}

	// Hash password
	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Insert user
	var user models.User
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, first_name, last_name)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, email, password_hash, first_name, last_name, created_at, updated_at`,
		req.Email, passwordHash, req.FirstName, req.LastName,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.FirstName, &user.LastName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "an account with this email already exists"})
		} else {
			slog.Error("failed to insert user", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		return
	}

	// Increment referral code usage
	_, err = tx.Exec(ctx,
		`UPDATE referral_codes SET used_count = used_count + 1, updated_at = NOW() WHERE code = $1`,
		req.ReferralCode,
	)
	if err != nil {
		slog.Error("failed to increment referral code usage", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("failed to commit transaction", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Generate tokens
	accessToken, refreshToken, err := h.generateTokens(ctx, user.ID)
	if err != nil {
		slog.Error("failed to generate tokens", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	h.setTokenCookies(w, accessToken, refreshToken)

	writeJSON(w, http.StatusCreated, models.AuthResponse{
		AccessToken: accessToken,
		User:        user.Public(),
	})
}

// Login authenticates a user with email and password.
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
	err := h.DB.QueryRow(ctx,
		`SELECT id, email, password_hash, first_name, last_name, created_at, updated_at
		 FROM users WHERE email = $1`,
		req.Email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.FirstName, &user.LastName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		} else {
			slog.Error("failed to query user", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		return
	}

	valid, err := VerifyPassword(user.PasswordHash, req.Password)
	if err != nil {
		slog.Error("failed to verify password", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if !valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}

	accessToken, refreshToken, err := h.generateTokens(ctx, user.ID)
	if err != nil {
		slog.Error("failed to generate tokens", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	h.setTokenCookies(w, accessToken, refreshToken)

	writeJSON(w, http.StatusOK, models.AuthResponse{
		AccessToken: accessToken,
		User:        user.Public(),
	})
}

// Logout invalidates the refresh token and clears cookies.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Try to delete the refresh token from DB
	if cookie, err := r.Cookie("refresh_token"); err == nil && cookie.Value != "" {
		tokenHash := hashToken(cookie.Value)
		_, delErr := h.DB.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash)
		if delErr != nil {
			slog.Error("failed to delete refresh token", "error", delErr)
		}
	}

	// Clear cookies regardless
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/auth/refresh",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// Me returns the currently authenticated user.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var user models.User
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, email, password_hash, first_name, last_name, created_at, updated_at
		 FROM users WHERE id = $1`,
		userID,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.FirstName, &user.LastName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		} else {
			slog.Error("failed to query user", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		return
	}

	writeJSON(w, http.StatusOK, user.Public())
}

// RefreshToken issues a new access token and rotates the refresh token.
func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get refresh token from cookie first, then from body
	var refreshTokenStr string
	if cookie, err := r.Cookie("refresh_token"); err == nil && cookie.Value != "" {
		refreshTokenStr = cookie.Value
	} else {
		var req models.RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			refreshTokenStr = req.RefreshToken
		}
	}

	if refreshTokenStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "refresh token is required"})
		return
	}

	// Validate the refresh token JWT
	token, err := jwt.Parse(refreshTokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(h.Cfg.RefreshSecret), nil
	})
	if err != nil || !token.Valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired refresh token"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token claims"})
		return
	}

	userID, err := claims.GetSubject()
	if err != nil || userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token subject"})
		return
	}

	// Verify token exists in DB (not revoked); accept recently-rotated tokens (30s grace)
	tokenHash := hashToken(refreshTokenStr)
	var dbUserID string
	var rotatedAt *time.Time
	err = h.DB.QueryRow(ctx,
		`SELECT user_id, rotated_at FROM refresh_tokens
		 WHERE token_hash = $1
		 AND expires_at > NOW()
		 AND (rotated_at IS NULL OR rotated_at > NOW() - INTERVAL '30 seconds')`,
		tokenHash,
	).Scan(&dbUserID, &rotatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "refresh token not found or expired"})
		} else {
			slog.Error("failed to query refresh token", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		return
	}

	// Soft-rotate: mark as rotated instead of deleting (first request wins)
	if rotatedAt == nil {
		_, _ = h.DB.Exec(ctx,
			`UPDATE refresh_tokens SET rotated_at = NOW() WHERE token_hash = $1 AND rotated_at IS NULL`,
			tokenHash)
	}

	// Issue new tokens
	accessToken, newRefreshToken, err := h.generateTokens(ctx, userID)
	if err != nil {
		slog.Error("failed to generate tokens", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	h.setTokenCookies(w, accessToken, newRefreshToken)

	writeJSON(w, http.StatusOK, map[string]string{"access_token": accessToken})
}

// generateTokens creates an access JWT and a refresh JWT, storing the refresh
// token hash in the database.
func (h *Handler) generateTokens(ctx context.Context, userID string) (string, string, error) {
	now := time.Now()

	// Access token
	accessClaims := jwt.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"exp": now.Add(h.Cfg.AccessTokenExpiry).Unix(),
		"typ": "access",
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenStr, err := accessToken.SignedString([]byte(h.Cfg.JWTSecret))
	if err != nil {
		return "", "", fmt.Errorf("signing access token: %w", err)
	}

	// Refresh token
	refreshExpiry := now.Add(h.Cfg.RefreshTokenExpiry)
	refreshClaims := jwt.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"exp": refreshExpiry.Unix(),
		"typ": "refresh",
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenStr, err := refreshToken.SignedString([]byte(h.Cfg.RefreshSecret))
	if err != nil {
		return "", "", fmt.Errorf("signing refresh token: %w", err)
	}

	// Store hashed refresh token in DB
	tokenHash := hashToken(refreshTokenStr)
	_, err = h.DB.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, refreshExpiry,
	)
	if err != nil {
		return "", "", fmt.Errorf("storing refresh token: %w", err)
	}

	// Clean up expired and stale rotated tokens for this user (best effort)
	_, _ = h.DB.Exec(ctx,
		`DELETE FROM refresh_tokens
		 WHERE user_id = $1
		 AND (expires_at < NOW() OR (rotated_at IS NOT NULL AND rotated_at < NOW() - INTERVAL '30 seconds'))`,
		userID,
	)

	return accessTokenStr, refreshTokenStr, nil
}

// setTokenCookies sets httpOnly cookies for both access and refresh tokens.
func (h *Handler) setTokenCookies(w http.ResponseWriter, accessToken, refreshToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		MaxAge:   int(h.Cfg.AccessTokenExpiry.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/api/auth/refresh",
		MaxAge:   int(h.Cfg.RefreshTokenExpiry.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// hashToken returns a hex-encoded SHA-256 hash of the token string.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// writeJSON is a helper that writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}
