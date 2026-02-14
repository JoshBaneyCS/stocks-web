-- =============================================================================
-- STOCKS INGEST — Complete Database Schema Reference
-- =============================================================================
-- Generated from: db/bootstrap.sql + db/perf_indexes.sql + backend/data/sql/feature_views.sql
-- Database: TimescaleDB (PostgreSQL + timescaledb extension)
-- Schema: ingest
-- =============================================================================


-- =============================================================================
-- PART 1: CORE SCHEMA (bootstrap.sql)
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 0. Extensions & Schema
-- ---------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS pg_trgm;  -- for ILIKE trigram search on instruments

DROP SCHEMA IF EXISTS ingest CASCADE;
CREATE SCHEMA ingest;

-- ---------------------------------------------------------------------------
-- 1. Catalog Tables (admin-editable reference data)
-- ---------------------------------------------------------------------------

CREATE TABLE ingest.asset_classes (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ingest.exchanges (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    mic         TEXT,                       -- ISO 10383 Market Identifier Code
    country     TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ingest.currencies (
    id          SERIAL PRIMARY KEY,
    code        TEXT NOT NULL UNIQUE,       -- ISO 4217 (USD, EUR, etc.)
    name        TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ingest.sectors (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ingest.industries (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    sector_id   INTEGER REFERENCES ingest.sectors(id) ON UPDATE CASCADE ON DELETE SET NULL,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- 2. Instruments (master registry)
-- ---------------------------------------------------------------------------

CREATE TABLE ingest.instruments (
    id              BIGSERIAL PRIMARY KEY,
    symbol          TEXT NOT NULL UNIQUE,       -- e.g. AAPL, 005930.KS, BTC-USD
    name            TEXT,
    exchange_id     INTEGER REFERENCES ingest.exchanges(id)     ON UPDATE CASCADE ON DELETE SET NULL,
    currency_id     INTEGER REFERENCES ingest.currencies(id)    ON UPDATE CASCADE ON DELETE SET NULL,
    country         TEXT,
    asset_class_id  INTEGER REFERENCES ingest.asset_classes(id) ON UPDATE CASCADE ON DELETE SET NULL,
    sector_id       INTEGER REFERENCES ingest.sectors(id)       ON UPDATE CASCADE ON DELETE SET NULL,
    industry_id     INTEGER REFERENCES ingest.industries(id)    ON UPDATE CASCADE ON DELETE SET NULL,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Trigram index for fast ILIKE search on symbol and name
CREATE INDEX idx_instruments_symbol_trgm ON ingest.instruments USING gin (symbol gin_trgm_ops);
CREATE INDEX idx_instruments_name_trgm   ON ingest.instruments USING gin (name gin_trgm_ops);

-- ---------------------------------------------------------------------------
-- 3. Tracked Instruments (which instruments to ingest and how)
-- ---------------------------------------------------------------------------

CREATE TABLE ingest.tracked_instruments (
    id                      SERIAL PRIMARY KEY,
    instrument_id           BIGINT NOT NULL UNIQUE
                            REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    enable_history          BOOLEAN NOT NULL DEFAULT TRUE,
    enable_intraday         BOOLEAN NOT NULL DEFAULT TRUE,
    enable_after_hours      BOOLEAN NOT NULL DEFAULT FALSE,
    enable_quotes           BOOLEAN NOT NULL DEFAULT TRUE,
    history_backfill_start  DATE,
    priority_rank           INTEGER,
    -- Tiered scheduling fields
    priority_tier           SMALLINT NOT NULL DEFAULT 2
                            CHECK (priority_tier BETWEEN 1 AND 3),
    poll_interval_seconds   INTEGER NOT NULL DEFAULT 60
                            CHECK (poll_interval_seconds >= 10),
    last_polled_at          TIMESTAMPTZ,
    next_poll_at            TIMESTAMPTZ,
    metadata_refresh_seconds INTEGER NOT NULL DEFAULT 3600,
    last_metadata_at        TIMESTAMPTZ,
    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes for the scheduler's "due instruments" query
CREATE INDEX idx_tracked_active_next_poll
    ON ingest.tracked_instruments (is_active, next_poll_at)
    WHERE is_active = TRUE;

CREATE INDEX idx_tracked_active_tier_next_poll
    ON ingest.tracked_instruments (is_active, priority_tier, next_poll_at)
    WHERE is_active = TRUE;

-- ---------------------------------------------------------------------------
-- 4. Quotes Hypertable (high-frequency snapshots)
-- ---------------------------------------------------------------------------
-- Primary key includes source so we can store batch_quote, batch_aftermarket,
-- and crypto_quote for the same instrument at close timestamps without conflict.

CREATE TABLE ingest.quotes (
    instrument_id   BIGINT NOT NULL
                    REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    ts              TIMESTAMPTZ NOT NULL,           -- when the quote applies
    collected_at    TIMESTAMPTZ NOT NULL DEFAULT now(), -- when we fetched it
    last_price      NUMERIC,
    bid             NUMERIC,
    ask             NUMERIC,
    volume          BIGINT,
    market_cap      NUMERIC,
    source          TEXT NOT NULL,                   -- batch_quote, batch_aftermarket, crypto_quote
    session         TEXT NOT NULL DEFAULT 'regular', -- regular | after_hours | 24_7
    raw_payload     JSONB,
    PRIMARY KEY (instrument_id, ts, source)
);

-- Convert to hypertable (chunk interval = 1 day for high write volume)
SELECT create_hypertable('ingest.quotes', 'ts',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- Index for time-descending lookups per instrument
CREATE INDEX idx_quotes_instrument_ts ON ingest.quotes (instrument_id, ts DESC);

-- Compression: segment by instrument_id, order by ts DESC
-- Compress chunks older than 7 days
ALTER TABLE ingest.quotes SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'instrument_id,source',
    timescaledb.compress_orderby = 'ts DESC'
);

SELECT add_compression_policy('ingest.quotes',
    compress_after => INTERVAL '7 days',
    if_not_exists => TRUE
);

-- Retention: drop raw quotes older than 90 days
SELECT add_retention_policy('ingest.quotes',
    drop_after => INTERVAL '90 days',
    if_not_exists => TRUE
);

-- ---------------------------------------------------------------------------
-- 5. Price Bars Hypertable (canonical OHLCV bars)
-- ---------------------------------------------------------------------------

CREATE TABLE ingest.price_bars (
    instrument_id   BIGINT NOT NULL
                    REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    ts              TIMESTAMPTZ NOT NULL,           -- bar timestamp (minute/hour/day boundary)
    interval        TEXT NOT NULL,                   -- '1min', '5min', '1h', '1d'
    session         TEXT NOT NULL DEFAULT 'regular', -- regular | after_hours | 24_7
    open            NUMERIC,
    high            NUMERIC,
    low             NUMERIC,
    close           NUMERIC,
    volume          BIGINT,
    vwap            NUMERIC,
    trades          BIGINT,
    collected_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    source          TEXT,                            -- derived_from_quotes, eod_bulk, intraday_1min
    raw_payload     JSONB,
    UNIQUE (instrument_id, ts, interval, session)
);

-- Convert to hypertable (chunk interval = 7 days)
SELECT create_hypertable('ingest.price_bars', 'ts',
    chunk_time_interval => INTERVAL '7 days',
    if_not_exists => TRUE
);

-- Primary lookup index: instrument + interval + time descending
CREATE INDEX idx_price_bars_instrument_interval_ts
    ON ingest.price_bars (instrument_id, interval, ts DESC);

-- BRIN index on ts for large sequential scans (e.g., exports)
CREATE INDEX idx_price_bars_ts_brin
    ON ingest.price_bars USING brin (ts);

-- Compression: segment by instrument_id, interval, session; order by ts DESC
-- Compress chunks older than 30 days
ALTER TABLE ingest.price_bars SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'instrument_id,interval,session',
    timescaledb.compress_orderby = 'ts DESC'
);

SELECT add_compression_policy('ingest.price_bars',
    compress_after => INTERVAL '30 days',
    if_not_exists => TRUE
);

-- Retention: DO NOT delete price_bars by default (we want maximum history).
-- Uncomment the following to enable retention if storage becomes a concern:
-- SELECT add_retention_policy('ingest.price_bars',
--     drop_after => INTERVAL '5 years',
--     if_not_exists => TRUE
-- );

-- ---------------------------------------------------------------------------
-- 6. Continuous Aggregates (fast rollups from 1min bars)
-- ---------------------------------------------------------------------------

-- 1-hour bars from 1-minute bars
CREATE MATERIALIZED VIEW ingest.cagg_price_bars_1h
WITH (timescaledb.continuous) AS
SELECT
    instrument_id,
    time_bucket('1 hour', ts) AS bucket,
    session,
    first(open, ts)  AS open,
    max(high)        AS high,
    min(low)         AS low,
    last(close, ts)  AS close,
    sum(volume)      AS volume,
    -- VWAP: weighted average using volume; NULL if no volume
    CASE WHEN sum(volume) > 0
         THEN sum(vwap * volume) / sum(volume)
    END AS vwap,
    sum(trades)      AS trades
FROM ingest.price_bars
WHERE interval = '1min'
GROUP BY instrument_id, bucket, session
WITH NO DATA;

-- Refresh policy: every 15 minutes, look back 2 days
SELECT add_continuous_aggregate_policy('ingest.cagg_price_bars_1h',
    start_offset    => INTERVAL '2 days',
    end_offset      => INTERVAL '1 hour',
    schedule_interval => INTERVAL '15 minutes',
    if_not_exists   => TRUE
);

-- 1-day bars from 1-minute bars
CREATE MATERIALIZED VIEW ingest.cagg_price_bars_1d
WITH (timescaledb.continuous) AS
SELECT
    instrument_id,
    time_bucket('1 day', ts) AS bucket,
    session,
    first(open, ts)  AS open,
    max(high)        AS high,
    min(low)         AS low,
    last(close, ts)  AS close,
    sum(volume)      AS volume,
    CASE WHEN sum(volume) > 0
         THEN sum(vwap * volume) / sum(volume)
    END AS vwap,
    sum(trades)      AS trades
FROM ingest.price_bars
WHERE interval = '1min'
GROUP BY instrument_id, bucket, session
WITH NO DATA;

-- Refresh policy: every hour, look back 30 days
SELECT add_continuous_aggregate_policy('ingest.cagg_price_bars_1d',
    start_offset    => INTERVAL '30 days',
    end_offset      => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists   => TRUE
);

-- ---------------------------------------------------------------------------
-- 7. Instrument Latest Snapshot (maintained by ingest-live worker)
-- ---------------------------------------------------------------------------

CREATE TABLE ingest.instrument_latest_snapshot (
    instrument_id   BIGINT PRIMARY KEY
                    REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    asof_ts         TIMESTAMPTZ,            -- timestamp of the datapoint
    collected_at    TIMESTAMPTZ,            -- when we fetched it
    last_price      NUMERIC,
    bid             NUMERIC,
    ask             NUMERIC,
    volume          BIGINT,
    market_cap      NUMERIC,
    session         TEXT,
    source          TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Fallback view: computes "latest" from raw data when snapshot table is stale.
-- Preference order: latest quote (within 10 min) > latest 1min bar > latest 1d bar close.
CREATE OR REPLACE VIEW ingest.v_instrument_latest_snapshot AS
SELECT
    i.id AS instrument_id,
    COALESCE(q.ts,      b1.ts,    bd.ts)    AS asof_ts,
    COALESCE(q.collected_at, b1.collected_at, bd.collected_at) AS collected_at,
    COALESCE(q.last_price,   b1.close, bd.close) AS last_price,
    q.bid,
    q.ask,
    COALESCE(q.volume::BIGINT, b1.volume, bd.volume) AS volume,
    q.market_cap,
    COALESCE(q.session,  b1.session, bd.session) AS session,
    COALESCE(q.source,   b1.source,  bd.source)  AS source
FROM ingest.instruments i
-- Latest quote within last 10 minutes
LEFT JOIN LATERAL (
    SELECT ts, collected_at, last_price, bid, ask, volume, market_cap, session, source
    FROM ingest.quotes
    WHERE instrument_id = i.id
      AND ts >= now() - INTERVAL '10 minutes'
    ORDER BY ts DESC
    LIMIT 1
) q ON TRUE
-- Latest 1min bar (fallback)
LEFT JOIN LATERAL (
    SELECT ts, collected_at, close, volume, session, source
    FROM ingest.price_bars
    WHERE instrument_id = i.id
      AND interval = '1min'
    ORDER BY ts DESC
    LIMIT 1
) b1 ON q.ts IS NULL
-- Latest 1d bar close (last resort)
LEFT JOIN LATERAL (
    SELECT ts, collected_at, close, volume, session, source
    FROM ingest.price_bars
    WHERE instrument_id = i.id
      AND interval = '1d'
    ORDER BY ts DESC
    LIMIT 1
) bd ON q.ts IS NULL AND b1.ts IS NULL;

-- ---------------------------------------------------------------------------
-- 8. Company Profiles & Instrument Metrics
-- ---------------------------------------------------------------------------

CREATE TABLE ingest.company_profiles (
    id              SERIAL PRIMARY KEY,
    instrument_id   BIGINT NOT NULL UNIQUE
                    REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    ceo             TEXT,
    description     TEXT,
    website         TEXT,
    ipo_date        DATE,
    employees       INTEGER,
    raw_payload     JSONB,
    collected_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_company_profiles_instrument ON ingest.company_profiles (instrument_id);

CREATE TABLE ingest.instrument_metrics (
    id              SERIAL PRIMARY KEY,
    instrument_id   BIGINT NOT NULL UNIQUE
                    REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    market_cap      NUMERIC,
    last_price      NUMERIC,
    beta            NUMERIC,
    vol_avg         BIGINT,                     -- average volume
    collected_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- 9. Fundamentals Tables
-- ---------------------------------------------------------------------------

CREATE TABLE ingest.financial_income_quarterly (
    instrument_id       BIGINT NOT NULL
                        REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    period_end_date     DATE NOT NULL,
    calendar_year       INTEGER,
    period              TEXT,                   -- Q1, Q2, Q3, Q4
    revenue             NUMERIC,
    gross_profit        NUMERIC,
    operating_income    NUMERIC,
    net_income          NUMERIC,
    eps                 NUMERIC,
    raw_payload         JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (instrument_id, period_end_date)
);

CREATE TABLE ingest.financial_income_as_reported_quarterly (
    instrument_id       BIGINT NOT NULL
                        REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    period_end_date     DATE NOT NULL,
    calendar_year       INTEGER,
    period              TEXT,
    revenue             NUMERIC,
    gross_profit        NUMERIC,
    operating_income    NUMERIC,
    net_income          NUMERIC,
    eps                 NUMERIC,
    raw_payload         JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (instrument_id, period_end_date)
);

-- ---------------------------------------------------------------------------
-- 10. Ingestion Job State
-- ---------------------------------------------------------------------------

CREATE TABLE ingest.ingest_state (
    id              SERIAL PRIMARY KEY,
    agent           TEXT NOT NULL,              -- history_backfill, metadata_refresh, fundamentals_refresh, live, bars
    instrument_id   BIGINT
                    REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    interval        TEXT,                       -- '1d', '1min', etc. (nullable for global jobs)
    cursor          JSONB,                      -- resumable state (from_date, last_processed, scope, etc.)
    last_success_at TIMESTAMPTZ,
    last_run_at     TIMESTAMPTZ,
    status          TEXT NOT NULL DEFAULT 'pending', -- pending, requested, running, completed, failed
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (agent, instrument_id, interval)
);

CREATE INDEX idx_ingest_state_agent_status ON ingest.ingest_state (agent, status);

-- ---------------------------------------------------------------------------
-- 11. Seed Data
-- ---------------------------------------------------------------------------

-- Asset classes
INSERT INTO ingest.asset_classes (name) VALUES
    ('equity'),
    ('etf'),
    ('index'),
    ('fund'),
    ('crypto'),
    ('forex'),
    ('commodity'),
    ('other');

-- Common currencies
INSERT INTO ingest.currencies (code, name) VALUES
    ('USD', 'US Dollar'),
    ('EUR', 'Euro'),
    ('GBP', 'British Pound'),
    ('JPY', 'Japanese Yen'),
    ('KRW', 'South Korean Won');


-- =============================================================================
-- PART 2: PERFORMANCE INDEXES & TUNING (perf_indexes.sql)
-- =============================================================================
-- Run AFTER core schema. All statements use IF NOT EXISTS or are safe to re-run.

-- ---------------------------------------------------------------------------
-- 1. Chunk Interval Tuning
-- ---------------------------------------------------------------------------
-- Quotes: 1 day chunks (high write volume, 90-day retention)
SELECT set_chunk_time_interval('ingest.quotes', INTERVAL '1 day');

-- Price Bars: 7 day chunks (moderate write volume, no retention)
SELECT set_chunk_time_interval('ingest.price_bars', INTERVAL '7 days');

-- ---------------------------------------------------------------------------
-- 2. Additional Indexes for Admin API Performance
-- ---------------------------------------------------------------------------

-- Instrument lookup by asset class (for filtered list queries)
CREATE INDEX IF NOT EXISTS idx_instruments_asset_class
    ON ingest.instruments (asset_class_id)
    WHERE is_active = true;

-- Instrument lookup by exchange
CREATE INDEX IF NOT EXISTS idx_instruments_exchange
    ON ingest.instruments (exchange_id)
    WHERE is_active = true;

-- Instrument lookup by country
CREATE INDEX IF NOT EXISTS idx_instruments_country
    ON ingest.instruments (country)
    WHERE is_active = true;

-- Ingest state: fast lookup of recent jobs
CREATE INDEX IF NOT EXISTS idx_ingest_state_last_run
    ON ingest.ingest_state (last_run_at DESC NULLS LAST);

-- Financial income: fast lookup by instrument + date
CREATE INDEX IF NOT EXISTS idx_financial_income_instrument
    ON ingest.financial_income_quarterly (instrument_id, period_end_date DESC);

-- ---------------------------------------------------------------------------
-- 3. Continuous Aggregate Indexes
-- ---------------------------------------------------------------------------
-- These improve queries on the hourly and daily rollups

CREATE INDEX IF NOT EXISTS idx_cagg_1h_instrument_bucket
    ON ingest.cagg_price_bars_1h (instrument_id, bucket DESC);

CREATE INDEX IF NOT EXISTS idx_cagg_1d_instrument_bucket
    ON ingest.cagg_price_bars_1d (instrument_id, bucket DESC);

-- ---------------------------------------------------------------------------
-- 4. Materialized View for Admin Instruments List
-- ---------------------------------------------------------------------------
-- Precomputes the 7-table join for the instruments list endpoint.
-- Refresh periodically (e.g., every 30 seconds) via pg_cron or from the admin API.

CREATE MATERIALIZED VIEW IF NOT EXISTS ingest.mv_admin_instruments AS
SELECT
    i.id,
    i.symbol,
    i.name,
    i.country,
    i.is_active,
    ac.name  AS asset_class,
    ex.name  AS exchange,
    cu.code  AS currency,
    se.name  AS sector,
    ind.name AS industry,
    t.id IS NOT NULL AND COALESCE(t.is_active, false) AS tracked,
    t.priority_tier,
    t.poll_interval_seconds,
    t.enable_intraday,
    t.enable_history,
    t.enable_after_hours,
    t.last_polled_at,
    t.next_poll_at,
    ls.last_price,
    ls.bid,
    ls.ask,
    ls.volume,
    ls.market_cap,
    ls.collected_at,
    ls.asof_ts,
    ls.session
FROM ingest.instruments i
LEFT JOIN ingest.tracked_instruments t   ON t.instrument_id = i.id
LEFT JOIN ingest.instrument_latest_snapshot ls ON ls.instrument_id = i.id
LEFT JOIN ingest.asset_classes ac ON ac.id = i.asset_class_id
LEFT JOIN ingest.exchanges ex    ON ex.id = i.exchange_id
LEFT JOIN ingest.currencies cu   ON cu.id = i.currency_id
LEFT JOIN ingest.sectors se      ON se.id = i.sector_id
LEFT JOIN ingest.industries ind  ON ind.id = i.industry_id
WHERE i.is_active = true;

-- Index the materialized view for fast searches
CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_admin_instruments_id
    ON ingest.mv_admin_instruments (id);
CREATE INDEX IF NOT EXISTS idx_mv_admin_instruments_symbol
    ON ingest.mv_admin_instruments (symbol);

-- To refresh: REFRESH MATERIALIZED VIEW CONCURRENTLY ingest.mv_admin_instruments;


-- =============================================================================
-- PART 3: AI FEATURE VIEWS (feature_views.sql)
-- =============================================================================
-- ML-ready feature extraction views with windowed technical indicators.

-- ---------------------------------------------------------------------------
-- 1. Base 1-Minute Feature View
-- ---------------------------------------------------------------------------
-- Joins 1min bars with instrument metadata and computes lag returns,
-- rolling volatility, and VWAP deviation.

CREATE OR REPLACE VIEW ingest.v_features_1min AS
SELECT
    pb.instrument_id,
    i.symbol,
    pb.ts,
    pb.session,
    pb.open,
    pb.high,
    pb.low,
    pb.close,
    pb.volume,
    pb.vwap,

    -- Returns
    (pb.close - LAG(pb.close, 1) OVER w) / NULLIF(LAG(pb.close, 1) OVER w, 0) AS return_1m,
    (pb.close - LAG(pb.close, 5) OVER w) / NULLIF(LAG(pb.close, 5) OVER w, 0) AS return_5m,
    (pb.close - LAG(pb.close, 15) OVER w) / NULLIF(LAG(pb.close, 15) OVER w, 0) AS return_15m,
    (pb.close - LAG(pb.close, 60) OVER w) / NULLIF(LAG(pb.close, 60) OVER w, 0) AS return_1h,

    -- Log returns
    LN(pb.close / NULLIF(LAG(pb.close, 1) OVER w, 0)) AS log_return_1m,

    -- Rolling volatility (standard deviation of 1m returns over N bars)
    STDDEV(
        (pb.close - LAG(pb.close, 1) OVER w) / NULLIF(LAG(pb.close, 1) OVER w, 0)
    ) OVER (PARTITION BY pb.instrument_id ORDER BY pb.ts ROWS BETWEEN 29 PRECEDING AND CURRENT ROW) AS rolling_vol_30m,

    -- VWAP deviation
    CASE WHEN pb.vwap > 0
         THEN (pb.close - pb.vwap) / pb.vwap
         ELSE NULL
    END AS vwap_deviation,

    -- Rolling high/low
    MAX(pb.high) OVER (PARTITION BY pb.instrument_id ORDER BY pb.ts ROWS BETWEEN 59 PRECEDING AND CURRENT ROW) AS rolling_high_1h,
    MIN(pb.low) OVER (PARTITION BY pb.instrument_id ORDER BY pb.ts ROWS BETWEEN 59 PRECEDING AND CURRENT ROW) AS rolling_low_1h,

    -- Volume moving average
    AVG(pb.volume) OVER (PARTITION BY pb.instrument_id ORDER BY pb.ts ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) AS vol_ma_20,

    -- Metadata
    ac.name AS asset_class,
    se.name AS sector,
    ind.name AS industry,
    cu.code AS currency,
    i.country

FROM ingest.price_bars pb
JOIN ingest.instruments i ON i.id = pb.instrument_id
LEFT JOIN ingest.asset_classes ac ON ac.id = i.asset_class_id
LEFT JOIN ingest.sectors se ON se.id = i.sector_id
LEFT JOIN ingest.industries ind ON ind.id = i.industry_id
LEFT JOIN ingest.currencies cu ON cu.id = i.currency_id
WHERE pb.interval = '1min'
WINDOW w AS (PARTITION BY pb.instrument_id ORDER BY pb.ts);

-- ---------------------------------------------------------------------------
-- 2. Daily Feature View
-- ---------------------------------------------------------------------------

CREATE OR REPLACE VIEW ingest.v_features_1d AS
SELECT
    pb.instrument_id,
    i.symbol,
    pb.ts,
    pb.session,
    pb.open,
    pb.high,
    pb.low,
    pb.close,
    pb.volume,

    -- Returns
    (pb.close - LAG(pb.close, 1) OVER w) / NULLIF(LAG(pb.close, 1) OVER w, 0) AS return_1d,
    (pb.close - LAG(pb.close, 5) OVER w) / NULLIF(LAG(pb.close, 5) OVER w, 0) AS return_5d,
    (pb.close - LAG(pb.close, 20) OVER w) / NULLIF(LAG(pb.close, 20) OVER w, 0) AS return_20d,

    -- Log return
    LN(pb.close / NULLIF(LAG(pb.close, 1) OVER w, 0)) AS log_return_1d,

    -- SMAs
    AVG(pb.close) OVER (PARTITION BY pb.instrument_id ORDER BY pb.ts ROWS BETWEEN 4 PRECEDING AND CURRENT ROW) AS sma_5,
    AVG(pb.close) OVER (PARTITION BY pb.instrument_id ORDER BY pb.ts ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) AS sma_20,
    AVG(pb.close) OVER (PARTITION BY pb.instrument_id ORDER BY pb.ts ROWS BETWEEN 49 PRECEDING AND CURRENT ROW) AS sma_50,

    -- Rolling volatility (20-day)
    STDDEV(
        (pb.close - LAG(pb.close, 1) OVER w) / NULLIF(LAG(pb.close, 1) OVER w, 0)
    ) OVER (PARTITION BY pb.instrument_id ORDER BY pb.ts ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) AS rolling_vol_20d,

    -- Average True Range (14-day approximation)
    AVG(pb.high - pb.low) OVER (PARTITION BY pb.instrument_id ORDER BY pb.ts ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) AS atr_14,

    -- Volume moving average
    AVG(pb.volume) OVER (PARTITION BY pb.instrument_id ORDER BY pb.ts ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) AS vol_ma_20,

    -- Bollinger Band width (20-day, 2 std)
    2 * STDDEV(pb.close) OVER (PARTITION BY pb.instrument_id ORDER BY pb.ts ROWS BETWEEN 19 PRECEDING AND CURRENT ROW)
        / NULLIF(AVG(pb.close) OVER (PARTITION BY pb.instrument_id ORDER BY pb.ts ROWS BETWEEN 19 PRECEDING AND CURRENT ROW), 0) AS bb_width_20,

    -- RSI proxy: % of up days in last 14
    (SUM(CASE WHEN pb.close > LAG(pb.close, 1) OVER w THEN 1 ELSE 0 END)
        OVER (PARTITION BY pb.instrument_id ORDER BY pb.ts ROWS BETWEEN 13 PRECEDING AND CURRENT ROW))::NUMERIC / 14.0 AS rsi_proxy_14,

    -- Metadata
    ac.name AS asset_class,
    se.name AS sector,
    ind.name AS industry,
    i.country,

    -- Fundamentals (last known quarterly before this bar)
    fiq.revenue AS q_revenue,
    fiq.net_income AS q_net_income,
    fiq.eps AS q_eps,
    im.market_cap,
    CASE WHEN fiq.net_income IS NOT NULL AND fiq.net_income != 0
         THEN im.market_cap / (fiq.net_income * 4)  -- annualized P/E proxy
         ELSE NULL
    END AS pe_proxy,

    -- Labels (forward returns - THESE LEAK FUTURE DATA, use only as targets)
    LEAD(pb.close, 1) OVER w / NULLIF(pb.close, 0) - 1 AS fwd_return_1d,
    LEAD(pb.close, 5) OVER w / NULLIF(pb.close, 0) - 1 AS fwd_return_5d,
    LEAD(pb.close, 20) OVER w / NULLIF(pb.close, 0) - 1 AS fwd_return_20d,
    CASE WHEN LEAD(pb.close, 1) OVER w > pb.close THEN 1 ELSE 0 END AS fwd_direction_1d

FROM ingest.price_bars pb
JOIN ingest.instruments i ON i.id = pb.instrument_id
LEFT JOIN ingest.asset_classes ac ON ac.id = i.asset_class_id
LEFT JOIN ingest.sectors se ON se.id = i.sector_id
LEFT JOIN ingest.industries ind ON ind.id = i.industry_id
LEFT JOIN ingest.instrument_metrics im ON im.instrument_id = i.id
-- Join last known quarterly fundamentals
LEFT JOIN LATERAL (
    SELECT revenue, net_income, eps
    FROM ingest.financial_income_quarterly
    WHERE instrument_id = pb.instrument_id
      AND period_end_date < pb.ts::date
    ORDER BY period_end_date DESC
    LIMIT 1
) fiq ON TRUE
WHERE pb.interval = '1d'
WINDOW w AS (PARTITION BY pb.instrument_id ORDER BY pb.ts);

-- =============================================================================
-- END OF SCHEMA REFERENCE
-- =============================================================================
