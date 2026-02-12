You are Claude Code acting as a Sr Principal Full Stack Engineer focusing on backend performance and correctness.

Build the Go backend service under:
- backend/

Must include:
- REST API for auth, instruments, prices, profiles, fundamentals, favorites
- SSE streaming for live updates (minute-by-minute) based on `quotes` and/or latest `price_bars`
- DB migrations for web-app tables (users/referrals/favorites/sessions)
- Very fast query patterns with proper indexes
- Strict input validation and secure auth

Database:
- Connect to Postgres `stock-data`.
- Market data tables already exist per schema:
  - instruments(id, symbol, name, exchange, currency, country, asset_class, is_active, created_at, updated_at)
  - tracked_universe(instrument_id, priority_rank, ...)
  - instrument_metrics(instrument_id, market_cap, last_price, updated_at)
  - price_bars(instrument_id, ts, interval, ohlcv, adj_close, session)
  - quotes(instrument_id, ts, last_price, bid, ask, volume, source)
  - company_profiles(instrument_id, market_cap, sector, industry, exchange, country, currency, raw_payload, ...)
  - financial_income_quarterly(instrument_id, period_end_date, revenue, net_income, eps, raw_payload, ...)
  - financial_income_as_reported_quarterly(...)
- You will add new tables for the web app:
  - users
  - referral_codes
  - user_favorites
  - sessions (if using cookie sessions) OR refresh_tokens (if JWT refresh)
- You may add views/materialized views for “latest price per instrument”.

Auth requirements:
- Signup requires referral_code (must exist and be active; optionally usage_limit).
- Password hashing: Argon2id preferred (or bcrypt if justified).
- Session cookies: httpOnly, secure in prod, sameSite=Lax.
- Rate limit auth endpoints (simple in-memory limiter ok).
- CORS must allow frontend origin.

API endpoints (minimum):
Auth:
- POST /api/auth/signup  {email,password,firstName,lastName,referralCode}
- POST /api/auth/login
- POST /api/auth/logout
- GET  /api/auth/me

Instruments & data:
- GET /api/instruments?search=&assetClass=&exchange=&country=&page=&pageSize=
- GET /api/instruments/{symbol}
- GET /api/instruments/{symbol}/profile
- GET /api/instruments/{symbol}/fundamentals?limit=40
- GET /api/instruments/{symbol}/prices?interval=1d|1h|1min&from=&to=&limit=
- GET /api/market/status  (open/closed/nextOpen/nextClose; supports equities vs crypto)
- GET /api/dashboard (favorites + latest prices + top movers + recent news if available later)

Favorites:
- GET /api/favorites
- PUT /api/favorites (batch replace or upsert) supports radio yes/no semantics

Streaming:
- GET /api/stream/instruments/{symbol} (SSE)
- GET /api/stream/favorites (SSE multiplex)
Rules:
- If market closed for equities, stream either stops or reduces frequency and indicates closed.
- Crypto streams 24/7.

Data selection logic:
- “Latest price”:
  - prefer latest quote within last N minutes
  - fallback to latest 1min bar
  - fallback to last daily close
- “Market closed gray UI” depends on /api/market/status response.

Implementation constraints:
- Use sqlc OR pgx + a small query layer. Favor speed and clarity.
- Use migrations (golang-migrate or goose) stored in backend/migrations.
- Add necessary DB indexes for hot queries:
  - price_bars(instrument_id, interval, ts desc)
  - quotes(instrument_id, ts desc)
  - instruments(symbol)
  - company_profiles(instrument_id)
  - fundamentals(instrument_id, period_end_date desc)
  - user_favorites(user_id, instrument_id unique)

Output rules:
- Write each file one at a time as its own artifact.
- After each file, ask to continue.
- Start by creating backend/README.md explaining local run, env vars, migrations.
