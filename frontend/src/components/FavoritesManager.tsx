import { useState, useEffect, useCallback } from 'react';
import { getStocks, getFavorites, updateFavorites } from '@/lib/api';
import type { Company, Favorite } from '@/lib/types';

export default function FavoritesManager() {
  const [allStocks, setAllStocks] = useState<Company[]>([]);
  const [favoriteIds, setFavoriteIds] = useState<Set<number>>(new Set());
  const [pendingIds, setPendingIds] = useState<Set<number>>(new Set());
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');

  // Load all stocks and current favorites
  useEffect(() => {
    let mounted = true;

    async function load() {
      try {
        // Fetch all stocks (large page size to get everything)
        const [stocksData, favs] = await Promise.all([
          getStocks({ pageSize: 500 }),
          getFavorites(),
        ]);

        if (!mounted) return;

        setAllStocks(stocksData.data);
        const ids = new Set(favs.map((f: Favorite) => f.company_id));
        setFavoriteIds(ids);
        setPendingIds(new Set(ids));
      } catch (err) {
        if (mounted) {
          setError(err instanceof Error ? err.message : 'Failed to load data');
        }
      } finally {
        if (mounted) setLoading(false);
      }
    }

    load();
    return () => { mounted = false; };
  }, []);

  const hasChanges = useCallback(() => {
    if (favoriteIds.size !== pendingIds.size) return true;
    for (const id of favoriteIds) {
      if (!pendingIds.has(id)) return true;
    }
    return false;
  }, [favoriteIds, pendingIds]);

  const handleToggle = (companyId: number, value: boolean) => {
    setPendingIds((prev) => {
      const next = new Set(prev);
      if (value) {
        next.add(companyId);
      } else {
        next.delete(companyId);
      }
      return next;
    });
    setSaved(false);
  };

  const handleSave = async () => {
    setSaving(true);
    setSaved(false);
    setError('');

    try {
      await updateFavorites({ company_ids: Array.from(pendingIds) });
      setFavoriteIds(new Set(pendingIds));
      setSaved(true);
      setTimeout(() => setSaved(false), 4000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save favorites');
    } finally {
      setSaving(false);
    }
  };

  const handleReset = () => {
    setPendingIds(new Set(favoriteIds));
    setSaved(false);
  };

  // Filter stocks by search
  const filteredStocks = search
    ? allStocks.filter(
        (s) =>
          s.symbol.toLowerCase().includes(search.toLowerCase()) ||
          s.name.toLowerCase().includes(search.toLowerCase()),
      )
    : allStocks;

  // Sort: favorites first, then alphabetical
  const sortedStocks = [...filteredStocks].sort((a, b) => {
    const aFav = pendingIds.has(a.id) ? 0 : 1;
    const bFav = pendingIds.has(b.id) ? 0 : 1;
    if (aFav !== bFav) return aFav - bFav;
    return a.symbol.localeCompare(b.symbol);
  });

  if (loading) {
    return (
      <div className="panel">
        <div className="panel-header">
          <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
            Manage Favorites
          </span>
        </div>
        <div className="panel-body space-y-3">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="animate-pulse flex items-center gap-4 py-2">
              <div className="h-4 bg-terminal-border rounded w-16" />
              <div className="flex-1 h-4 bg-terminal-border rounded w-40" />
              <div className="flex gap-3">
                <div className="h-6 bg-terminal-border rounded w-14" />
                <div className="h-6 bg-terminal-border rounded w-14" />
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Header + Controls */}
      <div className="panel">
        <div className="panel-header">
          <div>
            <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
              Manage Favorites
            </span>
            <p className="text-2xs text-terminal-muted mt-0.5">
              {pendingIds.size} stock{pendingIds.size !== 1 ? 's' : ''} selected
            </p>
          </div>
          <div className="flex items-center gap-2">
            {hasChanges() && (
              <button
                onClick={handleReset}
                className="btn-ghost text-xs px-3 py-1.5"
              >
                Reset
              </button>
            )}
            <button
              onClick={handleSave}
              disabled={!hasChanges() || saving}
              className="btn-primary text-xs px-6 py-1.5"
            >
              {saving ? (
                <span className="flex items-center gap-2">
                  <svg className="animate-spin h-3.5 w-3.5" viewBox="0 0 24 24" fill="none">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                  </svg>
                  Saving...
                </span>
              ) : (
                'Save Favorites'
              )}
            </button>
          </div>
        </div>

        {/* Search */}
        <div className="px-4 py-3 border-b border-terminal-border">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Filter stocks..."
            className="input-field text-sm font-mono"
          />
        </div>

        {/* Status messages */}
        {error && (
          <div className="mx-4 mt-3 rounded-md bg-terminal-red/10 border border-terminal-red/20 px-3 py-2 text-sm text-terminal-red animate-fade-in">
            {error}
          </div>
        )}

        {saved && !hasChanges() && (
          <div className="mx-4 mt-3 flex items-center gap-2 rounded-md bg-terminal-green/10 border border-terminal-green/20 px-3 py-2 text-sm text-terminal-green animate-fade-in">
            <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
              <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
            </svg>
            Favorites saved successfully!
          </div>
        )}

        {hasChanges() && (
          <div className="mx-4 mt-3 flex items-center gap-2 rounded-md bg-terminal-accent/10 border border-terminal-accent/20 px-3 py-2 text-sm text-terminal-accent animate-fade-in">
            <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
              <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clipRule="evenodd" />
            </svg>
            You have unsaved changes — click Save to apply
          </div>
        )}

        {/* Stock list with radio toggles */}
        <div className="divide-y divide-terminal-border/50 max-h-[600px] overflow-y-auto">
          {sortedStocks.map((stock) => {
            const isFav = pendingIds.has(stock.id);

            return (
              <div
                key={stock.id}
                className={`flex items-center gap-4 px-4 py-3 transition-colors ${
                  isFav ? 'bg-terminal-accent/5' : 'hover:bg-terminal-border/20'
                }`}
              >
                {/* Stock info */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-mono font-semibold text-sm text-terminal-accent">
                      {stock.symbol}
                    </span>
                    <span className="text-xs text-terminal-dim truncate">
                      {stock.name}
                    </span>
                  </div>
                  <div className="flex items-center gap-2 mt-0.5">
                    {stock.exchange && (
                      <span className="text-2xs text-terminal-muted">{stock.exchange}</span>
                    )}
                    {stock.sector && (
                      <span className="text-2xs text-terminal-muted">· {stock.sector}</span>
                    )}
                  </div>
                </div>

                {/* Radio Yes/No toggle */}
                <div className="flex items-center gap-1 shrink-0">
                  <label
                    className={`flex items-center gap-1.5 px-3 py-1.5 rounded-l-md border cursor-pointer transition-all text-xs font-medium ${
                      isFav
                        ? 'bg-terminal-green/15 border-terminal-green/40 text-terminal-green'
                        : 'bg-transparent border-terminal-border text-terminal-muted hover:border-terminal-muted'
                    }`}
                  >
                    <input
                      type="radio"
                      name={`fav-${stock.id}`}
                      checked={isFav}
                      onChange={() => handleToggle(stock.id, true)}
                      className="sr-only"
                    />
                    <span
                      className={`w-3 h-3 rounded-full border-2 flex items-center justify-center ${
                        isFav
                          ? 'border-terminal-green'
                          : 'border-terminal-muted'
                      }`}
                    >
                      {isFav && (
                        <span className="w-1.5 h-1.5 rounded-full bg-terminal-green" />
                      )}
                    </span>
                    Yes
                  </label>

                  <label
                    className={`flex items-center gap-1.5 px-3 py-1.5 rounded-r-md border cursor-pointer transition-all text-xs font-medium ${
                      !isFav
                        ? 'bg-terminal-border/30 border-terminal-border text-terminal-dim'
                        : 'bg-transparent border-terminal-border text-terminal-muted hover:border-terminal-muted'
                    }`}
                  >
                    <input
                      type="radio"
                      name={`fav-${stock.id}`}
                      checked={!isFav}
                      onChange={() => handleToggle(stock.id, false)}
                      className="sr-only"
                    />
                    <span
                      className={`w-3 h-3 rounded-full border-2 flex items-center justify-center ${
                        !isFav
                          ? 'border-terminal-dim'
                          : 'border-terminal-muted'
                      }`}
                    >
                      {!isFav && (
                        <span className="w-1.5 h-1.5 rounded-full bg-terminal-dim" />
                      )}
                    </span>
                    No
                  </label>
                </div>
              </div>
            );
          })}
        </div>

        {/* Empty filter state */}
        {filteredStocks.length === 0 && (
          <div className="px-4 py-8 text-center">
            <p className="text-sm text-terminal-muted">
              No stocks matching "{search}"
            </p>
          </div>
        )}
      </div>

      {/* Sticky save bar for mobile */}
      {hasChanges() && (
        <div className="fixed bottom-0 left-0 right-0 p-4 bg-terminal-surface/95 backdrop-blur-sm border-t border-terminal-border md:hidden z-40">
          <button
            onClick={handleSave}
            disabled={saving}
            className="btn-primary w-full"
          >
            {saving ? 'Saving...' : `Save Favorites (${pendingIds.size} selected)`}
          </button>
        </div>
      )}
    </div>
  );
}