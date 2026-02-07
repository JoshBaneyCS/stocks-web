/**
 * WASM integration for client-side performance-critical operations.
 *
 * Provides typed wrappers around the Rust/WASM module built from wasm/:
 *   - LTTB downsampling for long time-series charts
 *   - Technical indicators (SMA, EMA, RSI, VWAP)
 *   - Fast symbol search with scoring
 *
 * The WASM module is loaded lazily on first use.
 */

import type { PricePoint } from './types';

// ── WASM Module Types ───────────────────────────────────────────────

interface WasmExports {
  lttb_downsample(
    times: Float64Array,
    values: Float64Array,
    threshold: number,
  ): Float64Array;

  sma(values: Float64Array, period: number): Float64Array;
  ema(values: Float64Array, period: number): Float64Array;
  rsi(values: Float64Array, period: number): Float64Array;
  vwap(
    highs: Float64Array,
    lows: Float64Array,
    closes: Float64Array,
    volumes: Float64Array,
    timestamps: Float64Array,
  ): Float64Array;

  symbol_search(
    symbols_json: string,
    query: string,
    max_results: number,
  ): string;
}

// ── Singleton Loader ────────────────────────────────────────────────

let wasmModule: WasmExports | null = null;
let wasmLoading: Promise<WasmExports> | null = null;
let wasmFailed = false;

async function loadWasm(): Promise<WasmExports> {
  if (wasmModule) return wasmModule;
  if (wasmFailed) throw new Error('WASM module previously failed to load');

  if (!wasmLoading) {
    wasmLoading = (async () => {
      try {
        // wasm-pack output is copied to public/wasm/ during build
        const mod = await import('/wasm/stocks_wasm.js');
        await mod.default();
        wasmModule = mod as unknown as WasmExports;
        return wasmModule;
      } catch (err) {
        wasmFailed = true;
        console.warn('WASM module failed to load, falling back to JS:', err);
        throw err;
      }
    })();
  }

  return wasmLoading;
}

export function isWasmAvailable(): boolean {
  return wasmModule !== null;
}

// ── LTTB Downsampling ───────────────────────────────────────────────

/**
 * Downsample a time series using Largest-Triangle-Three-Buckets.
 * Falls back to naive nth-point sampling if WASM is unavailable.
 */
export async function downsample(
  points: PricePoint[],
  targetPoints: number,
): Promise<PricePoint[]> {
  if (points.length <= targetPoints) return points;

  try {
    const wasm = await loadWasm();
    const times = new Float64Array(points.map((p) => p.time));
    const values = new Float64Array(points.map((p) => p.value));

    const result = wasm.lttb_downsample(times, values, targetPoints);

    // Result is interleaved [time, value, time, value, ...]
    const out: PricePoint[] = [];
    for (let i = 0; i < result.length; i += 2) {
      out.push({ time: result[i], value: result[i + 1] });
    }
    return out;
  } catch {
    return fallbackDownsample(points, targetPoints);
  }
}

function fallbackDownsample(
  points: PricePoint[],
  targetPoints: number,
): PricePoint[] {
  const step = Math.max(1, Math.floor(points.length / targetPoints));
  const result: PricePoint[] = [];
  for (let i = 0; i < points.length; i += step) {
    result.push(points[i]);
  }
  // Always include last point
  if (result[result.length - 1] !== points[points.length - 1]) {
    result.push(points[points.length - 1]);
  }
  return result;
}

// ── Technical Indicators ────────────────────────────────────────────

export async function calculateSMA(
  closes: number[],
  period: number,
): Promise<number[]> {
  try {
    const wasm = await loadWasm();
    const result = wasm.sma(new Float64Array(closes), period);
    return Array.from(result);
  } catch {
    return fallbackSMA(closes, period);
  }
}

export async function calculateEMA(
  closes: number[],
  period: number,
): Promise<number[]> {
  try {
    const wasm = await loadWasm();
    const result = wasm.ema(new Float64Array(closes), period);
    return Array.from(result);
  } catch {
    return fallbackEMA(closes, period);
  }
}

