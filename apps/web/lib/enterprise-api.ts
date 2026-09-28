import {
  WatchlistSummary,
  Watchlist,
  WatchlistMarket,
  WatchlistIntelligence,
  SignalEvent,
  RadarEvent,
  RadarPage,
  AlertRule,
  AlertRuleInput,
  AlertEvent,
  AppNotification,
  NotificationsResponse,
  Severity,
  SignalType,
} from './api-types';
import { fetchApi, ApiError } from './api-client';

const API_BASE = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080';

export interface RadarQuery {
  severity?: Severity;
  signal_type?: SignalType;
  market_id?: string;
  watchlist_id?: string;
  limit?: number;
  cursor?: string;
  /** RFC3339 lower bound on observed_at. Omitted entirely when absent. */
  from?: string;
  /** RFC3339 upper bound on observed_at. Omitted entirely when absent. */
  to?: string;
}

async function fetchJson<T>(url: string, options: RequestInit = {}): Promise<T> {
  const fullUrl = new URL(url, API_BASE);
  const response = await fetch(fullUrl.toString(), {
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

// Watchlists
export async function getWatchlists(): Promise<WatchlistSummary[]> {
  return fetchJson<WatchlistSummary[]>('/api/v1/watchlists');
}

export async function createWatchlist(input: { name: string; description?: string | null }): Promise<Watchlist> {
  return fetchJson<Watchlist>('/api/v1/watchlists', {
    method: 'POST',
    body: JSON.stringify(input),
  });
}

export async function updateWatchlist(id: string, input: { name: string; description?: string | null }): Promise<Watchlist> {
  return fetchJson<Watchlist>(`/api/v1/watchlists/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  });
}

export async function deleteWatchlist(id: string): Promise<void> {
  await fetchJson(`/api/v1/watchlists/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export async function addMarketToWatchlist(watchlistId: string, marketId: string): Promise<void> {
  await fetchJson(`/api/v1/watchlists/${encodeURIComponent(watchlistId)}/markets/${encodeURIComponent(marketId)}`, { method: 'POST' });
}

export async function removeMarketFromWatchlist(watchlistId: string, marketId: string): Promise<void> {
  await fetchJson(`/api/v1/watchlists/${encodeURIComponent(watchlistId)}/markets/${encodeURIComponent(marketId)}`, { method: 'DELETE' });
}

export async function getWatchlistIntelligence(watchlistId: string): Promise<WatchlistIntelligence> {
  return fetchJson<WatchlistIntelligence>(`/api/v1/watchlists/${encodeURIComponent(watchlistId)}/intelligence`);
}

// Radar
export async function getRadar(params: RadarQuery = {}): Promise<RadarPage> {
  const searchParams = new URLSearchParams();
  if (params.severity) searchParams.set('severity', params.severity);
  if (params.signal_type) searchParams.set('signal_type', params.signal_type);
  if (params.market_id) searchParams.set('market_id', params.market_id);
  if (params.watchlist_id) searchParams.set('watchlist_id', params.watchlist_id);
  if (params.limit) searchParams.set('limit', String(params.limit));
  if (params.cursor) searchParams.set('cursor', params.cursor);
  if (params.from) searchParams.set('from', params.from);
  if (params.to) searchParams.set('to', params.to);

  const query = searchParams.toString();
  return fetchJson<RadarPage>(`/api/v1/intelligence/radar${query ? `?${query}` : ''}`);
}

// Alert Rules
export async function getAlertRules(): Promise<AlertRule[]> {
  return fetchJson<AlertRule[]>('/api/v1/alert-rules');
}

export async function createAlertRule(input: AlertRuleInput): Promise<AlertRule> {
  return fetchJson<AlertRule>('/api/v1/alert-rules', {
    method: 'POST',
    body: JSON.stringify(input),
  });
}

export async function updateAlertRule(id: string, input: AlertRuleInput): Promise<AlertRule> {
  return fetchJson<AlertRule>(`/api/v1/alert-rules/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  });
}

export async function deleteAlertRule(id: string): Promise<void> {
  await fetchJson(`/api/v1/alert-rules/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export async function getAlertEvents(limit?: number): Promise<AlertEvent[]> {
  const searchParams = new URLSearchParams();
  if (limit) searchParams.set('limit', String(limit));
  const query = searchParams.toString();
  return fetchJson<AlertEvent[]>(`/api/v1/alert-rules/events${query ? `?${query}` : ''}`);
}

// Notifications
export async function getNotifications(limit?: number, unreadOnly?: boolean): Promise<NotificationsResponse> {
  const searchParams = new URLSearchParams();
  if (limit) searchParams.set('limit', String(limit));
  if (unreadOnly) searchParams.set('unread_only', 'true');
  const query = searchParams.toString();
  return fetchJson<NotificationsResponse>(`/api/v1/notifications${query ? `?${query}` : ''}`);
}

export async function getUnreadCount(): Promise<{ unread_count: number }> {
  return fetchJson<{ unread_count: number }>('/api/v1/notifications/unread-count');
}

export async function markNotificationRead(id: number): Promise<void> {
  await fetchJson(`/api/v1/notifications/${encodeURIComponent(String(id))}/read`, { method: 'POST' });
}

export async function markAllNotificationsRead(): Promise<{ updated: number }> {
  return fetchJson<{ updated: number }>('/api/v1/notifications/read-all', { method: 'POST' });
}

// Re-export all enterprise types for consumers
export type {
  WatchlistSummary,
  Watchlist,
  WatchlistMarket,
  WatchlistIntelligence,
  SignalEvent,
  RadarEvent,
  RadarPage,
  AlertRule,
  AlertRuleInput,
  AlertEvent,
  AppNotification,
  NotificationsResponse,
  Severity,
  SignalType,
} from './api-types';
