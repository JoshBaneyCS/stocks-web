CREATE OR REPLACE VIEW latest_quote_per_instrument AS
SELECT DISTINCT ON (instrument_id)
    instrument_id, ts, last_price, bid, ask, volume, source
FROM quotes
ORDER BY instrument_id, ts DESC;

CREATE OR REPLACE VIEW latest_bar_per_instrument_interval AS
SELECT DISTINCT ON (instrument_id, interval)
    instrument_id, interval, ts, open, high, low, close, volume
FROM price_bars
ORDER BY instrument_id, interval, ts DESC;
