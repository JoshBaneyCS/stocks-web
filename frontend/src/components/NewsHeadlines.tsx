export default function NewsHeadlines() {
  return (
    <div className="terminal-panel p-4">
      <h3 className="text-sm font-medium text-terminal-muted uppercase tracking-wider mb-3">
        News Headlines
      </h3>
      <div className="flex flex-col items-center justify-center py-8">
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          className="w-10 h-10 text-terminal-muted/40 mb-3"
        >
          <path d="M4 22h16a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2H8a2 2 0 0 0-2 2v16a2 2 0 0 1-2 2Zm0 0a2 2 0 0 1-2-2v-9c0-1.1.9-2 2-2h2" />
          <path d="M18 14h-8" />
          <path d="M15 18h-5" />
          <path d="M10 6h8v4h-8V6Z" />
        </svg>
        <p className="text-sm text-terminal-muted">News coming soon</p>
        <p className="text-xs text-terminal-muted/60 mt-1">
          Real-time market news and analysis
        </p>
      </div>
    </div>
  );
}
