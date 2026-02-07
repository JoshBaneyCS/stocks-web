package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/auth"
	"github.com/JoshBaneyCS/stocks-web/backend/internal/models"
)

// StocksHandler handles stock list, detail, prices, and news endpoints.
type StocksHandler struct {
	pool *pgxpool.Pool
}

// NewStocksHandler creates a new stocks handler.
func NewStocksHandler(pool *pgxpool.Pool) *StocksHandler {
	return &StocksHandler{pool: pool}
}

// List handles GET /api/stocks?search=&exchange=&sector=&industry=&page=&pageSize=
// Returns a paginated, filterable list of all companies.
func (h *StocksHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	search := strings.TrimSpace(q.Get("search"))
	exchange := strings.TrimSpace(q.Get("exchange"))
	sector := strings.TrimSpace(q.Get("sector"))
	industry := strings.TrimSpace(q.Get("industry"))
	page := intParam(q.Get("page"), 1)
	pageSize := intParam(q.Get("pageSize"), 50)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	// Build dynamic WHERE clause
	conditions := []string{}
	args := []interface{}{}
	argIdx := 1

	if search != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(LOWER(symbol) LIKE LOWER($%d) OR LOWER(name) LIKE LOWER($%d))",
			argIdx, argIdx,
		))
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if exchange != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(exchange) = LOWER($%d)", argIdx))
		args = append(args, exchange)
		argIdx++
	}
	if sector != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(sector) = LOWER($%d)", argIdx))
		args = append(args, sector)
		argIdx++
	}
	if industry != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(industry) = LOWER($%d)", argIdx))
		args = append(args, industry)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total matching rows
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM companies %s", whereClause)
	var total int
	if err := h.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		slog.Error("stocks.list: count query", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	offset := (page - 1) * pageSize

	// Fetch page
	dataQuery := fmt.Sprintf(
		`SELECT id, symbol, name, exchange, sector, industry, market_cap
		 FROM companies
		 %s
		 ORDER BY symbol ASC
		 LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1,
	)
	args = append(args, pageSize, offset)

	rows, err := h.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		slog.Error("stocks.list: data query", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	defer rows.Close()

	stocks := make([]models.CompanyListItem, 0)
	for rows.Next() {
		var s models.CompanyListItem
		if err := rows.Scan(&s.ID, &s.Symbol, &s.Name, &s.Exchange, &s.Sector, &s.Industry, &s.MarketCap); err != nil {
			slog.Error("stocks.list: scan row", "error", err)
			continue
		}
		stocks = append(stocks, s)
	}

	writeJSON(w, http.StatusOK, models.StockListResponse{
		Stocks:     stocks,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

// Detail handles GET /api/stocks/{symbol}
// Returns full company info, latest bar, and whether the user has it favorited.
func (h *StocksHandler) Detail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "symbol is required"})
		return
	}

	// Fetch company
	var c models.Company
	err := h.pool.QueryRow(ctx,
		`SELECT id, symbol, name, exchange, sector, industry, market_cap,
		        week52_high, week52_low, prev_close, todays_high, todays_low, updated_at
		 FROM companies WHERE UPPER(symbol) = $1`,
		symbol,
	).Scan(&c.ID, &c.Symbol, &c.Name, &c.Exchange, &c.Sector, &c.Industry, &c.MarketCap,
		&c.Week52Hi, &c.Week52Lo, &c.PrevClose, &c.TodaysHi, &c.TodaysLo, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "stock not found"})
		return
	}
	if err != nil {
		slog.Error("stocks.detail: query company", "error", err, "symbol", symbol)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	// Fetch latest price bar (prefer 1min, fall back to 1d)
	var latestBar *models.PricePoint
	var pp models.PricePoint
	err = h.pool.QueryRow(ctx,
		`SELECT ts, open, high, low, close, volume
		 FROM price_bars
		 WHERE company_id = $1
		 ORDER BY ts DESC
		 LIMIT 1`,
		c.ID,
	).Scan(&pp.Timestamp, &pp.Open, &pp.High, &pp.Low, &pp.Close, &pp.Volume)
	if err == nil {
		latestBar = &pp
	} else if err != pgx.ErrNoRows {
		slog.Error("stocks.detail: query latest bar", "error", err, "symbol", symbol)
	}

	// Check if favorited by current user
	isFavorite := false
	userID, ok := auth.UserIDFromContext(ctx)
	if ok {
		var exists bool
		err = h.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM user_favorites WHERE user_id = $1 AND company_id = $2)`,
			userID, c.ID,
		).Scan(&exists)
		if err == nil {
			isFavorite = exists
		}
	}

	writeJSON(w, http.StatusOK, models.StockDetailResponse{
		Company:    c,
		LatestBar:  latestBar,
		IsFavorite: isFavorite,
	})
}

