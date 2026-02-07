# stocks-web

**Bloomberg-style stock data terminal** — a production-grade web application for browsing and visualizing stock market data collected from Financial Modeling Prep (FMP).

**Live at:** https://stocks.baneynet.net

---

## Architecture

```
                          ┌──────────────────────┐
                          │   stocks.baneynet.net │
                          │   (Ingress + TLS)     │
                          └───────┬──────┬────────┘
                                  │      │
                          /api/*  │      │  /*
                                  │      │
                   ┌──────────────▼┐  ┌──▼──────────────┐
                   │  Go Backend   │  │  Astro Frontend  │
                   │  (REST + SSE) │  │  (Nginx static)  │
                   │  Port 8080    │  │  Port 80         │
                   └───────┬───────┘  └──────────────────┘
                           │
                   ┌───────▼───────┐
                   │  PostgreSQL   │
                   │  stock-data   │
                   │               │
                   │  Existing:    │
                   │   companies   │
                   │   price_bars  │
                   │   news_*      │
                   │               │
                   │  New:         │
                   │   users       │
                   │   referral_   │
                   │    codes      │
                   │   user_       │
                   │    favorites  │
                   │   refresh_    │
                   │    tokens     │
                   └───────────────┘
```

### Tech Stack

| Layer | Technology |
|-------|-----------|
| Frontend | Astro 4 + React 18 + TypeScript + TailwindCSS |
| Charts | TradingView Lightweight Charts |
| WASM | Rust (LTTB downsampling, SMA/EMA/RSI/VWAP, symbol search) |
| Backend | Go (chi router, pgx, golang-jwt, argon2id) |
| Database | PostgreSQL 16 (shared with ingestor microservices) |
| Streaming | Server-Sent Events (SSE) |
| Deploy | Docker + Kubernetes (Kustomize) + cert-manager TLS |

---

## Features

- **Referral-gated signup** — users need a valid referral code to register
- **JWT authentication** — access + refresh tokens, secure httpOnly-ready
- **Stock browsing** — paginated list with search, exchange/sector filtering
- **Favorites** — radio-style Yes/No toggles per stock with batch save
- **Stock detail pages** — company info, key metrics, interactive charts, news
- **Intraday charts** — 1-minute area chart with WASM LTTB downsampling
- **Daily charts** — candlestick + volume histogram with full history
- **Live updates** — SSE streaming of price data during market hours
- **Market status** — real-time open/closed indicator with countdown timer
- **Market closed UX** — charts gray out with desaturation + "Market Closed" badge
- **News feed** — articles and press releases with date filtering
- **Dark terminal theme** — Bloomberg-inspired dark UI with JetBrains Mono font

---

## Repository Structure

