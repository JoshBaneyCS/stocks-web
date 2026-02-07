import { useState, useEffect } from 'react';
import { getMarketStatus } from '@/lib/api';
import type { MarketStatus as MarketStatusType } from '@/lib/types';

interface Props {
  compact?: boolean;
}

export default function MarketStatus({ compact = false }: Props) {
  const [status, setStatus] = useState<MarketStatusType | null>(null);
  const [countdown, setCountdown] = useState('');

  useEffect(() => {
    let mounted = true;

    const fetchStatus = () => {
      getMarketStatus()
        .then((data) => {
          if (mounted) setStatus(data);
        })
        .catch(() => {});
    };

    fetchStatus();
    const interval = setInterval(fetchStatus, 30_000);

    return () => {
      mounted = false;
      clearInterval(interval);
    };
  }, []);

  // Countdown timer
  useEffect(() => {
    if (!status) return;

    const targetTime = status.is_open ? status.next_close : status.next_open;
    if (!targetTime) return;

    const updateCountdown = () => {
      const now = Date.now();
      const target = new Date(targetTime).getTime();
      const diff = target - now;

      if (diff <= 0) {
        setCountdown('');
        // Refetch status when countdown expires
        getMarketStatus()
          .then(setStatus)
          .catch(() => {});
        return;
      }

      const hours = Math.floor(diff / 3_600_000);
      const minutes = Math.floor((diff % 3_600_000) / 60_000);
      const seconds = Math.floor((diff % 60_000) / 1_000);

      if (hours > 0) {
        setCountdown(`${hours}h ${minutes}m`);
      } else if (minutes > 0) {
        setCountdown(`${minutes}m ${seconds}s`);
      } else {
        setCountdown(`${seconds}s`);
      }
    };

    updateCountdown();
    const timer = setInterval(updateCountdown, 1_000);

    return () => clearInterval(timer);
  }, [status]);

  if (!status) {
    return (
      <div className={`${compact ? '' : 'panel'}`}>
        {!compact && <div className="panel-header"><span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">Market Status</span></div>}
        <div className={compact ? '' : 'panel-body'}>
          <div className="flex items-center gap-2">
            <span className="inline-block w-2 h-2 rounded-full bg-terminal-muted animate-pulse" />
            <span className="text-sm text-terminal-dim">Loading...</span>
          </div>
        </div>
      </div>
    );
  }

  if (compact) {
    return (
      <div className="flex items-center gap-2">
        <span
          className={`inline-block w-2.5 h-2.5 rounded-full ${
            status.is_open
              ? 'bg-terminal-green animate-pulse-slow'
              : 'bg-terminal-muted'
          }`}
        />
        <div className="flex flex-col">
          <span
            className={`text-xs font-mono font-semibold ${
              status.is_open ? 'text-terminal-green' : 'text-terminal-dim'
            }`}
          >
            {status.is_open ? 'OPEN' : 'CLOSED'}
          </span>
          {countdown && (
            <span className="text-2xs text-terminal-muted font-mono">
              {status.is_open ? 'Closes' : 'Opens'} in {countdown}
            </span>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="panel">
      <div className="panel-header">
        <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
          Market Status
        </span>
        <span className="text-2xs text-terminal-muted font-mono">
          NYSE / NASDAQ
        </span>
      </div>
      <div className="panel-body">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            {/* Status indicator */}
            <div
              className={`w-10 h-10 rounded-full flex items-center justify-center ${
                status.is_open
                  ? 'bg-terminal-green/10 border border-terminal-green/30'
                  : 'bg-terminal-muted/10 border border-terminal-muted/30'
              }`}
            >
              <span
                className={`inline-block w-3 h-3 rounded-full ${
                  status.is_open
                    ? 'bg-terminal-green animate-pulse-slow'
                    : 'bg-terminal-muted'
                }`}
              />
            </div>

            <div>
              <p
                className={`text-sm font-mono font-bold ${
                  status.is_open ? 'text-terminal-green' : 'text-terminal-dim'
                }`}
              >
                {status.is_open ? 'MARKET OPEN' : 'MARKET CLOSED'}
              </p>
              <p className="text-xs text-terminal-muted">
                {status.message}
              </p>
            </div>
          </div>

          {/* Countdown */}
          {countdown && (
            <div className="text-right">
              <p className="text-xs text-terminal-muted">
                {status.is_open ? 'Closes in' : 'Opens in'}
              </p>
              <p className="text-sm font-mono font-semibold text-terminal-text font-tabular">
                {countdown}
              </p>
            </div>
          )}
        </div>

        {/* Session times */}
        <div className="mt-3 pt-3 border-t border-terminal-border flex justify-between text-2xs text-terminal-muted font-mono">
          <span>
            Open:{' '}
            <span className="text-terminal-dim">09:30 ET</span>
          </span>
          <span>
            Close:{' '}
            <span className="text-terminal-dim">16:00 ET</span>
          </span>
          <span>
            TZ:{' '}
            <span className="text-terminal-dim">America/New_York</span>
          </span>
        </div>
      </div>
    </div>
  );
}