import { useAuthStore } from '../lib/store';
import { formatDate } from './utils';

export default function ProfileCard() {
  const { user } = useAuthStore();

  if (!user) {
    return (
      <div className="terminal-panel p-6 text-center">
        <p className="text-terminal-muted text-sm">No user data available</p>
      </div>
    );
  }

  return (
    <div className="terminal-panel p-4 space-y-4">
      <h3 className="text-sm font-medium text-terminal-muted uppercase tracking-wider">
        Profile
      </h3>

      <div className="flex items-center gap-4">
        {/* Avatar */}
        <div className="w-12 h-12 rounded-full bg-terminal-accent/20 flex items-center justify-center flex-shrink-0">
          <span className="text-terminal-accent font-bold text-lg">
            {user.first_name.charAt(0)}
            {user.last_name.charAt(0)}
          </span>
        </div>

        <div className="min-w-0">
          <div className="font-bold text-terminal-text">
            {user.first_name} {user.last_name}
          </div>
          <div className="text-sm text-terminal-muted truncate">{user.email}</div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4 pt-3 border-t border-terminal-border">
        <div>
          <div className="text-xs text-terminal-muted uppercase tracking-wider mb-1">
            Member Since
          </div>
          <div className="text-sm">{formatDate(user.created_at)}</div>
        </div>
        <div>
          <div className="text-xs text-terminal-muted uppercase tracking-wider mb-1">
            Last Updated
          </div>
          <div className="text-sm">{formatDate(user.updated_at)}</div>
        </div>
      </div>
    </div>
  );
}
