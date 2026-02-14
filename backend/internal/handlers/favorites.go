package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/auth"
	"github.com/JoshBaneyCS/stocks-web/backend/internal/models"
)

// FavoritesHandler handles user favorites endpoints.
type FavoritesHandler struct {
	AuthDB   *pgxpool.Pool
	MarketDB *pgxpool.Pool
}

// NewFavoritesHandler creates a new FavoritesHandler.
func NewFavoritesHandler(authDB, marketDB *pgxpool.Pool) *FavoritesHandler {
	return &FavoritesHandler{AuthDB: authDB, MarketDB: marketDB}
}

// Get returns the authenticated user's favorite instruments with latest prices.
func (h *FavoritesHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	ctx := r.Context()

	// Step 1: Get favorite instrument IDs from auth DB
	favIDs, err := fetchFavoriteIDs(ctx, h.AuthDB, userID)
	if err != nil {
		slog.Error("failed to query favorite IDs", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Step 2: Get instrument details from market DB
	items, err := fetchInstrumentsByIDs(ctx, h.MarketDB, favIDs)
	if err != nil {
		slog.Error("failed to query favorite instruments", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, items)
}

// Update performs a batch replace of the user's favorites.
// It deletes all existing favorites for the user and inserts the new set
// within a single transaction.
func (h *FavoritesHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req models.FavoritesUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()

	tx, err := h.AuthDB.Begin(ctx)
	if err != nil {
		slog.Error("failed to begin transaction", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer tx.Rollback(ctx)

	// Delete all existing favorites for this user
	_, err = tx.Exec(ctx, `DELETE FROM user_favorites WHERE user_id = $1`, userID)
	if err != nil {
		slog.Error("failed to delete existing favorites", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Insert new favorites
	for _, instrumentID := range req.InstrumentIDs {
		_, err = tx.Exec(ctx,
			`INSERT INTO user_favorites (user_id, instrument_id) VALUES ($1, $2)
			 ON CONFLICT DO NOTHING`,
			userID, instrumentID,
		)
		if err != nil {
			slog.Error("failed to insert favorite", "error", err, "instrument_id", instrumentID)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("failed to commit transaction", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "favorites updated",
		"count":   len(req.InstrumentIDs),
	})
}
