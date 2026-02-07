import { useState, useEffect } from 'react';
import { logout, getMe, getMarketStatus } from '@/lib/api';
import type { User, MarketStatus } from '@/lib/types';

export default function Navbar() {
  const [user, setUser] = useState<User | null>(null);
  const [market, setMarket] = useState<MarketStatus | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    getMe()
      .then(setUser)
      .catch(() => {
        window.location.href = '/login';
      });

    getMarketStatus()
      .then(setMarket)
      .catch(() => {});

    // Poll market status every 60s
    const interval = setInterval(() => {
      getMarketStatus()
        .then(setMarket)
        .catch(() => {});
    }, 60_000);

    return () => clearInterval(interval);
  }, []);

  const handleLogout = async () => {
    try {
      await logout();
    } finally {
      window.location.href = '/login';
    }
  };

  const currentPath =
    typeof window !== 'undefined' ? window.location.pathname : '';

  const navLinks = [
    { href: '/app', label: 'Dashboard' },
    { href: '/app/stocks', label: 'Stocks' },
    { href: '/app/settings', label: 'Settings' },
  ];

  return (
    <nav className="border-b border-terminal-border bg-terminal-surface/80 backdrop-blur-sm sticky top-0 z-50">
      <div className="max-w-screen-2xl mx-auto px-4">
        <div className="flex items-center justify-between h-14">
          {/* Logo + Brand */}
          <div className="flex items-center gap-6">
            <a
              href="/app"
              className="flex items-center gap-2 text-terminal-accent font-mono font-bold text-lg tracking-tight hover:opacity-90 transition-opacity"
            >
              <svg
                className="w-6 h-6"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <polyline points="22 7 13.5 15.5 8.5 10.5 2 17" />
                <polyline points="16 7 22 7 22 13" />
              </svg>
              <span>STOCKS</span>
            </a>

            {/* Nav Links */}
            <div className="hidden md:flex items-center gap-1">
              {navLinks.map((link) => {
                const isActive =
                  currentPath === link.href ||
                  (link.href !== '/app' &&
                    currentPath.startsWith(link.href));

                return (
                  <a
                    key={link.href}
                    href={link.href}
                    className={`px-3 py-1.5 rounded text-sm font-medium transition-colors ${
                      isActive
                        ? 'bg-terminal-accent/10 text-terminal-accent'
                        : 'text-terminal-dim hover:text-terminal-text hover:bg-terminal-border/30'
                    }`}
                  >
                    {link.label}
                  </a>
                );
              })}
            </div>
          </div>

          {/* Right side: Market Status + User */}
          <div className="flex items-center gap-4">
            {/* Market Status */}
            {market && (
              <div className="hidden sm:flex items-center gap-2 text-xs font-mono">
                <span
                  className={`inline-block w-2 h-2 rounded-full ${
                    market.is_open
                      ? 'bg-terminal-green animate-pulse-slow'
                      : 'bg-terminal-muted'
                  }`}
                />
                <span
                  className={
                    market.is_open ? 'text-terminal-green' : 'text-terminal-dim'
                  }
                >
                  {market.is_open ? 'MARKET OPEN' : 'MARKET CLOSED'}
                </span>
              </div>
            )}

            {/* User Menu */}
            <div className="relative">
              <button
                onClick={() => setMenuOpen(!menuOpen)}
                className="flex items-center gap-2 px-2 py-1.5 rounded text-sm
                           text-terminal-dim hover:text-terminal-text hover:bg-terminal-border/30
                           transition-colors"
              >
                <div className="w-7 h-7 rounded-full bg-terminal-accent/20 border border-terminal-accent/40 flex items-center justify-center text-terminal-accent text-xs font-bold">
                  {user?.first_name?.[0]?.toUpperCase() || '?'}
                </div>
                <span className="hidden md:inline font-mono">
                  {user?.first_name || '...'}
                </span>
                <svg
                  className={`w-3.5 h-3.5 transition-transform ${menuOpen ? 'rotate-180' : ''}`}
                  viewBox="0 0 20 20"
                  fill="currentColor"
                >
                  <path
                    fillRule="evenodd"
                    d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
                    clipRule="evenodd"
                  />
                </svg>
              </button>

              {menuOpen && (
                <>
                  {/* Backdrop */}
                  <div
                    className="fixed inset-0 z-40"
                    onClick={() => setMenuOpen(false)}
                  />
                  {/* Dropdown */}
                  <div className="absolute right-0 mt-1 w-48 rounded-md border border-terminal-border bg-terminal-surface shadow-lg z-50 animate-fade-in">
                    <div className="px-3 py-2 border-b border-terminal-border">
                      <p className="text-xs text-terminal-dim truncate">
                        {user?.email || ''}
                      </p>
                    </div>
                    {/* Mobile nav links */}
                    <div className="md:hidden border-b border-terminal-border">
                      {navLinks.map((link) => (
                        <a
                          key={link.href}
                          href={link.href}
                          className="block px-3 py-2 text-sm text-terminal-dim hover:text-terminal-text hover:bg-terminal-border/30 transition-colors"
                        >
                          {link.label}
                        </a>
                      ))}
                    </div>
                    <button
                      onClick={handleLogout}
                      className="w-full text-left px-3 py-2 text-sm text-terminal-red hover:bg-terminal-border/30 transition-colors rounded-b-md"
                    >
                      Sign Out
                    </button>
                  </div>
                </>
              )}
            </div>
          </div>
        </div>
      </div>
    </nav>
  );
}