```
stocks-web/
├── backend/                    # Go API service (Phase 1+2, already in repo)
│   ├── cmd/server/main.go      # Entry point, chi router, middleware
│   ├── internal/
│   │   ├── config/config.go    # Env var configuration
│   │   ├── db/db.go            # pgx connection pool
│   │   ├── models/models.go    # All data structs
│   │   ├── auth/               # Argon2id, JWT middleware, auth handlers
│   │   ├── market/status.go    # NYSE holiday computation
│   │   └── handlers/           # stocks, favorites, market, stream (SSE)
│   ├── migrations/             # SQL migrations (001-008)
│   └── Dockerfile
│
├── frontend/                   # Astro + React + TS (Phase 3)
│   ├── src/
│   │   ├── lib/
│   │   │   ├── api.ts          # Typed fetch wrapper + SSE helpers
│   │   │   ├── types.ts        # TypeScript interfaces
│   │   │   └── wasm.ts         # WASM loader with JS fallbacks
│   │   ├── layouts/
│   │   │   └── Layout.astro    # Base HTML shell
│   │   ├── components/
│   │   │   ├── Navbar.tsx
│   │   │   ├── LoginForm.tsx
│   │   │   ├── SignupForm.tsx
│   │   │   ├── Dashboard.tsx
│   │   │   ├── MarketStatus.tsx
│   │   │   ├── SparklineChart.tsx
│   │   │   ├── FavoritesList.tsx
│   │   │   ├── NewsHeadlines.tsx
│   │   │   ├── StockList.tsx
│   │   │   ├── StockChart.tsx       # TradingView Lightweight Charts
│   │   │   ├── StockDetail.tsx
│   │   │   ├── FavoritesManager.tsx # Radio Yes/No + Save
│   │   │   └── ProfileCard.tsx
│   │   ├── pages/
│   │   │   ├── login.astro
│   │   │   ├── signup.astro
│   │   │   └── app/
│   │   │       ├── index.astro          # Dashboard
│   │   │       ├── stocks/
│   │   │       │   ├── index.astro      # Stock list
│   │   │       │   └── [...symbol].astro # Stock detail (catch-all)
│   │   │       └── settings.astro       # Profile + favorites
│   │   └── styles/global.css
│   ├── public/favicon.svg
│   ├── package.json
│   ├── astro.config.mjs
│   ├── tailwind.config.mjs
│   ├── tsconfig.json
│   └── Dockerfile
│
├── wasm/                       # Rust/WASM module (Phase 2)
│   ├── Cargo.toml
│   └── src/lib.rs              # LTTB, SMA, EMA, RSI, VWAP, search
│
├── deploy/                     # Kubernetes manifests (Phase 4)
│   └── kustomize/
│       ├── kustomization.yaml
│       ├── namespace.yaml
│       ├── secrets.yaml
│       ├── backend-deployment.yaml
│       ├── backend-service.yaml
│       ├── frontend-deployment.yaml
│       ├── frontend-service.yaml
│       └── ingress.yaml
│
├── docker-compose.yml          # Local development
└── README.md
```

---

## Prerequisites

- **Docker** and **Docker Compose** (for local dev)
- **Go** 1.22+ (for backend development)
- **Node.js** 20+ (for frontend development)
- **Rust** + **wasm-pack** (for WASM module, optional — JS fallbacks work without it)
- **PostgreSQL** 16+ (provided via docker-compose or existing cluster)
- **kubectl** + **kustomize** (for k8s deployment)

---

## Quick Start (Docker Compose)

```bash
# 1. Clone the repo
git clone https://github.com/JoshBaneyCS/stocks-web.git
cd stocks-web

# 2. Start all services (postgres + backend + frontend)
docker compose up --build -d

# 3. Run database migrations
docker compose exec backend /app/server migrate

# 4. Create a referral code for signup
curl -X POST http://localhost:8080/api/admin/referral-codes \
  -H "Content-Type: application/json" \
  -H "X-Admin-Secret: dev-admin-secret" \
  -d '{"code": "WELCOME2025", "usage_limit": 10}'

# 5. Open the app
open http://localhost

# 6. Sign up with code WELCOME2025, then log in
```

---

## Local Development (without Docker)

### Backend

```bash
cd backend

# Set environment variables
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/stock-data?sslmode=disable"
export JWT_SECRET="dev-jwt-secret"
export REFRESH_SECRET="dev-refresh-secret"
export ADMIN_SECRET="dev-admin-secret"
export CORS_ORIGIN="http://localhost:4321"
export PORT="8080"

# Run migrations
go run cmd/server/main.go migrate

# Start the server
go run cmd/server/main.go
```

### Frontend

```bash
cd frontend

# Install dependencies
npm install

# Start dev server (proxies /api to localhost:8080)
npm run dev

# Open http://localhost:4321
```

### WASM Module (optional)

```bash
cd wasm

# Build with wasm-pack
wasm-pack build --target web --out-dir ../frontend/public/wasm

# The frontend will automatically load from /wasm/stocks_wasm.js
# If WASM is not built, JS fallbacks are used transparently
```

---

## Database Migrations

The Go backend manages migrations for the **new** tables it owns:

| Migration | Tables |
|-----------|--------|
| 001-002 | `users` (email, password hash, name, timestamps) |
| 003-004 | `referral_codes` (code, status, usage_limit, used_count) |
| 005-006 | `user_favorites` (user_id, company_id, unique constraint) |
| 007-008 | `refresh_tokens` (user_id, token_hash, expires_at) |

