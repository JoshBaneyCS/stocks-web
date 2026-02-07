import { useState, useEffect } from 'react';
import type { NewsArticle } from '@/lib/types';

interface Props {
  articles: NewsArticle[];
  loading?: boolean;
  maxItems?: number;
  showSymbols?: boolean;
  compact?: boolean;
}

function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diff = now - then;

  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 30) {
    return new Date(dateStr).toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: days > 365 ? 'numeric' : undefined,
    });
  }
  if (days > 0) return `${days}d ago`;
  if (hours > 0) return `${hours}h ago`;
  if (minutes > 0) return `${minutes}m ago`;
  return 'Just now';
}

function LoadingSkeleton({ count }: { count: number }) {
  return (
    <div className="space-y-3">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="animate-pulse flex gap-3">
          <div className="flex-1 space-y-2">
            <div className="h-3.5 bg-terminal-border rounded w-3/4" />
            <div className="h-3 bg-terminal-border rounded w-1/2" />
          </div>
          <div className="h-3 bg-terminal-border rounded w-12" />
        </div>
      ))}
    </div>
  );
}

export default function NewsHeadlines({
  articles,
  loading = false,
  maxItems = 10,
  showSymbols = true,
  compact = false,
}: Props) {
  const [visibleCount, setVisibleCount] = useState(maxItems);

  useEffect(() => {
    setVisibleCount(maxItems);
  }, [maxItems]);

  if (loading) {
    return (
      <div className={compact ? '' : 'panel'}>
        {!compact && (
          <div className="panel-header">
            <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
              Latest News
            </span>
          </div>
        )}
        <div className={compact ? '' : 'panel-body'}>
          <LoadingSkeleton count={5} />
        </div>
      </div>
    );
  }

  if (!articles.length) {
    return (
      <div className={compact ? '' : 'panel'}>
        {!compact && (
          <div className="panel-header">
            <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
              Latest News
            </span>
          </div>
        )}
        <div className={compact ? 'py-4' : 'panel-body'}>
          <p className="text-sm text-terminal-muted text-center">
            No news articles available
          </p>
        </div>
      </div>
    );
  }

  const visible = articles.slice(0, visibleCount);
  const hasMore = articles.length > visibleCount;

  return (
    <div className={compact ? '' : 'panel'}>
      {!compact && (
        <div className="panel-header">
          <span className="text-xs font-medium text-terminal-dim uppercase tracking-wider">
            Latest News
          </span>
          <span className="text-2xs text-terminal-muted font-mono">
            {articles.length} articles
          </span>
        </div>
      )}
      <div className={compact ? '' : 'panel-body'}>
        <div className="space-y-0">
          {visible.map((article) => (
            <a
              key={article.id}
              href={article.url}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-start gap-3 py-2.5 px-1 -mx-1 rounded
                         hover:bg-terminal-border/20 transition-colors group
                         border-b border-terminal-border/30 last:border-0"
            >
              <div className="flex-1 min-w-0">
                {/* Title */}
                <p className="text-sm text-terminal-text leading-snug group-hover:text-terminal-accent transition-colors line-clamp-2">
                  {article.title}
                </p>

                {/* Meta row */}
                <div className="flex items-center gap-2 mt-1 flex-wrap">
                  {/* Source */}
                  {article.source_name && (
                    <span className="text-2xs text-terminal-muted">
                      {article.source_name}
                    </span>
                  )}

                  {/* Provider badge */}
                  {article.provider && (
                    <span className="text-2xs px-1.5 py-0.5 rounded bg-terminal-border/50 text-terminal-dim">
                      {article.provider}
                    </span>
                  )}

                  {/* Symbols */}
                  {showSymbols && article.symbols?.length > 0 && (
                    <div className="flex items-center gap-1">
                      {article.symbols.slice(0, 3).map((sym) => (
                        <span key={sym} className="ticker-badge text-2xs">
                          {sym}
                        </span>
                      ))}
                      {article.symbols.length > 3 && (
                        <span className="text-2xs text-terminal-muted">
                          +{article.symbols.length - 3}
                        </span>
                      )}
                    </div>
                  )}
                </div>
              </div>

              {/* Time */}
              <span className="text-2xs text-terminal-muted whitespace-nowrap pt-0.5 font-mono">
                {timeAgo(article.published_at)}
              </span>
            </a>
          ))}
        </div>

        {/* Show more */}
        {hasMore && (
          <button
            onClick={(e) => {
              e.preventDefault();
              setVisibleCount((c) => c + maxItems);
            }}
            className="mt-3 w-full text-center text-xs text-terminal-accent hover:text-terminal-accent/80 transition-colors py-1.5"
          >
            Show more ({articles.length - visibleCount} remaining)
          </button>
        )}
      </div>
    </div>
  );
}