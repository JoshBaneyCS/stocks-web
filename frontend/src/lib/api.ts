import { useAuthStore } from './store';
import type {
  AuthResponse,
  DashboardResponse,
  FundamentalsRow,
  InstrumentDetail,
  InstrumentListItem,
  CompanyProfile,
  MarketStatus,
  PaginatedResponse,
  PriceBar,
  User,
} from './types';

class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

async function fetchWithAuth(
  url: string,
  options: RequestInit = {}
): Promise<Response> {
  const token = useAuthStore.getState().accessToken;
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  let response = await fetch(url, { ...options, headers });

  // If 401, attempt token refresh and retry once
  if (response.status === 401 && token) {
    const refreshed = await attemptRefresh();
    if (refreshed) {
      const newToken = useAuthStore.getState().accessToken;
      headers['Authorization'] = `Bearer ${newToken}`;
      response = await fetch(url, { ...options, headers });
    }
  }

  return response;
}

async function attemptRefresh(): Promise<boolean> {
  try {
    const response = await fetch('/api/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
    });
    if (response.ok) {
      const data = await response.json();
      useAuthStore.getState().setToken(data.access_token);
      return true;
    }
  } catch {
    // refresh failed
  }
  useAuthStore.getState().logout();
  return false;
}

async function apiGet<T>(url: string): Promise<T> {
  const response = await fetchWithAuth(url);
  if (!response.ok) {
    throw new ApiError(`GET ${url} failed: ${response.statusText}`, response.status);
  }
  return response.json();
}

async function apiPost<T>(url: string, body?: unknown): Promise<T> {
  const response = await fetchWithAuth(url, {
    method: 'POST',
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    const errorData = await response.json().catch(() => ({}));
    throw new ApiError(
      (errorData as Record<string, string>).error || `POST ${url} failed: ${response.statusText}`,
      response.status
    );
  }
  return response.json();
}

async function apiPut<T>(url: string, body?: unknown): Promise<T> {
  const response = await fetchWithAuth(url, {
    method: 'PUT',
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    throw new ApiError(`PUT ${url} failed: ${response.statusText}`, response.status);
  }
  return response.json();
}

// Auth endpoints
export async function login(email: string, password: string): Promise<AuthResponse> {
  return apiPost<AuthResponse>('/api/auth/login', { email, password });
}

export async function signup(
  email: string,
  password: string,
  firstName: string,
  lastName: string,
  referralCode?: string
): Promise<AuthResponse> {
  return apiPost<AuthResponse>('/api/auth/signup', {
    email,
    password,
    first_name: firstName,
    last_name: lastName,
    referral_code: referralCode || undefined,
  });
}

export async function logout(): Promise<void> {
  await fetchWithAuth('/api/auth/logout', { method: 'POST' });
}

export async function getMe(): Promise<User> {
  return apiGet<User>('/api/auth/me');
}

export async function refresh(): Promise<{ access_token: string }> {
  return apiPost<{ access_token: string }>('/api/auth/refresh');
}

// Instrument endpoints
export interface InstrumentSearchParams {
  search?: string;
  asset_class?: string;
  exchange?: string;
  country?: string;
  page?: number;
  page_size?: number;
}

export async function getInstruments(
  params: InstrumentSearchParams = {}
): Promise<PaginatedResponse<InstrumentListItem>> {
  const searchParams = new URLSearchParams();
  if (params.search) searchParams.set('search', params.search);
  if (params.asset_class) searchParams.set('asset_class', params.asset_class);
  if (params.exchange) searchParams.set('exchange', params.exchange);
  if (params.country) searchParams.set('country', params.country);
  if (params.page) searchParams.set('page', String(params.page));
  if (params.page_size) searchParams.set('page_size', String(params.page_size));

  const qs = searchParams.toString();
  return apiGet<PaginatedResponse<InstrumentListItem>>(
    `/api/instruments${qs ? `?${qs}` : ''}`
  );
}

export async function getInstrumentDetail(symbol: string): Promise<InstrumentDetail> {
  return apiGet<InstrumentDetail>(`/api/instruments/${encodeURIComponent(symbol)}`);
}

export async function getProfile(symbol: string): Promise<CompanyProfile> {
  return apiGet<CompanyProfile>(
    `/api/instruments/${encodeURIComponent(symbol)}/profile`
  );
}

export async function getFundamentals(
  symbol: string,
  limit = 40
): Promise<FundamentalsRow[]> {
  return apiGet<FundamentalsRow[]>(
    `/api/instruments/${encodeURIComponent(symbol)}/fundamentals?limit=${limit}`
  );
}

export interface PriceParams {
  interval?: '1d' | '1h' | '1min';
  from?: string;
  to?: string;
  limit?: number;
}

export async function getPrices(
  symbol: string,
  params: PriceParams = {}
): Promise<PriceBar[]> {
  const searchParams = new URLSearchParams();
  if (params.interval) searchParams.set('interval', params.interval);
  if (params.from) searchParams.set('from', params.from);
  if (params.to) searchParams.set('to', params.to);
  if (params.limit) searchParams.set('limit', String(params.limit));

  const qs = searchParams.toString();
  return apiGet<PriceBar[]>(
    `/api/instruments/${encodeURIComponent(symbol)}/prices${qs ? `?${qs}` : ''}`
  );
}

// Favorites endpoints
export async function getFavorites(): Promise<InstrumentListItem[]> {
  return apiGet<InstrumentListItem[]>('/api/favorites');
}

export async function updateFavorites(
  instrumentIds: number[]
): Promise<{ message: string }> {
  return apiPut<{ message: string }>('/api/favorites', {
    instrument_ids: instrumentIds,
  });
}

// Market endpoints
export async function getMarketStatus(): Promise<MarketStatus> {
  return apiGet<MarketStatus>('/api/market/status');
}

export async function getDashboard(): Promise<DashboardResponse> {
  return apiGet<DashboardResponse>('/api/dashboard');
}

// SSE helper
export function createEventSource(url: string): EventSource {
  const token = useAuthStore.getState().accessToken;
  const separator = url.includes('?') ? '&' : '?';
  const fullUrl = token ? `${url}${separator}token=${encodeURIComponent(token)}` : url;
  return new EventSource(fullUrl);
}
