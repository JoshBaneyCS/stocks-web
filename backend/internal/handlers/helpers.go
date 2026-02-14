package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JoshBaneyCS/stocks-web/backend/internal/models"
)

// writeJSON encodes data as JSON and writes it to the response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// fetchFavoriteIDs returns the instrument IDs favorited by the given user from the auth database.
func fetchFavoriteIDs(ctx context.Context, authDB *pgxpool.Pool, userID string) ([]int64, error) {
	rows, err := authDB.Query(ctx,
		`SELECT instrument_id FROM user_favorites WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// fetchInstrumentsByIDs returns instrument list items for the given IDs from the market database.
func fetchInstrumentsByIDs(ctx context.Context, marketDB *pgxpool.Pool, ids []int64) ([]models.InstrumentListItem, error) {
	if len(ids) == 0 {
		return []models.InstrumentListItem{}, nil
	}

	rows, err := marketDB.Query(ctx, `
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
		WHERE i.id = ANY($1)
		ORDER BY i.symbol ASC
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.InstrumentListItem, 0, len(ids))
	for rows.Next() {
		var item models.InstrumentListItem
		if err := rows.Scan(
			&item.ID, &item.Symbol, &item.Name, &item.Exchange, &item.Currency,
			&item.Country, &item.AssetClass, &item.IsActive,
			&item.LastPrice, &item.MarketCap, &item.Sector, &item.Industry,
		); err != nil {
			return nil, err
		}
		item.IsFavorite = true
		items = append(items, item)
	}
	return items, rows.Err()
}
