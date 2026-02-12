You are Claude Code acting as a Principal Data Engineer.

Implement DB migrations for the web app (NOT for the existing market data tables, which already exist).

Add tables:
- users
  - id (uuid pk)
  - email (unique)
  - password_hash
  - first_name
  - last_name
  - created_at, updated_at
- referral_codes
  - code (pk or unique)
  - is_active
  - usage_limit (nullable)
  - used_count
  - created_at, updated_at
- user_favorites
  - user_id (fk users)
  - instrument_id (fk instruments)
  - created_at
  - unique(user_id, instrument_id)
- sessions or refresh_tokens (depending on backend auth approach)
  - secure, expiring, revocable

Add indexes for performance:
- instruments(symbol)
- price_bars(instrument_id, interval, ts desc)
- quotes(instrument_id, ts desc)
- company_profiles(instrument_id)
- financial_income_quarterly(instrument_id, period_end_date desc)
- user_favorites(user_id)
- user_favorites(instrument_id)

Create optional views:
- latest_quote_per_instrument (view or materialized view)
- latest_bar_per_instrument_interval

Also add a CLI or SQL seed mechanism:
- create referral codes from env var list or a CLI command
- MUST support “I define the referral codes”

Output rules:
- Each migration file is output one at a time as its own artifact.
- After each file, ask to continue.
