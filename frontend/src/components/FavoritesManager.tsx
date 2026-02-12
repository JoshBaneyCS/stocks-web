import { useEffect, useState } from 'react';
import { getFavorites, getInstruments, updateFavorites } from '../lib/api';
import type { InstrumentListItem } from '../lib/types';

export default function FavoritesManager() {
  const [allInstruments, setAllInstruments] = useState<InstrumentListItem[]>([]);
  const [favoriteIds, setFavoriteIds] = useState<Set<number>>(new Set());
  const [pendingIds, setPendingIds] = useState<Set<number>>(new Set());
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);
  const [search, setSearch] = useState('');

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const [favs, instruments] = await Promise.all([
        getFavorites(),
        getInstruments({ page_size: 200 }),
      ]);
      setAllInstruments(instruments.data);
      const ids = new Set(favs.map((f) => f.id));
      setFavoriteIds(ids);
      setPendingIds(new Set(ids));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load data');
    } finally {
      setIsLoading(false);
    }
  };

  const handleToggle = (id: number, value: boolean) => {
    setPendingIds((prev) => {
      const next = new Set(prev);
      if (value) {
        next.add(id);
      } else {
        next.delete(id);
      }
      return next;
    });
    setSuccessMsg(null);
  };

  const hasChanges =
    pendingIds.size !== favoriteIds.size ||
    [...pendingIds].some((id) => !favoriteIds.has(id));

  const handleSave = async () => {
    setIsSaving(true);
    setError(null);
    setSuccessMsg(null);
    try {
      await updateFavorites([...pendingIds]);
      const favs = await getFavorites();
      setFavorites(favs);
      const ids = new Set(favs.map((f) => f.id));
      setFavoriteIds(ids);
      setPendingIds(new Set(ids));
      setSuccessMsg('Favorites updated successfully');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save favorites');
    } finally {
      setIsSaving(false);
    }
  };

  const filteredInstruments = search
    ? allInstruments.filter(
        (inst) =>
          inst.symbol.toLowerCase().includes(search.toLowerCase()) ||
          (inst.name ?? '').toLowerCase().includes(search.toLowerCase())
      )
    : allInstruments;

  if (isLoading) {
    return (
      <div className="terminal-panel p-6 flex items-center justify-center">
        <div className="w-5 h-5 border-2 border-terminal-accent border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div className="terminal-panel p-4 space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-terminal-muted uppercase tracking-wider">
          Manage Favorites
        </h3>
        {hasChanges && (
          <button
            onClick={handleSave}
            disabled={isSaving}
            className="btn-primary text-sm"
          >
            {isSaving ? 'Saving...' : 'Save Changes'}
          </button>
        )}
      </div>

      {error && (
        <div className="bg-terminal-red/10 border border-terminal-red/30 rounded px-3 py-2 text-sm text-terminal-red">
          {error}
        </div>
      )}

      {successMsg && (
        <div className="bg-terminal-green/10 border border-terminal-green/30 rounded px-3 py-2 text-sm text-terminal-green">
          {successMsg}
        </div>
      )}

      <input
        type="text"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        placeholder="Filter instruments..."
        className="terminal-input w-full text-sm"
      />

      <div className="max-h-[400px] overflow-y-auto space-y-1">
        {filteredInstruments.map((instrument) => {
          const isFav = pendingIds.has(instrument.id);
          return (
            <div
              key={instrument.id}
              className="flex items-center justify-between py-2 px-3 rounded hover:bg-terminal-border/30 transition-colors"
            >
              <div className="flex items-center gap-3 min-w-0">
                <span className="font-bold text-sm text-terminal-accent w-20 flex-shrink-0">
                  {instrument.symbol}
                </span>
                <span className="text-sm text-terminal-muted truncate">
                  {instrument.name ?? '--'}
                </span>
              </div>

              <div className="flex items-center gap-2 flex-shrink-0">
                <label className="flex items-center gap-1 cursor-pointer">
                  <input
                    type="radio"
                    name={`fav-${instrument.id}`}
                    checked={isFav}
                    onChange={() => handleToggle(instrument.id, true)}
                    className="text-terminal-green focus:ring-terminal-green bg-terminal-bg border-terminal-border"
                  />
                  <span className="text-xs text-terminal-green">Yes</span>
                </label>
                <label className="flex items-center gap-1 cursor-pointer">
                  <input
                    type="radio"
                    name={`fav-${instrument.id}`}
                    checked={!isFav}
                    onChange={() => handleToggle(instrument.id, false)}
                    className="text-terminal-red focus:ring-terminal-red bg-terminal-bg border-terminal-border"
                  />
                  <span className="text-xs text-terminal-muted">No</span>
                </label>
              </div>
            </div>
          );
        })}

        {filteredInstruments.length === 0 && (
          <p className="text-center text-terminal-muted text-sm py-4">
            No instruments found
          </p>
        )}
      </div>

      <div className="text-xs text-terminal-muted pt-2 border-t border-terminal-border">
        {pendingIds.size} instrument{pendingIds.size !== 1 ? 's' : ''} selected as favorites
      </div>
    </div>
  );
}
