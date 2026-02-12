-- Performance indexes on existing market-data tables.
-- Each uses IF NOT EXISTS so the migration is idempotent against
-- indexes that may have been created by the ingest pipeline.

CREATE INDEX IF NOT EXISTS idx_instruments_symbol
    ON instruments(symbol);

CREATE INDEX IF NOT EXISTS idx_price_bars_instrument_interval_ts
    ON price_bars(instrument_id, interval, ts DESC);

CREATE INDEX IF NOT EXISTS idx_quotes_instrument_ts
    ON quotes(instrument_id, ts DESC);

CREATE INDEX IF NOT EXISTS idx_company_profiles_instrument
    ON company_profiles(instrument_id);

CREATE INDEX IF NOT EXISTS idx_financial_income_q_instrument_period
    ON financial_income_quarterly(instrument_id, period_end_date DESC);
