CREATE TABLE referral_codes (
    code TEXT PRIMARY KEY,
    is_active BOOLEAN NOT NULL DEFAULT true,
    usage_limit INTEGER,
    used_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_referral_codes_active ON referral_codes(is_active) WHERE is_active = true;
