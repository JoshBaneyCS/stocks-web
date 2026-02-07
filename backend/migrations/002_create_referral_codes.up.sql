-- 002_create_referral_codes.up.sql
-- Referral codes for invite-only signup gating.

CREATE TABLE IF NOT EXISTS referral_codes (
    id          SERIAL PRIMARY KEY,
    code        TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active',  -- 'active' or 'disabled'
    usage_limit INTEGER,                          -- NULL = unlimited
    used_count  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_referral_codes_code UNIQUE (code),
    CONSTRAINT chk_referral_codes_status CHECK (status IN ('active', 'disabled'))
);
