import { useEffect, useRef, useState, useCallback } from 'react';
import {
  createChart,
  type IChartApi,
  type ISeriesApi,
  ColorType,
  type CandlestickData,
  type LineData,
  type AreaData,
  type Time,
} from 'lightweight-charts';
import { getPrices, createEventSource, type PriceParams } from '../lib/api';
import type { PriceBar, PriceEvent } from '../lib/types';
import { initWasm, lttbDownsample, calcSma, calcEma, calcRsi, calcVwap } from '../lib/wasm';

interface InstrumentChartProps {
  symbol: string;
  marketClosed?: boolean;
}

type DateRange = 'live' | '1D' | '1W' | '1M' | '3M' | '6M' | '1Y' | '2Y';
type BarInterval = '1min' | '5min' | '15min' | '1h' | '1d' | '1w' | '1m';
type Indicator = 'sma' | 'ema' | 'vwap' | 'rsi';

const INDICATOR_COLORS: Record<Indicator, string> = {
  sma: '#eab308',
  ema: '#a855f7',
  vwap: '#06b6d4',
  rsi: '#f97316',
};

const DEFAULT_INTERVALS: Record<DateRange, BarInterval> = {
  live: '1min',
  '1D': '1min',
  '1W': '1h',
  '1M': '1d',
  '3M': '1d',
  '6M': '1d',
  '1Y': '1d',
  '2Y': '1w',
};

const VALID_INTERVALS: Record<DateRange, BarInterval[]> = {
  live: ['1min'],
  '1D': ['1min', '5min', '15min'],
  '1W': ['5min', '15min', '1h'],
  '1M': ['15min', '1h', '1d'],
  '3M': ['1h', '1d'],
  '6M': ['1d', '1w'],
  '1Y': ['1d', '1w'],
  '2Y': ['1d', '1w', '1m'],
};

const INTERVAL_LABELS: Record<BarInterval, string> = {
  '1min': '1m',
  '5min': '5m',
  '15min': '15m',
  '1h': '1H',
  '1d': '1D',
  '1w': '1W',
  '1m': '1M',
};

const DATE_RANGE_OFFSETS: Record<string, number> = {
  '1D': 1,
  '1W': 7,
  '1M': 30,
  '3M': 90,
  '6M': 180,
  '1Y': 365,
  '2Y': 730,
};

function getDateRangeParams(range: DateRange): { from?: string } {
  if (range === 'live') return {};
  const days = DATE_RANGE_OFFSETS[range];
  if (!days) return {};
  const from = new Date(Date.now() - days * 86400000);
  return { from: from.toISOString() };
}

