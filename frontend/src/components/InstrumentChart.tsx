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

type Interval = '1min' | '1h' | '1d';
type Indicator = 'sma' | 'ema' | 'vwap' | 'rsi';

const INDICATOR_COLORS: Record<Indicator, string> = {
  sma: '#eab308',
  ema: '#a855f7',
  vwap: '#06b6d4',
  rsi: '#f97316',
};

export default function InstrumentChart({ symbol, marketClosed = false }: InstrumentChartProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const rsiContainerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const rsiChartRef = useRef<IChartApi | null>(null);
  const mainSeriesRef = useRef<ISeriesApi<'Candlestick'> | ISeriesApi<'Area'> | null>(null);
  const indicatorSeriesRef = useRef<Map<string, ISeriesApi<'Line'>>>(new Map());
  const rsiSeriesRef = useRef<ISeriesApi<'Line'> | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);

  const [interval, setInterval] = useState<Interval>('1d');
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

  // Fetch price data
  const fetchPrices = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const params: PriceParams = { interval };
      if (interval === '1min') {
        params.limit = 390; // one trading day
      } else if (interval === '1h') {
        params.limit = 500;
      } else {
        params.limit = 365;
      }
      const data = await getPrices(symbol, params);
      setPriceData(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load price data');
    } finally {
      setIsLoading(false);
    }
  }, [symbol, interval]);

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
        timeVisible: interval !== '1d',
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

    if (interval === '1d') {
      // Candlestick for daily
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
    } else {
      // Area chart for intraday
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
    }

    // Render indicator overlays
    const renderIndicators = () => {
      // Remove old indicator series
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
          timeVisible: interval !== '1d',
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
  }, [priceData, interval, indicators]);

  // SSE live updates
  useEffect(() => {
    if (marketClosed) return;

    const es = createEventSource(`/api/stream/instruments/${encodeURIComponent(symbol)}`);

    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data) as PriceEvent;
        const time = (new Date(data.ts).getTime() / 1000) as Time;

        if (mainSeriesRef.current) {
          if (interval === '1d') {
            // Update the last candlestick
            (mainSeriesRef.current as ISeriesApi<'Candlestick'>).update({
              time,
              open: data.last_price,
              high: data.last_price,
              low: data.last_price,
              close: data.last_price,
            });
          } else {
            (mainSeriesRef.current as ISeriesApi<'Area'>).update({
              time,
              value: data.last_price,
            });
          }
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
  }, [symbol, interval, marketClosed]);

  const intervals: { key: Interval; label: string }[] = [
    { key: '1min', label: '1m' },
    { key: '1h', label: '1H' },
    { key: '1d', label: '1D' },
  ];

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
        {/* Interval selector */}
        <div className="flex items-center gap-1 bg-terminal-bg rounded p-1">
          {intervals.map((int) => (
            <button
              key={int.key}
              onClick={() => setInterval(int.key)}
              className={`px-3 py-1 text-xs rounded transition-colors ${
                interval === int.key
                  ? 'bg-terminal-accent text-white'
                  : 'text-terminal-muted hover:text-terminal-text'
              }`}
            >
              {int.label}
            </button>
          ))}
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
