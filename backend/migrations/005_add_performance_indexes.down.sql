-- Drop only the indexes this migration created.
-- Uses IF EXISTS so the down migration is safe to re-run.

DROP INDEX IF EXISTS idx_instruments_symbol;
DROP INDEX IF EXISTS idx_price_bars_instrument_interval_ts;
DROP INDEX IF EXISTS idx_quotes_instrument_ts;
DROP INDEX IF EXISTS idx_company_profiles_instrument;
DROP INDEX IF EXISTS idx_financial_income_q_instrument_period;
