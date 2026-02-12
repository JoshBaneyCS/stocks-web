import { useEffect, useRef, useState } from 'react';
import type { InstrumentListItem, PriceEvent } from '../lib/types';
import { createEventSource } from '../lib/api';
import SparklineChart from './SparklineChart';
import { formatPrice, formatLargeNumber } from './utils';

interface FavoritesListProps {
  favorites: InstrumentListItem[];
}

export default function FavoritesList({ favorites }: FavoritesListProps) {
  const [livePrices, setLivePrices] = useState<Record<string, number>>({});
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (favorites.length === 0) return;

    const es = createEventSource('/api/stream/favorites');

    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data) as PriceEvent;
        setLivePrices((prev) => ({
          ...prev,
          [data.symbol]: data.last_price,
        }));
      } catch {
        // ignore parse errors
      }
    };

    es.onerror = () => {
      // SSE will auto-reconnect
    };

    eventSourceRef.current = es;

    return () => {
      es.close();
      eventSourceRef.current = null;
    };
  }, [favorites]);

  if (favorites.length === 0) {
    return (
      <div className="terminal-panel p-8 text-center">
        <div className="text-terminal-muted mb-4">
          <svg
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            className="w-12 h-12 mx-auto mb-3 opacity-50"
          >
            <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />
          </svg>
          <p className="text-sm">No favorites yet</p>
          <p className="text-xs mt-1">
            Add instruments from the{' '}
            <a href="/app/instruments" className="text-terminal-accent hover:underline">
              Instruments
            </a>{' '}
            page
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {favorites.map((instrument) => {
        const currentPrice = livePrices[instrument.symbol] ?? instrument.last_price;
        const originalPrice = instrument.last_price;
        const priceChange =
          currentPrice && originalPrice
            ? ((currentPrice - originalPrice) / originalPrice) * 100
            : 0;
        const isPositive = priceChange >= 0;

        // Generate mock sparkline data from the instrument info
        const sparkData = generateSparklineData(currentPrice ?? 0);

        return (
          <a
            key={instrument.id}
            href={`/app/instruments/${instrument.symbol}`}
            className="terminal-panel p-3 flex items-center justify-between gap-4 hover:bg-terminal-border/30 transition-colors cursor-pointer block"
          >
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="font-bold text-sm text-terminal-text">
                  {instrument.symbol}
                </span>
                <span className="text-xs text-terminal-muted truncate">
                  {instrument.name}
                </span>
              </div>
              {instrument.market_cap && (
                <span className="text-xs text-terminal-muted">
                  MCap {formatLargeNumber(instrument.market_cap)}
                </span>
              )}
            </div>

            <div className="flex-shrink-0">
              <SparklineChart data={sparkData} width={80} height={30} />
            </div>

            <div className="text-right flex-shrink-0 min-w-[90px]">
              <div className="text-sm font-medium">
                {currentPrice ? formatPrice(currentPrice) : '--'}
              </div>
              <div
                className={`text-xs ${
                  isPositive ? 'text-terminal-green' : 'text-terminal-red'
                }`}
              >
                {isPositive ? '+' : ''}
                {priceChange.toFixed(2)}%
              </div>
            </div>
          </a>
        );
      })}
    </div>
  );
}

function generateSparklineData(basePrice: number): { time: string; value: number }[] {
  const points: { time: string; value: number }[] = [];
  const now = new Date();
  let price = basePrice || 100;

  for (let i = 29; i >= 0; i--) {
    const date = new Date(now.getTime() - i * 24 * 60 * 60 * 1000);
    const dateStr = date.toISOString().split('T')[0];
    price = price * (1 + (Math.random() - 0.48) * 0.03);
    points.push({ time: dateStr, value: Number(price.toFixed(2)) });
  }

  return points;
}
