import {
  Market,
  Signal,
  Observation,
  MarketListResponse,
  SignalListResponse,
  MarketDetailResponse,
  HealthResponse,
  MarketQueryParams,
  SignalQueryParams,
} from './api-types';

const API_BASE = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080';

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

async function fetchApi<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const url = new URL(endpoint, API_BASE);
  
  const response = await fetch(url.toString(), {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });

  if (!response.ok) {
    throw new ApiError(response.status, `API error: ${response.statusText}`);
  }

  return response.json();
}

// Health check
export async function getHealth(): Promise<HealthResponse> {
  return fetchApi<HealthResponse>('/health');
}

// Markets
export async function getMarkets(params: MarketQueryParams = {}): Promise<MarketListResponse> {
  const searchParams = new URLSearchParams();
  if (params.cursor) searchParams.set('cursor', params.cursor);
  if (params.limit) searchParams.set('limit', String(params.limit));
  if (params.status) searchParams.set('status', params.status);
  if (params.category) searchParams.set('category', params.category);
  if (params.search) searchParams.set('search', params.search);

  const query = searchParams.toString();
  return fetchApi<MarketListResponse>(`/intelligence/markets${query ? `?${query}` : ''}`);
}

export async function getMarket(id: string): Promise<MarketDetailResponse> {
  return fetchApi<MarketDetailResponse>(`/intelligence/markets/${id}`);
}

// Signals
export async function getSignals(params: SignalQueryParams = {}): Promise<SignalListResponse> {
  const searchParams = new URLSearchParams();
  if (params.cursor) searchParams.set('cursor', params.cursor);
  if (params.limit) searchParams.set('limit', String(params.limit));
  if (params.market_id) searchParams.set('market_id', params.market_id);
  if (params.severity) searchParams.set('severity', params.severity);
  if (params.signal_type) searchParams.set('signal_type', params.signal_type);
  if (params.from) searchParams.set('from', params.from);
  if (params.to) searchParams.set('to', params.to);

  const query = searchParams.toString();
  return fetchApi<SignalListResponse>(`/intelligence/signals${query ? `?${query}` : ''}`);
}

export async function getRecentSignals(limit = 10): Promise<SignalListResponse> {
  return getSignals({ limit });
}

// Market search
export async function searchMarkets(query: string, limit = 20): Promise<MarketListResponse> {
  return getMarkets({ search: query, limit });
}

// Statistics / Summary
export interface SummaryStats {
  marketsTracked: number;
  activeSignals: number;
  significantChanges: number;
  marketsUpdated: string | null;
}

export async function getSummaryStats(): Promise<SummaryStats> {
  try {
    const [markets, signals] = await Promise.all([
      getMarkets({ limit: 1 }),
      getSignals({ limit: 1 }),
    ]);

    // Get significant signals count
    const significantSignals = await getSignals({ severity: 'SIGNIFICANT', limit: 1 });

    return {
      marketsTracked: markets.pagination.total,
      activeSignals: signals.pagination.total,
      significantChanges: significantSignals.pagination.total,
      marketsUpdated: new Date().toISOString(),
    };
  } catch (error) {
    console.error('Failed to fetch summary stats:', error);
    return {
      marketsTracked: 0,
      activeSignals: 0,
      significantChanges: 0,
      marketsUpdated: null,
    };
  }
}