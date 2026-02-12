import { useEffect, useState } from 'react';
import { getFundamentals } from '../lib/api';
import type { FundamentalsRow } from '../lib/types';
import { formatLargeNumber, formatEps } from './utils';

interface FundamentalsTableProps {
  symbol: string;
}

export default function FundamentalsTable({ symbol }: FundamentalsTableProps) {
  const [data, setData] = useState<FundamentalsRow[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadFundamentals();
  }, [symbol]);

  const loadFundamentals = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await getFundamentals(symbol);
      setData(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load fundamentals');
    } finally {
      setIsLoading(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="w-5 h-5 border-2 border-terminal-accent border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="terminal-panel p-6 text-center">
        <p className="text-terminal-red text-sm mb-3">{error}</p>
        <button onClick={loadFundamentals} className="btn-primary text-sm">
          Retry
        </button>
      </div>
    );
  }

  if (data.length === 0) {
    return (
      <div className="terminal-panel p-8 text-center">
        <p className="text-terminal-muted text-sm">No fundamentals data available for {symbol}</p>
      </div>
    );
  }

  // Revenue trend mini bar chart
  const maxRevenue = Math.max(
    ...data.filter((r) => r.revenue != null).map((r) => Math.abs(r.revenue!))
  );

  return (
    <div className="space-y-6">
      {/* Revenue Trend */}
      {data.some((r) => r.revenue != null) && (
        <div className="terminal-panel p-4">
          <h3 className="text-xs text-terminal-muted uppercase tracking-wider mb-3">
            Revenue Trend
          </h3>
          <div className="flex items-end gap-1 h-24">
            {data
              .slice()
              .reverse()
              .slice(-16)
              .map((row, i) => {
                const rev = row.revenue ?? 0;
                const height = maxRevenue > 0 ? (Math.abs(rev) / maxRevenue) * 100 : 0;
                const isPositive = rev >= 0;
                return (
                  <div
                    key={`bar-${i}`}
                    className="flex-1 flex flex-col items-center justify-end"
                  >
                    <div
                      className={`w-full rounded-t-sm ${
                        isPositive ? 'bg-terminal-accent' : 'bg-terminal-red'
                      }`}
                      style={{ height: `${Math.max(height, 2)}%` }}
                      title={`${row.period ?? ''} ${row.calendar_year ?? ''}: ${formatLargeNumber(rev)}`}
                    />
                    <span className="text-[8px] text-terminal-muted mt-1 truncate w-full text-center">
                      {row.period ?? ''}
                    </span>
                  </div>
                );
              })}
          </div>
        </div>
      )}

      {/* Table */}
      <div className="terminal-panel overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-terminal-border text-xs text-terminal-muted uppercase tracking-wider">
              <th className="text-left px-4 py-3">Period</th>
              <th className="text-right px-4 py-3">Revenue</th>
              <th className="text-right px-4 py-3">Gross Profit</th>
              <th className="text-right px-4 py-3 hidden md:table-cell">Op. Income</th>
              <th className="text-right px-4 py-3">Net Income</th>
              <th className="text-right px-4 py-3">EPS</th>
            </tr>
          </thead>
          <tbody>
            {data.map((row, index) => (
              <tr
                key={`${row.period_end_date}-${index}`}
                className={`border-b border-terminal-border/50 hover:bg-terminal-border/30 transition-colors ${
                  index % 2 === 0 ? '' : 'bg-terminal-bg/50'
                }`}
              >
                <td className="px-4 py-2.5 text-left">
                  <div className="font-medium">
                    {row.period ?? '--'} {row.calendar_year ?? ''}
                  </div>
                  <div className="text-xs text-terminal-muted">
                    {row.period_end_date}
                  </div>
                </td>
                <td className="px-4 py-2.5 text-right font-medium">
                  {formatLargeNumber(row.revenue)}
                </td>
                <td className="px-4 py-2.5 text-right">
                  {formatLargeNumber(row.gross_profit)}
                </td>
                <td className="px-4 py-2.5 text-right hidden md:table-cell">
                  <span
                    className={
                      row.operating_income != null && row.operating_income < 0
                        ? 'text-terminal-red'
                        : ''
                    }
                  >
                    {formatLargeNumber(row.operating_income)}
                  </span>
                </td>
                <td className="px-4 py-2.5 text-right">
                  <span
                    className={
                      row.net_income != null && row.net_income < 0
                        ? 'text-terminal-red'
                        : ''
                    }
                  >
                    {formatLargeNumber(row.net_income)}
                  </span>
                </td>
                <td className="px-4 py-2.5 text-right">
                  <span
                    className={
                      row.eps != null && row.eps < 0 ? 'text-terminal-red' : ''
                    }
                  >
                    {formatEps(row.eps)}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
