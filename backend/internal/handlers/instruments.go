package handlers

import (
	"context"
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
	AuthDB   *pgxpool.Pool
	MarketDB *pgxpool.Pool
}

// NewInstrumentsHandler creates a new InstrumentsHandler.
func NewInstrumentsHandler(authDB, marketDB *pgxpool.Pool) *InstrumentsHandler {
	return &InstrumentsHandler{AuthDB: authDB, MarketDB: marketDB}
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
		conditions = append(conditions, fmt.Sprintf("ac.name = $%d", argIdx))
		args = append(args, assetClass)
		argIdx++
	}
	if exchange != "" {
		conditions = append(conditions, fmt.Sprintf("ex.name = $%d", argIdx))
		args = append(args, exchange)
		argIdx++
	}
	if country != "" {
		conditions = append(conditions, fmt.Sprintf("i.country = $%d", argIdx))
		args = append(args, country)
		argIdx++
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	// Pre-fetch favorite IDs from auth DB (cross-DB)
	var favoriteSet map[int64]bool
	if userID != "" {
		favIDs, err := fetchFavoriteIDs(ctx, h.AuthDB, userID)
		if err != nil {
			slog.Error("failed to fetch favorite IDs", "error", err)
		} else {
			favoriteSet = make(map[int64]bool, len(favIDs))
			for _, id := range favIDs {
				favoriteSet[id] = true
			}
		}
	}

	// Count query
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM ingest.instruments i
		LEFT JOIN ingest.exchanges ex ON ex.id = i.exchange_id
		LEFT JOIN ingest.asset_classes ac ON ac.id = i.asset_class_id
		%s`, whereClause)

	var totalCount int
	if err := h.MarketDB.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		slog.Error("failed to count instruments", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Main data query
	dataQuery := fmt.Sprintf(`
		SELECT i.id, i.symbol, i.name,
		       ex.name, cur.code, i.country,
		       ac.name, i.is_active,
		       im.last_price, im.market_cap,
		       sec.name, ind.name
		FROM ingest.instruments i
		LEFT JOIN ingest.exchanges ex ON ex.id = i.exchange_id
		LEFT JOIN ingest.currencies cur ON cur.id = i.currency_id
		LEFT JOIN ingest.asset_classes ac ON ac.id = i.asset_class_id
		LEFT JOIN ingest.sectors sec ON sec.id = i.sector_id
		LEFT JOIN ingest.industries ind ON ind.id = i.industry_id
		LEFT JOIN ingest.instrument_metrics im ON im.instrument_id = i.id
		%s
		ORDER BY i.symbol ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, pageSize, offset)

	rows, err := h.MarketDB.Query(ctx, dataQuery, args...)
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
		); err != nil {
			slog.Error("failed to scan instrument row", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if favoriteSet != nil {
			item.IsFavorite = favoriteSet[item.ID]
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

	// Fetch instrument + metrics from market DB
	err := h.MarketDB.QueryRow(ctx, `
		SELECT i.id, i.symbol, i.name,
		       ex.name, cur.code, i.country,
		       ac.name, i.is_active,
		       im.last_price, im.market_cap
		FROM ingest.instruments i
		LEFT JOIN ingest.exchanges ex ON ex.id = i.exchange_id
		LEFT JOIN ingest.currencies cur ON cur.id = i.currency_id
		LEFT JOIN ingest.asset_classes ac ON ac.id = i.asset_class_id
		LEFT JOIN ingest.instrument_metrics im ON im.instrument_id = i.id
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

	// Fetch company profile + sector/industry from instrument FKs
	err = h.MarketDB.QueryRow(ctx, `
		SELECT im.market_cap,
		       sec.name, ind.name,
		       ex.name, i.country, cur.code
		FROM ingest.instruments i
		LEFT JOIN ingest.company_profiles cp ON cp.instrument_id = i.id
		LEFT JOIN ingest.instrument_metrics im ON im.instrument_id = i.id
		LEFT JOIN ingest.exchanges ex ON ex.id = i.exchange_id
		LEFT JOIN ingest.currencies cur ON cur.id = i.currency_id
		LEFT JOIN ingest.sectors sec ON sec.id = i.sector_id
		LEFT JOIN ingest.industries ind ON ind.id = i.industry_id
		WHERE i.id = $1
	`, detail.ID).Scan(
		&profile.MarketCap, &profile.Sector, &profile.Industry,
		&profile.Exchange, &profile.Country, &profile.Currency,
	)
	if err == nil {
		hasProfile = true
	} else if err != pgx.ErrNoRows {
		slog.Error("failed to query company profile", "error", err)
	}

	// Fetch latest quote from instrument_latest_snapshot
	err = h.MarketDB.QueryRow(ctx, `
		SELECT asof_ts, last_price, bid, ask, volume, source
		FROM ingest.instrument_latest_snapshot
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
	var instrumentID int64
	err := h.MarketDB.QueryRow(ctx, `SELECT id FROM ingest.instruments WHERE symbol = $1`, symbol).Scan(&instrumentID)
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
	err = h.MarketDB.QueryRow(ctx, `
		SELECT im.market_cap,
		       sec.name, ind.name,
		       ex.name, i.country, cur.code
		FROM ingest.instruments i
		LEFT JOIN ingest.company_profiles cp ON cp.instrument_id = i.id
		LEFT JOIN ingest.instrument_metrics im ON im.instrument_id = i.id
		LEFT JOIN ingest.exchanges ex ON ex.id = i.exchange_id
		LEFT JOIN ingest.currencies cur ON cur.id = i.currency_id
		LEFT JOIN ingest.sectors sec ON sec.id = i.sector_id
		LEFT JOIN ingest.industries ind ON ind.id = i.industry_id
		WHERE i.id = $1
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
	var instrumentID int64
	err := h.MarketDB.QueryRow(ctx, `SELECT id FROM ingest.instruments WHERE symbol = $1`, symbol).Scan(&instrumentID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "instrument not found")
		} else {
			slog.Error("failed to query instrument", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	rows, err := h.MarketDB.Query(ctx, `
		SELECT period_end_date, calendar_year, period, revenue, gross_profit,
		       operating_income, net_income, eps
		FROM ingest.financial_income_quarterly
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
// Query params: interval (1min|5min|15min|1h|1d|1w|1m), from, to, limit
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
	validIntervals := map[string]bool{
		"1min": true, "5min": true, "15min": true,
		"1h": true, "1d": true, "1w": true, "1m": true,
	}
	if !validIntervals[interval] {
		writeError(w, http.StatusBadRequest, "interval must be one of: 1min, 5min, 15min, 1h, 1d, 1w, 1m")
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
	var instrumentID int64
	err := h.MarketDB.QueryRow(ctx, `SELECT id FROM ingest.instruments WHERE symbol = $1`, symbol).Scan(&instrumentID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "instrument not found")
		} else {
			slog.Error("failed to query instrument", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	var bars []models.PriceBar
	switch interval {
	case "1min":
		bars, err = h.queryPriceBars(ctx, instrumentID, "1min", fromTime, toTime, limit)
	case "5min":
		bars, err = h.queryCagg(ctx, instrumentID, "ingest.cagg_price_bars_5min", fromTime, toTime, limit)
	case "15min":
		bars, err = h.queryCagg(ctx, instrumentID, "ingest.cagg_price_bars_15min", fromTime, toTime, limit)
	case "1h":
		bars, err = h.queryCagg(ctx, instrumentID, "ingest.cagg_price_bars_1h", fromTime, toTime, limit)
	case "1d":
		bars, err = h.queryCagg(ctx, instrumentID, "ingest.cagg_price_bars_1d", fromTime, toTime, limit)
	case "1w":
		bars, err = h.queryAggregated(ctx, instrumentID, "1 week", fromTime, toTime, limit)
	case "1m":
		bars, err = h.queryAggregated(ctx, instrumentID, "1 month", fromTime, toTime, limit)
	}

	if err != nil {
		slog.Error("failed to query price bars", "error", err, "interval", interval)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, bars)
}

// queryPriceBars queries the raw price_bars table for a given interval.
func (h *InstrumentsHandler) queryPriceBars(ctx context.Context, instrumentID int64, interval string, from, to *time.Time, limit int) ([]models.PriceBar, error) {
	rows, err := h.MarketDB.Query(ctx, `
		SELECT ts, open, high, low, close, volume
		FROM ingest.price_bars
		WHERE instrument_id = $1 AND interval = $2
		AND ($3::timestamptz IS NULL OR ts >= $3)
		AND ($4::timestamptz IS NULL OR ts <= $4)
		ORDER BY ts ASC
		LIMIT $5
	`, instrumentID, interval, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPriceBars(rows)
}

// queryCagg queries a continuous aggregate view.
func (h *InstrumentsHandler) queryCagg(ctx context.Context, instrumentID int64, viewName string, from, to *time.Time, limit int) ([]models.PriceBar, error) {
	query := fmt.Sprintf(`
		SELECT bucket, open, high, low, close, volume
		FROM %s
		WHERE instrument_id = $1
		AND ($2::timestamptz IS NULL OR bucket >= $2)
		AND ($3::timestamptz IS NULL OR bucket <= $3)
		ORDER BY bucket ASC
		LIMIT $4
	`, viewName)
	rows, err := h.MarketDB.Query(ctx, query, instrumentID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPriceBars(rows)
}

// queryAggregated performs on-the-fly aggregation from daily continuous aggregate data.
func (h *InstrumentsHandler) queryAggregated(ctx context.Context, instrumentID int64, bucketSize string, from, to *time.Time, limit int) ([]models.PriceBar, error) {
	query := fmt.Sprintf(`
		SELECT time_bucket('%s', bucket) AS period,
		       first(open, bucket) AS open,
		       max(high) AS high,
		       min(low) AS low,
		       last(close, bucket) AS close,
		       sum(volume) AS volume
		FROM ingest.cagg_price_bars_1d
		WHERE instrument_id = $1
		AND ($2::timestamptz IS NULL OR bucket >= $2)
		AND ($3::timestamptz IS NULL OR bucket <= $3)
		GROUP BY period
		ORDER BY period ASC
		LIMIT $4
	`, bucketSize)
	rows, err := h.MarketDB.Query(ctx, query, instrumentID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPriceBars(rows)
}

// scanPriceBars scans rows into a slice of PriceBar.
func scanPriceBars(rows pgx.Rows) ([]models.PriceBar, error) {
	bars := make([]models.PriceBar, 0)
	for rows.Next() {
		var bar models.PriceBar
		if err := rows.Scan(
			&bar.Timestamp, &bar.Open, &bar.High, &bar.Low, &bar.Close,
			&bar.Volume,
		); err != nil {
			return nil, err
		}
		bars = append(bars, bar)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return bars, nil
}

// FilterOptions holds the distinct filter values available for instrument queries.
type FilterOptions struct {
	Exchanges    []string `json:"exchanges"`
	AssetClasses []string `json:"asset_classes"`
	Countries    []string `json:"countries"`
}

// Filters returns distinct filter values from the database.
func (h *InstrumentsHandler) Filters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var opts FilterOptions

	// Exchanges with active instruments
	exRows, err := h.MarketDB.Query(ctx, `
		SELECT DISTINCT ex.name
		FROM ingest.exchanges ex
		INNER JOIN ingest.instruments i ON i.exchange_id = ex.id
		WHERE i.is_active = true AND ex.name IS NOT NULL
		ORDER BY ex.name
	`)
	if err != nil {
		slog.Error("failed to query exchanges", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer exRows.Close()
	for exRows.Next() {
		var name string
		if err := exRows.Scan(&name); err != nil {
			slog.Error("failed to scan exchange", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		opts.Exchanges = append(opts.Exchanges, name)
	}

	// Asset classes with active instruments
	acRows, err := h.MarketDB.Query(ctx, `
		SELECT DISTINCT ac.name
		FROM ingest.asset_classes ac
		INNER JOIN ingest.instruments i ON i.asset_class_id = ac.id
		WHERE i.is_active = true AND ac.name IS NOT NULL
		ORDER BY ac.name
	`)
	if err != nil {
		slog.Error("failed to query asset classes", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer acRows.Close()
	for acRows.Next() {
		var name string
		if err := acRows.Scan(&name); err != nil {
			slog.Error("failed to scan asset class", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		opts.AssetClasses = append(opts.AssetClasses, name)
	}

	// Countries with active instruments
	cRows, err := h.MarketDB.Query(ctx, `
		SELECT DISTINCT country
		FROM ingest.instruments
		WHERE is_active = true AND country IS NOT NULL
		ORDER BY country
	`)
	if err != nil {
		slog.Error("failed to query countries", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer cRows.Close()
	for cRows.Next() {
		var name string
		if err := cRows.Scan(&name); err != nil {
			slog.Error("failed to scan country", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		opts.Countries = append(opts.Countries, name)
	}

	if opts.Exchanges == nil {
		opts.Exchanges = []string{}
	}
	if opts.AssetClasses == nil {
		opts.AssetClasses = []string{}
	}
	if opts.Countries == nil {
		opts.Countries = []string{}
	}

	writeJSON(w, http.StatusOK, opts)
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
