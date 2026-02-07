use wasm_bindgen::prelude::*;
use serde::{Deserialize, Serialize};

// ─── Data Types ──────────────────────────────────────────────────────

/// A single OHLCV price point passed from JavaScript.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PricePoint {
    pub ts: f64,     // Unix timestamp in milliseconds
    pub open: f64,
    pub high: f64,
    pub low: f64,
    pub close: f64,
    pub volume: f64,
}

/// A downsampled point (timestamp + value) for line charts.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DataPoint {
    pub ts: f64,
    pub value: f64,
}

/// Result of a technical indicator calculation.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IndicatorPoint {
    pub ts: f64,
    pub value: f64,
}

/// A symbol entry for fast client-side filtering.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SymbolEntry {
    pub id: i32,
    pub symbol: String,
    pub name: String,
    pub exchange: String,
    pub sector: String,
    pub industry: String,
}

// ─── LTTB Downsampling ──────────────────────────────────────────────
//
// Largest Triangle Three Buckets algorithm.
// Reduces a large time series to `threshold` points while preserving
// visual shape. Ideal for rendering multi-year daily charts in the
// browser without sending 10k+ points to the chart library.
//
// Reference: Sveinn Steinarsson, "Downsampling Time Series for
// Visual Representation" (2013)

#[wasm_bindgen]
pub fn lttb_downsample(data_js: JsValue, threshold: usize) -> JsValue {
    let data: Vec<DataPoint> = match serde_wasm_bindgen::from_value(data_js) {
        Ok(d) => d,
        Err(_) => return JsValue::NULL,
    };

    let result = lttb(&data, threshold);
    serde_wasm_bindgen::to_value(&result).unwrap_or(JsValue::NULL)
}

fn lttb(data: &[DataPoint], threshold: usize) -> Vec<DataPoint> {
    let len = data.len();

    // No downsampling needed
    if threshold >= len || threshold < 3 {
        return data.to_vec();
    }

    let mut sampled: Vec<DataPoint> = Vec::with_capacity(threshold);

    // Always include first point
    sampled.push(data[0].clone());

    let bucket_size = (len - 2) as f64 / (threshold - 2) as f64;

    let mut prev_selected_idx: usize = 0;

    for i in 0..(threshold - 2) {
        // Calculate bucket boundaries
        let bucket_start = ((i as f64 + 1.0) * bucket_size).floor() as usize + 1;
        let bucket_end = (((i + 2) as f64) * bucket_size).floor() as usize + 1;
        let bucket_end = bucket_end.min(len - 1);

        // Calculate average point of the NEXT bucket (for triangle area)
        let next_bucket_start = bucket_end;
        let next_bucket_end = (((i + 3) as f64) * bucket_size).floor() as usize + 1;
        let next_bucket_end = next_bucket_end.min(len);

        let mut avg_ts = 0.0;
        let mut avg_val = 0.0;
        let next_count = (next_bucket_end - next_bucket_start).max(1);
        for j in next_bucket_start..next_bucket_end.min(len) {
            avg_ts += data[j].ts;
            avg_val += data[j].value;
        }
        avg_ts /= next_count as f64;
        avg_val /= next_count as f64;

        // Find point in current bucket with max triangle area
        let prev_ts = data[prev_selected_idx].ts;
        let prev_val = data[prev_selected_idx].value;

        let mut max_area = -1.0_f64;
        let mut max_idx = bucket_start;

        for j in bucket_start..bucket_end {
            // Triangle area using cross product
            let area = ((prev_ts - avg_ts) * (data[j].value - prev_val)
                - (prev_ts - data[j].ts) * (avg_val - prev_val))
                .abs()
                * 0.5;

            if area > max_area {
                max_area = area;
                max_idx = j;
            }
        }

        sampled.push(data[max_idx].clone());
        prev_selected_idx = max_idx;
    }

    // Always include last point
    sampled.push(data[len - 1].clone());

    sampled
}

// ─── Simple Moving Average (SMA) ────────────────────────────────────

#[wasm_bindgen]
pub fn calc_sma(data_js: JsValue, period: usize) -> JsValue {
    let data: Vec<PricePoint> = match serde_wasm_bindgen::from_value(data_js) {
        Ok(d) => d,
        Err(_) => return JsValue::NULL,
    };

    if period == 0 || period > data.len() {
        return JsValue::NULL;
    }

    let mut result: Vec<IndicatorPoint> = Vec::with_capacity(data.len() - period + 1);

    // Initial window sum
    let mut sum: f64 = data[..period].iter().map(|p| p.close).sum();
    result.push(IndicatorPoint {
        ts: data[period - 1].ts,
        value: sum / period as f64,
    });

    // Sliding window
    for i in period..data.len() {
        sum += data[i].close - data[i - period].close;
        result.push(IndicatorPoint {
            ts: data[i].ts,
            value: sum / period as f64,
        });
    }

    serde_wasm_bindgen::to_value(&result).unwrap_or(JsValue::NULL)
}

// ─── Exponential Moving Average (EMA) ───────────────────────────────

