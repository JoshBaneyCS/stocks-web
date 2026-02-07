import { useState, useEffect, useRef, useCallback } from 'react';
import { createChart, type IChartApi, type ISeriesApi, type DeepPartial, type ChartOptions, ColorType } from 'lightweight-charts';
import { getStockPrices, createPriceStream } from '@/lib/api';
import { downsample } from '@/lib/wasm';
import type { PriceBar, PricePoint, SSEPriceEvent } from '@/lib/types';

type Interval = '1min' | '1d';

interface Props {
  symbol: string;
  marketOpen: boolean;
}

const CHART_COLORS = {
  bg: '#111827',
  grid: '#1e293b',
  text: '#94a3b8',
  border: '#1e293b',
  crosshair: '#475569',
  upColor: '#22c55e',
  downColor: '#ef4444',
  lineColor: '#f97316',
  areaTop: 'rgba(249, 115, 22, 0.2)',
  areaBottom: 'rgba(249, 115, 22, 0.0)',
  volumeUp: 'rgba(34, 197, 94, 0.3)',
  volumeDown: 'rgba(239, 68, 68, 0.3)',
};

const CHART_COLORS_MUTED = {
  ...CHART_COLORS,
  upColor: '#334155',
  downColor: '#334155',
  lineColor: '#475569',
  areaTop: 'rgba(71, 85, 105, 0.15)',
  areaBottom: 'rgba(71, 85, 105, 0.0)',
  volumeUp: 'rgba(71, 85, 105, 0.2)',
  volumeDown: 'rgba(71, 85, 105, 0.2)',
};

const MAX_CHART_POINTS = 800;

function barToTime(ts: string): number {
  return Math.floor(new Date(ts).getTime() / 1000);
}

