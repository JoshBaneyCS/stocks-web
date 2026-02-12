package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/models"
)

// AdminHandler handles admin-only endpoints.
type AdminHandler struct {
	DB          *pgxpool.Pool
	AdminSecret string
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(db *pgxpool.Pool, adminSecret string) *AdminHandler {
	return &AdminHandler{DB: db, AdminSecret: adminSecret}
}

// RequireAdminSecret returns middleware that checks the X-Admin-Secret header.
func (h *AdminHandler) RequireAdminSecret(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := strings.TrimSpace(r.Header.Get("X-Admin-Secret"))
		if secret == "" || secret != h.AdminSecret {
			writeError(w, http.StatusForbidden, "invalid or missing admin secret")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CreateReferralCode creates a new referral code.
// POST /api/admin/referral-codes
func (h *AdminHandler) CreateReferralCode(w http.ResponseWriter, r *http.Request) {
	var req models.CreateReferralCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Code = strings.TrimSpace(req.Code)
	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	ctx := r.Context()

	var result models.ReferralCode
	err := h.DB.QueryRow(ctx, `
		INSERT INTO referral_codes (code, is_active, usage_limit)
		VALUES ($1, true, $2)
		RETURNING code, is_active, usage_limit, used_count, created_at
	`, req.Code, req.UsageLimit).Scan(
		&result.Code, &result.IsActive, &result.UsageLimit, &result.UsedCount, &result.CreatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "referral code already exists")
		} else {
			slog.Error("failed to create referral code", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, result)
}
