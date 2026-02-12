import type { TimeSeriesPoint, SymbolEntry } from './types';

interface WasmModule {
  lttb_downsample: (data: TimeSeriesPoint[], threshold: number) => TimeSeriesPoint[];
  calc_sma: (data: TimeSeriesPoint[], period: number) => TimeSeriesPoint[];
  calc_ema: (data: TimeSeriesPoint[], period: number) => TimeSeriesPoint[];
  calc_rsi: (data: TimeSeriesPoint[], period: number) => TimeSeriesPoint[];
  calc_vwap: (data: TimeSeriesPoint[]) => TimeSeriesPoint[];
  filter_symbols: (entries: SymbolEntry[], query: string, max: number) => SymbolEntry[];
}

let wasmModule: WasmModule | null = null;
let initPromise: Promise<WasmModule | null> | null = null;

async function loadWasm(): Promise<WasmModule | null> {
  try {
    const wasmPath = '/wasm/stocks_wasm.js';
    const wasm = await import(/* @vite-ignore */ wasmPath);
    await wasm.default();
    return wasm as unknown as WasmModule;
  } catch (err) {
    console.warn('WASM module not available, using JS fallbacks:', err);
    return null;
  }
}

export async function initWasm(): Promise<void> {
  if (!initPromise) {
    initPromise = loadWasm();
  }
  wasmModule = await initPromise;
}

// --- JS Fallback implementations ---

function jsLttbDownsample(data: TimeSeriesPoint[], threshold: number): TimeSeriesPoint[] {
  if (data.length <= threshold) return data;

  const sampled: TimeSeriesPoint[] = [];
  const bucketSize = (data.length - 2) / (threshold - 2);

  sampled.push(data[0]);

  let prevIndex = 0;

  for (let i = 0; i < threshold - 2; i++) {
    const avgRangeStart = Math.floor((i + 1) * bucketSize) + 1;
    const avgRangeEnd = Math.min(Math.floor((i + 2) * bucketSize) + 1, data.length);

    let avgTs = 0;
    let avgVal = 0;
    const avgCount = avgRangeEnd - avgRangeStart;

    for (let j = avgRangeStart; j < avgRangeEnd; j++) {
      avgTs += data[j].ts;
      avgVal += data[j].value;
    }
    avgTs /= avgCount;
    avgVal /= avgCount;

    const rangeStart = Math.floor(i * bucketSize) + 1;
    const rangeEnd = Math.min(Math.floor((i + 1) * bucketSize) + 1, data.length);

    let maxArea = -1;
    let maxIndex = rangeStart;

    const pointA = data[prevIndex];

    for (let j = rangeStart; j < rangeEnd; j++) {
      const area = Math.abs(
        (pointA.ts - avgTs) * (data[j].value - pointA.value) -
        (pointA.ts - data[j].ts) * (avgVal - pointA.value)
      );
      if (area > maxArea) {
        maxArea = area;
        maxIndex = j;
      }
    }

    sampled.push(data[maxIndex]);
    prevIndex = maxIndex;
  }

  sampled.push(data[data.length - 1]);
  return sampled;
}

function jsCalcSma(data: TimeSeriesPoint[], period: number): TimeSeriesPoint[] {
  if (data.length < period) return [];
  const result: TimeSeriesPoint[] = [];
  let sum = 0;

  for (let i = 0; i < period; i++) {
    sum += data[i].value;
  }
  result.push({ ts: data[period - 1].ts, value: sum / period });

  for (let i = period; i < data.length; i++) {
    sum += data[i].value - data[i - period].value;
    result.push({ ts: data[i].ts, value: sum / period });
  }

  return result;
}

function jsCalcEma(data: TimeSeriesPoint[], period: number): TimeSeriesPoint[] {
  if (data.length < period) return [];
  const result: TimeSeriesPoint[] = [];
  const multiplier = 2 / (period + 1);

  // Start with SMA for the first value
  let sum = 0;
  for (let i = 0; i < period; i++) {
    sum += data[i].value;
  }
  let ema = sum / period;
  result.push({ ts: data[period - 1].ts, value: ema });

  for (let i = period; i < data.length; i++) {
    ema = (data[i].value - ema) * multiplier + ema;
    result.push({ ts: data[i].ts, value: ema });
  }

  return result;
}

