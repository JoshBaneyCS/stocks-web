import { useEffect, useState, type ReactNode } from 'react';
import { useAuthStore } from '../lib/store';
import { useMarketStore } from '../lib/store';
import Navbar from './Navbar';
import MarketStatusBar from './MarketStatus';

interface AppShellProps {
  children?: ReactNode;
}

export default function AppShell({ children }: AppShellProps) {
  const { checkAuth, isAuthenticated, isLoading } = useAuthStore();
  const { startPolling } = useMarketStore();
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    checkAuth().finally(() => setChecked(true));
  }, [checkAuth]);

  useEffect(() => {
    if (isAuthenticated) {
      const stop = startPolling();
      return stop;
    }
  }, [isAuthenticated, startPolling]);

  useEffect(() => {
    if (checked && !isLoading && !isAuthenticated) {
      window.location.href = '/login';
    }
  }, [checked, isLoading, isAuthenticated]);

  if (!checked || isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="flex flex-col items-center gap-4">
          <div className="w-8 h-8 border-2 border-terminal-accent border-t-transparent rounded-full animate-spin" />
          <span className="text-terminal-muted text-sm">Loading...</span>
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return null;
  }

  return (
    <div className="min-h-screen flex flex-col">
      <Navbar />
      <MarketStatusBar />
      <main className="flex-1 p-4 lg:p-6 max-w-[1600px] mx-auto w-full">
        {children}
      </main>
    </div>
  );
}
