package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/auth"
	"github.com/JoshBaneyCS/stocks-web/backend/internal/market"
	"github.com/JoshBaneyCS/stocks-web/backend/internal/models"
)

const (
	// Polling intervals
	pollIntervalMarketOpen   = 5 * time.Second
	pollIntervalMarketClosed = 30 * time.Second
	heartbeatInterval        = 30 * time.Second
	favIDRefreshInterval     = 60 * time.Second
)

// StreamHandler handles SSE streaming endpoints.
type StreamHandler struct {
	AuthDB   *pgxpool.Pool
	MarketDB *pgxpool.Pool
	Checker  *market.Checker
}

// NewStreamHandler creates a new StreamHandler.
func NewStreamHandler(authDB, marketDB *pgxpool.Pool, checker *market.Checker) *StreamHandler {
	return &StreamHandler{AuthDB: authDB, MarketDB: marketDB, Checker: checker}
}

// InstrumentStream is an SSE endpoint that streams price updates for a single instrument.
// GET /api/stream/{symbol}?token=...
func (h *StreamHandler) InstrumentStream(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	ctx := r.Context()

	// Look up instrument from market DB
	var instrumentID int64
	err := h.MarketDB.QueryRow(ctx, `SELECT id FROM ingest.instruments WHERE symbol = $1`, symbol).Scan(&instrumentID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "instrument not found")
		} else {
			slog.Error("failed to query instrument for stream", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		slog.Error("streaming not supported")
		return
	}

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"symbol\":%q}\n\n", symbol)
	flusher.Flush()

	lastHeartbeat := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Determine poll interval based on market status
		pollInterval := pollIntervalMarketClosed
		if h.Checker.IsMarketOpen() {
			pollInterval = pollIntervalMarketOpen
		}

		// Fetch latest quote from market DB
		var event models.PriceEvent
		var ts time.Time
		err := h.MarketDB.QueryRow(ctx, `
			SELECT last_price, bid, ask, volume, asof_ts
			FROM ingest.instrument_latest_snapshot
			WHERE instrument_id = $1
		`, instrumentID).Scan(&event.LastPrice, &event.Bid, &event.Ask, &event.Volume, &ts)

		if err == nil {
			event.Symbol = symbol
			event.Timestamp = ts.Format(time.RFC3339)
			data, jsonErr := json.Marshal(event)
			if jsonErr == nil {
				fmt.Fprintf(w, "event: price\ndata: %s\n\n", data)
				flusher.Flush()
			}
		} else if err != pgx.ErrNoRows {
			slog.Error("failed to fetch quote for stream", "error", err, "symbol", symbol)
		}

		// Send heartbeat if needed
		if time.Since(lastHeartbeat) >= heartbeatInterval {
			fmt.Fprintf(w, "event: heartbeat\ndata: {\"ts\":%q}\n\n", time.Now().Format(time.RFC3339))
			flusher.Flush()
			lastHeartbeat = time.Now()
		}

		// Sleep for the poll interval, checking for context cancellation
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// FavoritesStream is an SSE endpoint that multiplexes price updates for all of a user's favorites.
// GET /api/stream/favorites?token=...
func (h *StreamHandler) FavoritesStream(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	ctx := r.Context()

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		slog.Error("streaming not supported")
		return
	}

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"stream\":\"favorites\"}\n\n")
	flusher.Flush()

	lastHeartbeat := time.Now()

	// Pre-fetch favorite IDs from auth DB, cache and refresh periodically
	favIDs, _ := fetchFavoriteIDs(ctx, h.AuthDB, userID)
	lastFavRefresh := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		pollInterval := pollIntervalMarketClosed
		if h.Checker.IsMarketOpen() {
			pollInterval = pollIntervalMarketOpen
		}

		// Refresh favorite IDs periodically
		if time.Since(lastFavRefresh) >= favIDRefreshInterval {
			if newIDs, err := fetchFavoriteIDs(ctx, h.AuthDB, userID); err == nil {
				favIDs = newIDs
			}
			lastFavRefresh = time.Now()
		}

		// Fetch latest quotes for favorites from market DB
		if len(favIDs) > 0 {
			rows, err := h.MarketDB.Query(ctx, `
				SELECT i.symbol, ls.last_price, ls.bid, ls.ask, ls.volume, ls.asof_ts
				FROM ingest.instruments i
				LEFT JOIN ingest.instrument_latest_snapshot ls ON ls.instrument_id = i.id
				WHERE i.id = ANY($1)
			`, favIDs)
			if err != nil {
				slog.Error("failed to query favorites for stream", "error", err)
			} else {
				for rows.Next() {
					var event models.PriceEvent
					var ts *time.Time
					if scanErr := rows.Scan(&event.Symbol, &event.LastPrice, &event.Bid, &event.Ask, &event.Volume, &ts); scanErr != nil {
						slog.Error("failed to scan favorite stream row", "error", scanErr)
						continue
					}
					if ts != nil {
						event.Timestamp = ts.Format(time.RFC3339)
					}
					data, jsonErr := json.Marshal(event)
					if jsonErr == nil {
						fmt.Fprintf(w, "event: price\ndata: %s\n\n", data)
					}
				}
				rows.Close()
				flusher.Flush()
			}
		}

		// Heartbeat
		if time.Since(lastHeartbeat) >= heartbeatInterval {
			fmt.Fprintf(w, "event: heartbeat\ndata: {\"ts\":%q}\n\n", time.Now().Format(time.RFC3339))
			flusher.Flush()
			lastHeartbeat = time.Now()
		}

		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
