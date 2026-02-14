package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/models"
)

// APIV1Handler handles public API v1 endpoints.
type APIV1Handler struct {
	AuthDB   *pgxpool.Pool
	MarketDB *pgxpool.Pool
}

// NewAPIV1Handler creates a new APIV1Handler.
func NewAPIV1Handler(authDB, marketDB *pgxpool.Pool) *APIV1Handler {
	return &APIV1Handler{AuthDB: authDB, MarketDB: marketDB}
}

// APIResponse wraps all v1 API responses in a consistent envelope.
type APIResponse struct {
	Data  interface{} `json:"data"`
	Meta  *APIMeta    `json:"meta,omitempty"`
	Error *string     `json:"error,omitempty"`
}

// APIMeta holds pagination metadata.
type APIMeta struct {
	Page       int `json:"page,omitempty"`
	PageSize   int `json:"page_size,omitempty"`
	TotalCount int `json:"total_count,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := APIResponse{Error: &message}
	data, _ := json.Marshal(resp)
	_, _ = w.Write(data)
}

func writeAPIJSON(w http.ResponseWriter, status int, data interface{}, meta *APIMeta) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := APIResponse{Data: data, Meta: meta}
	jsonData, _ := json.Marshal(resp)
	_, _ = w.Write(jsonData)
}

// ListInstruments returns a paginated list of instruments.
// GET /api/v1/instruments
func (h *APIV1Handler) ListInstruments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	assetClass := strings.TrimSpace(r.URL.Query().Get("asset_class"))
	exchange := strings.TrimSpace(r.URL.Query().Get("exchange"))
	country := strings.TrimSpace(r.URL.Query().Get("country"))

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

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM ingest.instruments i
		LEFT JOIN ingest.exchanges ex ON ex.id = i.exchange_id
		LEFT JOIN ingest.asset_classes ac ON ac.id = i.asset_class_id
		%s`, whereClause)

	var totalCount int
	if err := h.MarketDB.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		slog.Error("v1: failed to count instruments", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal server error")
		return
	}

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
		slog.Error("v1: failed to query instruments", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal server error")
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
			slog.Error("v1: failed to scan instrument", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		slog.Error("v1: row iteration error", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(pageSize)))

	writeAPIJSON(w, http.StatusOK, items, &APIMeta{
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
		TotalPages: totalPages,
	})
}

