import { useEffect, useState } from 'react';
import { useMarketStore } from '../lib/store';

function formatCountdown(targetDate: string | null): string {
  if (!targetDate) return '';
  const now = new Date().getTime();
  const target = new Date(targetDate).getTime();
  const diff = target - now;

  if (diff <= 0) return 'now';

  const hours = Math.floor(diff / (1000 * 60 * 60));
  const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));
  const seconds = Math.floor((diff % (1000 * 60)) / 1000);

  if (hours > 24) {
    const days = Math.floor(hours / 24);
    return `${days}d ${hours % 24}h`;
  }
  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }
  return `${minutes}m ${seconds}s`;
}

export default function MarketStatusBar() {
  const { status, isOpen } = useMarketStore();
  const [countdown, setCountdown] = useState('');

  useEffect(() => {
    const target = isOpen ? status?.next_close : status?.next_open;
    if (!target) return;

    setCountdown(formatCountdown(target));
    const interval = setInterval(() => {
      setCountdown(formatCountdown(target));
    }, 1000);

    return () => clearInterval(interval);
  }, [status, isOpen]);

  if (!status) return null;

  return (
    <div className="bg-terminal-panel/50 border-b border-terminal-border px-4 lg:px-6 py-1.5">
      <div className="max-w-[1600px] mx-auto flex items-center justify-between text-xs">
        <div className="flex items-center gap-2">
          <span
            className={`inline-block w-1.5 h-1.5 rounded-full ${
              isOpen ? 'bg-terminal-green live-dot' : 'bg-terminal-muted'
            }`}
          />
          <span className={isOpen ? 'text-terminal-green' : 'text-terminal-muted'}>
            {isOpen ? 'Market Open' : 'Market Closed'}
          </span>
          {status.message && (
            <span className="text-terminal-muted ml-2">{status.message}</span>
          )}
        </div>

        {countdown && (
          <span className="text-terminal-muted">
            {isOpen ? 'Closes in ' : 'Opens in '}
            <span className="text-terminal-text font-medium">{countdown}</span>
          </span>
        )}
      </div>
    </div>
  );
}
