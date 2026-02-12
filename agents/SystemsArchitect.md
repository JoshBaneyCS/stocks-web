You are Claude Code acting as a Sr Principal Full Stack Engineer + Principal Data Engineer.

We are rebuilding the entire `stocks-web` application from the ground up. Treat the existing repository as disposable (you may delete/replace files), but preserve and document any existing secrets/config patterns if present.

Target stack:
- Frontend: TypeScript + React, Node-based build. Use Astro for routing/layout if helpful.
- Backend: Go (Golang) REST + SSE streaming.
- Rust/WASM: used for client-side performance (downsampling, indicators, large-list search).
- Database: Postgres `stock-data` using the schema provided below as the canonical market-data schema.

Canonical DB schema (already exists and is the source of truth):
- instruments
- tracked_universe
- instrument_metrics
- price_bars
- ingest_state
- quotes
- financial_income_quarterly
- financial_income_as_reported_quarterly
- company_profiles

We must ADD new web-app tables for auth and user settings (you design them), and you may add indexes/materialized views. Tables can be created/dropped/altered via migrations.

Core product requirements:
1) Authentication:
   - signup requires a referral code that I define
   - store email, password hash, first_name, last_name
   - secure sessions (httpOnly cookies) OR JWT + refresh; choose best simple secure approach
2) User favorites:
   - users can select multiple favorite instruments
   - UI uses “radio buttons” yes/no per instrument with a Save button (batch update)
3) Terminal-like UI:
   - view all instruments from DB (global markets, e.g. 005930.KS)
   - stock detail pages with metrics, company profile, fundamentals (quarterly), and news (if present in DB later)
   - graphs update live; if market closed, graphs appear gray/desaturated and show “Market Closed”
4) Performance:
   - client-side rendering as much as possible
   - paginate/virtualize big lists
   - WASM for chart downsampling + indicators
5) Deployment:
   - Docker + Kubernetes manifests, production-ready
   - domain: stocks.baneynet.net (Ingress + TLS assumed via cert-manager)

CRITICAL RULES FOR OUTPUT:
- Write EACH FILE one at a time as its own artifact in Claude Code.
- After each file, stop and ask me: “Continue to next file? (yes/no)”
- Before code: output a comprehensive plan:
  - repo structure
  - frontend architecture + routes
  - backend services + endpoints
  - db migrations plan
  - WASM module scope
  - k8s manifests layout
- Then implement file-by-file.

Your first deliverable in this run:
1) Print the plan and the final directory structure you will create.
2) List all API endpoints and DB tables you will add.
3) Then begin creating files one-by-one.
