import type {
  AuthResponse,
  Company,
  Favorite,
  FavoritesUpdateRequest,
  LoginRequest,
  MarketStatus,
  NewsArticle,
  PaginatedResponse,
  PriceBar,
  SignupRequest,
  StockFilters,
  User,
} from './types';

// ── Config ──────────────────────────────────────────────────────────

const BASE_URL = import.meta.env.PUBLIC_API_URL || '/api';

// ── Token Storage ───────────────────────────────────────────────────

let accessToken: string | null = null;

export function getAccessToken(): string | null {
  if (accessToken) return accessToken;
  if (typeof window !== 'undefined') {
    accessToken = localStorage.getItem('access_token');
  }
  return accessToken;
}

export function setTokens(access: string, refresh: string): void {
  accessToken = access;
  if (typeof window !== 'undefined') {
    localStorage.setItem('access_token', access);
    localStorage.setItem('refresh_token', refresh);
  }
}

export function clearTokens(): void {
  accessToken = null;
  if (typeof window !== 'undefined') {
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
  }
}

function getRefreshToken(): string | null {
  if (typeof window !== 'undefined') {
    return localStorage.getItem('refresh_token');
  }
  return null;
}

// ── Base Fetch ──────────────────────────────────────────────────────

class ApiError extends Error {
  status: number;
  body: unknown;

  constructor(status: number, message: string, body?: unknown) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.body = body;
  }
}

async function request<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const url = `${BASE_URL}${path}`;
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> || {}),
  };

  const token = getAccessToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(url, {
    ...options,
    headers,
  });

  // Handle 401 — try refresh once
  if (res.status === 401 && token) {
    const refreshed = await tryRefresh();
    if (refreshed) {
      headers['Authorization'] = `Bearer ${getAccessToken()}`;
      const retryRes = await fetch(url, { ...options, headers });
      if (!retryRes.ok) {
        const body = await retryRes.json().catch(() => null);
        throw new ApiError(retryRes.status, body?.error || retryRes.statusText, body);
      }
      return retryRes.json() as Promise<T>;
    }
    // Refresh failed — force logout
    clearTokens();
    if (typeof window !== 'undefined') {
      window.location.href = '/login';
    }
    throw new ApiError(401, 'Session expired');
  }

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new ApiError(res.status, body?.error || res.statusText, body);
  }

  // Handle 204 No Content
  if (res.status === 204) {
    return undefined as T;
  }

  return res.json() as Promise<T>;
}