export async function calculateRSI(
  closes: number[],
  period: number = 14,
): Promise<number[]> {
  try {
    const wasm = await loadWasm();
    const result = wasm.rsi(new Float64Array(closes), period);
    return Array.from(result);
  } catch {
    return fallbackRSI(closes, period);
  }
}

export async function calculateVWAP(
  highs: number[],
  lows: number[],
  closes: number[],
  volumes: number[],
  timestamps: number[],
): Promise<number[]> {
  try {
    const wasm = await loadWasm();
    const result = wasm.vwap(
      new Float64Array(highs),
      new Float64Array(lows),
      new Float64Array(closes),
      new Float64Array(volumes),
      new Float64Array(timestamps),
    );
    return Array.from(result);
  } catch {
    return fallbackVWAP(highs, lows, closes, volumes);
  }
}

// ── Symbol Search ───────────────────────────────────────────────────

interface SymbolEntry {
  symbol: string;
  name: string;
}

interface SearchResult {
  symbol: string;
  name: string;
  score: number;
}

export async function searchSymbols(
  symbols: SymbolEntry[],
  query: string,
  maxResults: number = 20,
): Promise<SearchResult[]> {
  if (!query.trim()) return [];

  try {
    const wasm = await loadWasm();
    const json = wasm.symbol_search(
      JSON.stringify(symbols),
      query,
      maxResults,
    );
    return JSON.parse(json) as SearchResult[];
  } catch {
    return fallbackSearch(symbols, query, maxResults);
  }
}

function fallbackSearch(
  symbols: SymbolEntry[],
  query: string,
  maxResults: number,
): SearchResult[] {
  const q = query.toLowerCase();
  return symbols
    .filter(
      (s) =>
        s.symbol.toLowerCase().includes(q) ||
        s.name.toLowerCase().includes(q),
    )
    .slice(0, maxResults)
    .map((s) => ({ ...s, score: s.symbol.toLowerCase() === q ? 1 : 0.5 }));
}

// ── JS Fallback Implementations ─────────────────────────────────────

function fallbackSMA(values: number[], period: number): number[] {
  const result: number[] = new Array(values.length).fill(NaN);
  for (let i = period - 1; i < values.length; i++) {
    let sum = 0;
    for (let j = i - period + 1; j <= i; j++) sum += values[j];
    result[i] = sum / period;
  }
  return result;
}

function fallbackEMA(values: number[], period: number): number[] {
  const result: number[] = new Array(values.length).fill(NaN);
  const k = 2 / (period + 1);

  // Seed with SMA
  let sum = 0;
  for (let i = 0; i < period; i++) sum += values[i];
  result[period - 1] = sum / period;

  for (let i = period; i < values.length; i++) {
    result[i] = values[i] * k + result[i - 1] * (1 - k);
  }
  return result;
}

function fallbackRSI(values: number[], period: number): number[] {
  const result: number[] = new Array(values.length).fill(NaN);
  if (values.length < period + 1) return result;

  let avgGain = 0;
  let avgLoss = 0;

  for (let i = 1; i <= period; i++) {
    const delta = values[i] - values[i - 1];
    if (delta > 0) avgGain += delta;
    else avgLoss -= delta;
  }
  avgGain /= period;
  avgLoss /= period;

  result[period] = avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss);

  for (let i = period + 1; i < values.length; i++) {
    const delta = values[i] - values[i - 1];
    const gain = delta > 0 ? delta : 0;
    const loss = delta < 0 ? -delta : 0;
    avgGain = (avgGain * (period - 1) + gain) / period;
    avgLoss = (avgLoss * (period - 1) + loss) / period;
    result[i] = avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss);
  }

  return result;
}

function fallbackVWAP(
  highs: number[],
  lows: number[],
  closes: number[],
  volumes: number[],
): number[] {
  const result: number[] = [];
  let cumTPV = 0;
  let cumVol = 0;

  for (let i = 0; i < closes.length; i++) {
    const tp = (highs[i] + lows[i] + closes[i]) / 3;
    cumTPV += tp * volumes[i];
    cumVol += volumes[i];
    result.push(cumVol === 0 ? tp : cumTPV / cumVol);
  }

  return result;
}