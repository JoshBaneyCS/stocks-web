package handlers

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/auth"
	"github.com/JoshBaneyCS/stocks-web/backend/internal/market"
	"github.com/JoshBaneyCS/stocks-web/backend/internal/models"
)

// DashboardHandler handles the dashboard endpoint.
type DashboardHandler struct {
	DB      *pgxpool.Pool
	Checker *market.Checker
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(db *pgxpool.Pool, checker *market.Checker) *DashboardHandler {
	return &DashboardHandler{DB: db, Checker: checker}
}

// Get returns the dashboard data: user favorites with prices + market status.
func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
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
		slog.Error("failed to query dashboard favorites", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	favorites := make([]models.InstrumentListItem, 0)
	for rows.Next() {
		var item models.InstrumentListItem
		if err := rows.Scan(
			&item.ID, &item.Symbol, &item.Name, &item.Exchange, &item.Currency,
			&item.Country, &item.AssetClass, &item.IsActive,
			&item.LastPrice, &item.MarketCap, &item.Sector, &item.Industry,
		); err != nil {
			slog.Error("failed to scan dashboard favorite row", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		item.IsFavorite = true
		favorites = append(favorites, item)
	}
	if err := rows.Err(); err != nil {
		slog.Error("row iteration error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := models.DashboardResponse{
		Favorites:    favorites,
		MarketStatus: h.Checker.GetMarketStatus(),
	}

	writeJSON(w, http.StatusOK, resp)
}
