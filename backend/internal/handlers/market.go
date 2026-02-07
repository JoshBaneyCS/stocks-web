package handlers

import (
	"net/http"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/market"
)

// MarketHandler handles market status endpoints.
type MarketHandler struct {
	checker *market.Checker
}

// NewMarketHandler creates a new market handler.
func NewMarketHandler(checker *market.Checker) *MarketHandler {
	return &MarketHandler{checker: checker}
}

// Status handles GET /api/market/status
// Returns whether the market is currently open, current ET time,
// and either next_open (if closed) or next_close (if open).
func (h *MarketHandler) Status(w http.ResponseWriter, r *http.Request) {
	status := h.checker.Check()
	writeJSON(w, http.StatusOK, status)
}
