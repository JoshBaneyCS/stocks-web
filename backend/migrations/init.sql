-- =============================================================================
-- stocks-web: Auth Schema (stocks database)
-- =============================================================================
-- Creates auth tables in the "auth" schema of the stocks TimescaleDB database.
-- Market data lives in the "ingest" schema.
--
-- Safe to run multiple times — uses IF NOT EXISTS throughout.
--
-- Run with:  psql -d stocks -f init.sql
-- =============================================================================

-- Create auth schema
CREATE SCHEMA IF NOT EXISTS auth;

SET search_path TO auth;

BEGIN;

-- ---------------------------------------------------------------------------
-- Extensions (pgcrypto needed for gen_random_uuid)
-- ---------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS "pgcrypto" SCHEMA auth;

-- ===========================================================================
-- Auth & User Tables
-- ===========================================================================

-- 1. users — application user accounts (UUID primary key)
CREATE TABLE IF NOT EXISTS auth.users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        UNIQUE NOT NULL,
    password_hash TEXT        NOT NULL,
    first_name    TEXT        NOT NULL,
    last_name     TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON auth.users(email);

-- 2. referral_codes — gated signup codes
CREATE TABLE IF NOT EXISTS auth.referral_codes (
    code       TEXT        PRIMARY KEY,
    is_active  BOOLEAN     NOT NULL DEFAULT true,
    usage_limit INTEGER,
    used_count INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_referral_codes_active
    ON auth.referral_codes(is_active) WHERE is_active = true;

-- 3. user_favorites — links users to their favorite instruments
--    instrument_id references instruments in the ingest schema (no FK across schemas)
CREATE TABLE IF NOT EXISTS auth.user_favorites (
    user_id       UUID   NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    instrument_id BIGINT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, instrument_id)
);

CREATE INDEX IF NOT EXISTS idx_user_favorites_user       ON auth.user_favorites(user_id);
CREATE INDEX IF NOT EXISTS idx_user_favorites_instrument ON auth.user_favorites(instrument_id);

-- 4. refresh_tokens — JWT refresh token storage
CREATE TABLE IF NOT EXISTS auth.refresh_tokens (
    id         SERIAL      PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash TEXT        UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user    ON auth.refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash    ON auth.refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires ON auth.refresh_tokens(expires_at);

-- ===========================================================================
-- Seed Data
-- ===========================================================================

-- Default referral codes (safe to re-run)
INSERT INTO auth.referral_codes (code, is_active, usage_limit) VALUES
    ('EARLY-ACCESS-2026', true, 50),
    ('FOUNDER-INVITE', true, 10)
ON CONFLICT (code) DO NOTHING;

COMMIT;