// Prices handles GET /api/stocks/{symbol}/prices?interval=1min|1d&from=&to=&limit=
// Returns OHLCV price data for charting.
func (h *StocksHandler) Prices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "symbol is required"})
		return
	}

	q := r.URL.Query()
	interval := q.Get("interval")
	if interval != "1min" && interval != "1d" {
		interval = "1d"
	}

	limit := intParam(q.Get("limit"), 500)
	if limit < 1 || limit > 10000 {
		limit = 500
	}

	// Resolve company_id
	var companyID int
	err := h.pool.QueryRow(ctx,
		`SELECT id FROM companies WHERE UPPER(symbol) = $1`, symbol,
	).Scan(&companyID)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "stock not found"})
		return
	}
	if err != nil {
		slog.Error("stocks.prices: resolve company", "error", err, "symbol", symbol)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	// Build query with optional date filters
	conditions := []string{
		"company_id = $1",
		"interval = $2",
	}
	args := []interface{}{companyID, interval}
	argIdx := 3

	if fromStr := q.Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			conditions = append(conditions, fmt.Sprintf("ts >= $%d", argIdx))
			args = append(args, t)
			argIdx++
		}
	}
	if toStr := q.Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			conditions = append(conditions, fmt.Sprintf("ts <= $%d", argIdx))
			args = append(args, t)
			argIdx++
		}
	}

	query := fmt.Sprintf(
		`SELECT ts, open, high, low, close, volume
		 FROM price_bars
		 WHERE %s
		 ORDER BY ts ASC
		 LIMIT $%d`,
		strings.Join(conditions, " AND "), argIdx,
	)
	args = append(args, limit)

	rows, err := h.pool.Query(ctx, query, args...)
	if err != nil {
		slog.Error("stocks.prices: query", "error", err, "symbol", symbol)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	defer rows.Close()

	points := make([]models.PricePoint, 0)
	for rows.Next() {
		var p models.PricePoint
		if err := rows.Scan(&p.Timestamp, &p.Open, &p.High, &p.Low, &p.Close, &p.Volume); err != nil {
			slog.Error("stocks.prices: scan row", "error", err)
			continue
		}
		points = append(points, p)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"symbol":   symbol,
		"interval": interval,
		"count":    len(points),
		"prices":   points,
	})
}

// News handles GET /api/stocks/{symbol}/news?from=&to=&limit=
// Returns news articles and press releases for a given stock symbol.
func (h *StocksHandler) News(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	if symbol == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "symbol is required"})
		return
	}

	q := r.URL.Query()
	limit := intParam(q.Get("limit"), 50)
	if limit < 1 || limit > 200 {
		limit = 50
	}

	// Build query with optional date filters
	conditions := []string{"m.symbol = $1"}
	args := []interface{}{symbol}
	argIdx := 2

	if fromStr := q.Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			conditions = append(conditions, fmt.Sprintf("a.published_at >= $%d", argIdx))
			args = append(args, t)
			argIdx++
		}
	}
	if toStr := q.Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			conditions = append(conditions, fmt.Sprintf("a.published_at <= $%d", argIdx))
			args = append(args, t)
			argIdx++
		}
	}

	query := fmt.Sprintf(
		`SELECT a.id, a.provider, a.source_name, a.url_original, a.title, a.summary, a.published_at
		 FROM news_articles a
		 JOIN news_mentions m ON m.article_id = a.id
		 WHERE %s
		 ORDER BY a.published_at DESC
		 LIMIT $%d`,
		strings.Join(conditions, " AND "), argIdx,
	)
	args = append(args, limit)

	rows, err := h.pool.Query(ctx, query, args...)
	if err != nil {
		slog.Error("stocks.news: query", "error", err, "symbol", symbol)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	defer rows.Close()

	articles := make([]models.NewsArticle, 0)
	for rows.Next() {
		var a models.NewsArticle
		if err := rows.Scan(&a.ID, &a.Provider, &a.SourceName, &a.URLOriginal, &a.Title, &a.Summary, &a.PublishedAt); err != nil {
			slog.Error("stocks.news: scan row", "error", err)
			continue
		}
		articles = append(articles, a)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"symbol":   symbol,
		"count":    len(articles),
		"articles": articles,
	})
}

// ─── Helpers ─────────────────────────────────────────────────────────

func intParam(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
