import { useState, useEffect, useCallback, useRef } from 'react';
import { getStocks, getFavorites, updateFavorites } from '@/lib/api';
import type { Company, Favorite, StockFilters, PaginatedResponse } from '@/lib/types';

const PAGE_SIZE = 25;

export default function StockList() {
  const [stocks, setStocks] = useState<PaginatedResponse<Company> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Filters
  const [search, setSearch] = useState('');
  const [exchange, setExchange] = useState('');
  const [sector, setSector] = useState('');
  const [industry, setIndustry] = useState('');
  const [page, setPage] = useState(1);

  // Favorites
  const [favoriteIds, setFavoriteIds] = useState<Set<number>>(new Set());
  const [pendingFavIds, setPendingFavIds] = useState<Set<number>>(new Set());
  const [savingFavs, setSavingFavs] = useState(false);
  const [favSaved, setFavSaved] = useState(false);

  const searchTimeout = useRef<ReturnType<typeof setTimeout>>();

  // Has unsaved changes?
  const hasChanges = (() => {
    if (favoriteIds.size !== pendingFavIds.size) return true;
    for (const id of favoriteIds) {
      if (!pendingFavIds.has(id)) return true;
    }
    return false;
  })();

  // Load favorites
  useEffect(() => {
    getFavorites()
      .then((favs: Favorite[]) => {
        const ids = new Set(favs.map((f) => f.company_id));
        setFavoriteIds(ids);
        setPendingFavIds(new Set(ids));
      })
      .catch(() => {});
  }, []);

  // Load stocks
  const loadStocks = useCallback(async (filters: StockFilters) => {
    setLoading(true);
    setError('');
    try {
      const data = await getStocks(filters);
      setStocks(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load stocks');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadStocks({
      search,
      exchange,
      sector,
      industry,
      page,
      pageSize: PAGE_SIZE,
    });
  }, [search, exchange, sector, industry, page, loadStocks]);

  // Debounced search
  const handleSearchChange = (value: string) => {
    if (searchTimeout.current) clearTimeout(searchTimeout.current);
    searchTimeout.current = setTimeout(() => {
      setSearch(value);
      setPage(1);
    }, 300);
  };

  // Toggle favorite
  const toggleFavorite = (companyId: number) => {
    setPendingFavIds((prev) => {
      const next = new Set(prev);
      if (next.has(companyId)) {
        next.delete(companyId);
      } else {
        next.add(companyId);
      }
      return next;
    });
    setFavSaved(false);
  };

  // Save favorites
  const saveFavorites = async () => {
    setSavingFavs(true);
    setFavSaved(false);
    try {
      await updateFavorites({ company_ids: Array.from(pendingFavIds) });
      setFavoriteIds(new Set(pendingFavIds));
      setFavSaved(true);
      setTimeout(() => setFavSaved(false), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save favorites');
    } finally {
      setSavingFavs(false);
    }
  };

  const handleFilterChange = (setter: (v: string) => void) => (e: React.ChangeEvent<HTMLSelectElement>) => {
    setter(e.target.value);
    setPage(1);
  };

  // Format large numbers
  const formatMarketCap = (cap: number): string => {
    if (!cap) return '—';
    if (cap >= 1e12) return `$${(cap / 1e12).toFixed(2)}T`;
    if (cap >= 1e9) return `$${(cap / 1e9).toFixed(2)}B`;
    if (cap >= 1e6) return `$${(cap / 1e6).toFixed(1)}M`;
    return `$${cap.toLocaleString()}`;
  };

  // Unique values for filter dropdowns (from current page — backend handles full filtering)
  const exchanges = stocks?.data
    ? [...new Set(stocks.data.map((s) => s.exchange).filter(Boolean))].sort()
    : [];
  const sectors = stocks?.data
    ? [...new Set(stocks.data.map((s) => s.sector).filter(Boolean))].sort()
    : [];

  return (
    <div className="space-y-4">
      {/* Filters bar */}
      <div className="panel">
        <div className="panel-body">
          <div className="flex flex-col md:flex-row gap-3">
            {/* Search */}
            <div className="flex-1">
              <input
                type="text"
                placeholder="Search by symbol or company name..."
                defaultValue={search}
                onChange={(e) => handleSearchChange(e.target.value)}
                className="input-field font-mono text-sm"
              />
            </div>

            {/* Exchange filter */}
            <select
              value={exchange}
              onChange={handleFilterChange(setExchange)}
              className="input-field w-full md:w-40 text-sm"
            >
              <option value="">All Exchanges</option>
              {exchanges.map((ex) => (
                <option key={ex} value={ex}>{ex}</option>
              ))}
            </select>

            {/* Sector filter */}
            <select
              value={sector}
              onChange={handleFilterChange(setSector)}
              className="input-field w-full md:w-48 text-sm"
            >
              <option value="">All Sectors</option>
              {sectors.map((s) => (
                <option key={s} value={s}>{s}</option>
              ))}
            </select>
          </div>
        </div>
      </div>

      {/* Favorites save bar */}
      {hasChanges && (
        <div className="flex items-center justify-between px-4 py-2.5 rounded-lg bg-terminal-accent/10 border border-terminal-accent/20 animate-fade-in">
          <span className="text-sm text-terminal-accent">
            You have unsaved favorites changes
          </span>
          <button
            onClick={saveFavorites}
            disabled={savingFavs}
            className="btn-primary text-xs px-6"
          >
            {savingFavs ? 'Saving...' : 'Save Favorites'}
          </button>
        </div>
      )}

      {favSaved && !hasChanges && (
        <div className="flex items-center gap-2 px-4 py-2.5 rounded-lg bg-terminal-green/10 border border-terminal-green/20 animate-fade-in">
          <svg className="w-4 h-4 text-terminal-green" viewBox="0 0 20 20" fill="currentColor">
            <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
          </svg>
          <span className="text-sm text-terminal-green">Favorites saved successfully</span>
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="px-4 py-2.5 rounded-lg bg-terminal-red/10 border border-terminal-red/20 text-sm text-terminal-red">
          {error}
        </div>
      )}

      {/* Table */}
      <div className="panel overflow-hidden">
        <div className="overflow-x-auto">
          <table className="data-table">
            <thead>
              <tr>
                <th className="w-10">
                  <span className="sr-only">Favorite</span>
                  ★
                </th>
                <th>Symbol</th>
                <th className="hidden sm:table-cell">Company</th>
                <th className="hidden md:table-cell">Exchange</th>
                <th className="hidden lg:table-cell">Sector</th>
                <th className="text-right">Mkt Cap</th>
                <th className="text-right hidden sm:table-cell">Prev Close</th>
                <th className="text-right hidden md:table-cell">Volume</th>
              </tr>
            </thead>
            <tbody>
              {loading && !stocks?.data?.length
                ? Array.from({ length: 10 }).map((_, i) => (
                    <tr key={i} className="animate-pulse">
                      <td><div className="h-4 w-4 bg-terminal-border rounded" /></td>
                      <td><div className="h-4 bg-terminal-border rounded w-16" /></td>
                      <td className="hidden sm:table-cell"><div className="h-4 bg-terminal-border rounded w-32" /></td>
                      <td className="hidden md:table-cell"><div className="h-4 bg-terminal-border rounded w-16" /></td>
                      <td className="hidden lg:table-cell"><div className="h-4 bg-terminal-border rounded w-24" /></td>
                      <td><div className="h-4 bg-terminal-border rounded w-16 ml-auto" /></td>
                      <td className="hidden sm:table-cell"><div className="h-4 bg-terminal-border rounded w-16 ml-auto" /></td>
                      <td className="hidden md:table-cell"><div className="h-4 bg-terminal-border rounded w-20 ml-auto" /></td>
                    </tr>
                  ))
                : stocks?.data?.map((stock) => (
                    <tr key={stock.id} className="group">
                      {/* Favorite toggle */}
                      <td>
                        <button
                          onClick={(e) => {
                            e.preventDefault();
                            toggleFavorite(stock.id);
                          }}
                          className={`w-6 h-6 rounded flex items-center justify-center text-sm transition-colors ${
                            pendingFavIds.has(stock.id)
                              ? 'text-terminal-yellow bg-terminal-yellow/10'
                              : 'text-terminal-muted hover:text-terminal-yellow/60'
                          }`}
                          title={pendingFavIds.has(stock.id) ? 'Remove from favorites' : 'Add to favorites'}
                        >
                          {pendingFavIds.has(stock.id) ? '★' : '☆'}
                        </button>
                      </td>

                      {/* Symbol */}
                      <td>
                        <a
                          href={`/app/stocks/${stock.symbol}`}
                          className="font-mono font-semibold text-terminal-accent hover:underline"
                        >
                          {stock.symbol}
                        </a>
                      </td>

                      {/* Name */}
                      <td className="hidden sm:table-cell max-w-[200px] truncate text-terminal-dim">
                        {stock.name}
                      </td>

                      {/* Exchange */}
                      <td className="hidden md:table-cell">
                        <span className="text-2xs px-1.5 py-0.5 rounded bg-terminal-border/50 text-terminal-dim">
                          {stock.exchange}
                        </span>
                      </td>

                      {/* Sector */}
                      <td className="hidden lg:table-cell text-terminal-dim text-xs truncate max-w-[160px]">
                        {stock.sector || '—'}
                      </td>

                      {/* Market Cap */}
                      <td className="text-right font-mono text-xs font-tabular">
                        {formatMarketCap(stock.market_cap)}
                      </td>

                      {/* Prev Close */}
                      <td className="hidden sm:table-cell text-right font-mono text-xs font-tabular">
                        {stock.prev_close ? `$${stock.prev_close.toFixed(2)}` : '—'}
                      </td>

                      {/* Volume */}
                      <td className="hidden md:table-cell text-right font-mono text-xs font-tabular text-terminal-dim">
                        {stock.volume ? stock.volume.toLocaleString() : '—'}
                      </td>
                    </tr>
                  ))}
            </tbody>
          </table>
        </div>

        {/* Empty state */}
        {!loading && stocks?.data?.length === 0 && (
          <div className="text-center py-8">
            <p className="text-sm text-terminal-muted">No stocks found matching your filters</p>
          </div>
        )}
      </div>

      {/* Pagination */}
      {stocks && stocks.total_pages > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-xs text-terminal-muted font-mono">
            Showing {((page - 1) * PAGE_SIZE) + 1}–{Math.min(page * PAGE_SIZE, stocks.total)} of {stocks.total} stocks
          </p>
          <div className="flex items-center gap-1">
            <button
              onClick={() => setPage(1)}
              disabled={page === 1}
              className="btn-ghost text-xs px-2 py-1"
              title="First page"
            >
              ««
            </button>
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1}
              className="btn-ghost text-xs px-2 py-1"
            >
              ‹ Prev
            </button>
            <span className="px-3 text-xs font-mono text-terminal-dim">
              {page} / {stocks.total_pages}
            </span>
            <button
              onClick={() => setPage((p) => Math.min(stocks.total_pages, p + 1))}
              disabled={page === stocks.total_pages}
              className="btn-ghost text-xs px-2 py-1"
            >
              Next ›
            </button>
            <button
              onClick={() => setPage(stocks.total_pages)}
              disabled={page === stocks.total_pages}
              className="btn-ghost text-xs px-2 py-1"
              title="Last page"
            >
              »»
            </button>
          </div>
        </div>
      )}
    </div>
  );
}