package handlers

import (
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

// InstrumentsHandler handles instrument-related HTTP endpoints.
type InstrumentsHandler struct {
	DB *pgxpool.Pool
}

// NewInstrumentsHandler creates a new InstrumentsHandler.
func NewInstrumentsHandler(db *pgxpool.Pool) *InstrumentsHandler {
	return &InstrumentsHandler{DB: db}
}

// List returns a paginated list of instruments with optional filters.
// Query params: search, asset_class, exchange, country, page, page_size
func (h *InstrumentsHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFromContext(ctx)

	// Parse pagination
	page := intQueryParam(r, "page", 1)
	if page < 1 {
		page = 1
	}
	pageSize := intQueryParam(r, "page_size", 50)
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	offset := (page - 1) * pageSize

	// Parse filters
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	assetClass := strings.TrimSpace(r.URL.Query().Get("asset_class"))
	exchange := strings.TrimSpace(r.URL.Query().Get("exchange"))
	country := strings.TrimSpace(r.URL.Query().Get("country"))

	// Build WHERE clause
	conditions := []string{"i.is_active = true"}
	args := []interface{}{}
	argIdx := 1

	if search != "" {
		conditions = append(conditions, fmt.Sprintf("(i.symbol ILIKE $%d OR i.name ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if assetClass != "" {
		conditions = append(conditions, fmt.Sprintf("i.asset_class = $%d", argIdx))
		args = append(args, assetClass)
		argIdx++
	}
	if exchange != "" {
		conditions = append(conditions, fmt.Sprintf("i.exchange = $%d", argIdx))
		args = append(args, exchange)
		argIdx++
	}
	if country != "" {
		conditions = append(conditions, fmt.Sprintf("i.country = $%d", argIdx))
		args = append(args, country)
		argIdx++
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	// Count query
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM instruments i %s`, whereClause)
	var totalCount int
	if err := h.DB.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		slog.Error("failed to count instruments", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Build the main query
	favJoin := ""
	favSelect := "false AS is_favorite"
	if userID != "" {
		favJoin = fmt.Sprintf("LEFT JOIN user_favorites uf ON uf.instrument_id = i.id AND uf.user_id = $%d", argIdx)
		args = append(args, userID)
		argIdx++
		favSelect = "COALESCE(uf.user_id IS NOT NULL, false) AS is_favorite"
	}

	dataQuery := fmt.Sprintf(`
		SELECT i.id, i.symbol, i.name, i.exchange, i.currency, i.country, i.asset_class, i.is_active,
		       im.last_price, im.market_cap, cp.sector, cp.industry,
		       %s
		FROM instruments i
		LEFT JOIN instrument_metrics im ON im.instrument_id = i.id
		LEFT JOIN company_profiles cp ON cp.instrument_id = i.id
		%s
		%s
		ORDER BY i.symbol ASC
		LIMIT $%d OFFSET $%d
	`, favSelect, favJoin, whereClause, argIdx, argIdx+1)

	args = append(args, pageSize, offset)

	rows, err := h.DB.Query(ctx, dataQuery, args...)
	if err != nil {
		slog.Error("failed to query instruments", "error", err)
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
			&item.IsFavorite,
		); err != nil {
			slog.Error("failed to scan instrument row", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		slog.Error("row iteration error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(pageSize)))

	resp := models.PaginatedResponse[models.InstrumentListItem]{
		Data:       items,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
		TotalPages: totalPages,
	}

	writeJSON(w, http.StatusOK, resp)
}

// Detail returns detailed information for a single instrument by symbol.
func (h *InstrumentsHandler) Detail(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	ctx := r.Context()

	var detail models.InstrumentDetail
	var profile models.CompanyProfile
	var quote models.Quote
	var hasProfile, hasQuote bool

	// Fetch instrument + metrics
	err := h.DB.QueryRow(ctx, `
		SELECT i.id, i.symbol, i.name, i.exchange, i.currency, i.country, i.asset_class, i.is_active,
		       im.last_price, im.market_cap
		FROM instruments i
		LEFT JOIN instrument_metrics im ON im.instrument_id = i.id
		WHERE i.symbol = $1
	`, symbol).Scan(
		&detail.ID, &detail.Symbol, &detail.Name, &detail.Exchange, &detail.Currency,
		&detail.Country, &detail.AssetClass, &detail.IsActive,
		&detail.LastPrice, &detail.MarketCap,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "instrument not found")
		} else {
			slog.Error("failed to query instrument detail", "error", err, "symbol", symbol)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	// Fetch company profile
	err = h.DB.QueryRow(ctx, `
		SELECT market_cap, sector, industry, exchange, country, currency
		FROM company_profiles
		WHERE instrument_id = $1
	`, detail.ID).Scan(
		&profile.MarketCap, &profile.Sector, &profile.Industry,
		&profile.Exchange, &profile.Country, &profile.Currency,
	)
	if err == nil {
		hasProfile = true
	} else if err != pgx.ErrNoRows {
		slog.Error("failed to query company profile", "error", err)
	}

	// Fetch latest quote
	err = h.DB.QueryRow(ctx, `
		SELECT ts, last_price, bid, ask, volume, source
		FROM latest_quote_per_instrument
		WHERE instrument_id = $1
	`, detail.ID).Scan(
		&quote.Timestamp, &quote.LastPrice, &quote.Bid, &quote.Ask, &quote.Volume, &quote.Source,
	)
	if err == nil {
		hasQuote = true
	} else if err != pgx.ErrNoRows {
		slog.Error("failed to query latest quote", "error", err)
	}

	if hasProfile {
		detail.Profile = &profile
	}
	if hasQuote {
		detail.LatestQuote = &quote
	}

	writeJSON(w, http.StatusOK, detail)
}

// Profile returns the company profile for an instrument.
func (h *InstrumentsHandler) Profile(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	ctx := r.Context()

	// Look up instrument ID
	var instrumentID int
	err := h.DB.QueryRow(ctx, `SELECT id FROM instruments WHERE symbol = $1`, symbol).Scan(&instrumentID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "instrument not found")
		} else {
			slog.Error("failed to query instrument", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	var profile models.CompanyProfile
	err = h.DB.QueryRow(ctx, `
		SELECT market_cap, sector, industry, exchange, country, currency
		FROM company_profiles
		WHERE instrument_id = $1
	`, instrumentID).Scan(
		&profile.MarketCap, &profile.Sector, &profile.Industry,
		&profile.Exchange, &profile.Country, &profile.Currency,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "company profile not found")
		} else {
			slog.Error("failed to query company profile", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

// Fundamentals returns quarterly financial data for an instrument.
func (h *InstrumentsHandler) Fundamentals(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	limit := intQueryParam(r, "limit", 20)
	if limit < 1 || limit > 100 {
		limit = 20
	}

	ctx := r.Context()

	// Look up instrument ID
	var instrumentID int
	err := h.DB.QueryRow(ctx, `SELECT id FROM instruments WHERE symbol = $1`, symbol).Scan(&instrumentID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "instrument not found")
		} else {
			slog.Error("failed to query instrument", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	rows, err := h.DB.Query(ctx, `
		SELECT period_end_date, calendar_year, period, revenue, gross_profit,
		       operating_income, net_income, eps
		FROM financial_income_quarterly
		WHERE instrument_id = $1
		ORDER BY period_end_date DESC
		LIMIT $2
	`, instrumentID, limit)
	if err != nil {
		slog.Error("failed to query fundamentals", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	results := make([]models.FundamentalsRow, 0)
	for rows.Next() {
		var row models.FundamentalsRow
		var periodEndDate time.Time
		if err := rows.Scan(
			&periodEndDate, &row.CalendarYear, &row.Period,
			&row.Revenue, &row.GrossProfit, &row.OperatingIncome,
			&row.NetIncome, &row.EPS,
		); err != nil {
			slog.Error("failed to scan fundamentals row", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		row.PeriodEndDate = periodEndDate.Format("2006-01-02")
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		slog.Error("row iteration error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, results)
}

// Prices returns historical price bars for an instrument.
// Query params: interval (1d|1h|1min), from, to, limit
func (h *InstrumentsHandler) Prices(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1d"
	}
	if interval != "1d" && interval != "1h" && interval != "1min" {
		writeError(w, http.StatusBadRequest, "interval must be 1d, 1h, or 1min")
		return
	}

	limit := intQueryParam(r, "limit", 500)
	if limit < 1 || limit > 5000 {
		limit = 500
	}

	var fromTime, toTime *time.Time
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid 'from' timestamp, use RFC3339 format")
			return
		}
		fromTime = &t
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid 'to' timestamp, use RFC3339 format")
			return
		}
		toTime = &t
	}

	ctx := r.Context()

	// Look up instrument ID
	var instrumentID int
	err := h.DB.QueryRow(ctx, `SELECT id FROM instruments WHERE symbol = $1`, symbol).Scan(&instrumentID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "instrument not found")
		} else {
			slog.Error("failed to query instrument", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	rows, err := h.DB.Query(ctx, `
		SELECT ts, open, high, low, close, volume, adj_close
		FROM price_bars
		WHERE instrument_id = $1 AND interval = $2
		AND ($3::timestamptz IS NULL OR ts >= $3)
		AND ($4::timestamptz IS NULL OR ts <= $4)
		ORDER BY ts ASC
		LIMIT $5
	`, instrumentID, interval, fromTime, toTime, limit)
	if err != nil {
		slog.Error("failed to query price bars", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	bars := make([]models.PriceBar, 0)
	for rows.Next() {
		var bar models.PriceBar
		if err := rows.Scan(
			&bar.Timestamp, &bar.Open, &bar.High, &bar.Low, &bar.Close,
			&bar.Volume, &bar.AdjClose,
		); err != nil {
			slog.Error("failed to scan price bar", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		bars = append(bars, bar)
	}
	if err := rows.Err(); err != nil {
		slog.Error("row iteration error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, bars)
}

// intQueryParam extracts an integer query parameter with a default value.
func intQueryParam(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