export default function StockChart({ symbol, marketOpen }: Props) {
  const [interval, setInterval_] = useState<Interval>('1d');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [barCount, setBarCount] = useState(0);

  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const mainSeriesRef = useRef<ISeriesApi<'Candlestick'> | ISeriesApi<'Area'> | null>(null);
  const volumeSeriesRef = useRef<ISeriesApi<'Histogram'> | null>(null);
  const streamRef = useRef<EventSource | null>(null);
  const barsRef = useRef<PriceBar[]>([]);

  // Chart options
  const getChartOptions = useCallback(
    (isMuted: boolean): DeepPartial<ChartOptions> => {
      const colors = isMuted ? CHART_COLORS_MUTED : CHART_COLORS;
      return {
        layout: {
          background: { type: ColorType.Solid, color: colors.bg },
          textColor: colors.text,
          fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
          fontSize: 11,
        },
        grid: {
          vertLines: { color: colors.grid },
          horzLines: { color: colors.grid },
        },
        crosshair: {
          vertLine: { color: colors.crosshair, width: 1, style: 3, labelBackgroundColor: '#334155' },
          horzLine: { color: colors.crosshair, width: 1, style: 3, labelBackgroundColor: '#334155' },
        },
        rightPriceScale: {
          borderColor: colors.border,
          scaleMargins: { top: 0.1, bottom: 0.25 },
        },
        timeScale: {
          borderColor: colors.border,
          timeVisible: interval === '1min',
          secondsVisible: false,
        },
        handleScroll: { vertTouchDrag: false },
      };
    },
    [interval],
  );

  // Create chart
  useEffect(() => {
    if (!containerRef.current) return;

    const chart = createChart(containerRef.current, {
      ...getChartOptions(!marketOpen),
      width: containerRef.current.clientWidth,
      height: 400,
      autoSize: true,
    });

    chartRef.current = chart;

    // Resize observer
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width } = entry.contentRect;
        chart.applyOptions({ width });
      }
    });
    ro.observe(containerRef.current);

    return () => {
      ro.disconnect();
      chart.remove();
      chartRef.current = null;
      mainSeriesRef.current = null;
      volumeSeriesRef.current = null;
    };
  }, []);

  // Update chart colors when market status changes
  useEffect(() => {
    if (!chartRef.current) return;
    chartRef.current.applyOptions(getChartOptions(!marketOpen));
  }, [marketOpen, getChartOptions]);

  // Load data and set series
  useEffect(() => {
    if (!chartRef.current) return;

    const chart = chartRef.current;

    // Remove existing series
    if (mainSeriesRef.current) {
      chart.removeSeries(mainSeriesRef.current);
      mainSeriesRef.current = null;
    }
    if (volumeSeriesRef.current) {
      chart.removeSeries(volumeSeriesRef.current);
      volumeSeriesRef.current = null;
    }

    setLoading(true);
    setError('');

    const isMuted = !marketOpen;
    const colors = isMuted ? CHART_COLORS_MUTED : CHART_COLORS;

    async function loadData() {
      try {
        const bars = await getStockPrices(symbol, interval);
        barsRef.current = bars;
        setBarCount(bars.length);

        if (!bars.length) {
          setError('No price data available');
          setLoading(false);
          return;
        }

        if (interval === '1d') {
          // Candlestick for daily
          const candleSeries = chart.addCandlestickSeries({
            upColor: colors.upColor,
            downColor: colors.downColor,
            borderUpColor: colors.upColor,
            borderDownColor: colors.downColor,
            wickUpColor: colors.upColor,
            wickDownColor: colors.downColor,
          });

          const candleData = bars.map((b) => ({
            time: barToTime(b.ts) as any,
            open: b.open,
            high: b.high,
            low: b.low,
            close: b.close,
          }));

          candleSeries.setData(candleData);
          mainSeriesRef.current = candleSeries as any;

          // Volume histogram
          const volSeries = chart.addHistogramSeries({
            priceFormat: { type: 'volume' },
            priceScaleId: 'volume',
          });

          chart.priceScale('volume').applyOptions({
            scaleMargins: { top: 0.8, bottom: 0 },
          });

          const volData = bars.map((b) => ({
            time: barToTime(b.ts) as any,
            value: b.volume,
            color: b.close >= b.open ? colors.volumeUp : colors.volumeDown,
          }));

          volSeries.setData(volData);
          volumeSeriesRef.current = volSeries;
        } else {
          // Area chart for intraday — downsample if needed
          let points: PricePoint[] = bars.map((b) => ({
            time: barToTime(b.ts),
            value: b.close,
          }));

          if (points.length > MAX_CHART_POINTS) {
            points = await downsample(points, MAX_CHART_POINTS);
          }

          const areaSeries = chart.addAreaSeries({
            lineColor: colors.lineColor,
            topColor: colors.areaTop,
            bottomColor: colors.areaBottom,
            lineWidth: 2,
          });

          const areaData = points.map((p) => ({
            time: p.time as any,
            value: p.value,
          }));

          areaSeries.setData(areaData);
          mainSeriesRef.current = areaSeries as any;

          // Volume histogram for intraday too
          const volSeries = chart.addHistogramSeries({
            priceFormat: { type: 'volume' },
            priceScaleId: 'volume',
          });

          chart.priceScale('volume').applyOptions({
            scaleMargins: { top: 0.8, bottom: 0 },
          });

          const volData = bars.map((b) => ({
            time: barToTime(b.ts) as any,
            value: b.volume,
            color: colors.volumeUp,
          }));

          volSeries.setData(volData);
          volumeSeriesRef.current = volSeries;
        }

        chart.timeScale().fitContent();
        setLoading(false);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load chart data');
        setLoading(false);
      }
    }

    loadData();
  }, [symbol, interval, marketOpen]);

  // SSE streaming for live updates (intraday only, market open only)
  useEffect(() => {
    if (!marketOpen || interval !== '1min') {
      if (streamRef.current) {
        streamRef.current.close();
        streamRef.current = null;
      }
      return;
    }

    const es = createPriceStream(
      symbol,
      (event) => {
        const data = event as SSEPriceEvent;
        if (data.type !== 'price') return;

        const time = barToTime(data.ts) as any;

        // Update area series
        if (mainSeriesRef.current) {
          try {
            (mainSeriesRef.current as ISeriesApi<'Area'>).update({
              time,
              value: data.close,
            });
          } catch { /* series might not support update */ }
        }

        // Update volume
        if (volumeSeriesRef.current) {
          try {
            volumeSeriesRef.current.update({
              time,
              value: data.volume,
              color: CHART_COLORS.volumeUp,
            } as any);
          } catch { /* ignore */ }
        }
      },
      () => {
        // On error, try reconnecting after 5s
        setTimeout(() => {
          if (streamRef.current) {
            streamRef.current.close();
            streamRef.current = null;
          }
        }, 5000);
      },
    );

    streamRef.current = es;

    return () => {
      es.close();
      streamRef.current = null;
    };
  }, [symbol, interval, marketOpen]);

  return (
    <div className="panel">
      <div className="panel-header">
        <div className="flex items-center gap-3">
          <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
            Price Chart
          </span>
          {barCount > 0 && (
            <span className="text-2xs text-terminal-muted font-mono">
              {barCount.toLocaleString()} bars
            </span>
          )}
          {marketOpen && interval === '1min' && (
            <span className="flex items-center gap-1 text-2xs text-terminal-green">
              <span className="live-dot" />
              LIVE
            </span>
          )}
        </div>

        {/* Interval tabs */}
        <div className="flex items-center gap-1">
          <button
            onClick={() => setInterval_('1min')}
            className={`px-3 py-1 rounded text-xs font-mono transition-colors ${
              interval === '1min'
                ? 'bg-terminal-accent/10 text-terminal-accent'
                : 'text-terminal-muted hover:text-terminal-text'
            }`}
          >
            1min
          </button>
          <button
            onClick={() => setInterval_('1d')}
            className={`px-3 py-1 rounded text-xs font-mono transition-colors ${
              interval === '1d'
                ? 'bg-terminal-accent/10 text-terminal-accent'
                : 'text-terminal-muted hover:text-terminal-text'
            }`}
          >
            1D
          </button>
        </div>
      </div>

      {/* Chart container with market closed overlay */}
      <div className="relative">
        {/* Loading overlay */}
        {loading && (
          <div className="absolute inset-0 flex items-center justify-center bg-terminal-surface/80 z-10">
            <div className="flex items-center gap-2 text-terminal-dim">
              <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
              <span className="text-sm font-mono">Loading chart...</span>
            </div>
          </div>
        )}

        {/* Error overlay */}
        {error && !loading && (
          <div className="absolute inset-0 flex items-center justify-center bg-terminal-surface/80 z-10">
            <div className="text-center">
              <p className="text-sm text-terminal-red mb-2">{error}</p>
              <button
                onClick={() => {
                  setError('');
                  setLoading(true);
                  // Trigger re-fetch by toggling interval
                  const current = interval;
                  setInterval_('1d');
                  setTimeout(() => setInterval_(current), 0);
                }}
                className="btn-ghost text-xs"
              >
                Retry
              </button>
            </div>
          </div>
        )}

        {/* Market closed overlay */}
        {!marketOpen && !loading && !error && (
          <div className="absolute top-3 right-3 z-10 flex items-center gap-2 px-3 py-1.5 rounded bg-terminal-bg/80 border border-terminal-border">
            <span className="inline-block w-2 h-2 rounded-full bg-terminal-muted" />
            <span className="text-xs font-mono text-terminal-dim">Market Closed</span>
          </div>
        )}

        {/* Chart */}
        <div
          ref={containerRef}
          className={`w-full ${!marketOpen && !loading ? 'opacity-70 saturate-[0.3]' : ''}`}
          style={{ minHeight: 400 }}
        />
      </div>
    </div>
  );
}