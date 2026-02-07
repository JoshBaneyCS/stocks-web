import { useState, useEffect } from 'react';
import { getMe } from '@/lib/api';
import type { User } from '@/lib/types';

export default function ProfileCard() {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getMe()
      .then(setUser)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="panel">
        <div className="panel-header">
          <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
            Profile
          </span>
        </div>
        <div className="panel-body animate-pulse space-y-3">
          <div className="h-4 bg-terminal-border rounded w-48" />
          <div className="h-4 bg-terminal-border rounded w-64" />
          <div className="h-4 bg-terminal-border rounded w-40" />
        </div>
      </div>
    );
  }

  if (!user) return null;

  return (
    <div className="panel">
      <div className="panel-header">
        <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
          Profile
        </span>
      </div>
      <div className="panel-body">
        <div className="flex items-center gap-4">
          <div className="w-12 h-12 rounded-full bg-terminal-accent/20 border border-terminal-accent/40 flex items-center justify-center text-terminal-accent text-lg font-bold">
            {user.first_name?.[0]?.toUpperCase()}{user.last_name?.[0]?.toUpperCase()}
          </div>
          <div>
            <p className="text-sm font-semibold text-terminal-text">
              {user.first_name} {user.last_name}
            </p>
            <p className="text-xs text-terminal-dim font-mono">{user.email}</p>
            <p className="text-2xs text-terminal-muted mt-0.5">
              Member since {new Date(user.created_at).toLocaleDateString('en-US', {
                month: 'long',
                year: 'numeric',
              })}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}