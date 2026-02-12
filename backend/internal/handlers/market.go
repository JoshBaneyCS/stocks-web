package handlers

import (
	"net/http"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/market"
)

// MarketHandler handles market status endpoints.
type MarketHandler struct {
	Checker *market.Checker
}

// NewMarketHandler creates a new MarketHandler.
func NewMarketHandler(checker *market.Checker) *MarketHandler {
	return &MarketHandler{Checker: checker}
}

// Status returns the current NYSE market status.
func (h *MarketHandler) Status(w http.ResponseWriter, r *http.Request) {
	status := h.Checker.GetMarketStatus()
	writeJSON(w, http.StatusOK, status)
}
