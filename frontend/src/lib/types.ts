export interface User {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  created_at: string;
  updated_at: string;
}

export interface Instrument {
  id: number;
  symbol: string;
  name: string | null;
  exchange: string | null;
  currency: string | null;
  country: string | null;
  asset_class: string;
  is_active: boolean;
}

export interface InstrumentListItem extends Instrument {
  last_price: number | null;
  market_cap: number | null;
  sector: string | null;
  industry: string | null;
  is_favorite: boolean;
}

export interface InstrumentDetail extends Instrument {
  last_price: number | null;
  market_cap: number | null;
  profile: CompanyProfile | null;
  latest_quote: Quote | null;
}

export interface CompanyProfile {
  market_cap: number | null;
  sector: string | null;
  industry: string | null;
  exchange: string | null;
  country: string | null;
  currency: string | null;
}

export interface PriceBar {
  ts: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
  adj_close?: number;
}

export interface Quote {
  ts: string;
  last_price: number | null;
  bid: number | null;
  ask: number | null;
  volume: number | null;
  source: string | null;
}

export interface FundamentalsRow {
  period_end_date: string;
  calendar_year: number | null;
  period: string | null;
  revenue: number | null;
  gross_profit: number | null;
  operating_income: number | null;
  net_income: number | null;
  eps: number | null;
}

export interface MarketStatus {
  is_open: boolean;
  next_open: string | null;
  next_close: string | null;
  message: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  page: number;
  page_size: number;
  total_count: number;
  total_pages: number;
}

export interface AuthResponse {
  access_token: string;
  user: User;
}

export interface DashboardResponse {
  favorites: InstrumentListItem[];
  market_status: MarketStatus;
}

export interface PriceEvent {
  symbol: string;
  last_price: number;
  bid: number | null;
  ask: number | null;
  volume: number | null;
  ts: string;
}

export interface TimeSeriesPoint {
  ts: number;
  value: number;
}

export interface SymbolEntry {
  symbol: string;
  name: string;
}