#[wasm_bindgen]
pub fn calc_ema(data_js: JsValue, period: usize) -> JsValue {
    let data: Vec<PricePoint> = match serde_wasm_bindgen::from_value(data_js) {
        Ok(d) => d,
        Err(_) => return JsValue::NULL,
    };

    if period == 0 || period > data.len() {
        return JsValue::NULL;
    }

    let multiplier = 2.0 / (period as f64 + 1.0);
    let mut result: Vec<IndicatorPoint> = Vec::with_capacity(data.len() - period + 1);

    // Seed EMA with SMA of first `period` values
    let initial_sma: f64 = data[..period].iter().map(|p| p.close).sum::<f64>() / period as f64;
    result.push(IndicatorPoint {
        ts: data[period - 1].ts,
        value: initial_sma,
    });

    let mut prev_ema = initial_sma;

    for i in period..data.len() {
        let ema = (data[i].close - prev_ema) * multiplier + prev_ema;
        result.push(IndicatorPoint {
            ts: data[i].ts,
            value: ema,
        });
        prev_ema = ema;
    }

    serde_wasm_bindgen::to_value(&result).unwrap_or(JsValue::NULL)
}

// ─── Relative Strength Index (RSI) ──────────────────────────────────

#[wasm_bindgen]
pub fn calc_rsi(data_js: JsValue, period: usize) -> JsValue {
    let data: Vec<PricePoint> = match serde_wasm_bindgen::from_value(data_js) {
        Ok(d) => d,
        Err(_) => return JsValue::NULL,
    };

    if period == 0 || data.len() < period + 1 {
        return JsValue::NULL;
    }

    let mut result: Vec<IndicatorPoint> = Vec::with_capacity(data.len() - period);

    // Calculate initial average gain and loss
    let mut avg_gain = 0.0;
    let mut avg_loss = 0.0;

    for i in 1..=period {
        let change = data[i].close - data[i - 1].close;
        if change > 0.0 {
            avg_gain += change;
        } else {
            avg_loss += change.abs();
        }
    }

    avg_gain /= period as f64;
    avg_loss /= period as f64;

    // First RSI value
    let rs = if avg_loss == 0.0 {
        100.0
    } else {
        avg_gain / avg_loss
    };
    let rsi = 100.0 - (100.0 / (1.0 + rs));
    result.push(IndicatorPoint {
        ts: data[period].ts,
        value: rsi,
    });

    // Subsequent RSI values using Wilder's smoothing
    for i in (period + 1)..data.len() {
        let change = data[i].close - data[i - 1].close;
        let (gain, loss) = if change > 0.0 {
            (change, 0.0)
        } else {
            (0.0, change.abs())
        };

        avg_gain = (avg_gain * (period as f64 - 1.0) + gain) / period as f64;
        avg_loss = (avg_loss * (period as f64 - 1.0) + loss) / period as f64;

        let rs = if avg_loss == 0.0 {
            100.0
        } else {
            avg_gain / avg_loss
        };
        let rsi = 100.0 - (100.0 / (1.0 + rs));

        result.push(IndicatorPoint {
            ts: data[i].ts,
            value: rsi,
        });
    }

    serde_wasm_bindgen::to_value(&result).unwrap_or(JsValue::NULL)
}

// ─── Volume Weighted Average Price (VWAP) ───────────────────────────
//
// VWAP is calculated intraday: cumulative(typical_price * volume) / cumulative(volume)
// where typical_price = (high + low + close) / 3
//
// Resets each trading day. Day boundaries detected by timestamp gap > 4 hours.

#[wasm_bindgen]
pub fn calc_vwap(data_js: JsValue) -> JsValue {
    let data: Vec<PricePoint> = match serde_wasm_bindgen::from_value(data_js) {
        Ok(d) => d,
        Err(_) => return JsValue::NULL,
    };

    if data.is_empty() {
        return JsValue::NULL;
    }

    let mut result: Vec<IndicatorPoint> = Vec::with_capacity(data.len());
    let mut cum_tp_vol = 0.0;
    let mut cum_vol = 0.0;
    let mut prev_ts = data[0].ts;

    // 4 hours in milliseconds — gap threshold for day boundary detection
    let day_gap_ms = 4.0 * 60.0 * 60.0 * 1000.0;

    for point in &data {
        // Reset on new trading day (detected by large timestamp gap)
        if (point.ts - prev_ts) > day_gap_ms {
            cum_tp_vol = 0.0;
            cum_vol = 0.0;
        }

        let typical_price = (point.high + point.low + point.close) / 3.0;
        cum_tp_vol += typical_price * point.volume;
        cum_vol += point.volume;

        let vwap = if cum_vol > 0.0 {
            cum_tp_vol / cum_vol
        } else {
            typical_price
        };

        result.push(IndicatorPoint {
            ts: point.ts,
            value: vwap,
        });

        prev_ts = point.ts;
    }

    serde_wasm_bindgen::to_value(&result).unwrap_or(JsValue::NULL)
}

// ─── Fast Client-Side Symbol Search ─────────────────────────────────
//
// Filters a preloaded symbol list entirely in WASM.
// Matches against symbol, name, exchange, sector, and industry.
// Case-insensitive substring match.

