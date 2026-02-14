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
	AuthDB   *pgxpool.Pool
	MarketDB *pgxpool.Pool
	Checker  *market.Checker
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(authDB, marketDB *pgxpool.Pool, checker *market.Checker) *DashboardHandler {
	return &DashboardHandler{AuthDB: authDB, MarketDB: marketDB, Checker: checker}
}

// Get returns the dashboard data: user favorites with prices + market status.
func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	ctx := r.Context()

	// Step 1: Get favorite IDs from auth DB
	favIDs, err := fetchFavoriteIDs(ctx, h.AuthDB, userID)
	if err != nil {
		slog.Error("failed to query dashboard favorite IDs", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Step 2: Get instrument details from market DB
	favorites, err := fetchInstrumentsByIDs(ctx, h.MarketDB, favIDs)
	if err != nil {
		slog.Error("failed to query dashboard instruments", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := models.DashboardResponse{
		Favorites:    favorites,
		MarketStatus: h.Checker.GetMarketStatus(),
	}

	writeJSON(w, http.StatusOK, resp)
}
