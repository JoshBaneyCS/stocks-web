package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/market"
	"github.com/JoshBaneyCS/stocks-web/backend/internal/models"
)

// StreamHandler handles SSE streaming endpoints for live price data.
type StreamHandler struct {
	pool    *pgxpool.Pool
	checker *market.Checker
}

// NewStreamHandler creates a new stream handler.
func NewStreamHandler(pool *pgxpool.Pool, checker *market.Checker) *StreamHandler {
	return &StreamHandler{pool: pool, checker: checker}
}

// sseEvent represents a Server-Sent Event.
type sseEvent struct {
	Event string      `json:"-"`
	Data  interface{} `json:"data"`
}

// StockStream handles GET /api/stream/stocks/{symbol}
//
// SSE protocol:
//   - event: price        → new 1-minute bar data
//   - event: market_status → open/closed state change
//   - event: heartbeat    → keepalive (every 30s)
//
// Behavior:
//   - When market is OPEN: polls DB every 10s for new 1-minute bars
//   - When market is CLOSED: sends market_closed event, then heartbeats only
//   - Detects market open/close transitions and sends status events
func (h *StreamHandler) StockStream(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	if symbol == "" {
		http.Error(w, `{"error":"symbol is required"}`, http.StatusBadRequest)
		return
	}

	// Resolve company_id
	var companyID int
	err := h.pool.QueryRow(r.Context(),
		`SELECT id FROM companies WHERE UPPER(symbol) = $1`, symbol,
	).Scan(&companyID)
	if err == pgx.ErrNoRows {
		http.Error(w, `{"error":"stock not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Error("stream: resolve company", "error", err, "symbol", symbol)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// Send initial market status
	status := h.checker.Check()
	sendSSE(w, flusher, "market_status", status)

	// Track last seen timestamp to only send new bars
	lastSeenTS := h.getLatestBarTS(ctx, companyID)

	// Track market state for transition detection
	wasOpen := status.IsOpen

	// Polling intervals
	pollOpen := 10 * time.Second   // poll DB every 10s when market open
	pollClosed := 60 * time.Second // poll less frequently when closed
	heartbeat := 30 * time.Second  // keepalive interval

	pollInterval := pollClosed
	if wasOpen {
		pollInterval = pollOpen
	}

	pollTicker := time.NewTicker(pollInterval)
	defer pollTicker.Stop()

	heartbeatTicker := time.NewTicker(heartbeat)
	defer heartbeatTicker.Stop()

	slog.Info("stream started", "symbol", symbol, "company_id", companyID, "market_open", wasOpen)

	for {
		select {
		case <-ctx.Done():
			slog.Info("stream closed by client", "symbol", symbol)
			return

		case <-heartbeatTicker.C:
			sendSSE(w, flusher, "heartbeat", map[string]interface{}{
				"ts":     time.Now().UTC(),
				"symbol": symbol,
			})

		case <-pollTicker.C:
			// Check for market state transitions
			currentStatus := h.checker.Check()
			isOpen := currentStatus.IsOpen

			if isOpen != wasOpen {
				sendSSE(w, flusher, "market_status", currentStatus)
				wasOpen = isOpen

				// Adjust poll interval
				pollTicker.Stop()
				if isOpen {
					pollInterval = pollOpen
				} else {
					pollInterval = pollClosed
				}
				pollTicker = time.NewTicker(pollInterval)
				slog.Info("market state changed", "symbol", symbol, "is_open", isOpen)
			}

			// Only poll for new bars when market is open
			if !isOpen {
				continue
			}

			// Query for new bars since lastSeenTS
			newBars := h.getNewBars(ctx, companyID, lastSeenTS)
			for _, bar := range newBars {
				sendSSE(w, flusher, "price", map[string]interface{}{
					"symbol": symbol,
					"bar":    bar,
				})
				if bar.Timestamp.After(lastSeenTS) {
					lastSeenTS = bar.Timestamp
				}
			}
		}
	}
}

// getLatestBarTS returns the most recent 1-minute bar timestamp for a company.
func (h *StreamHandler) getLatestBarTS(ctx context.Context, companyID int) time.Time {
	var ts time.Time
	err := h.pool.QueryRow(ctx,
		`SELECT ts FROM price_bars
		 WHERE company_id = $1 AND interval = '1min'
		 ORDER BY ts DESC LIMIT 1`,
		companyID,
	).Scan(&ts)
	if err != nil {
		// If no bars exist, start from beginning of today
		return time.Now().Truncate(24 * time.Hour)
	}
	return ts
}

// getNewBars fetches 1-minute bars newer than the given timestamp.
func (h *StreamHandler) getNewBars(ctx context.Context, companyID int, after time.Time) []models.PricePoint {
	rows, err := h.pool.Query(ctx,
		`SELECT ts, open, high, low, close, volume
		 FROM price_bars
		 WHERE company_id = $1 AND interval = '1min' AND ts > $2
		 ORDER BY ts ASC
		 LIMIT 10`,
		companyID, after,
	)
	if err != nil {
		slog.Error("stream: query new bars", "error", err, "company_id", companyID)
		return nil
	}
	defer rows.Close()

	bars := make([]models.PricePoint, 0)
	for rows.Next() {
		var p models.PricePoint
		if err := rows.Scan(&p.Timestamp, &p.Open, &p.High, &p.Low, &p.Close, &p.Volume); err != nil {
			slog.Error("stream: scan bar", "error", err)
			continue
		}
		bars = append(bars, p)
	}
	return bars
}

// sendSSE writes a single Server-Sent Event to the response writer.
func sendSSE(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		slog.Error("sse: marshal data", "error", err, "event", event)
		return
	}

	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	flusher.Flush()
}