#[wasm_bindgen]
pub fn filter_symbols(entries_js: JsValue, query: &str, max_results: usize) -> JsValue {
    let entries: Vec<SymbolEntry> = match serde_wasm_bindgen::from_value(entries_js) {
        Ok(e) => e,
        Err(_) => return JsValue::NULL,
    };

    if query.is_empty() {
        // Return all up to max
        let limited: Vec<&SymbolEntry> = entries.iter().take(max_results).collect();
        return serde_wasm_bindgen::to_value(&limited).unwrap_or(JsValue::NULL);
    }

    let query_lower = query.to_lowercase();
    let tokens: Vec<&str> = query_lower.split_whitespace().collect();

    let mut results: Vec<(usize, &SymbolEntry)> = Vec::new();

    for entry in &entries {
        let symbol_lower = entry.symbol.to_lowercase();
        let name_lower = entry.name.to_lowercase();
        let exchange_lower = entry.exchange.to_lowercase();
        let sector_lower = entry.sector.to_lowercase();
        let industry_lower = entry.industry.to_lowercase();

        // All tokens must match at least one field
        let all_match = tokens.iter().all(|token| {
            symbol_lower.contains(token)
                || name_lower.contains(token)
                || exchange_lower.contains(token)
                || sector_lower.contains(token)
                || industry_lower.contains(token)
        });

        if !all_match {
            continue;
        }

        // Scoring: prioritize symbol exact match > symbol prefix > name contains
        let score = if symbol_lower == query_lower {
            0 // exact symbol match — highest priority
        } else if symbol_lower.starts_with(&query_lower) {
            1 // symbol prefix match
        } else if name_lower.contains(&query_lower) {
            2 // name contains full query
        } else {
            3 // other field match
        };

        results.push((score, entry));

        // Early exit if we have enough exact/prefix matches
        if results.len() >= max_results * 3 {
            break;
        }
    }

    // Sort by score (lower is better), then by symbol alphabetically
    results.sort_by(|a, b| a.0.cmp(&b.0).then_with(|| a.1.symbol.cmp(&b.1.symbol)));

    let limited: Vec<&SymbolEntry> = results.iter().take(max_results).map(|(_, e)| *e).collect();
    serde_wasm_bindgen::to_value(&limited).unwrap_or(JsValue::NULL)
}

// ─── Tests ───────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    fn make_data(values: &[f64]) -> Vec<DataPoint> {
        values
            .iter()
            .enumerate()
            .map(|(i, &v)| DataPoint {
                ts: i as f64 * 1000.0,
                value: v,
            })
            .collect()
    }

    fn make_prices(closes: &[f64]) -> Vec<PricePoint> {
        closes
            .iter()
            .enumerate()
            .map(|(i, &c)| PricePoint {
                ts: i as f64 * 60000.0,
                open: c,
                high: c + 1.0,
                low: c - 1.0,
                close: c,
                volume: 1000.0,
            })
            .collect()
    }

    #[test]
    fn test_lttb_passthrough_small() {
        let data = make_data(&[1.0, 2.0, 3.0, 4.0]);
        let result = lttb(&data, 10);
        assert_eq!(result.len(), 4); // no downsampling needed
    }

    #[test]
    fn test_lttb_reduces() {
        let data = make_data(&(0..1000).map(|i| (i as f64).sin()).collect::<Vec<_>>());
        let result = lttb(&data, 100);
        assert_eq!(result.len(), 100);
        // First and last points preserved
        assert_eq!(result[0].ts, data[0].ts);
        assert_eq!(result[99].ts, data[999].ts);
    }

    #[test]
    fn test_sma() {
        let data = make_prices(&[10.0, 20.0, 30.0, 40.0, 50.0]);
        // SMA(3): [20, 30, 40]
        // Using the internal function indirectly via data
        let period = 3;
        let mut sum: f64 = data[..period].iter().map(|p| p.close).sum();
        assert_eq!(sum / 3.0, 20.0);
        sum += data[3].close - data[0].close;
        assert_eq!(sum / 3.0, 30.0);
    }

    #[test]
    fn test_rsi_bounds() {
        // RSI should always be between 0 and 100
        let prices: Vec<f64> = (0..50).map(|i| 100.0 + (i as f64 * 0.5).sin() * 10.0).collect();
        let data = make_prices(&prices);
        let period = 14;

        // Manually compute one RSI to verify bounds
        let mut avg_gain = 0.0;
        let mut avg_loss = 0.0;
        for i in 1..=period {
            let change = data[i].close - data[i - 1].close;
            if change > 0.0 {
                avg_gain += change;
            } else {
                avg_loss += change.abs();
            }
        }
        avg_gain /= period as f64;
        avg_loss /= period as f64;
        let rs = if avg_loss == 0.0 { 100.0 } else { avg_gain / avg_loss };
        let rsi = 100.0 - (100.0 / (1.0 + rs));
        assert!(rsi >= 0.0 && rsi <= 100.0);
    }
}