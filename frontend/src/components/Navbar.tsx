import { useAuthStore } from '../lib/store';
import { useMarketStore } from '../lib/store';

export default function Navbar() {
  const { user, logout } = useAuthStore();
  const { isOpen } = useMarketStore();

  const handleLogout = async () => {
    await logout();
    window.location.href = '/login';
  };

  const currentPath = typeof window !== 'undefined' ? window.location.pathname : '';

  const navLinks = [
    { href: '/app', label: 'Dashboard' },
    { href: '/app/instruments', label: 'Instruments' },
    { href: '/app/settings', label: 'Settings' },
  ];

  const isActive = (href: string) => {
    if (href === '/app') return currentPath === '/app' || currentPath === '/app/';
    return currentPath.startsWith(href);
  };

  return (
    <nav className="bg-terminal-panel border-b border-terminal-border sticky top-0 z-50">
      <div className="max-w-[1600px] mx-auto px-4 lg:px-6">
        <div className="flex items-center justify-between h-14">
          {/* Left: Logo + Nav Links */}
          <div className="flex items-center gap-6">
            <a
              href="/app"
              className="flex items-center gap-2 text-terminal-accent font-bold text-lg tracking-tight"
            >
              <svg
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                className="w-5 h-5"
              >
                <polyline points="22 7 13.5 15.5 8.5 10.5 2 17" />
                <polyline points="16 7 22 7 22 13" />
              </svg>
              <span className="hidden sm:inline">STOCKS</span>
            </a>

            <div className="flex items-center gap-1">
              {navLinks.map((link) => (
                <a
                  key={link.href}
                  href={link.href}
                  className={`px-3 py-2 rounded text-sm transition-colors ${
                    isActive(link.href)
                      ? 'bg-terminal-accent/10 text-terminal-accent'
                      : 'text-terminal-muted hover:text-terminal-text hover:bg-terminal-border/50'
                  }`}
                >
                  {link.label}
                </a>
              ))}
            </div>
          </div>

          {/* Right: Market dot + User + Logout */}
          <div className="flex items-center gap-4">
            <span
              className={`inline-block w-2 h-2 rounded-full ${
                isOpen ? 'bg-terminal-green live-dot' : 'bg-terminal-muted'
              }`}
              title={isOpen ? 'Market Open' : 'Market Closed'}
            />

            {user && (
              <span className="text-sm text-terminal-muted hidden sm:inline">
                {user.first_name} {user.last_name}
              </span>
            )}

            <button
              onClick={handleLogout}
              className="text-sm text-terminal-muted hover:text-terminal-red transition-colors px-2 py-1"
            >
              Logout
            </button>
          </div>
        </div>
      </div>
    </nav>
  );
}
