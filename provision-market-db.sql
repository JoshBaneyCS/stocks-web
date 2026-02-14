-- =============================================================================
-- stocks-web: Market Database Provisioning (stocks / TimescaleDB)
-- =============================================================================
-- Run against: postgres://stocksadmin:...@10.36.0.12:5432/stocks
-- Safe to re-run — uses IF NOT EXISTS and ON CONFLICT throughout.
-- =============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 0. Extensions & Schema
-- ---------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE SCHEMA IF NOT EXISTS ingest;

-- ---------------------------------------------------------------------------
-- 1. Catalog Tables
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ingest.asset_classes (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ingest.exchanges (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    mic         TEXT,
    country     TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ingest.currencies (
    id          SERIAL PRIMARY KEY,
    code        TEXT NOT NULL UNIQUE,
    name        TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ingest.sectors (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ingest.industries (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    sector_id   INTEGER REFERENCES ingest.sectors(id) ON UPDATE CASCADE ON DELETE SET NULL,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- 2. Instruments
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ingest.instruments (
    id              BIGSERIAL PRIMARY KEY,
    symbol          TEXT NOT NULL UNIQUE,
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

CREATE INDEX IF NOT EXISTS idx_instruments_symbol_trgm
    ON ingest.instruments USING gin (symbol gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_instruments_name_trgm
    ON ingest.instruments USING gin (name gin_trgm_ops);

-- ---------------------------------------------------------------------------
-- 3. Tracked Instruments
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ingest.tracked_instruments (
    id                      SERIAL PRIMARY KEY,
    instrument_id           BIGINT NOT NULL UNIQUE
                            REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    enable_history          BOOLEAN NOT NULL DEFAULT TRUE,
    enable_intraday         BOOLEAN NOT NULL DEFAULT TRUE,
    enable_after_hours      BOOLEAN NOT NULL DEFAULT FALSE,
    enable_quotes           BOOLEAN NOT NULL DEFAULT TRUE,
    history_backfill_start  DATE,
    priority_rank           INTEGER,
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

CREATE INDEX IF NOT EXISTS idx_tracked_active_next_poll
    ON ingest.tracked_instruments (is_active, next_poll_at)
    WHERE is_active = TRUE;

CREATE INDEX IF NOT EXISTS idx_tracked_active_tier_next_poll
    ON ingest.tracked_instruments (is_active, priority_tier, next_poll_at)
    WHERE is_active = TRUE;

-- ---------------------------------------------------------------------------
-- 4. Quotes Hypertable
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ingest.quotes (
    instrument_id   BIGINT NOT NULL
                    REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    ts              TIMESTAMPTZ NOT NULL,
    collected_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_price      NUMERIC,
    bid             NUMERIC,
    ask             NUMERIC,
    volume          BIGINT,
    market_cap      NUMERIC,
    source          TEXT NOT NULL,
    session         TEXT NOT NULL DEFAULT 'regular',
    raw_payload     JSONB,
    PRIMARY KEY (instrument_id, ts, source)
);

DO $$ BEGIN
    PERFORM create_hypertable('ingest.quotes', 'ts',
        chunk_time_interval => INTERVAL '1 day',
        if_not_exists => TRUE);
END $$;

CREATE INDEX IF NOT EXISTS idx_quotes_instrument_ts
    ON ingest.quotes (instrument_id, ts DESC);

ALTER TABLE ingest.quotes SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'instrument_id,source',
    timescaledb.compress_orderby = 'ts DESC'
);

DO $$ BEGIN
    PERFORM add_compression_policy('ingest.quotes',
        compress_after => INTERVAL '7 days',
        if_not_exists => TRUE);
    PERFORM add_retention_policy('ingest.quotes',
        drop_after => INTERVAL '90 days',
        if_not_exists => TRUE);
END $$;

-- ---------------------------------------------------------------------------
-- 5. Price Bars Hypertable
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ingest.price_bars (
    instrument_id   BIGINT NOT NULL
                    REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    ts              TIMESTAMPTZ NOT NULL,
    interval        TEXT NOT NULL,
    session         TEXT NOT NULL DEFAULT 'regular',
    open            NUMERIC,
    high            NUMERIC,
    low             NUMERIC,
    close           NUMERIC,
    volume          BIGINT,
    vwap            NUMERIC,
    trades          BIGINT,
    collected_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    source          TEXT,
    raw_payload     JSONB,
    UNIQUE (instrument_id, ts, interval, session)
);

DO $$ BEGIN
    PERFORM create_hypertable('ingest.price_bars', 'ts',
        chunk_time_interval => INTERVAL '7 days',
        if_not_exists => TRUE);
END $$;

CREATE INDEX IF NOT EXISTS idx_price_bars_instrument_interval_ts
    ON ingest.price_bars (instrument_id, interval, ts DESC);

CREATE INDEX IF NOT EXISTS idx_price_bars_ts_brin
    ON ingest.price_bars USING brin (ts);

ALTER TABLE ingest.price_bars SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'instrument_id,interval,session',
    timescaledb.compress_orderby = 'ts DESC'
);

DO $$ BEGIN
    PERFORM add_compression_policy('ingest.price_bars',
        compress_after => INTERVAL '30 days',
        if_not_exists => TRUE);
END $$;

-- ---------------------------------------------------------------------------
-- 6. Continuous Aggregates
-- ---------------------------------------------------------------------------

CREATE MATERIALIZED VIEW IF NOT EXISTS ingest.cagg_price_bars_1h
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
    CASE WHEN sum(volume) > 0
         THEN sum(vwap * volume) / sum(volume)
    END AS vwap,
    sum(trades)      AS trades
FROM ingest.price_bars
WHERE interval = '1min'
GROUP BY instrument_id, bucket, session
WITH NO DATA;

DO $$ BEGIN
    PERFORM add_continuous_aggregate_policy('ingest.cagg_price_bars_1h',
        start_offset    => INTERVAL '2 days',
        end_offset      => INTERVAL '1 hour',
        schedule_interval => INTERVAL '15 minutes',
        if_not_exists   => TRUE);
END $$;

CREATE MATERIALIZED VIEW IF NOT EXISTS ingest.cagg_price_bars_1d
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

DO $$ BEGIN
    PERFORM add_continuous_aggregate_policy('ingest.cagg_price_bars_1d',
        start_offset    => INTERVAL '30 days',
        end_offset      => INTERVAL '1 day',
        schedule_interval => INTERVAL '1 hour',
        if_not_exists   => TRUE);
END $$;

-- ---------------------------------------------------------------------------
-- 7. Instrument Latest Snapshot
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ingest.instrument_latest_snapshot (
    instrument_id   BIGINT PRIMARY KEY
                    REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    asof_ts         TIMESTAMPTZ,
    collected_at    TIMESTAMPTZ,
    last_price      NUMERIC,
    bid             NUMERIC,
    ask             NUMERIC,
    volume          BIGINT,
    market_cap      NUMERIC,
    session         TEXT,
    source          TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Fallback view
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
LEFT JOIN LATERAL (
    SELECT ts, collected_at, last_price, bid, ask, volume, market_cap, session, source
    FROM ingest.quotes
    WHERE instrument_id = i.id
      AND ts >= now() - INTERVAL '10 minutes'
    ORDER BY ts DESC
    LIMIT 1
) q ON TRUE
LEFT JOIN LATERAL (
    SELECT ts, collected_at, close, volume, session, source
    FROM ingest.price_bars
    WHERE instrument_id = i.id
      AND interval = '1min'
    ORDER BY ts DESC
    LIMIT 1
) b1 ON q.ts IS NULL
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

CREATE TABLE IF NOT EXISTS ingest.company_profiles (
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

CREATE INDEX IF NOT EXISTS idx_company_profiles_instrument
    ON ingest.company_profiles (instrument_id);

CREATE TABLE IF NOT EXISTS ingest.instrument_metrics (
    id              SERIAL PRIMARY KEY,
    instrument_id   BIGINT NOT NULL UNIQUE
                    REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    market_cap      NUMERIC,
    last_price      NUMERIC,
    beta            NUMERIC,
    vol_avg         BIGINT,
    collected_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- 9. Fundamentals
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS ingest.financial_income_quarterly (
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

CREATE TABLE IF NOT EXISTS ingest.financial_income_as_reported_quarterly (
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

CREATE TABLE IF NOT EXISTS ingest.ingest_state (
    id              SERIAL PRIMARY KEY,
    agent           TEXT NOT NULL,
    instrument_id   BIGINT
                    REFERENCES ingest.instruments(id) ON UPDATE CASCADE ON DELETE CASCADE,
    interval        TEXT,
    cursor          JSONB,
    last_success_at TIMESTAMPTZ,
    last_run_at     TIMESTAMPTZ,
    status          TEXT NOT NULL DEFAULT 'pending',
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (agent, instrument_id, interval)
);

CREATE INDEX IF NOT EXISTS idx_ingest_state_agent_status
    ON ingest.ingest_state (agent, status);

-- ---------------------------------------------------------------------------
-- 11. Performance Indexes
-- ---------------------------------------------------------------------------

CREATE INDEX IF NOT EXISTS idx_instruments_asset_class
    ON ingest.instruments (asset_class_id) WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_instruments_exchange
    ON ingest.instruments (exchange_id) WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_instruments_country
    ON ingest.instruments (country) WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_ingest_state_last_run
    ON ingest.ingest_state (last_run_at DESC NULLS LAST);

CREATE INDEX IF NOT EXISTS idx_financial_income_instrument
    ON ingest.financial_income_quarterly (instrument_id, period_end_date DESC);

-- ---------------------------------------------------------------------------
-- 12. Seed Data
-- ---------------------------------------------------------------------------

INSERT INTO ingest.asset_classes (name) VALUES
    ('equity'), ('etf'), ('index'), ('fund'),
    ('crypto'), ('forex'), ('commodity'), ('other')
ON CONFLICT (name) DO NOTHING;

INSERT INTO ingest.currencies (code, name) VALUES
    ('USD', 'US Dollar'),
    ('EUR', 'Euro'),
    ('GBP', 'British Pound'),
    ('JPY', 'Japanese Yen'),
    ('KRW', 'South Korean Won')
ON CONFLICT (code) DO NOTHING;

COMMIT;
