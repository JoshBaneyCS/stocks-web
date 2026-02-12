# Stocks Web Backend

Go backend service for the stocks-web application. Provides REST API endpoints for authentication, instrument data, favorites, market status, and real-time SSE streaming.

## Prerequisites

- Go 1.22+
- PostgreSQL with the `stock-data` database
- Migrations applied (see below)

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `JWT_SECRET` | Yes | - | Secret key for signing access JWTs |
| `REFRESH_SECRET` | Yes | - | Secret key for signing refresh JWTs |
| `ADMIN_SECRET` | Yes | - | Secret for admin API endpoints (X-Admin-Secret header) |
| `PORT` | No | `8080` | HTTP server port |
| `CORS_ORIGIN` | No | `*` | Allowed CORS origin |
| `ACCESS_TOKEN_EXPIRY` | No | `15m` | Access token TTL (Go duration or `Nd` for days) |
| `REFRESH_TOKEN_EXPIRY` | No | `7d` | Refresh token TTL |

## Local Development

```bash
# Copy and edit environment file
cp .env.example .env

# Install dependencies
go mod tidy

# Run the server
go run ./cmd/server

# Or build and run
go build -o server ./cmd/server
./server
```

## Migrations

Migrations are in `migrations/` and should be applied using a migration tool such as `golang-migrate`:

```bash
# Install migrate CLI
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run migrations up
migrate -path migrations -database "$DATABASE_URL" up

# Rollback last migration
migrate -path migrations -database "$DATABASE_URL" down 1
```

## Docker

```bash
# Build
docker build -t stocks-backend .

# Run
docker run -p 8080:8080 \
  -e DATABASE_URL="postgres://..." \
  -e JWT_SECRET="your-jwt-secret" \
  -e REFRESH_SECRET="your-refresh-secret" \
  -e ADMIN_SECRET="your-admin-secret" \
  stocks-backend
```

## API Endpoints

### Health
- `GET /healthz` - Liveness probe
- `GET /readyz` - Readiness probe (pings database)

### Authentication
- `POST /api/auth/signup` - Register with referral code
- `POST /api/auth/login` - Login with email/password
- `POST /api/auth/logout` - Logout (clears tokens)
- `POST /api/auth/refresh` - Refresh access token
- `GET /api/auth/me` - Get current user (requires auth)

### Instruments
- `GET /api/instruments/` - List instruments (paginated, filterable)
- `GET /api/instruments/{symbol}` - Instrument detail
- `GET /api/instruments/{symbol}/profile` - Company profile
- `GET /api/instruments/{symbol}/fundamentals` - Quarterly financials
- `GET /api/instruments/{symbol}/prices` - Historical price bars

### Favorites
- `GET /api/favorites/` - Get user favorites (requires auth)
- `PUT /api/favorites/` - Update user favorites (requires auth)

### Market
- `GET /api/market/status` - NYSE market status

### Dashboard
- `GET /api/dashboard` - User dashboard data (requires auth)

### Streaming (SSE)
- `GET /api/stream/{symbol}` - Price stream for a single instrument
- `GET /api/stream/favorites` - Price stream for all user favorites (requires auth)

### Admin
- `POST /api/admin/referral-codes` - Create referral code (requires X-Admin-Secret header)
