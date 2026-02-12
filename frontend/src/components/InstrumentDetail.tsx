import { useEffect, useState } from 'react';
import { getInstrumentDetail } from '../lib/api';
import type { InstrumentDetail as InstrumentDetailType } from '../lib/types';
import { useMarketStore, useFavoritesStore } from '../lib/store';
import { formatPrice, formatLargeNumber } from './utils';
import InstrumentChart from './InstrumentChart';
import FundamentalsTable from './FundamentalsTable';

interface InstrumentDetailProps {
  symbol: string;
}

type Tab = 'overview' | 'chart' | 'fundamentals';

export default function InstrumentDetail({ symbol }: InstrumentDetailProps) {
  const [instrument, setInstrument] = useState<InstrumentDetailType | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const { isOpen } = useMarketStore();
  const { pendingIds, toggleFavorite, saveFavorites, hasChanges, loadFavorites } =
    useFavoritesStore();

  useEffect(() => {
    loadFavorites();
  }, [loadFavorites]);

  useEffect(() => {
    loadInstrument();
  }, [symbol]);

  const loadInstrument = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await getInstrumentDetail(symbol);
      setInstrument(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load instrument');
    } finally {
      setIsLoading(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="flex flex-col items-center gap-3">
          <div className="w-6 h-6 border-2 border-terminal-accent border-t-transparent rounded-full animate-spin" />
          <span className="text-terminal-muted text-sm">Loading {symbol}...</span>
        </div>
      </div>
    );
  }

  if (error || !instrument) {
    return (
      <div className="terminal-panel p-6 text-center">
        <p className="text-terminal-red text-sm mb-3">{error || 'Instrument not found'}</p>
        <button onClick={loadInstrument} className="btn-primary text-sm">
          Retry
        </button>
      </div>
    );
  }

  const isEquity = instrument.asset_class === 'equity';
  const marketClosed = isEquity && !isOpen;
  const isFav = pendingIds.has(instrument.id);

  const tabs: { key: Tab; label: string }[] = [
    { key: 'overview', label: 'Overview' },
    { key: 'chart', label: 'Chart' },
    { key: 'fundamentals', label: 'Fundamentals' },
  ];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <div className="flex items-center gap-3 mb-1">
            <h1 className="text-2xl font-bold">{instrument.symbol}</h1>
            <button
              onClick={() => {
                toggleFavorite(instrument.id);
              }}
              title={isFav ? 'Remove from favorites' : 'Add to favorites'}
            >
              <svg
                viewBox="0 0 24 24"
                fill={isFav ? 'currentColor' : 'none'}
                stroke="currentColor"
                strokeWidth="2"
                className={`w-5 h-5 transition-colors ${
                  isFav ? 'text-terminal-yellow' : 'text-terminal-muted hover:text-terminal-yellow'
                }`}
              >
                <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />
              </svg>
            </button>
            {hasChanges && (
              <button onClick={saveFavorites} className="btn-primary text-xs px-2 py-1">
                Save
              </button>
            )}
            {marketClosed && (
              <span className="text-xs bg-terminal-muted/20 text-terminal-muted px-2 py-0.5 rounded">
                Market Closed
              </span>
            )}
          </div>
          <p className="text-terminal-muted text-sm">{instrument.name}</p>
        </div>

        <div className="text-right">
          <div className="text-2xl font-bold">
            {formatPrice(instrument.last_price)}
          </div>
          {instrument.latest_quote && (
            <div className="text-xs text-terminal-muted mt-1">
              Bid: {formatPrice(instrument.latest_quote.bid)} / Ask:{' '}
              {formatPrice(instrument.latest_quote.ask)}
            </div>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-terminal-border">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`px-4 py-2 text-sm border-b-2 transition-colors ${
              activeTab === tab.key
                ? 'border-terminal-accent text-terminal-accent'
                : 'border-transparent text-terminal-muted hover:text-terminal-text'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div className={marketClosed ? 'relative' : ''}>
        {activeTab === 'overview' && (
          <OverviewTab instrument={instrument} />
        )}

        {activeTab === 'chart' && (
          <div className={marketClosed ? 'market-closed-overlay' : ''}>
            <InstrumentChart symbol={symbol} marketClosed={marketClosed} />
          </div>
        )}

        {activeTab === 'fundamentals' && (
          <FundamentalsTable symbol={symbol} />
        )}
      </div>
    </div>
  );
}

function OverviewTab({ instrument }: { instrument: InstrumentDetailType }) {
  const profile = instrument.profile;

  const metrics = [
    { label: 'Last Price', value: formatPrice(instrument.last_price) },
    { label: 'Market Cap', value: formatLargeNumber(instrument.market_cap) },
    { label: 'Exchange', value: instrument.exchange ?? '--' },
    { label: 'Asset Class', value: instrument.asset_class },
    { label: 'Currency', value: instrument.currency ?? '--' },
    { label: 'Country', value: instrument.country ?? '--' },
    { label: 'Sector', value: profile?.sector ?? '--' },
    { label: 'Industry', value: profile?.industry ?? '--' },
  ];

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      {metrics.map((metric) => (
        <div key={metric.label} className="terminal-panel p-4">
          <div className="text-xs text-terminal-muted uppercase tracking-wider mb-1">
            {metric.label}
          </div>
          <div className="text-lg font-bold">{metric.value}</div>
        </div>
      ))}

      {instrument.latest_quote && (
        <>
          <div className="terminal-panel p-4">
            <div className="text-xs text-terminal-muted uppercase tracking-wider mb-1">
              Bid
            </div>
            <div className="text-lg font-bold">
              {formatPrice(instrument.latest_quote.bid)}
            </div>
          </div>
          <div className="terminal-panel p-4">
            <div className="text-xs text-terminal-muted uppercase tracking-wider mb-1">
              Ask
            </div>
            <div className="text-lg font-bold">
              {formatPrice(instrument.latest_quote.ask)}
            </div>
          </div>
          <div className="terminal-panel p-4">
            <div className="text-xs text-terminal-muted uppercase tracking-wider mb-1">
              Volume
            </div>
            <div className="text-lg font-bold">
              {instrument.latest_quote.volume?.toLocaleString() ?? '--'}
            </div>
          </div>
          <div className="terminal-panel p-4">
            <div className="text-xs text-terminal-muted uppercase tracking-wider mb-1">
              Source
            </div>
            <div className="text-lg font-bold">
              {instrument.latest_quote.source ?? '--'}
            </div>
          </div>
        </>
      )}
    </div>
  );
}
