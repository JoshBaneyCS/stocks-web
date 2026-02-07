package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/auth"
	"github.com/JoshBaneyCS/stocks-web/backend/internal/models"
)

// FavoritesHandler handles user favorites management.
type FavoritesHandler struct {
	pool *pgxpool.Pool
}

// NewFavoritesHandler creates a new favorites handler.
func NewFavoritesHandler(pool *pgxpool.Pool) *FavoritesHandler {
	return &FavoritesHandler{pool: pool}
}

// FavoriteItem is the response shape for a single favorite.
type FavoriteItem struct {
	CompanyID int      `json:"company_id"`
	Symbol    string   `json:"symbol"`
	Name      *string  `json:"name"`
	Exchange  *string  `json:"exchange"`
	Sector    *string  `json:"sector"`
	Industry  *string  `json:"industry"`
	MarketCap *float64 `json:"market_cap"`
}

// Get handles GET /api/favorites
// Returns the authenticated user's favorite stocks with company info.
func (h *FavoritesHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	rows, err := h.pool.Query(ctx,
		`SELECT c.id, c.symbol, c.name, c.exchange, c.sector, c.industry, c.market_cap
		 FROM user_favorites uf
		 JOIN companies c ON c.id = uf.company_id
		 WHERE uf.user_id = $1
		 ORDER BY c.symbol ASC`,
		userID,
	)
	if err != nil {
		slog.Error("favorites.get: query", "error", err, "user_id", userID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	defer rows.Close()

	favorites := make([]FavoriteItem, 0)
	for rows.Next() {
		var f FavoriteItem
		if err := rows.Scan(&f.CompanyID, &f.Symbol, &f.Name, &f.Exchange, &f.Sector, &f.Industry, &f.MarketCap); err != nil {
			slog.Error("favorites.get: scan row", "error", err)
			continue
		}
		favorites = append(favorites, f)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":     len(favorites),
		"favorites": favorites,
	})
}

// Update handles PUT /api/favorites
// Batch replaces all favorites for the authenticated user.
// Accepts {"company_ids": [1, 5, 12, ...]}
// An empty array clears all favorites.
func (h *FavoritesHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req models.FavoritesUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Limit max favorites to prevent abuse
	if len(req.CompanyIDs) > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "maximum 100 favorites allowed"})
		return
	}

	// Transaction: delete all existing, then insert new set
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		slog.Error("favorites.update: begin tx", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	defer tx.Rollback(ctx)

	// Clear existing favorites
	_, err = tx.Exec(ctx, `DELETE FROM user_favorites WHERE user_id = $1`, userID)
	if err != nil {
		slog.Error("favorites.update: delete existing", "error", err, "user_id", userID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	// Insert new favorites (skip invalid company_ids silently)
	if len(req.CompanyIDs) > 0 {
		// Validate that all company_ids actually exist
		// Use a single query with ANY to batch-validate
		rows, err := tx.Query(ctx,
			`SELECT id FROM companies WHERE id = ANY($1)`,
			req.CompanyIDs,
		)
		if err != nil {
			slog.Error("favorites.update: validate companies", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		validIDs := make(map[int]bool)
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err == nil {
				validIDs[id] = true
			}
		}
		rows.Close()

		// Insert only valid IDs
		insertedCount := 0
		for _, companyID := range req.CompanyIDs {
			if !validIDs[companyID] {
				continue
			}
			_, err := tx.Exec(ctx,
				`INSERT INTO user_favorites (user_id, company_id, created_at)
				 VALUES ($1, $2, NOW())
				 ON CONFLICT (user_id, company_id) DO NOTHING`,
				userID, companyID,
			)
			if err != nil {
				slog.Error("favorites.update: insert favorite", "error", err,
					"user_id", userID, "company_id", companyID)
				continue
			}
			insertedCount++
		}

		slog.Info("favorites updated",
			"user_id", userID,
			"requested", len(req.CompanyIDs),
			"inserted", insertedCount,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("favorites.update: commit tx", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "favorites updated"})
}
