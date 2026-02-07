// ── Company ─────────────────────────────────────────────────────────

export interface Company {
  id: number;
  symbol: string;
  name: string;
  exchange: string;
  sector: string;
  industry: string;
  market_cap: number;
  week52_high: number;
  week52_low: number;
  prev_close: number;
  todays_high: number;
  todays_low: number;
  volume: number;
  updated_at: string;
}

// ── Price Bars ──────────────────────────────────────────────────────

export interface PriceBar {
  ts: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

/** Lightweight point for sparklines and WASM downsampling */
export interface PricePoint {
  time: number; // unix seconds
  value: number;
}

/** OHLC point for candlestick charts */
export interface OHLCPoint {
  time: number; // unix seconds
  open: number;
  high: number;
  low: number;
  close: number;
}

// ── News ────────────────────────────────────────────────────────────

export interface NewsArticle {
  id: string;
  provider: string;
  source_name: string;
  title: string;
  summary: string;
  url: string;
  published_at: string;
  symbols: string[];
}

// ── Auth ────────────────────────────────────────────────────────────

export interface User {
  id: number;
  email: string;
  first_name: string;
  last_name: string;
  created_at: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface SignupRequest {
  email: string;
  password: string;
  first_name: string;
  last_name: string;
  referral_code: string;
}

export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  user: User;
}

// ── Favorites ───────────────────────────────────────────────────────

export interface Favorite {
  company_id: number;
  symbol: string;
  name: string;
  added_at: string;
}

export interface FavoritesUpdateRequest {
  company_ids: number[];
}

// ── Market Status ───────────────────────────────────────────────────

export interface MarketStatus {
  is_open: boolean;
  current_time: string;
  next_open: string;
  next_close: string;
  message: string;
}

// ── API Responses ───────────────────────────────────────────────────

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface ApiError {
  error: string;
  message?: string;
}

// ── SSE Events ──────────────────────────────────────────────────────

export interface SSEPriceEvent {
  type: 'price';
  symbol: string;
  ts: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

export interface SSEMarketStatusEvent {
  type: 'market_status';
  is_open: boolean;
  message: string;
}

export interface SSEHeartbeatEvent {
  type: 'heartbeat';
  ts: string;
}

export type SSEEvent = SSEPriceEvent | SSEMarketStatusEvent | SSEHeartbeatEvent;

// ── Stock List Filters ──────────────────────────────────────────────

export interface StockFilters {
  search?: string;
  exchange?: string;
  sector?: string;
  industry?: string;
  page?: number;
  pageSize?: number;
}