// GetInstrument returns detailed information for a single instrument.
// GET /api/v1/instruments/{symbol}
func (h *APIV1Handler) GetInstrument(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	if symbol == "" {
		writeAPIError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	ctx := r.Context()

	var detail models.InstrumentDetail
	var profile models.CompanyProfile
	var quote models.Quote

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
			writeAPIError(w, http.StatusNotFound, "instrument not found")
		} else {
			slog.Error("v1: failed to query instrument", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	// Profile
	err = h.MarketDB.QueryRow(ctx, `
		SELECT im.market_cap, sec.name, ind.name, ex.name, i.country, cur.code
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
		detail.Profile = &profile
	}

	// Quote
	err = h.MarketDB.QueryRow(ctx, `
		SELECT asof_ts, last_price, bid, ask, volume, source
		FROM ingest.instrument_latest_snapshot
		WHERE instrument_id = $1
	`, detail.ID).Scan(
		&quote.Timestamp, &quote.LastPrice, &quote.Bid, &quote.Ask, &quote.Volume, &quote.Source,
	)
	if err == nil {
		detail.LatestQuote = &quote
	}

	writeAPIJSON(w, http.StatusOK, detail, nil)
}

// GetPrices returns price bars for an instrument.
// GET /api/v1/instruments/{symbol}/prices
func (h *APIV1Handler) GetPrices(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	if symbol == "" {
		writeAPIError(w, http.StatusBadRequest, "symbol is required")
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
		writeAPIError(w, http.StatusBadRequest, "interval must be one of: 1min, 5min, 15min, 1h, 1d, 1w, 1m")
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
			writeAPIError(w, http.StatusBadRequest, "invalid 'from' timestamp, use RFC3339 format")
			return
		}
		fromTime = &t
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid 'to' timestamp, use RFC3339 format")
			return
		}
		toTime = &t
	}

	ctx := r.Context()

	var instrumentID int64
	err := h.MarketDB.QueryRow(ctx, `SELECT id FROM ingest.instruments WHERE symbol = $1`, symbol).Scan(&instrumentID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeAPIError(w, http.StatusNotFound, "instrument not found")
		} else {
			slog.Error("v1: failed to query instrument", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	// Reuse the InstrumentsHandler query helpers via a temporary instance
	ih := &InstrumentsHandler{MarketDB: h.MarketDB}
	var bars []models.PriceBar
	switch interval {
	case "1min":
		bars, err = ih.queryPriceBars(ctx, instrumentID, "1min", fromTime, toTime, limit)
	case "5min":
		bars, err = ih.queryCagg(ctx, instrumentID, "ingest.cagg_price_bars_5min", fromTime, toTime, limit)
	case "15min":
		bars, err = ih.queryCagg(ctx, instrumentID, "ingest.cagg_price_bars_15min", fromTime, toTime, limit)
	case "1h":
		bars, err = ih.queryCagg(ctx, instrumentID, "ingest.cagg_price_bars_1h", fromTime, toTime, limit)
	case "1d":
		bars, err = ih.queryCagg(ctx, instrumentID, "ingest.cagg_price_bars_1d", fromTime, toTime, limit)
	case "1w":
		bars, err = ih.queryAggregated(ctx, instrumentID, "1 week", fromTime, toTime, limit)
	case "1m":
		bars, err = ih.queryAggregated(ctx, instrumentID, "1 month", fromTime, toTime, limit)
	}

	if err != nil {
		slog.Error("v1: failed to query prices", "error", err, "interval", interval)
		writeAPIError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeAPIJSON(w, http.StatusOK, bars, nil)
}

// GetQuotes returns the latest quote for an instrument.
// GET /api/v1/instruments/{symbol}/quotes
func (h *APIV1Handler) GetQuotes(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	if symbol == "" {
		writeAPIError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	ctx := r.Context()

	var instrumentID int64
	err := h.MarketDB.QueryRow(ctx, `SELECT id FROM ingest.instruments WHERE symbol = $1`, symbol).Scan(&instrumentID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeAPIError(w, http.StatusNotFound, "instrument not found")
		} else {
			slog.Error("v1: failed to query instrument", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	var quote models.Quote
	err = h.MarketDB.QueryRow(ctx, `
		SELECT asof_ts, last_price, bid, ask, volume, source
		FROM ingest.instrument_latest_snapshot
		WHERE instrument_id = $1
	`, instrumentID).Scan(
		&quote.Timestamp, &quote.LastPrice, &quote.Bid, &quote.Ask, &quote.Volume, &quote.Source,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeAPIError(w, http.StatusNotFound, "no quote available")
		} else {
			slog.Error("v1: failed to query quote", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeAPIJSON(w, http.StatusOK, quote, nil)
}

// GetProfile returns the company profile for an instrument.
// GET /api/v1/instruments/{symbol}/profile
func (h *APIV1Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	if symbol == "" {
		writeAPIError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	ctx := r.Context()

	var instrumentID int64
	err := h.MarketDB.QueryRow(ctx, `SELECT id FROM ingest.instruments WHERE symbol = $1`, symbol).Scan(&instrumentID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeAPIError(w, http.StatusNotFound, "instrument not found")
		} else {
			slog.Error("v1: failed to query instrument", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	var profile models.CompanyProfile
	err = h.MarketDB.QueryRow(ctx, `
		SELECT im.market_cap, sec.name, ind.name, ex.name, i.country, cur.code
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
			writeAPIError(w, http.StatusNotFound, "profile not found")
		} else {
			slog.Error("v1: failed to query profile", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeAPIJSON(w, http.StatusOK, profile, nil)
}

// GetFundamentals returns quarterly financial data for an instrument.
// GET /api/v1/instruments/{symbol}/fundamentals
func (h *APIV1Handler) GetFundamentals(w http.ResponseWriter, r *http.Request) {
	symbol := strings.ToUpper(chi.URLParam(r, "symbol"))
	if symbol == "" {
		writeAPIError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	limit := intQueryParam(r, "limit", 20)
	if limit < 1 || limit > 100 {
		limit = 20
	}

	ctx := r.Context()

	var instrumentID int64
	err := h.MarketDB.QueryRow(ctx, `SELECT id FROM ingest.instruments WHERE symbol = $1`, symbol).Scan(&instrumentID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeAPIError(w, http.StatusNotFound, "instrument not found")
		} else {
			slog.Error("v1: failed to query instrument", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal server error")
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
		slog.Error("v1: failed to query fundamentals", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal server error")
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
			slog.Error("v1: failed to scan fundamentals", "error", err)
			writeAPIError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		row.PeriodEndDate = periodEndDate.Format("2006-01-02")
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		slog.Error("v1: row iteration error", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeAPIJSON(w, http.StatusOK, results, nil)
}
