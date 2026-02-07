import { useRef, useEffect } from 'react';
import type { PricePoint } from '@/lib/types';

interface Props {
  data: PricePoint[];
  width?: number;
  height?: number;
  positive?: boolean;
  muted?: boolean;
}

/**
 * Lightweight canvas-based sparkline chart.
 * Green if price is up (or positive=true), red if down, gray if muted.
 */
export default function SparklineChart({
  data,
  width = 120,
  height = 32,
  positive,
  muted = false,
}: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || data.length < 2) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    // Handle high-DPI displays
    const dpr = window.devicePixelRatio || 1;
    canvas.width = width * dpr;
    canvas.height = height * dpr;
    ctx.scale(dpr, dpr);

    // Clear
    ctx.clearRect(0, 0, width, height);

    // Determine direction
    const first = data[0].value;
    const last = data[data.length - 1].value;
    const isPositive = positive ?? last >= first;

    // Colors
    const lineColor = muted
      ? '#334155' // terminal-muted
      : isPositive
        ? '#22c55e' // terminal-green
        : '#ef4444'; // terminal-red

    const fillColor = muted
      ? 'rgba(51, 65, 85, 0.1)'
      : isPositive
        ? 'rgba(34, 197, 94, 0.08)'
        : 'rgba(239, 68, 68, 0.08)';

    // Compute bounds
    const values = data.map((d) => d.value);
    const minVal = Math.min(...values);
    const maxVal = Math.max(...values);
    const range = maxVal - minVal || 1;

    const padding = 2;
    const chartWidth = width - padding * 2;
    const chartHeight = height - padding * 2;

    // Map data to coordinates
    const points: [number, number][] = data.map((d, i) => [
      padding + (i / (data.length - 1)) * chartWidth,
      padding + chartHeight - ((d.value - minVal) / range) * chartHeight,
    ]);

    // Draw filled area
    ctx.beginPath();
    ctx.moveTo(points[0][0], height);
    for (const [x, y] of points) {
      ctx.lineTo(x, y);
    }
    ctx.lineTo(points[points.length - 1][0], height);
    ctx.closePath();
    ctx.fillStyle = fillColor;
    ctx.fill();

    // Draw line
    ctx.beginPath();
    ctx.moveTo(points[0][0], points[0][1]);
    for (let i = 1; i < points.length; i++) {
      ctx.lineTo(points[i][0], points[i][1]);
    }
    ctx.strokeStyle = lineColor;
    ctx.lineWidth = 1.5;
    ctx.lineJoin = 'round';
    ctx.lineCap = 'round';
    ctx.stroke();

    // Draw end dot
    const [lastX, lastY] = points[points.length - 1];
    ctx.beginPath();
    ctx.arc(lastX, lastY, 2, 0, Math.PI * 2);
    ctx.fillStyle = lineColor;
    ctx.fill();
  }, [data, width, height, positive, muted]);

  if (data.length < 2) {
    return (
      <div
        className="flex items-center justify-center text-2xs text-terminal-muted"
        style={{ width, height }}
      >
        No data
      </div>
    );
  }

  return (
    <canvas
      ref={canvasRef}
      style={{ width, height }}
      className="block"
    />
  );
}