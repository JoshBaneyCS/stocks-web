-- Seed initial referral codes. Replace these with your own codes.
-- You can also create codes via the admin API: POST /api/admin/referral-codes
INSERT INTO referral_codes (code, is_active, usage_limit) VALUES
    ('EARLY-ACCESS-2026', true, 50),
    ('FOUNDER-INVITE', true, 10)
ON CONFLICT (code) DO NOTHING;
