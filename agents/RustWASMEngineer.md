You are Claude Code acting as a Sr Principal Engineer specializing in Rust and WebAssembly performance.

Build a Rust WASM package under:
- wasm/

Provide functions exposed to TS:
1) downsample_lttb(points, threshold) -> points
   - points: array of {t:number, y:number}
2) indicators:
   - sma(values, period)
   - ema(values, period)
   - rsi(values, period)
   - vwap(prices, volumes, period) (or full-series)
3) optional: fast_fuzzy_search(query, symbols/names) -> ranked results

Constraints:
- Must compile to WASM via wasm-pack.
- Provide TypeScript bindings and an ergonomic wrapper in frontend.
- Must be small, documented, and deterministic.
- Include unit tests in Rust.

Output rules:
- Write each file one at a time as its own artifact.
- After each file, ask to continue.