function jsCalcRsi(data: TimeSeriesPoint[], period: number): TimeSeriesPoint[] {
  if (data.length < period + 1) return [];
  const result: TimeSeriesPoint[] = [];

  let gainSum = 0;
  let lossSum = 0;

  for (let i = 1; i <= period; i++) {
    const change = data[i].value - data[i - 1].value;
    if (change > 0) gainSum += change;
    else lossSum += Math.abs(change);
  }

  let avgGain = gainSum / period;
  let avgLoss = lossSum / period;

  const rsi = avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss);
  result.push({ ts: data[period].ts, value: rsi });

  for (let i = period + 1; i < data.length; i++) {
    const change = data[i].value - data[i - 1].value;
    const gain = change > 0 ? change : 0;
    const loss = change < 0 ? Math.abs(change) : 0;

    avgGain = (avgGain * (period - 1) + gain) / period;
    avgLoss = (avgLoss * (period - 1) + loss) / period;

    const rsiVal = avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss);
    result.push({ ts: data[i].ts, value: rsiVal });
  }

  return result;
}

function jsCalcVwap(data: TimeSeriesPoint[]): TimeSeriesPoint[] {
  // Simple VWAP approximation: cumulative average price
  if (data.length === 0) return [];
  const result: TimeSeriesPoint[] = [];
  let cumVal = 0;

  for (let i = 0; i < data.length; i++) {
    cumVal += data[i].value;
    result.push({ ts: data[i].ts, value: cumVal / (i + 1) });
  }

  return result;
}

function jsFilterSymbols(entries: SymbolEntry[], query: string, max: number): SymbolEntry[] {
  const q = query.toLowerCase();
  const results: SymbolEntry[] = [];

  for (const entry of entries) {
    if (results.length >= max) break;
    if (
      entry.symbol.toLowerCase().includes(q) ||
      entry.name.toLowerCase().includes(q)
    ) {
      results.push(entry);
    }
  }

  return results;
}

// --- Exported wrapper functions ---

export function lttbDownsample(data: TimeSeriesPoint[], threshold: number): TimeSeriesPoint[] {
  if (wasmModule) {
    try {
      return wasmModule.lttb_downsample(data, threshold);
    } catch {
      // fall through to JS
    }
  }
  return jsLttbDownsample(data, threshold);
}

export function calcSma(data: TimeSeriesPoint[], period: number): TimeSeriesPoint[] {
  if (wasmModule) {
    try {
      return wasmModule.calc_sma(data, period);
    } catch {
      // fall through to JS
    }
  }
  return jsCalcSma(data, period);
}

export function calcEma(data: TimeSeriesPoint[], period: number): TimeSeriesPoint[] {
  if (wasmModule) {
    try {
      return wasmModule.calc_ema(data, period);
    } catch {
      // fall through to JS
    }
  }
  return jsCalcEma(data, period);
}

export function calcRsi(data: TimeSeriesPoint[], period: number): TimeSeriesPoint[] {
  if (wasmModule) {
    try {
      return wasmModule.calc_rsi(data, period);
    } catch {
      // fall through to JS
    }
  }
  return jsCalcRsi(data, period);
}

export function calcVwap(data: TimeSeriesPoint[]): TimeSeriesPoint[] {
  if (wasmModule) {
    try {
      return wasmModule.calc_vwap(data);
    } catch {
      // fall through to JS
    }
  }
  return jsCalcVwap(data);
}

export function filterSymbols(entries: SymbolEntry[], query: string, max: number): SymbolEntry[] {
  if (wasmModule) {
    try {
      return wasmModule.filter_symbols(entries, query, max);
    } catch {
      // fall through to JS
    }
  }
  return jsFilterSymbols(entries, query, max);
}
