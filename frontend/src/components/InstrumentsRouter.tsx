import { useState, useEffect } from 'react';
import InstrumentList from './InstrumentList';
import InstrumentDetail from './InstrumentDetail';

export default function InstrumentsRouter() {
  const [symbol, setSymbol] = useState<string | null>(null);

  useEffect(() => {
    function parseSymbol() {
      const path = window.location.pathname;
      const match = path.match(/^\/app\/instruments\/(.+)$/);
      if (match && match[1]) {
        setSymbol(decodeURIComponent(match[1]).toUpperCase());
      } else {
        setSymbol(null);
      }
    }

    parseSymbol();
    window.addEventListener('popstate', parseSymbol);
    return () => window.removeEventListener('popstate', parseSymbol);
  }, []);

  if (symbol) {
    return <InstrumentDetail symbol={symbol} />;
  }

  return <InstrumentList />;
}
