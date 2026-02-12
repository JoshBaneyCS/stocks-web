You are Claude Code acting as a Sr Principal Full Stack Engineer specializing in high-performance UX.

Build the frontend under:
- frontend/
Use:
- TypeScript
- Astro for routing/layout + server-side shell
- React components for interactive client-side rendering
- Client-side rendering as much as possible; keep server usage minimal (mostly to deliver the shell).

Core pages:
- /login
- /signup (requires referral code)
- /app (dashboard)
- /app/instruments (list + filters + favorites selection with radio yes/no + Save)
- /app/instruments/[symbol] (detail page)
- /app/settings (profile, change password optional)

UI requirements:
- Bloomberg-terminal-inspired dark theme, modern and clean.
- Market status indicator always visible (Open/Closed; next open).
- If market closed for equities:
  - charts and price widgets render in gray/desaturated state + show “Market Closed”
- If crypto:
  - always open indicator.

Instrument list:
- Must handle 300+ instruments and be fast:
  - server-side pagination + search
  - client-side list virtualization (react-window or similar)
- Favorites selection UX:
  - For each instrument row: radio Yes/No (Favorite) + batch Save button at top/bottom
  - Must support multiple favorites overall.

Charts:
- Intraday chart (1min) and historical (1d/1h).
- Must support live updates:
  - Use SSE endpoints from backend.
  - Append points incrementally (no full re-render).
- Long history needs downsampling:
  - call WASM module for LTTB downsampling for chart rendering.
- Provide overlays for SMA/EMA/VWAP/RSI computed client-side via WASM.

Data displayed:
- instrument metadata (exchange, country, currency, asset_class)
- company profile (sector, industry, market cap)
- fundamentals (quarterly revenue, net income, EPS) with small table and a mini chart
- (news can be stubbed if not yet in DB; leave pluggable UI component)

State management:
- Keep simple: React context or lightweight store (zustand).
- Auth:
  - backend sets cookie session; frontend calls /api/auth/me at startup.
  - redirect to /login if unauthenticated.

Performance:
- Avoid huge JSON payloads: request only needed ranges.
- Use memoization and virtualization.
- Use WASM for heavy computations.

Output rules:
- Write each file one at a time as its own artifact.
- After each file, ask to continue.
- Start with frontend/README.md + package.json + Astro config, then routes, then components.