**Existing tables** (managed by ingestor microservices, read-only):
- `companies`, `price_bars`, `ingest_jobs`
- `news_articles`, `news_mentions`, `news_fetch_state`

---

## API Reference

### Auth

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/auth/signup` | Register (requires `referral_code`) |
| `POST` | `/api/auth/login` | Login → returns JWT tokens |
| `POST` | `/api/auth/logout` | Invalidate refresh token |
| `GET` | `/api/auth/me` | Get current user profile |

### Stocks

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/stocks` | List stocks (paginated, filterable) |
| `GET` | `/api/stocks/{symbol}` | Get stock detail |
| `GET` | `/api/stocks/{symbol}/prices` | Get price bars (`interval=1min\|1d`) |
| `GET` | `/api/stocks/{symbol}/news` | Get news articles |

### Market

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/market/status` | Market open/closed + next open/close |

### Favorites

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/favorites` | Get user's favorites |
| `PUT` | `/api/favorites` | Batch update favorites |

### Streaming

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/stream/stocks/{symbol}` | SSE: live price updates |
| `GET` | `/api/stream/favorites` | SSE: updates for all favorites |

### Admin

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/admin/referral-codes` | Create referral code (requires `X-Admin-Secret` header) |

---

## Kubernetes Deployment

### 1. Build and push images

```bash
# Backend
docker build -t ghcr.io/joshbaneycs/stocks-web-backend:latest ./backend
docker push ghcr.io/joshbaneycs/stocks-web-backend:latest

# Frontend
docker build -t ghcr.io/joshbaneycs/stocks-web-frontend:latest ./frontend
docker push ghcr.io/joshbaneycs/stocks-web-frontend:latest
```

### 2. Configure secrets

Edit `deploy/kustomize/secrets.yaml` with production values:

```yaml
stringData:
  DATABASE_URL: "postgres://user:pass@db-host:5432/stock-data?sslmode=require"
  JWT_SECRET: "$(openssl rand -base64 32)"
  REFRESH_SECRET: "$(openssl rand -base64 32)"
  ADMIN_SECRET: "your-admin-secret"
```

### 3. Deploy

```bash
# Apply all resources
kubectl apply -k deploy/kustomize/

# Verify
kubectl -n stocks-web get pods
kubectl -n stocks-web get ingress

# Check logs
kubectl -n stocks-web logs -f deployment/stocks-web-backend
kubectl -n stocks-web logs -f deployment/stocks-web-frontend
```

### 4. Create initial referral code

```bash
kubectl -n stocks-web exec -it deployment/stocks-web-backend -- \
  wget -qO- --post-data='{"code":"INVITE2025","usage_limit":20}' \
  --header='Content-Type: application/json' \
  --header='X-Admin-Secret: your-admin-secret' \
  http://localhost:8080/api/admin/referral-codes
```

### 5. Verify TLS

```bash
# cert-manager should provision the certificate automatically
kubectl -n stocks-web get certificate stocks-web-tls
curl -v https://stocks.baneynet.net/api/market/status
```

---

## Environment Variables

### Backend

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | (required) | Postgres connection string |
| `JWT_SECRET` | (required) | Secret for signing access tokens |
| `REFRESH_SECRET` | (required) | Secret for signing refresh tokens |
| `ADMIN_SECRET` | (required) | Secret for admin endpoints |
| `CORS_ORIGIN` | `*` | Allowed CORS origin |
| `PORT` | `8080` | HTTP listen port |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |

### Frontend

| Variable | Default | Description |
|----------|---------|-------------|
| `PUBLIC_API_URL` | `/api` | Backend API base URL (build-time) |

---

## Security

- **Argon2id** password hashing (PHC string format)
- **JWT** access tokens (15min) + refresh tokens (7 days)
- **Parameterized SQL** everywhere (pgx, no string interpolation)
- **CORS** restricted to configured origin
- **Rate limiting** on auth endpoints via ingress annotations
- **Non-root containers** with read-only root filesystem
- **TLS** via cert-manager + Let's Encrypt
- **Referral-gated registration** — no open signup

---

## License

MIT