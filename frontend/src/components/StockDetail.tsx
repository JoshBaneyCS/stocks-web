import { useState, useEffect } from 'react';
import { getStock, getStockNews, getMarketStatus } from '@/lib/api';
import type { Company, NewsArticle, MarketStatus } from '@/lib/types';
import StockChart from './StockChart';
import NewsHeadlines from './NewsHeadlines';

interface Props {
  symbol: string;
}

type Tab = 'chart' | 'news';

function formatNumber(n: number | null | undefined): string {
  if (n == null || n === 0) return '—';
  return n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function formatMarketCap(cap: number | null | undefined): string {
  if (!cap) return '—';
  if (cap >= 1e12) return `$${(cap / 1e12).toFixed(2)}T`;
  if (cap >= 1e9) return `$${(cap / 1e9).toFixed(2)}B`;
  if (cap >= 1e6) return `$${(cap / 1e6).toFixed(1)}M`;
  return `$${cap.toLocaleString()}`;
}

function formatVolume(vol: number | null | undefined): string {
  if (!vol) return '—';
  if (vol >= 1e9) return `${(vol / 1e9).toFixed(2)}B`;
  if (vol >= 1e6) return `${(vol / 1e6).toFixed(2)}M`;
  if (vol >= 1e3) return `${(vol / 1e3).toFixed(1)}K`;
  return vol.toLocaleString();
}

function MetricCard({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="px-3 py-2.5">
      <p className="text-2xs text-terminal-muted uppercase tracking-wider mb-0.5">{label}</p>
      <p className="text-sm font-mono font-semibold font-tabular text-terminal-text">{value}</p>
      {sub && <p className="text-2xs text-terminal-dim font-mono mt-0.5">{sub}</p>}
    </div>
  );
}

declare global {
  interface Window {
    __STOCK_SYMBOL__?: string;
  }
}

export default function StockDetail({ symbol: propSymbol }: Props) {
  // In static Astro build, symbol comes from window global set by inline script
  const symbol = propSymbol || (typeof window !== 'undefined' ? window.__STOCK_SYMBOL__ : '') || '';

  const [company, setCompany] = useState<Company | null>(null);
  const [news, setNews] = useState<NewsArticle[]>([]);
  const [market, setMarket] = useState<MarketStatus | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>('chart');
  const [loadingCompany, setLoadingCompany] = useState(true);
  const [loadingNews, setLoadingNews] = useState(true);
  const [error, setError] = useState('');

  // News date filters
  const [newsFrom, setNewsFrom] = useState('');
  const [newsTo, setNewsTo] = useState('');

  useEffect(() => {
    let mounted = true;

    async function loadAll() {
      try {
        const [companyData, marketData] = await Promise.all([
          getStock(symbol),
          getMarketStatus(),
        ]);
        if (!mounted) return;
        setCompany(companyData);
        setMarket(marketData);
        setLoadingCompany(false);
      } catch (err) {
        if (mounted) {
          setError(err instanceof Error ? err.message : 'Failed to load stock data');
          setLoadingCompany(false);
        }
      }

      try {
        const newsData = await getStockNews(symbol, undefined, undefined, 50);
        if (mounted) {
          setNews(newsData);
          setLoadingNews(false);
        }
      } catch {
        if (mounted) setLoadingNews(false);
      }
    }

    loadAll();
    return () => { mounted = false; };
  }, [symbol]);

  // Reload news when date filters change
  useEffect(() => {
    if (!newsFrom && !newsTo) return;

    setLoadingNews(true);
    getStockNews(symbol, newsFrom || undefined, newsTo || undefined, 50)
      .then(setNews)
      .catch(() => {})
      .finally(() => setLoadingNews(false));
  }, [symbol, newsFrom, newsTo]);

  const marketOpen = market?.is_open ?? false;

  if (!symbol) {
    return (
      <div className="max-w-screen-xl mx-auto px-4 py-8">
        <div className="panel">
          <div className="panel-body text-center py-12">
            <p className="text-sm text-terminal-muted mb-4">Loading stock...</p>
          </div>
        </div>
      </div>
    );
  }

  // Compute price change from company data
  const priceChange = company?.prev_close && company?.todays_high
    ? ((company.todays_high + company.todays_low) / 2 - company.prev_close)
    : 0;
  const priceChangePct = company?.prev_close
    ? (priceChange / company.prev_close) * 100
    : 0;

  if (error && !company) {
    return (
      <div className="max-w-screen-xl mx-auto px-4 py-8">
        <div className="panel">
          <div className="panel-body text-center py-12">
            <p className="text-terminal-red text-sm mb-4">{error}</p>
            <a href="/app/stocks" className="btn-ghost text-xs">
              ← Back to Stocks
            </a>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-screen-xl mx-auto px-4 py-6 space-y-4">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
        <div className="flex items-center gap-3">
          <a
            href="/app/stocks"
            className="text-terminal-muted hover:text-terminal-text transition-colors"
            title="Back to stocks"
          >
            <svg className="w-5 h-5" viewBox="0 0 20 20" fill="currentColor">
              <path fillRule="evenodd" d="M9.707 16.707a1 1 0 01-1.414 0l-6-6a1 1 0 010-1.414l6-6a1 1 0 011.414 1.414L5.414 9H17a1 1 0 110 2H5.414l4.293 4.293a1 1 0 010 1.414z" clipRule="evenodd" />
            </svg>
          </a>

          {loadingCompany ? (
            <div className="animate-pulse flex items-center gap-3">
              <div className="h-7 bg-terminal-border rounded w-20" />
              <div className="h-5 bg-terminal-border rounded w-48" />
            </div>
          ) : (
            <>
              <span className="ticker-badge text-lg px-3 py-1">{symbol}</span>
              <div>
                <h1 className="text-lg font-semibold text-terminal-text">
                  {company?.name}
                </h1>
                <div className="flex items-center gap-2 mt-0.5">
                  {company?.exchange && (
                    <span className="text-2xs px-1.5 py-0.5 rounded bg-terminal-border/50 text-terminal-dim">
                      {company.exchange}
                    </span>
                  )}
                  {company?.sector && (
                    <span className="text-2xs text-terminal-muted">
                      {company.sector}
                    </span>
                  )}
                  {company?.industry && (
                    <span className="text-2xs text-terminal-muted">
                      · {company.industry}
                    </span>
                  )}
                </div>
              </div>
            </>
          )}
        </div>

        {/* Price display */}
        {company && (
          <div className="flex items-baseline gap-3">
            <span className="text-2xl font-mono font-bold font-tabular text-terminal-text">
              ${formatNumber(company.prev_close)}
            </span>
            <span
              className={`text-sm font-mono font-tabular ${
                priceChange > 0
                  ? 'text-terminal-green'
                  : priceChange < 0
                    ? 'text-terminal-red'
                    : 'text-terminal-dim'
              }`}
            >
              {priceChange >= 0 ? '+' : ''}{priceChange.toFixed(2)} ({priceChangePct >= 0 ? '+' : ''}{priceChangePct.toFixed(2)}%)
            </span>
            {!marketOpen && (
              <span className="text-2xs text-terminal-muted font-mono">
                at close
              </span>
            )}
          </div>
        )}
      </div>

      {/* Key Metrics Grid */}
      {company && (
        <div className="panel">
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 divide-x divide-y divide-terminal-border/50">
            <MetricCard label="Market Cap" value={formatMarketCap(company.market_cap)} />
            <MetricCard label="Volume" value={formatVolume(company.volume)} />
            <MetricCard label="Prev Close" value={`$${formatNumber(company.prev_close)}`} />
            <MetricCard
              label="Day Range"
              value={company.todays_low && company.todays_high
                ? `$${formatNumber(company.todays_low)} – $${formatNumber(company.todays_high)}`
                : '—'}
            />
            <MetricCard
              label="52W Range"
              value={company.week52_low && company.week52_high
                ? `$${formatNumber(company.week52_low)} – $${formatNumber(company.week52_high)}`
                : '—'}
            />
            <MetricCard
              label="52W High"
              value={company.week52_high ? `$${formatNumber(company.week52_high)}` : '—'}
              sub={company.prev_close && company.week52_high
                ? `${(((company.prev_close - company.week52_high) / company.week52_high) * 100).toFixed(1)}% from high`
                : undefined}
            />
          </div>
        </div>
      )}

      {/* Tabs */}
      <div className="border-b border-terminal-border flex items-center gap-0">
        <button
          onClick={() => setActiveTab('chart')}
          className={activeTab === 'chart' ? 'tab-active' : 'tab-inactive'}
        >
          Chart
        </button>
        <button
          onClick={() => setActiveTab('news')}
          className={activeTab === 'news' ? 'tab-active' : 'tab-inactive'}
        >
          News
          {news.length > 0 && (
            <span className="ml-1.5 text-2xs px-1.5 py-0.5 rounded-full bg-terminal-border text-terminal-dim">
              {news.length}
            </span>
          )}
        </button>
      </div>

      {/* Tab Content */}
      {activeTab === 'chart' && (
        <StockChart symbol={symbol} marketOpen={marketOpen} />
      )}

      {activeTab === 'news' && (
        <div className="space-y-4">
          {/* Date filters */}
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2">
              <label className="text-2xs text-terminal-muted uppercase tracking-wider">
                From
              </label>
              <input
                type="date"
                value={newsFrom}
                onChange={(e) => setNewsFrom(e.target.value)}
                className="input-field text-xs w-36 font-mono"
              />
            </div>
            <div className="flex items-center gap-2">
              <label className="text-2xs text-terminal-muted uppercase tracking-wider">
                To
              </label>
              <input
                type="date"
                value={newsTo}
                onChange={(e) => setNewsTo(e.target.value)}
                className="input-field text-xs w-36 font-mono"
              />
            </div>
            {(newsFrom || newsTo) && (
              <button
                onClick={() => {
                  setNewsFrom('');
                  setNewsTo('');
                }}
                className="text-2xs text-terminal-accent hover:underline"
              >
                Clear
              </button>
            )}
          </div>

          <NewsHeadlines
            articles={news}
            loading={loadingNews}
            maxItems={20}
            showSymbols={false}
            compact
          />
        </div>
      )}
    </div>
  );
}