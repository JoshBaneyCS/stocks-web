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
	DB *pgxpool.Pool
}

// NewFavoritesHandler creates a new FavoritesHandler.
func NewFavoritesHandler(db *pgxpool.Pool) *FavoritesHandler {
	return &FavoritesHandler{DB: db}
}

// Get returns the authenticated user's favorite instruments with latest prices.
func (h *FavoritesHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	ctx := r.Context()

	rows, err := h.DB.Query(ctx, `
		SELECT i.id, i.symbol, i.name, i.exchange, i.currency, i.country, i.asset_class, i.is_active,
		       im.last_price, im.market_cap, cp.sector, cp.industry
		FROM user_favorites uf
		JOIN instruments i ON i.id = uf.instrument_id
		LEFT JOIN instrument_metrics im ON im.instrument_id = i.id
		LEFT JOIN company_profiles cp ON cp.instrument_id = i.id
		WHERE uf.user_id = $1
		ORDER BY i.symbol ASC
	`, userID)
	if err != nil {
		slog.Error("failed to query favorites", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	items := make([]models.InstrumentListItem, 0)
	for rows.Next() {
		var item models.InstrumentListItem
		if err := rows.Scan(
			&item.ID, &item.Symbol, &item.Name, &item.Exchange, &item.Currency,
			&item.Country, &item.AssetClass, &item.IsActive,
			&item.LastPrice, &item.MarketCap, &item.Sector, &item.Industry,
		); err != nil {
			slog.Error("failed to scan favorite row", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		item.IsFavorite = true
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		slog.Error("row iteration error", "error", err)
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

	tx, err := h.DB.Begin(ctx)
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
