import { useState, useEffect } from 'react';
import { getMarketStatus, getFavorites, getStockNews } from '@/lib/api';
import type { MarketStatus as MarketStatusType, Favorite, NewsArticle } from '@/lib/types';
import MarketStatus from './MarketStatus';
import FavoritesList from './FavoritesList';
import NewsHeadlines from './NewsHeadlines';

export default function Dashboard() {
  const [market, setMarket] = useState<MarketStatusType | null>(null);
  const [news, setNews] = useState<NewsArticle[]>([]);
  const [loadingNews, setLoadingNews] = useState(true);

  useEffect(() => {
    let mounted = true;

    // Fetch market status
    getMarketStatus()
      .then((data) => {
        if (mounted) setMarket(data);
      })
      .catch(() => {});

    // Fetch favorites then get news for those symbols
    async function loadNews() {
      try {
        const favs = await getFavorites();
        if (!mounted || favs.length === 0) {
          setLoadingNews(false);
          return;
        }

        // Fetch news for each favorite (first 5 articles each)
        const newsPromises = favs.slice(0, 10).map((f: Favorite) =>
          getStockNews(f.symbol, undefined, undefined, 5).catch(() => [] as NewsArticle[]),
        );

        const results = await Promise.all(newsPromises);
        if (!mounted) return;

        // Flatten, deduplicate by id, sort by published_at desc
        const allNews = results
          .flat()
          .reduce((acc: NewsArticle[], article) => {
            if (!acc.find((a) => a.id === article.id)) {
              acc.push(article);
            }
            return acc;
          }, [])
          .sort(
            (a, b) =>
              new Date(b.published_at).getTime() - new Date(a.published_at).getTime(),
          )
          .slice(0, 20);

        setNews(allNews);
      } catch {
        // Silently handle — news is non-critical
      } finally {
        if (mounted) setLoadingNews(false);
      }
    }

    loadNews();

    // Poll market status
    const interval = setInterval(() => {
      getMarketStatus()
        .then((data) => {
          if (mounted) setMarket(data);
        })
        .catch(() => {});
    }, 30_000);

    return () => {
      mounted = false;
      clearInterval(interval);
    };
  }, []);

  const marketOpen = market?.is_open ?? false;

  return (
    <div className="max-w-screen-xl mx-auto px-4 py-6">
      {/* Top row: Market Status */}
      <div className="mb-6">
        <MarketStatus />
      </div>

      {/* Main grid: Favorites + News */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Favorites — takes 2 columns on large screens */}
        <div className="lg:col-span-2">
          <FavoritesList marketOpen={marketOpen} />
        </div>

        {/* News sidebar */}
        <div className="lg:col-span-1">
          <div className="panel">
            <div className="panel-header">
              <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
                News for Favorites
              </span>
              {news.length > 0 && (
                <span className="text-2xs text-terminal-muted font-mono">
                  {news.length} articles
                </span>
              )}
            </div>
            <div className="panel-body">
              {loadingNews ? (
                <div className="space-y-3">
                  {Array.from({ length: 5 }).map((_, i) => (
                    <div key={i} className="animate-pulse space-y-2">
                      <div className="h-3.5 bg-terminal-border rounded w-full" />
                      <div className="h-3 bg-terminal-border rounded w-2/3" />
                    </div>
                  ))}
                </div>
              ) : news.length === 0 ? (
                <div className="text-center py-6">
                  <p className="text-sm text-terminal-muted mb-1">No recent news</p>
                  <p className="text-2xs text-terminal-muted">
                    Add favorites to see their latest headlines here
                  </p>
                </div>
              ) : (
                <NewsHeadlines
                  articles={news}
                  maxItems={10}
                  showSymbols
                  compact
                />
              )}
            </div>
          </div>

          {/* Quick links */}
          <div className="panel mt-4">
            <div className="panel-header">
              <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
                Quick Links
              </span>
            </div>
            <div className="panel-body space-y-1">
              <a
                href="/app/stocks"
                className="flex items-center gap-2 px-2 py-2 rounded text-sm text-terminal-dim hover:text-terminal-text hover:bg-terminal-border/20 transition-colors"
              >
                <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
                  <path d="M3 12v3c0 1.657 3.134 3 7 3s7-1.343 7-3v-3c0 1.657-3.134 3-7 3s-7-1.343-7-3z" />
                  <path d="M3 7v3c0 1.657 3.134 3 7 3s7-1.343 7-3V7c0 1.657-3.134 3-7 3S3 8.657 3 7z" />
                  <path d="M17 5c0 1.657-3.134 3-7 3S3 6.657 3 5s3.134-3 7-3 7 1.343 7 3z" />
                </svg>
                Browse All Stocks
              </a>
              <a
                href="/app/settings"
                className="flex items-center gap-2 px-2 py-2 rounded text-sm text-terminal-dim hover:text-terminal-text hover:bg-terminal-border/20 transition-colors"
              >
                <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
                  <path fillRule="evenodd" d="M3.172 5.172a4 4 0 015.656 0L10 6.343l1.172-1.171a4 4 0 115.656 5.656L10 17.657l-6.828-6.829a4 4 0 010-5.656z" clipRule="evenodd" />
                </svg>
                Manage Favorites
              </a>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}