async function tryRefresh(): Promise<boolean> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) return false;

  try {
    const res = await fetch(`${BASE_URL}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });

    if (!res.ok) return false;

    const data = (await res.json()) as AuthResponse;
    setTokens(data.access_token, data.refresh_token);
    return true;
  } catch {
    return false;
  }
}

// ── Auth API ────────────────────────────────────────────────────────

export async function login(req: LoginRequest): Promise<AuthResponse> {
  const data = await request<AuthResponse>('/auth/login', {
    method: 'POST',
    body: JSON.stringify(req),
  });
  setTokens(data.access_token, data.refresh_token);
  return data;
}

export async function signup(req: SignupRequest): Promise<AuthResponse> {
  const data = await request<AuthResponse>('/auth/signup', {
    method: 'POST',
    body: JSON.stringify(req),
  });
  setTokens(data.access_token, data.refresh_token);
  return data;
}

export async function logout(): Promise<void> {
  try {
    await request<void>('/auth/logout', { method: 'POST' });
  } finally {
    clearTokens();
  }
}

export async function getMe(): Promise<User> {
  return request<User>('/auth/me');
}

// ── Stocks API ──────────────────────────────────────────────────────

export async function getStocks(
  filters: StockFilters = {},
): Promise<PaginatedResponse<Company>> {
  const params = new URLSearchParams();
  if (filters.search) params.set('search', filters.search);
  if (filters.exchange) params.set('exchange', filters.exchange);
  if (filters.sector) params.set('sector', filters.sector);
  if (filters.industry) params.set('industry', filters.industry);
  if (filters.page) params.set('page', String(filters.page));
  if (filters.pageSize) params.set('pageSize', String(filters.pageSize));

  const qs = params.toString();
  return request<PaginatedResponse<Company>>(`/stocks${qs ? `?${qs}` : ''}`);
}

export async function getStock(symbol: string): Promise<Company> {
  return request<Company>(`/stocks/${encodeURIComponent(symbol)}`);
}

export async function getStockPrices(
  symbol: string,
  interval: '1min' | '1d',
  from?: string,
  to?: string,
  limit?: number,
): Promise<PriceBar[]> {
  const params = new URLSearchParams({ interval });
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  if (limit) params.set('limit', String(limit));

  return request<PriceBar[]>(
    `/stocks/${encodeURIComponent(symbol)}/prices?${params.toString()}`,
  );
}

export async function getStockNews(
  symbol: string,
  from?: string,
  to?: string,
  limit?: number,
): Promise<NewsArticle[]> {
  const params = new URLSearchParams();
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  if (limit) params.set('limit', String(limit));

  const qs = params.toString();
  return request<NewsArticle[]>(
    `/stocks/${encodeURIComponent(symbol)}/news${qs ? `?${qs}` : ''}`,
  );
}

// ── Market Status API ───────────────────────────────────────────────

export async function getMarketStatus(): Promise<MarketStatus> {
  return request<MarketStatus>('/market/status');
}

// ── Favorites API ───────────────────────────────────────────────────

export async function getFavorites(): Promise<Favorite[]> {
  return request<Favorite[]>('/favorites');
}

export async function updateFavorites(
  req: FavoritesUpdateRequest,
): Promise<Favorite[]> {
  return request<Favorite[]>('/favorites', {
    method: 'PUT',
    body: JSON.stringify(req),
  });
}

// ── SSE Helpers ─────────────────────────────────────────────────────

export function createPriceStream(
  symbol: string,
  onEvent: (data: unknown) => void,
  onError?: (err: Event) => void,
): EventSource {
  const token = getAccessToken();
  const url = `${BASE_URL}/stream/stocks/${encodeURIComponent(symbol)}${
    token ? `?token=${encodeURIComponent(token)}` : ''
  }`;

  const es = new EventSource(url);

  es.addEventListener('price', (e) => {
    try {
      onEvent(JSON.parse(e.data));
    } catch { /* ignore parse errors */ }
  });

  es.addEventListener('market_status', (e) => {
    try {
      onEvent(JSON.parse(e.data));
    } catch { /* ignore */ }
  });

  es.addEventListener('heartbeat', (e) => {
    try {
      onEvent(JSON.parse(e.data));
    } catch { /* ignore */ }
  });

  if (onError) {
    es.onerror = onError;
  }

  return es;
}

export function createFavoritesStream(
  onEvent: (data: unknown) => void,
  onError?: (err: Event) => void,
): EventSource {
  const token = getAccessToken();
  const url = `${BASE_URL}/stream/favorites${
    token ? `?token=${encodeURIComponent(token)}` : ''
  }`;

  const es = new EventSource(url);

  es.addEventListener('price', (e) => {
    try {
      onEvent(JSON.parse(e.data));
    } catch { /* ignore */ }
  });

  es.addEventListener('market_status', (e) => {
    try {
      onEvent(JSON.parse(e.data));
    } catch { /* ignore */ }
  });

  es.addEventListener('heartbeat', (e) => {
    try {
      onEvent(JSON.parse(e.data));
    } catch { /* ignore */ }
  });

  if (onError) {
    es.onerror = onError;
  }

  return es;
}

// ── Auth State Check ────────────────────────────────────────────────

export function isAuthenticated(): boolean {
  return !!getAccessToken();
}