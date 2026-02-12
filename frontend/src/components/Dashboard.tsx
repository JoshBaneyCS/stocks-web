import { useEffect, useState } from 'react';
import { getDashboard } from '../lib/api';
import type { DashboardResponse } from '../lib/types';
import { useMarketStore } from '../lib/store';
import FavoritesList from './FavoritesList';

export default function Dashboard() {
  const [data, setData] = useState<DashboardResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const { isOpen, status } = useMarketStore();

  useEffect(() => {
    loadDashboard();
  }, []);

  const loadDashboard = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await getDashboard();
      setData(response);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load dashboard');
    } finally {
      setIsLoading(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="flex flex-col items-center gap-3">
          <div className="w-6 h-6 border-2 border-terminal-accent border-t-transparent rounded-full animate-spin" />
          <span className="text-terminal-muted text-sm">Loading dashboard...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="terminal-panel p-6 text-center">
        <p className="text-terminal-red text-sm mb-3">{error}</p>
        <button onClick={loadDashboard} className="btn-primary text-sm">
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">Dashboard</h1>
        <span className="text-xs text-terminal-muted">
          Last updated: {new Date().toLocaleTimeString()}
        </span>
      </div>

      {/* Grid layout */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: Favorites */}
        <div className="lg:col-span-2 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-medium text-terminal-muted uppercase tracking-wider">
              Watchlist
            </h2>
            <a
              href="/app/instruments"
              className="text-xs text-terminal-accent hover:underline"
            >
              + Add instruments
            </a>
          </div>
          <FavoritesList favorites={data?.favorites ?? []} />
        </div>

        {/* Right: Sidebar */}
        <div className="space-y-4">
          {/* Market Status Card */}
          <div className="terminal-panel p-4">
            <h3 className="text-xs font-medium text-terminal-muted uppercase tracking-wider mb-3">
              Market Status
            </h3>
            <div className="flex items-center gap-3 mb-3">
              <span
                className={`w-3 h-3 rounded-full ${
                  isOpen ? 'bg-terminal-green live-dot' : 'bg-terminal-muted'
                }`}
              />
              <span className={`text-lg font-bold ${isOpen ? 'text-terminal-green' : 'text-terminal-muted'}`}>
                {isOpen ? 'OPEN' : 'CLOSED'}
              </span>
            </div>
            {status?.message && (
              <p className="text-xs text-terminal-muted">{status.message}</p>
            )}
          </div>

          {/* Quick Links Card */}
          <div className="terminal-panel p-4">
            <h3 className="text-xs font-medium text-terminal-muted uppercase tracking-wider mb-3">
              Quick Links
            </h3>
            <div className="space-y-2">
              <a
                href="/app/instruments"
                className="flex items-center gap-2 text-sm text-terminal-text hover:text-terminal-accent transition-colors py-1"
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-4 h-4">
                  <circle cx="11" cy="11" r="8" />
                  <path d="m21 21-4.3-4.3" />
                </svg>
                Browse Instruments
              </a>
              <a
                href="/app/settings"
                className="flex items-center gap-2 text-sm text-terminal-text hover:text-terminal-accent transition-colors py-1"
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-4 h-4">
                  <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
                  <circle cx="12" cy="12" r="3" />
                </svg>
                Manage Settings
              </a>
            </div>
          </div>

          {/* News stub */}
          <div className="terminal-panel p-4">
            <h3 className="text-xs font-medium text-terminal-muted uppercase tracking-wider mb-3">
              Headlines
            </h3>
            <p className="text-xs text-terminal-muted italic">News coming soon</p>
          </div>
        </div>
      </div>
    </div>
  );
}
