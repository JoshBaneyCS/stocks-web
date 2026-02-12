use serde::{Deserialize, Serialize};
use wasm_bindgen::prelude::*;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct DataPoint {
    pub ts: f64,
    pub value: f64,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct PricePoint {
    pub ts: f64,
    pub open: f64,
    pub high: f64,
    pub low: f64,
    pub close: f64,
    pub volume: f64,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct IndicatorPoint {
    pub ts: f64,
    pub value: f64,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct SymbolEntry {
    pub symbol: String,
    pub name: String,
}

#[derive(Serialize, Deserialize, Clone, Debug)]
struct ScoredEntry {
    symbol: String,
    name: String,
    score: i32,
}

// ---------------------------------------------------------------------------
// Internal (pure-Rust) implementations — testable without wasm_bindgen
// ---------------------------------------------------------------------------

/// Largest Triangle Three Buckets downsampling algorithm.
///
/// Given a time-series of (ts, value) pairs and a target `threshold` count,
/// returns a visually representative subset that preserves the shape of the
/// original series.
pub fn lttb_downsample_impl(data: &[DataPoint], threshold: usize) -> Vec<DataPoint> {
    let len = data.len();

    // Edge cases: nothing to downsample.
    if len == 0 {
        return Vec::new();
    }
    if threshold < 3 || len <= threshold {
        return data.to_vec();
    }

    let mut result: Vec<DataPoint> = Vec::with_capacity(threshold);

    // Always keep the first point.
    result.push(data[0].clone());

    // Bucket size (the first and last points are fixed, so we distribute
    // the remaining threshold-2 buckets across the interior points).
    let bucket_size: f64 = (len - 2) as f64 / (threshold - 2) as f64;

    let mut prev_selected: usize = 0;

    for i in 0..(threshold - 2) {
        // Current bucket range
        let bucket_start = ((i as f64 * bucket_size) + 1.0).floor() as usize;
        let bucket_end = (((i + 1) as f64 * bucket_size) + 1.0).floor().min(len as f64) as usize;

        // Next bucket range (used to compute the average point)
        let next_bucket_start = (((i + 1) as f64 * bucket_size) + 1.0).floor() as usize;
        let next_bucket_end =
            (((i + 2) as f64 * bucket_size) + 1.0).floor().min(len as f64) as usize;

        // Average point of the *next* bucket
        let mut avg_ts: f64 = 0.0;
        let mut avg_val: f64 = 0.0;
        let next_bucket_len = (next_bucket_end - next_bucket_start).max(1);
        for j in next_bucket_start..next_bucket_end.min(len) {
            avg_ts += data[j].ts;
            avg_val += data[j].value;
        }
        avg_ts /= next_bucket_len as f64;
        avg_val /= next_bucket_len as f64;

        // Select the point in the current bucket that forms the largest
        // triangle with the previously selected point and the average of
        // the next bucket.
        let mut max_area: f64 = -1.0;
        let mut max_idx: usize = bucket_start;

        let prev_ts = data[prev_selected].ts;
        let prev_val = data[prev_selected].value;

        for j in bucket_start..bucket_end.min(len) {
            // Triangle area (doubled, sign doesn't matter — we want max abs).
            let area = ((prev_ts - avg_ts) * (data[j].value - prev_val)
                - (prev_ts - data[j].ts) * (avg_val - prev_val))
                .abs();

            if area > max_area {
                max_area = area;
                max_idx = j;
            }
        }

        result.push(data[max_idx].clone());
        prev_selected = max_idx;
    }

    // Always keep the last point.
    result.push(data[len - 1].clone());

    result
}

/// Simple Moving Average over close prices.
pub fn calc_sma_impl(data: &[PricePoint], period: usize) -> Vec<IndicatorPoint> {
    if period == 0 || period > data.len() {
        return Vec::new();
    }

    let mut result: Vec<IndicatorPoint> = Vec::with_capacity(data.len() - period + 1);

    // Initial window sum.
    let mut window_sum: f64 = data[..period].iter().map(|p| p.close).sum();
    result.push(IndicatorPoint {
        ts: data[period - 1].ts,
        value: window_sum / period as f64,
    });

    // Slide the window forward.
    for i in period..data.len() {
        window_sum += data[i].close - data[i - period].close;
        result.push(IndicatorPoint {
            ts: data[i].ts,
            value: window_sum / period as f64,
        });
    }

    result
}

/// Exponential Moving Average over close prices.
///
/// The first EMA value is seeded with the SMA of the first `period` values.
/// Multiplier k = 2 / (period + 1).
pub fn calc_ema_impl(data: &[PricePoint], period: usize) -> Vec<IndicatorPoint> {
    if period == 0 || period > data.len() {
        return Vec::new();
    }

    let k: f64 = 2.0 / (period + 1) as f64;

    // Seed: SMA of first `period` closes.
    let sma: f64 = data[..period].iter().map(|p| p.close).sum::<f64>() / period as f64;

    let mut result: Vec<IndicatorPoint> = Vec::with_capacity(data.len() - period + 1);
    result.push(IndicatorPoint {
        ts: data[period - 1].ts,
        value: sma,
    });

    let mut prev_ema = sma;
    for i in period..data.len() {
        let ema = data[i].close * k + prev_ema * (1.0 - k);
        result.push(IndicatorPoint {
            ts: data[i].ts,
            value: ema,
        });
        prev_ema = ema;
    }

    result
}

/// Relative Strength Index using Wilder's smoothing.
///
/// Returns values in the 0..=100 range. If all changes are gains the RSI is
/// 100; if all are losses the RSI is 0.
pub fn calc_rsi_impl(data: &[PricePoint], period: usize) -> Vec<IndicatorPoint> {
    if period == 0 || data.len() < period + 1 {
        return Vec::new();
    }

    let mut result: Vec<IndicatorPoint> = Vec::with_capacity(data.len() - period);

    // Compute initial average gain / loss over the first `period` changes.
    let mut avg_gain: f64 = 0.0;
    let mut avg_loss: f64 = 0.0;

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

    // First RSI value.
    let rsi = if avg_loss == 0.0 {
        100.0
    } else if avg_gain == 0.0 {
        0.0
    } else {
        let rs = avg_gain / avg_loss;
        100.0 - (100.0 / (1.0 + rs))
    };

    result.push(IndicatorPoint {
        ts: data[period].ts,
        value: rsi,
    });

    // Subsequent values using Wilder's smoothing.
    for i in (period + 1)..data.len() {
        let change = data[i].close - data[i - 1].close;
        let (gain, loss) = if change > 0.0 {
            (change, 0.0)
        } else {
            (0.0, change.abs())
        };

        avg_gain = (avg_gain * (period as f64 - 1.0) + gain) / period as f64;
        avg_loss = (avg_loss * (period as f64 - 1.0) + loss) / period as f64;

        let rsi = if avg_loss == 0.0 {
            100.0
        } else if avg_gain == 0.0 {
            0.0
        } else {
            let rs = avg_gain / avg_loss;
            100.0 - (100.0 / (1.0 + rs))
        };

        result.push(IndicatorPoint {
            ts: data[i].ts,
            value: rsi,
        });
    }

    result
}

/// Volume-Weighted Average Price.
///
/// VWAP = cumulative(typical_price * volume) / cumulative(volume)
/// where typical_price = (high + low + close) / 3.
pub fn calc_vwap_impl(data: &[PricePoint]) -> Vec<IndicatorPoint> {
    if data.is_empty() {
        return Vec::new();
    }

    let mut result: Vec<IndicatorPoint> = Vec::with_capacity(data.len());
    let mut cum_tp_vol: f64 = 0.0;
    let mut cum_vol: f64 = 0.0;

    for p in data {
        let typical_price = (p.high + p.low + p.close) / 3.0;
        cum_tp_vol += typical_price * p.volume;
        cum_vol += p.volume;

        let vwap = if cum_vol == 0.0 { 0.0 } else { cum_tp_vol / cum_vol };

        result.push(IndicatorPoint {
            ts: p.ts,
            value: vwap,
        });
    }

    result
}

/// Case-insensitive symbol / name search with relevance scoring.
///
/// Scoring rules (highest applicable score wins per entry):
///   - Exact symbol match       -> 100
///   - Symbol starts with query -> 80
///   - Symbol contains query    -> 60
///   - Name starts with query   -> 40
///   - Name contains query      -> 20
///
/// Results are sorted by score descending and capped at `max_results`.
pub fn filter_symbols_impl(
    entries: &[SymbolEntry],
    query: &str,
    max_results: usize,
) -> Vec<SymbolEntry> {
    if query.is_empty() {
        return entries.iter().take(max_results).cloned().collect();
    }

    let q = query.to_lowercase();

    let mut scored: Vec<ScoredEntry> = Vec::new();

    for entry in entries {
        let sym = entry.symbol.to_lowercase();
        let name = entry.name.to_lowercase();

        let score = if sym == q {
            100
        } else if sym.starts_with(&q) {
            80
        } else if sym.contains(&q) {
            60
        } else if name.starts_with(&q) {
            40
        } else if name.contains(&q) {
            20
        } else {
            continue; // no match
        };

        scored.push(ScoredEntry {
            symbol: entry.symbol.clone(),
            name: entry.name.clone(),
            score,
        });
    }

    // Sort descending by score, then alphabetically by symbol for stability.
    scored.sort_by(|a, b| b.score.cmp(&a.score).then_with(|| a.symbol.cmp(&b.symbol)));
    scored.truncate(max_results);

    scored
        .into_iter()
        .map(|s| SymbolEntry {
            symbol: s.symbol,
            name: s.name,
        })
        .collect()
}

// ---------------------------------------------------------------------------
// wasm_bindgen thin wrappers (JsValue <-> native types)
// ---------------------------------------------------------------------------

#[wasm_bindgen]
pub fn lttb_downsample(data: JsValue, threshold: usize) -> JsValue {
    let points: Vec<DataPoint> = serde_wasm_bindgen::from_value(data).unwrap_or_default();
    let result = lttb_downsample_impl(&points, threshold);
    serde_wasm_bindgen::to_value(&result).unwrap_or(JsValue::NULL)
}

#[wasm_bindgen]
pub fn calc_sma(data: JsValue, period: usize) -> JsValue {
    let points: Vec<PricePoint> = serde_wasm_bindgen::from_value(data).unwrap_or_default();
    let result = calc_sma_impl(&points, period);
    serde_wasm_bindgen::to_value(&result).unwrap_or(JsValue::NULL)
}

#[wasm_bindgen]
pub fn calc_ema(data: JsValue, period: usize) -> JsValue {
    let points: Vec<PricePoint> = serde_wasm_bindgen::from_value(data).unwrap_or_default();
    let result = calc_ema_impl(&points, period);
    serde_wasm_bindgen::to_value(&result).unwrap_or(JsValue::NULL)
}

#[wasm_bindgen]
pub fn calc_rsi(data: JsValue, period: usize) -> JsValue {
    let points: Vec<PricePoint> = serde_wasm_bindgen::from_value(data).unwrap_or_default();
    let result = calc_rsi_impl(&points, period);
    serde_wasm_bindgen::to_value(&result).unwrap_or(JsValue::NULL)
}

#[wasm_bindgen]
pub fn calc_vwap(data: JsValue) -> JsValue {
    let points: Vec<PricePoint> = serde_wasm_bindgen::from_value(data).unwrap_or_default();
    let result = calc_vwap_impl(&points);
    serde_wasm_bindgen::to_value(&result).unwrap_or(JsValue::NULL)
}

#[wasm_bindgen]
pub fn filter_symbols(entries: JsValue, query: JsValue, max_results: usize) -> JsValue {
    let entries: Vec<SymbolEntry> = serde_wasm_bindgen::from_value(entries).unwrap_or_default();
    let query: String = serde_wasm_bindgen::from_value(query).unwrap_or_default();
    let result = filter_symbols_impl(&entries, &query, max_results);
    serde_wasm_bindgen::to_value(&result).unwrap_or(JsValue::NULL)
}

// ---------------------------------------------------------------------------
// Unit tests (pure Rust — no JsValue required)
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    // -----------------------------------------------------------------------
    // Helpers
    // -----------------------------------------------------------------------

    fn dp(ts: f64, value: f64) -> DataPoint {
        DataPoint { ts, value }
    }

    fn pp(ts: f64, open: f64, high: f64, low: f64, close: f64, volume: f64) -> PricePoint {
        PricePoint {
            ts,
            open,
            high,
            low,
            close,
            volume,
        }
    }

    fn sample_prices() -> Vec<PricePoint> {
        vec![
            pp(1.0, 10.0, 12.0, 9.0, 11.0, 100.0),
            pp(2.0, 11.0, 13.0, 10.0, 12.0, 150.0),
            pp(3.0, 12.0, 14.0, 11.0, 13.0, 200.0),
            pp(4.0, 13.0, 15.0, 12.0, 14.0, 120.0),
            pp(5.0, 14.0, 16.0, 13.0, 15.0, 180.0),
            pp(6.0, 15.0, 17.0, 14.0, 14.0, 160.0),
            pp(7.0, 14.0, 15.0, 12.0, 13.0, 140.0),
            pp(8.0, 13.0, 14.0, 11.0, 12.0, 130.0),
            pp(9.0, 12.0, 13.0, 10.0, 11.0, 110.0),
            pp(10.0, 11.0, 12.0, 9.0, 10.0, 100.0),
        ]
    }

    // -----------------------------------------------------------------------
    // LTTB downsampling
    // -----------------------------------------------------------------------

    #[test]
    fn lttb_empty_input() {
        let result = lttb_downsample_impl(&[], 5);
        assert!(result.is_empty());
    }

    #[test]
    fn lttb_data_smaller_than_threshold() {
        let data = vec![dp(1.0, 10.0), dp(2.0, 20.0)];
        let result = lttb_downsample_impl(&data, 5);
        assert_eq!(result.len(), 2);
        assert_eq!(result[0].ts, 1.0);
        assert_eq!(result[1].ts, 2.0);
    }

    #[test]
    fn lttb_threshold_less_than_3() {
        let data = vec![dp(1.0, 10.0), dp(2.0, 20.0), dp(3.0, 30.0)];
        let result = lttb_downsample_impl(&data, 2);
        assert_eq!(result.len(), 3, "threshold < 3 should return data as-is");
    }

    #[test]
    fn lttb_normal_downsample() {
        let data: Vec<DataPoint> = (0..100).map(|i| dp(i as f64, (i as f64).sin())).collect();
        let result = lttb_downsample_impl(&data, 20);
        assert_eq!(result.len(), 20);
        // First and last points must be preserved.
        assert_eq!(result[0].ts, 0.0);
        assert_eq!(result[19].ts, 99.0);
    }

    #[test]
    fn lttb_threshold_of_3() {
        let data: Vec<DataPoint> = (0..10).map(|i| dp(i as f64, (i * i) as f64)).collect();
        let result = lttb_downsample_impl(&data, 3);
        assert_eq!(result.len(), 3);
        assert_eq!(result[0].ts, 0.0);
        assert_eq!(result[2].ts, 9.0);
    }

    #[test]
    fn lttb_data_equal_to_threshold() {
        let data: Vec<DataPoint> = (0..5).map(|i| dp(i as f64, i as f64)).collect();
        let result = lttb_downsample_impl(&data, 5);
        assert_eq!(result.len(), 5);
    }

    // -----------------------------------------------------------------------
    // SMA
    // -----------------------------------------------------------------------

    #[test]
    fn sma_normal_case() {
        let data = sample_prices();
        let result = calc_sma_impl(&data, 3);
        // First value: avg of close[0..3] = (11+12+13)/3 = 12.0
        assert_eq!(result.len(), 8);
        assert!((result[0].value - 12.0).abs() < 1e-9);
        assert_eq!(result[0].ts, 3.0);
        // Second value: avg of close[1..4] = (12+13+14)/3 = 13.0
        assert!((result[1].value - 13.0).abs() < 1e-9);
    }

    #[test]
    fn sma_period_greater_than_data() {
        let data = sample_prices();
        let result = calc_sma_impl(&data, 100);
        assert!(result.is_empty());
    }

    #[test]
    fn sma_period_of_1() {
        let data = sample_prices();
        let result = calc_sma_impl(&data, 1);
        assert_eq!(result.len(), data.len());
        for (i, point) in result.iter().enumerate() {
            assert!((point.value - data[i].close).abs() < 1e-9);
        }
    }

    #[test]
    fn sma_period_equals_data_len() {
        let data = sample_prices();
        let n = data.len();
        let result = calc_sma_impl(&data, n);
        assert_eq!(result.len(), 1);
        let expected: f64 = data.iter().map(|p| p.close).sum::<f64>() / n as f64;
        assert!((result[0].value - expected).abs() < 1e-9);
    }

    // -----------------------------------------------------------------------
    // EMA
    // -----------------------------------------------------------------------

    #[test]
    fn ema_normal_case() {
        let data = sample_prices();
        let result = calc_ema_impl(&data, 3);
        assert_eq!(result.len(), 8);
        // First EMA value must equal the SMA of the first 3 closes.
        let first_sma = (11.0 + 12.0 + 13.0) / 3.0;
        assert!((result[0].value - first_sma).abs() < 1e-9);
    }

    #[test]
    fn ema_first_value_matches_sma() {
        let data = sample_prices();
        let period = 5;
        let ema_result = calc_ema_impl(&data, period);
        let sma_result = calc_sma_impl(&data, period);
        // The first EMA value should equal the first SMA value.
        assert!(
            (ema_result[0].value - sma_result[0].value).abs() < 1e-9,
            "First EMA ({}) should match first SMA ({})",
            ema_result[0].value,
            sma_result[0].value
        );
    }

    #[test]
    fn ema_period_greater_than_data() {
        let data = sample_prices();
        let result = calc_ema_impl(&data, 100);
        assert!(result.is_empty());
    }

    #[test]
    fn ema_multiplier_correctness() {
        let data = sample_prices();
        let period = 3;
        let k = 2.0 / (period as f64 + 1.0); // 0.5
        let result = calc_ema_impl(&data, period);
        // Verify second EMA value manually.
        let first_ema = (11.0 + 12.0 + 13.0) / 3.0; // 12.0
        let second_ema = data[3].close * k + first_ema * (1.0 - k); // 14*0.5 + 12*0.5 = 13.0
        assert!((result[1].value - second_ema).abs() < 1e-9);
    }

    // -----------------------------------------------------------------------
    // RSI
    // -----------------------------------------------------------------------

    #[test]
    fn rsi_normal_case() {
        let data = sample_prices();
        let result = calc_rsi_impl(&data, 5);
        assert!(!result.is_empty());
        for point in &result {
            assert!(point.value >= 0.0 && point.value <= 100.0);
        }
    }

    #[test]
    fn rsi_all_gains() {
        // Monotonically increasing closes -> RSI = 100.
        let data: Vec<PricePoint> = (0..10)
            .map(|i| pp(i as f64, 0.0, 0.0, 0.0, 10.0 + i as f64, 100.0))
            .collect();
        let result = calc_rsi_impl(&data, 5);
        assert!(!result.is_empty());
        for point in &result {
            assert!(
                (point.value - 100.0).abs() < 1e-9,
                "Expected RSI=100, got {}",
                point.value
            );
        }
    }

    #[test]
    fn rsi_all_losses() {
        // Monotonically decreasing closes -> RSI = 0.
        let data: Vec<PricePoint> = (0..10)
            .map(|i| pp(i as f64, 0.0, 0.0, 0.0, 100.0 - i as f64, 100.0))
            .collect();
        let result = calc_rsi_impl(&data, 5);
        assert!(!result.is_empty());
        for point in &result {
            assert!(
                point.value.abs() < 1e-9,
                "Expected RSI=0, got {}",
                point.value
            );
        }
    }

    #[test]
    fn rsi_insufficient_data() {
        let data = vec![pp(1.0, 0.0, 0.0, 0.0, 10.0, 100.0)];
        let result = calc_rsi_impl(&data, 5);
        assert!(result.is_empty());
    }

    // -----------------------------------------------------------------------
    // VWAP
    // -----------------------------------------------------------------------

    #[test]
    fn vwap_normal_case() {
        let data = sample_prices();
        let result = calc_vwap_impl(&data);
        assert_eq!(result.len(), data.len());

        // First VWAP value: typical = (12+9+11)/3 = 32/3 ≈ 10.6667
        let tp0 = (12.0 + 9.0 + 11.0) / 3.0;
        assert!(
            (result[0].value - tp0).abs() < 1e-4,
            "Expected ~{}, got {}",
            tp0,
            result[0].value
        );
    }

    #[test]
    fn vwap_single_point() {
        let data = vec![pp(1.0, 10.0, 15.0, 5.0, 12.0, 1000.0)];
        let result = calc_vwap_impl(&data);
        assert_eq!(result.len(), 1);
        let expected_tp = (15.0 + 5.0 + 12.0) / 3.0; // ~10.6667
        assert!((result[0].value - expected_tp).abs() < 1e-9);
    }

    #[test]
    fn vwap_empty() {
        let result = calc_vwap_impl(&[]);
        assert!(result.is_empty());
    }

    #[test]
    fn vwap_cumulative_behaviour() {
        let data = vec![
            pp(1.0, 0.0, 30.0, 10.0, 20.0, 100.0),
            pp(2.0, 0.0, 60.0, 20.0, 40.0, 200.0),
        ];
        let result = calc_vwap_impl(&data);
        // Point 1: tp = (30+10+20)/3 = 20, cum_tp_vol = 2000, cum_vol = 100, vwap = 20
        assert!((result[0].value - 20.0).abs() < 1e-9);
        // Point 2: tp = (60+20+40)/3 = 40, cum_tp_vol = 2000+8000 = 10000, cum_vol = 300
        let expected = 10000.0 / 300.0;
        assert!((result[1].value - expected).abs() < 1e-9);
    }

    // -----------------------------------------------------------------------
    // filter_symbols
    // -----------------------------------------------------------------------

    fn sample_entries() -> Vec<SymbolEntry> {
        vec![
            SymbolEntry {
                symbol: "AAPL".to_string(),
                name: "Apple Inc.".to_string(),
            },
            SymbolEntry {
                symbol: "MSFT".to_string(),
                name: "Microsoft Corporation".to_string(),
            },
            SymbolEntry {
                symbol: "AMZN".to_string(),
                name: "Amazon.com Inc.".to_string(),
            },
            SymbolEntry {
                symbol: "GOOG".to_string(),
                name: "Alphabet Inc.".to_string(),
            },
            SymbolEntry {
                symbol: "META".to_string(),
                name: "Meta Platforms Inc.".to_string(),
            },
            SymbolEntry {
                symbol: "TSLA".to_string(),
                name: "Tesla Inc.".to_string(),
            },
            SymbolEntry {
                symbol: "AA".to_string(),
                name: "Alcoa Corporation".to_string(),
            },
        ]
    }

    #[test]
    fn filter_exact_match_scores_highest() {
        let entries = sample_entries();
        let result = filter_symbols_impl(&entries, "AAPL", 10);
        assert!(!result.is_empty());
        assert_eq!(result[0].symbol, "AAPL");
    }

    #[test]
    fn filter_prefix_match() {
        let entries = sample_entries();
        let result = filter_symbols_impl(&entries, "AA", 10);
        // "AA" exact match on AA (100), "AAPL" starts with "AA" (80)
        assert!(result.len() >= 2);
        assert_eq!(result[0].symbol, "AA", "Exact match should rank first");
        assert_eq!(result[1].symbol, "AAPL", "Prefix match should rank second");
    }

    #[test]
    fn filter_no_match() {
        let entries = sample_entries();
        let result = filter_symbols_impl(&entries, "XYZ", 10);
        assert!(result.is_empty());
    }

    #[test]
    fn filter_case_insensitivity() {
        let entries = sample_entries();
        let result = filter_symbols_impl(&entries, "aapl", 10);
        assert!(!result.is_empty());
        assert_eq!(result[0].symbol, "AAPL");
    }

    #[test]
    fn filter_name_match() {
        let entries = sample_entries();
        let result = filter_symbols_impl(&entries, "Tesla", 10);
        assert_eq!(result.len(), 1);
        assert_eq!(result[0].symbol, "TSLA");
    }

    #[test]
    fn filter_max_results_limit() {
        let entries = sample_entries();
        let result = filter_symbols_impl(&entries, "a", 2);
        assert_eq!(result.len(), 2);
    }

    #[test]
    fn filter_name_contains() {
        let entries = sample_entries();
        // "form" is contained in "Meta Platforms Inc." -> score 20
        let result = filter_symbols_impl(&entries, "Platforms", 10);
        assert_eq!(result.len(), 1);
        assert_eq!(result[0].symbol, "META");
    }

    #[test]
    fn filter_empty_query_returns_up_to_max() {
        let entries = sample_entries();
        let result = filter_symbols_impl(&entries, "", 3);
        assert_eq!(result.len(), 3);
    }

    #[test]
    fn filter_symbol_contains() {
        let entries = sample_entries();
        // "OO" is contained in "GOOG" -> symbol contains = 60
        let result = filter_symbols_impl(&entries, "OO", 10);
        assert!(!result.is_empty());
        assert_eq!(result[0].symbol, "GOOG");
    }
}
