import { useEffect, useRef } from 'react';
import { createChart, type IChartApi, ColorType } from 'lightweight-charts';

interface SparklineChartProps {
  data: { time: string; value: number }[];
  width?: number;
  height?: number;
  color?: string;
}

export default function SparklineChart({
  data,
  width = 120,
  height = 40,
  color = '#3b82f6',
}: SparklineChartProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);

  useEffect(() => {
    if (!containerRef.current || data.length === 0) return;

    const chart = createChart(containerRef.current, {
      width,
      height,
      layout: {
        background: { type: ColorType.Solid, color: 'transparent' },
        textColor: 'transparent',
      },
      grid: {
        vertLines: { visible: false },
        horzLines: { visible: false },
      },
      leftPriceScale: { visible: false },
      rightPriceScale: { visible: false },
      timeScale: { visible: false },
      crosshair: {
        vertLine: { visible: false },
        horzLine: { visible: false },
      },
      handleScroll: false,
      handleScale: false,
    });

    // Determine color based on trend
    const firstVal = data[0]?.value ?? 0;
    const lastVal = data[data.length - 1]?.value ?? 0;
    const lineColor = lastVal >= firstVal ? '#22c55e' : '#ef4444';
    const topColor = lastVal >= firstVal ? 'rgba(34,197,94,0.2)' : 'rgba(239,68,68,0.2)';

    const series = chart.addAreaSeries({
      lineColor: color === '#3b82f6' ? lineColor : color,
      topColor: color === '#3b82f6' ? topColor : `${color}33`,
      bottomColor: 'transparent',
      lineWidth: 1,
      priceLineVisible: false,
      crosshairMarkerVisible: false,
      lastValueVisible: false,
    });

    series.setData(data as { time: string; value: number }[]);
    chart.timeScale().fitContent();
    chartRef.current = chart;

    return () => {
      chart.remove();
      chartRef.current = null;
    };
  }, [data, width, height, color]);

  return <div ref={containerRef} className="inline-block" />;
}
