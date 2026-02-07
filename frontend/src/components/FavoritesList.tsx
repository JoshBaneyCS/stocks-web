import { useState, useEffect, useRef } from 'react';
import { getFavorites, getStockPrices, createFavoritesStream } from '@/lib/api';
import type { Favorite, PriceBar, PricePoint, SSEPriceEvent } from '@/lib/types';
import SparklineChart from './SparklineChart';

interface FavoriteWithData extends Favorite {
  sparkline: PricePoint[];
  lastPrice: number;
  prevClose: number;
  change: number;
  changePct: number;
  loading: boolean;
}

export default function FavoritesList({ marketOpen }: { marketOpen: boolean }) {
  const [favorites, setFavorites] = useState<FavoriteWithData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const streamRef = useRef<EventSource | null>(null);

  useEffect(() => {
    let mounted = true;

    async function load() {
      try {
        const favs = await getFavorites();
        if (!mounted) return;

        if (!favs.length) {
          setFavorites([]);
          setLoading(false);
          return;
        }

        // Initialize with loading state
        const initial: FavoriteWithData[] = favs.map((f) => ({
          ...f,
          sparkline: [],
          lastPrice: 0,
          prevClose: 0,
          change: 0,
          changePct: 0,
          loading: true,
        }));
        setFavorites(initial);
        setLoading(false);

        // Fetch sparkline data for each favorite
        const promises = favs.map(async (fav) => {
          try {
            const bars = await getStockPrices(fav.symbol, '1d', undefined, undefined, 30);
            const sparkline: PricePoint[] = bars.map((b: PriceBar) => ({
              time: new Date(b.ts).getTime() / 1000,
              value: b.close,
            }));

            const lastBar = bars[bars.length - 1];
            const prevBar = bars.length > 1 ? bars[bars.length - 2] : null;
            const lastPrice = lastBar?.close ?? 0;
            const prevClose = prevBar?.close ?? lastPrice;
            const change = lastPrice - prevClose;
            const changePct = prevClose ? (change / prevClose) * 100 : 0;

            return {
              symbol: fav.symbol,
              sparkline,
              lastPrice,
              prevClose,
              change,
              changePct,
            };
          } catch {
            return { symbol: fav.symbol, sparkline: [], lastPrice: 0, prevClose: 0, change: 0, changePct: 0 };
          }
        });

        const results = await Promise.all(promises);
        if (!mounted) return;

        setFavorites((prev) =>
          prev.map((f) => {
            const result = results.find((r) => r.symbol === f.symbol);
            if (!result) return { ...f, loading: false };
            return {
              ...f,
              ...result,
              loading: false,
            };
          }),
        );
      } catch (err) {
        if (mounted) {
          setError(err instanceof Error ? err.message : 'Failed to load favorites');
          setLoading(false);
        }
      }
    }

    load();

    return () => {
      mounted = false;
    };
  }, []);

  // SSE stream for live updates
  useEffect(() => {
    if (!marketOpen || favorites.length === 0 || favorites.every((f) => f.loading)) return;

    // Clean up previous stream
    if (streamRef.current) {
      streamRef.current.close();
    }

    const es = createFavoritesStream((event) => {
      const data = event as SSEPriceEvent;
      if (data.type !== 'price') return;

      setFavorites((prev) =>
        prev.map((f) => {
          if (f.symbol !== data.symbol) return f;

          const newPoint: PricePoint = {
            time: new Date(data.ts).getTime() / 1000,
            value: data.close,
          };

          const change = data.close - f.prevClose;
          const changePct = f.prevClose ? (change / f.prevClose) * 100 : 0;

          return {
            ...f,
            lastPrice: data.close,
            change,
            changePct,
            sparkline: [...f.sparkline.slice(-29), newPoint],
          };
        }),
      );
    });

    streamRef.current = es;

    return () => {
      es.close();
      streamRef.current = null;
    };
  }, [marketOpen, favorites.length]);

  if (loading) {
    return (
      <div className="panel">
        <div className="panel-header">
          <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
            Favorites
          </span>
        </div>
        <div className="panel-body space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="animate-pulse flex items-center gap-4">
              <div className="h-4 bg-terminal-border rounded w-16" />
              <div className="flex-1 h-8 bg-terminal-border rounded" />
              <div className="h-4 bg-terminal-border rounded w-20" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="panel">
        <div className="panel-header">
          <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
            Favorites
          </span>
        </div>
        <div className="panel-body">
          <p className="text-sm text-terminal-red">{error}</p>
        </div>
      </div>
    );
  }

  if (!favorites.length) {
    return (
      <div className="panel">
        <div className="panel-header">
          <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
            Favorites
          </span>
        </div>
        <div className="panel-body text-center py-6">
          <p className="text-sm text-terminal-muted mb-2">No favorites yet</p>
          <a
            href="/app/stocks"
            className="text-xs text-terminal-accent hover:underline"
          >
            Browse stocks to add favorites →
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="panel">
      <div className="panel-header">
        <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
          Favorites
        </span>
        <div className="flex items-center gap-2">
          {marketOpen && (
            <span className="flex items-center gap-1 text-2xs text-terminal-green">
              <span className="live-dot" />
              LIVE
            </span>
          )}
          <a
            href="/app/settings"
            className="text-2xs text-terminal-muted hover:text-terminal-accent transition-colors"
          >
            Edit
          </a>
        </div>
      </div>
      <div className="divide-y divide-terminal-border/50">
        {favorites.map((fav) => (
          <a
            key={fav.symbol}
            href={`/app/stocks/${fav.symbol}`}
            className="flex items-center gap-4 px-4 py-3 hover:bg-terminal-border/20 transition-colors group"
          >
            {/* Symbol + Name */}
            <div className="w-28 min-w-0">
              <p className="text-sm font-mono font-semibold text-terminal-text group-hover:text-terminal-accent transition-colors">
                {fav.symbol}
              </p>
              <p className="text-2xs text-terminal-muted truncate">
                {fav.name}
              </p>
            </div>

            {/* Sparkline */}
            <div className="flex-1 flex justify-center">
              {fav.loading ? (
                <div className="h-8 w-full max-w-[120px] bg-terminal-border/30 rounded animate-pulse" />
              ) : (
                <SparklineChart
                  data={fav.sparkline}
                  width={120}
                  height={32}
                  muted={!marketOpen}
                />
              )}
            </div>

            {/* Price + Change */}
            <div className="text-right w-28">
              {fav.loading ? (
                <div className="space-y-1.5">
                  <div className="h-4 bg-terminal-border rounded w-16 ml-auto" />
                  <div className="h-3 bg-terminal-border rounded w-12 ml-auto" />
                </div>
              ) : (
                <>
                  <p className="text-sm font-mono font-semibold font-tabular text-terminal-text">
                    ${fav.lastPrice.toFixed(2)}
                  </p>
                  <p
                    className={`text-2xs font-mono font-tabular ${
                      fav.change > 0
                        ? 'text-terminal-green'
                        : fav.change < 0
                          ? 'text-terminal-red'
                          : 'text-terminal-dim'
                    }`}
                  >
                    {fav.change >= 0 ? '+' : ''}
                    {fav.change.toFixed(2)} ({fav.changePct >= 0 ? '+' : ''}
                    {fav.changePct.toFixed(2)}%)
                  </p>
                </>
              )}
            </div>
          </a>
        ))}
      </div>
    </div>
  );
}