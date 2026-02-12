-- =============================================================================
-- stocks-web: Full Database Schema
-- =============================================================================
-- This file creates ALL tables needed by both the ingest system
-- (stocks-intraday) and the web application (stocks-web).
--
-- Safe to run multiple times — uses IF NOT EXISTS throughout.
--
-- Run with:  psql -d stock-data -f init.sql
-- =============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Extensions
-- ---------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ===========================================================================
-- PART 1: Market-Data Tables (owned by the ingest system)
-- ===========================================================================

-- 1. instruments — master registry of all financial instruments
CREATE TABLE IF NOT EXISTS instruments (
    id          SERIAL          PRIMARY KEY,
    symbol      VARCHAR(30)     NOT NULL,
    name        TEXT,
    exchange    VARCHAR(50),
    currency    VARCHAR(10),
    country     VARCHAR(10),
    asset_class VARCHAR(20)     NOT NULL DEFAULT 'equity',
    is_active   BOOLEAN         NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_instruments_symbol UNIQUE (symbol)
);

-- 2. tracked_universe — active subset being tracked by the ingest agents
CREATE TABLE IF NOT EXISTS tracked_universe (
    id               SERIAL      PRIMARY KEY,
    instrument_id    INTEGER     NOT NULL REFERENCES instruments(id),
    priority_rank    INTEGER     NOT NULL,
    selection_reason TEXT        NOT NULL,
    is_active        BOOLEAN     NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_tracked_universe_instrument UNIQUE (instrument_id)
);

-- 3. instrument_metrics — market cap & last price snapshot
CREATE TABLE IF NOT EXISTS instrument_metrics (
    id            SERIAL      PRIMARY KEY,
    instrument_id INTEGER     NOT NULL REFERENCES instruments(id),
    market_cap    NUMERIC,
    last_price    NUMERIC,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_instrument_metrics_instrument UNIQUE (instrument_id)
);

-- 4. price_bars — OHLCV bars at 1d / 1h / 1min intervals
CREATE TABLE IF NOT EXISTS price_bars (
    id            SERIAL      PRIMARY KEY,
    instrument_id INTEGER     NOT NULL REFERENCES instruments(id),
    ts            TIMESTAMPTZ NOT NULL,
    interval      VARCHAR(10) NOT NULL,
    open          NUMERIC,
    high          NUMERIC,
    low           NUMERIC,
    close         NUMERIC,
    volume        BIGINT,
    adj_close     NUMERIC,
    session       TEXT,
    CONSTRAINT uq_pricebar_instrument_ts_interval
        UNIQUE (instrument_id, ts, interval)
);

CREATE INDEX IF NOT EXISTS ix_pricebar_instrument_interval_ts
    ON price_bars (instrument_id, interval, ts DESC);

-- 5. ingest_state — backfill / ingestion progress per agent
CREATE TABLE IF NOT EXISTS ingest_state (
    id                   SERIAL      PRIMARY KEY,
    agent                VARCHAR(50) NOT NULL,
    instrument_id        INTEGER     NOT NULL REFERENCES instruments(id),
    interval             VARCHAR(20) NOT NULL,
    earliest_ts_present  TIMESTAMPTZ,
    cursor               JSONB,
    last_run_at          TIMESTAMPTZ,
    status               VARCHAR(20) NOT NULL DEFAULT 'pending',
    error                TEXT,
    CONSTRAINT uq_ingest_state_agent_instrument_interval
        UNIQUE (agent, instrument_id, interval)
);

-- 6. quotes — raw quote snapshots (batch/crypto every 15s)
CREATE TABLE IF NOT EXISTS quotes (
    id            SERIAL      PRIMARY KEY,
    instrument_id INTEGER     NOT NULL REFERENCES instruments(id),
    ts            TIMESTAMPTZ NOT NULL,
    last_price    NUMERIC,
    bid           NUMERIC,
    ask           NUMERIC,
    volume        BIGINT,
    source        TEXT        NOT NULL,
    CONSTRAINT uq_quotes_instrument_ts_source
        UNIQUE (instrument_id, ts, source)
);

CREATE INDEX IF NOT EXISTS ix_quotes_instrument_ts
    ON quotes (instrument_id, ts);