export default function InstrumentChart({ symbol, marketClosed = false }: InstrumentChartProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const rsiContainerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const rsiChartRef = useRef<IChartApi | null>(null);
  const mainSeriesRef = useRef<ISeriesApi<'Candlestick'> | ISeriesApi<'Area'> | null>(null);
  const indicatorSeriesRef = useRef<Map<string, ISeriesApi<'Line'>>>(new Map());
  const rsiSeriesRef = useRef<ISeriesApi<'Line'> | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);

  const [dateRange, setDateRange] = useState<DateRange>('1M');
  const [barInterval, setBarInterval] = useState<BarInterval>('1d');
  const [indicators, setIndicators] = useState<Set<Indicator>>(new Set());
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [priceData, setPriceData] = useState<PriceBar[]>([]);

  // Initialize WASM
  useEffect(() => {
    initWasm();
  }, []);

  const toggleIndicator = (ind: Indicator) => {
    setIndicators((prev) => {
      const next = new Set(prev);
      if (next.has(ind)) {
        next.delete(ind);
      } else {
        next.add(ind);
      }
      return next;
    });
  };

  const handleDateRangeChange = (range: DateRange) => {
    setDateRange(range);
    setBarInterval(DEFAULT_INTERVALS[range]);
  };

  // Fetch price data
  const fetchPrices = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const rangeParams = getDateRangeParams(dateRange);
      const params: PriceParams = {
        interval: barInterval,
        from: rangeParams.from,
      };

      // Set sensible limits based on range
      if (dateRange === 'live' || dateRange === '1D') {
        params.limit = 500;
      } else if (dateRange === '1W') {
        params.limit = 1000;
      } else {
        params.limit = 2000;
      }

      const data = await getPrices(symbol, params);
      setPriceData(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load price data');
    } finally {
      setIsLoading(false);
    }
  }, [symbol, dateRange, barInterval]);

  useEffect(() => {
    fetchPrices();
  }, [fetchPrices]);

  // Build and render chart
  useEffect(() => {
    if (!containerRef.current || priceData.length === 0) return;

    // Clean up existing charts
    if (chartRef.current) {
      chartRef.current.remove();
      chartRef.current = null;
    }
    if (rsiChartRef.current) {
      rsiChartRef.current.remove();
      rsiChartRef.current = null;
    }
    indicatorSeriesRef.current.clear();
    rsiSeriesRef.current = null;

    const showTimeAxis = barInterval !== '1d' && barInterval !== '1w' && barInterval !== '1m';

    const chart = createChart(containerRef.current, {
      width: containerRef.current.clientWidth,
      height: 400,
      layout: {
        background: { type: ColorType.Solid, color: '#111827' },
        textColor: '#6b7280',
      },
      grid: {
        vertLines: { color: '#1f293722' },
        horzLines: { color: '#1f293722' },
      },
      crosshair: {
        vertLine: { color: '#3b82f6', width: 1, style: 2 },
        horzLine: { color: '#3b82f6', width: 1, style: 2 },
      },
      rightPriceScale: {
        borderColor: '#1f2937',
      },
      timeScale: {
        borderColor: '#1f2937',
        timeVisible: showTimeAxis,
        secondsVisible: false,
      },
    });

    chartRef.current = chart;

    // Prepare time series data with LTTB downsampling
    const closePoints = priceData.map((bar) => ({
      ts: new Date(bar.ts).getTime() / 1000,
      value: bar.close,
    }));

    const downsampled =
      closePoints.length > 1000 ? lttbDownsample(closePoints, 1000) : closePoints;

    // Use area chart for live mode, candlestick for everything else
    if (dateRange === 'live') {
      const areaData: AreaData[] = downsampled.map((pt) => ({
        time: pt.ts as Time,
        value: pt.value,
      }));

      const series = chart.addAreaSeries({
        lineColor: '#3b82f6',
        topColor: 'rgba(59, 130, 246, 0.2)',
        bottomColor: 'transparent',
        lineWidth: 2,
      });
      series.setData(areaData);
      mainSeriesRef.current = series;
    } else {
      // Candlestick for all OHLC intervals
      const candleData: CandlestickData[] = priceData.map((bar) => ({
        time: (new Date(bar.ts).getTime() / 1000) as Time,
        open: bar.open,
        high: bar.high,
        low: bar.low,
        close: bar.close,
      }));

      const series = chart.addCandlestickSeries({
        upColor: '#22c55e',
        downColor: '#ef4444',
        borderUpColor: '#22c55e',
        borderDownColor: '#ef4444',
        wickUpColor: '#22c55e',
        wickDownColor: '#ef4444',
      });
      series.setData(candleData);
      mainSeriesRef.current = series;
    }

    // Render indicator overlays
    const renderIndicators = () => {
      indicatorSeriesRef.current.forEach((series) => {
        try { chart.removeSeries(series); } catch { /* already removed */ }
      });
      indicatorSeriesRef.current.clear();

      if (indicators.has('sma')) {
        const smaData = calcSma(closePoints, 20);
        const series = chart.addLineSeries({
          color: INDICATOR_COLORS.sma,
          lineWidth: 1,
          priceLineVisible: false,
          lastValueVisible: false,
        });
        series.setData(
          smaData.map((pt) => ({ time: pt.ts as Time, value: pt.value })) as LineData[]
        );
        indicatorSeriesRef.current.set('sma', series);
      }

      if (indicators.has('ema')) {
        const emaData = calcEma(closePoints, 20);
        const series = chart.addLineSeries({
          color: INDICATOR_COLORS.ema,
          lineWidth: 1,
          priceLineVisible: false,
          lastValueVisible: false,
        });
        series.setData(
          emaData.map((pt) => ({ time: pt.ts as Time, value: pt.value })) as LineData[]
        );
        indicatorSeriesRef.current.set('ema', series);
      }

      if (indicators.has('vwap')) {
        const vwapData = calcVwap(closePoints);
        const series = chart.addLineSeries({
          color: INDICATOR_COLORS.vwap,
          lineWidth: 1,
          priceLineVisible: false,
          lastValueVisible: false,
        });
        series.setData(
          vwapData.map((pt) => ({ time: pt.ts as Time, value: pt.value })) as LineData[]
        );
        indicatorSeriesRef.current.set('vwap', series);
      }
    };

    renderIndicators();

    // RSI in separate pane
    if (indicators.has('rsi') && rsiContainerRef.current) {
      const rsiChart = createChart(rsiContainerRef.current, {
        width: rsiContainerRef.current.clientWidth,
        height: 120,
        layout: {
          background: { type: ColorType.Solid, color: '#111827' },
          textColor: '#6b7280',
        },
        grid: {
          vertLines: { color: '#1f293722' },
          horzLines: { color: '#1f293722' },
        },
        rightPriceScale: {
          borderColor: '#1f2937',
          scaleMargins: { top: 0.1, bottom: 0.1 },
        },
        timeScale: {
          borderColor: '#1f2937',
          timeVisible: showTimeAxis,
          visible: false,
        },
        crosshair: {
          vertLine: { visible: false },
          horzLine: { color: '#3b82f6', width: 1, style: 2 },
        },
      });

      const rsiData = calcRsi(closePoints, 14);
      const rsiSeries = rsiChart.addLineSeries({
        color: INDICATOR_COLORS.rsi,
        lineWidth: 1,
        priceLineVisible: false,
        lastValueVisible: true,
      });
      rsiSeries.setData(
        rsiData.map((pt) => ({ time: pt.ts as Time, value: pt.value })) as LineData[]
      );

      // Overbought/oversold reference lines
      const overbought = rsiChart.addLineSeries({
        color: '#ef444444',
        lineWidth: 1,
        lineStyle: 2,
        priceLineVisible: false,
        lastValueVisible: false,
      });
      const oversold = rsiChart.addLineSeries({
        color: '#22c55e44',
        lineWidth: 1,
        lineStyle: 2,
        priceLineVisible: false,
        lastValueVisible: false,
      });

      if (rsiData.length >= 2) {
        const times = rsiData.map((pt) => pt.ts as Time);
        overbought.setData([
          { time: times[0], value: 70 },
          { time: times[times.length - 1], value: 70 },
        ] as LineData[]);
        oversold.setData([
          { time: times[0], value: 30 },
          { time: times[times.length - 1], value: 30 },
        ] as LineData[]);
      }

      rsiChartRef.current = rsiChart;
      rsiSeriesRef.current = rsiSeries;
      rsiChart.timeScale().fitContent();
    }

    chart.timeScale().fitContent();

    // Resize handler
    const handleResize = () => {
      if (containerRef.current && chartRef.current) {
        chartRef.current.applyOptions({ width: containerRef.current.clientWidth });
      }
      if (rsiContainerRef.current && rsiChartRef.current) {
        rsiChartRef.current.applyOptions({ width: rsiContainerRef.current.clientWidth });
      }
    };

    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
      if (chartRef.current) {
        chartRef.current.remove();
        chartRef.current = null;
      }
      if (rsiChartRef.current) {
        rsiChartRef.current.remove();
        rsiChartRef.current = null;
      }
    };
  }, [priceData, barInterval, dateRange, indicators]);

  // SSE live updates (only in live mode)
  useEffect(() => {
    if (marketClosed || dateRange !== 'live') return;

    const es = createEventSource(`/api/stream/instruments/${encodeURIComponent(symbol)}`);

    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data) as PriceEvent;
        const time = (new Date(data.ts).getTime() / 1000) as Time;

        if (mainSeriesRef.current) {
          (mainSeriesRef.current as ISeriesApi<'Area'>).update({
            time,
            value: data.last_price,
          });
        }
      } catch {
        // ignore parse errors
      }
    };

    eventSourceRef.current = es;

    return () => {
      es.close();
      eventSourceRef.current = null;
    };
  }, [symbol, dateRange, marketClosed]);

  const dateRanges: { key: DateRange; label: string }[] = [
    { key: 'live', label: 'Live' },
    { key: '1D', label: '1D' },
    { key: '1W', label: '1W' },
    { key: '1M', label: '1M' },
    { key: '3M', label: '3M' },
    { key: '6M', label: '6M' },
    { key: '1Y', label: '1Y' },
    { key: '2Y', label: '2Y' },
  ];

  const validIntervals = VALID_INTERVALS[dateRange];

  const indicatorButtons: { key: Indicator; label: string }[] = [
    { key: 'sma', label: 'SMA(20)' },
    { key: 'ema', label: 'EMA(20)' },
    { key: 'vwap', label: 'VWAP' },
    { key: 'rsi', label: 'RSI(14)' },
  ];

  return (
    <div className="space-y-3">
      {/* Controls */}
      <div className="flex items-center justify-between flex-wrap gap-2">
        <div className="flex items-center gap-2 flex-wrap">
          {/* Date range selector */}
          <div className="flex items-center gap-1 bg-terminal-bg rounded p-1">
            {dateRanges.map((dr) => (
              <button
                key={dr.key}
                onClick={() => handleDateRangeChange(dr.key)}
                className={`px-3 py-1 text-xs rounded transition-colors ${
                  dateRange === dr.key
                    ? 'bg-terminal-accent text-white'
                    : 'text-terminal-muted hover:text-terminal-text'
                }`}
              >
                {dr.label}
              </button>
            ))}
          </div>

          {/* Bar interval selector */}
          {validIntervals.length > 1 && (
            <div className="flex items-center gap-1 bg-terminal-bg rounded p-1">
              {validIntervals.map((int) => (
                <button
                  key={int}
                  onClick={() => setBarInterval(int)}
                  className={`px-2 py-1 text-xs rounded transition-colors ${
                    barInterval === int
                      ? 'bg-terminal-panel text-terminal-text border border-terminal-border'
                      : 'text-terminal-muted hover:text-terminal-text'
                  }`}
                >
                  {INTERVAL_LABELS[int]}
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Indicator toggles */}
        <div className="flex items-center gap-1">
          {indicatorButtons.map((ind) => (
            <button
              key={ind.key}
              onClick={() => toggleIndicator(ind.key)}
              className={`px-2 py-1 text-xs rounded border transition-colors ${
                indicators.has(ind.key)
                  ? 'border-current text-white'
                  : 'border-terminal-border text-terminal-muted hover:text-terminal-text'
              }`}
              style={indicators.has(ind.key) ? { color: INDICATOR_COLORS[ind.key], borderColor: INDICATOR_COLORS[ind.key] } : undefined}
            >
              {ind.label}
            </button>
          ))}
        </div>
      </div>

      {/* Chart container */}
      <div className={`terminal-panel overflow-hidden relative ${marketClosed ? 'market-closed-overlay' : ''}`}>
        {marketClosed && (
          <div className="absolute top-3 right-3 z-10 bg-terminal-bg/80 text-terminal-muted text-xs px-2 py-1 rounded border border-terminal-border">
            Market Closed
          </div>
        )}

        {isLoading && (
          <div className="flex items-center justify-center h-[400px]">
            <div className="w-5 h-5 border-2 border-terminal-accent border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        {error && !isLoading && (
          <div className="flex items-center justify-center h-[400px]">
            <div className="text-center">
              <p className="text-terminal-red text-sm mb-2">{error}</p>
              <button onClick={fetchPrices} className="btn-primary text-xs">
                Retry
              </button>
            </div>
          </div>
        )}

        {!isLoading && !error && priceData.length === 0 && (
          <div className="flex items-center justify-center h-[400px]">
            <p className="text-terminal-muted text-sm">No price data available</p>
          </div>
        )}

        <div
          ref={containerRef}
          className={priceData.length === 0 || isLoading || error ? 'hidden' : ''}
        />

        {/* RSI pane */}
        {indicators.has('rsi') && (
          <div className="border-t border-terminal-border">
            <div className="text-xs text-terminal-muted px-2 py-1">RSI(14)</div>
            <div ref={rsiContainerRef} />
          </div>
        )}
      </div>

      {/* Legend */}
      {indicators.size > 0 && (
        <div className="flex items-center gap-4 text-xs">
          {[...indicators].map((ind) => (
            <span key={ind} className="flex items-center gap-1">
              <span
                className="inline-block w-3 h-0.5"
                style={{ backgroundColor: INDICATOR_COLORS[ind] }}
              />
              <span className="text-terminal-muted uppercase">{ind}</span>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
