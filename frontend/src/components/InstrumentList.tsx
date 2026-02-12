import { useEffect, useState, useCallback, useMemo } from 'react';
import { FixedSizeList as VirtualList } from 'react-window';
import { getInstruments, type InstrumentSearchParams } from '../lib/api';
import { useFavoritesStore } from '../lib/store';
import type { InstrumentListItem, PaginatedResponse } from '../lib/types';
import { formatPrice, formatLargeNumber, debounce } from './utils';

const PAGE_SIZE = 50;
const ROW_HEIGHT = 48;

export default function InstrumentList() {
  const [data, setData] = useState<PaginatedResponse<InstrumentListItem> | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filters
  const [search, setSearch] = useState('');
  const [assetClass, setAssetClass] = useState('');
  const [exchange, setExchange] = useState('');
  const [country, setCountry] = useState('');
  const [page, setPage] = useState(1);

  // Favorites
  const { pendingIds, hasChanges, toggleFavorite, saveFavorites, loadFavorites, isSaving } =
    useFavoritesStore();

  useEffect(() => {
    loadFavorites();
  }, [loadFavorites]);

  const fetchData = useCallback(
    async (params: InstrumentSearchParams) => {
      setIsLoading(true);
      setError(null);
      try {
        const result = await getInstruments(params);
        setData(result);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load instruments');
      } finally {
        setIsLoading(false);
      }
    },
    []
  );

  // Debounced search
  const debouncedFetch = useMemo(
    () =>
      debounce((searchVal: string) => {
        setPage(1);
        fetchData({
          search: searchVal,
          asset_class: assetClass,
          exchange,
          country,
          page: 1,
          page_size: PAGE_SIZE,
        });
      }, 300),
    [fetchData, assetClass, exchange, country]
  );

  useEffect(() => {
    fetchData({
      search,
      asset_class: assetClass,
      exchange,
      country,
      page,
      page_size: PAGE_SIZE,
    });
  }, [fetchData, assetClass, exchange, country, page]);

  const handleSearchChange = (value: string) => {
    setSearch(value);
    debouncedFetch(value);
  };

  const handleSave = async () => {
    await saveFavorites();
  };

  const instruments = data?.data ?? [];
  const totalPages = data?.total_pages ?? 0;
  const totalCount = data?.total_count ?? 0;
  const useVirtualization = instruments.length > 50;

  const renderRow = (instrument: InstrumentListItem, index: number, style?: React.CSSProperties) => {
    const isFav = pendingIds.has(instrument.id);
    return (
      <div
        key={instrument.id}
        style={style}
        className={`flex items-center px-4 py-2 border-b border-terminal-border/50 hover:bg-terminal-border/30 transition-colors ${
          index % 2 === 0 ? 'bg-terminal-panel' : 'bg-terminal-bg'
        }`}
      >
        {/* Favorite toggle */}
        <button
          onClick={(e) => {
            e.preventDefault();
            toggleFavorite(instrument.id);
          }}
          className="mr-3 flex-shrink-0"
          title={isFav ? 'Remove from favorites' : 'Add to favorites'}
        >
          <svg
            viewBox="0 0 24 24"
            fill={isFav ? 'currentColor' : 'none'}
            stroke="currentColor"
            strokeWidth="2"
            className={`w-4 h-4 transition-colors ${
              isFav ? 'text-terminal-yellow' : 'text-terminal-muted hover:text-terminal-yellow'
            }`}
          >
            <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />
          </svg>
        </button>

        {/* Symbol */}
        <a
          href={`/app/instruments/${instrument.symbol}`}
          className="flex-shrink-0 w-24 font-bold text-sm text-terminal-accent hover:underline"
        >
          {instrument.symbol}
        </a>

        {/* Name */}
        <div className="flex-1 min-w-0 text-sm text-terminal-text truncate mr-4">
          {instrument.name ?? '--'}
        </div>

        {/* Exchange */}
        <div className="hidden md:block flex-shrink-0 w-20 text-xs text-terminal-muted">
          {instrument.exchange ?? '--'}
        </div>

        {/* Price */}
        <div className="flex-shrink-0 w-24 text-sm text-right font-medium">
          {formatPrice(instrument.last_price)}
        </div>

        {/* Market Cap */}
        <div className="hidden lg:block flex-shrink-0 w-24 text-xs text-terminal-muted text-right">
          {formatLargeNumber(instrument.market_cap)}
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">Instruments</h1>
        {hasChanges && (
          <button
            onClick={handleSave}
            disabled={isSaving}
            className="btn-primary text-sm"
          >
            {isSaving ? 'Saving...' : 'Save Favorites'}
          </button>
        )}
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-3">
        <div className="flex-1 min-w-[200px]">
          <input
            type="text"
            value={search}
            onChange={(e) => handleSearchChange(e.target.value)}
            placeholder="Search symbol or name..."
            className="terminal-input w-full text-sm"
          />
        </div>

        <select
          value={assetClass}
          onChange={(e) => {
            setAssetClass(e.target.value);
            setPage(1);
          }}
          className="terminal-input text-sm"
        >
          <option value="">All Asset Classes</option>
          <option value="equity">Equity</option>
          <option value="etf">ETF</option>
          <option value="crypto">Crypto</option>
          <option value="forex">Forex</option>
          <option value="index">Index</option>
        </select>

        <select
          value={exchange}
          onChange={(e) => {
            setExchange(e.target.value);
            setPage(1);
          }}
          className="terminal-input text-sm"
        >
          <option value="">All Exchanges</option>
          <option value="NYSE">NYSE</option>
          <option value="NASDAQ">NASDAQ</option>
          <option value="AMEX">AMEX</option>
        </select>

        <select
          value={country}
          onChange={(e) => {
            setCountry(e.target.value);
            setPage(1);
          }}
          className="terminal-input text-sm"
        >
          <option value="">All Countries</option>
          <option value="US">US</option>
          <option value="GB">GB</option>
          <option value="CA">CA</option>
        </select>
      </div>

      {/* Results count */}
      <div className="text-xs text-terminal-muted">
        {totalCount > 0
          ? `Showing ${(page - 1) * PAGE_SIZE + 1}-${Math.min(
              page * PAGE_SIZE,
              totalCount
            )} of ${totalCount.toLocaleString()} instruments`
          : 'No instruments found'}
      </div>

      {/* Table */}
      <div className="terminal-panel overflow-hidden">
        {/* Header row */}
        <div className="flex items-center px-4 py-2 border-b border-terminal-border bg-terminal-panel text-xs text-terminal-muted uppercase tracking-wider">
          <div className="mr-3 w-4" /> {/* Star column */}
          <div className="flex-shrink-0 w-24">Symbol</div>
          <div className="flex-1">Name</div>
          <div className="hidden md:block flex-shrink-0 w-20">Exchange</div>
          <div className="flex-shrink-0 w-24 text-right">Price</div>
          <div className="hidden lg:block flex-shrink-0 w-24 text-right">Mkt Cap</div>
        </div>

        {/* Loading state */}
        {isLoading && (
          <div className="flex items-center justify-center py-12">
            <div className="w-5 h-5 border-2 border-terminal-accent border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        {/* Error state */}
        {error && !isLoading && (
          <div className="text-center py-12">
            <p className="text-terminal-red text-sm">{error}</p>
          </div>
        )}

        {/* Data rows */}
        {!isLoading && !error && instruments.length === 0 && (
          <div className="text-center py-12 text-terminal-muted text-sm">
            No instruments found matching your criteria.
          </div>
        )}

        {!isLoading && !error && instruments.length > 0 && (
          <>
            {useVirtualization ? (
              <VirtualList
                height={Math.min(instruments.length * ROW_HEIGHT, 600)}
                itemCount={instruments.length}
                itemSize={ROW_HEIGHT}
                width="100%"
              >
                {({ index, style }) => renderRow(instruments[index], index, style)}
              </VirtualList>
            ) : (
              <div>
                {instruments.map((instrument, index) =>
                  renderRow(instrument, index)
                )}
              </div>
            )}
          </>
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2">
          <button
            onClick={() => setPage(Math.max(1, page - 1))}
            disabled={page <= 1}
            className="btn-ghost text-sm disabled:opacity-30"
          >
            Previous
          </button>

          <div className="flex items-center gap-1">
            {generatePageNumbers(page, totalPages).map((p, i) =>
              p === '...' ? (
                <span key={`dots-${i}`} className="px-2 text-terminal-muted text-sm">
                  ...
                </span>
              ) : (
                <button
                  key={p}
                  onClick={() => setPage(Number(p))}
                  className={`px-3 py-1 rounded text-sm transition-colors ${
                    Number(p) === page
                      ? 'bg-terminal-accent text-white'
                      : 'text-terminal-muted hover:text-terminal-text hover:bg-terminal-border/50'
                  }`}
                >
                  {p}
                </button>
              )
            )}
          </div>

          <button
            onClick={() => setPage(Math.min(totalPages, page + 1))}
            disabled={page >= totalPages}
            className="btn-ghost text-sm disabled:opacity-30"
          >
            Next
          </button>
        </div>
      )}
    </div>
  );
}

function generatePageNumbers(current: number, total: number): (number | string)[] {
  if (total <= 7) {
    return Array.from({ length: total }, (_, i) => i + 1);
  }

  const pages: (number | string)[] = [1];

  if (current > 3) pages.push('...');

  const start = Math.max(2, current - 1);
  const end = Math.min(total - 1, current + 1);

  for (let i = start; i <= end; i++) {
    pages.push(i);
  }

  if (current < total - 2) pages.push('...');

  pages.push(total);
  return pages;
}