-- 7. financial_income_quarterly — standardised quarterly income statements
CREATE TABLE IF NOT EXISTS financial_income_quarterly (
    id              SERIAL      PRIMARY KEY,
    instrument_id   INTEGER     NOT NULL REFERENCES instruments(id),
    period_end_date DATE        NOT NULL,
    calendar_year   INTEGER,
    period          VARCHAR(10),
    revenue         NUMERIC,
    gross_profit    NUMERIC,
    operating_income NUMERIC,
    net_income      NUMERIC,
    eps             NUMERIC,
    raw_payload     JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_financial_income_quarterly_instrument_period
        UNIQUE (instrument_id, period_end_date)
);

-- 8. financial_income_as_reported_quarterly — raw filing data
CREATE TABLE IF NOT EXISTS financial_income_as_reported_quarterly (
    id              SERIAL      PRIMARY KEY,
    instrument_id   INTEGER     NOT NULL REFERENCES instruments(id),
    period_end_date DATE        NOT NULL,
    calendar_year   INTEGER,
    period          VARCHAR(10),
    raw_payload     JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_financial_as_reported_instrument_period
        UNIQUE (instrument_id, period_end_date)
);

-- 9. company_profiles — cached company metadata
CREATE TABLE IF NOT EXISTS company_profiles (
    id            SERIAL      PRIMARY KEY,
    instrument_id INTEGER     NOT NULL REFERENCES instruments(id),
    market_cap    NUMERIC,
    sector        TEXT,
    industry      TEXT,
    exchange      VARCHAR(50),
    country       VARCHAR(10),
    currency      VARCHAR(10),
    raw_payload   JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_company_profiles_instrument UNIQUE (instrument_id)
);

-- ===========================================================================
-- PART 2: Web-Application Tables
-- ===========================================================================

-- 10. users — application user accounts (UUID primary key)
CREATE TABLE IF NOT EXISTS users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        UNIQUE NOT NULL,
    password_hash TEXT        NOT NULL,
    first_name    TEXT        NOT NULL,
    last_name     TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- 11. referral_codes — gated signup codes
CREATE TABLE IF NOT EXISTS referral_codes (
    code       TEXT        PRIMARY KEY,
    is_active  BOOLEAN     NOT NULL DEFAULT true,
    usage_limit INTEGER,
    used_count INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_referral_codes_active
    ON referral_codes(is_active) WHERE is_active = true;

-- 12. user_favorites — links users to their favorite instruments
CREATE TABLE IF NOT EXISTS user_favorites (
    user_id       UUID    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    instrument_id INTEGER NOT NULL REFERENCES instruments(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, instrument_id)
);

CREATE INDEX IF NOT EXISTS idx_user_favorites_user       ON user_favorites(user_id);
CREATE INDEX IF NOT EXISTS idx_user_favorites_instrument ON user_favorites(instrument_id);

-- 13. refresh_tokens — JWT refresh token storage
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         SERIAL      PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT        UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user    ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash    ON refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires ON refresh_tokens(expires_at);

-- ===========================================================================
-- PART 3: Performance Indexes on Market-Data Tables
-- ===========================================================================

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

-- ===========================================================================
-- PART 4: Views
-- ===========================================================================

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

-- ===========================================================================
-- PART 5: Seed Data
-- ===========================================================================

-- Default referral codes (safe to re-run)
INSERT INTO referral_codes (code, is_active, usage_limit) VALUES
    ('EARLY-ACCESS-2026', true, 50),
    ('FOUNDER-INVITE', true, 10)
ON CONFLICT (code) DO NOTHING;

-- ===========================================================================
-- PART 6: Alembic Compatibility
-- ===========================================================================
-- If the ingest system (stocks-intraday) uses Alembic and you run this file
-- BEFORE running `alembic upgrade head`, insert the latest revision so
-- Alembic knows the tables already exist.

CREATE TABLE IF NOT EXISTS alembic_version (
    version_num VARCHAR(32) NOT NULL,
    CONSTRAINT alembic_version_pkc PRIMARY KEY (version_num)
);

INSERT INTO alembic_version (version_num) VALUES ('009')
ON CONFLICT (version_num) DO NOTHING;

COMMIT;
