// Shared types for Prophet API
// These mirror the Gateway's internal models

export interface Market {
  id: string;
  source: string;
  source_market_id: string;
  title: string;
  description: string | null;
  category: string | null;
  status: string;
  phase: string;
  yes_probability: string | null;
  no_probability: string | null;
  liquidity: string | null;
  volume_usdc: string | null;
  created_at: string;
  closes_at: string | null;
  resolution_status: string | null;
  updated_at?: string;
}

export interface Observation {
  id: number;
  market_id: string;
  observed_at: string;
  yes_probability: string | null;
  no_probability: string | null;
  volume_usdc: string | null;
  liquidity: string | null;
}

export interface Signal {
  id: number;
  market_id: string;
  observation_id: number;
  timestamp: string;
  signal_type: string;
  severity: 'INFO' | 'WATCH' | 'SIGNIFICANT' | 'CRITICAL';
  metric: string;
  previous_value: string | null;
  current_value: string | null;
  absolute_change: string | null;
  percentage_change: string | null;
  percentage_points: string | null;
  observation_window: string;
  source: string;
}

export interface MarketWithSignals extends Market {
  signals: Signal[];
  observations: Observation[];
}

// API Response types
export interface PaginatedResponse<T> {
  data: T[];
  pagination: {
    cursor: string | null;
    has_more: boolean;
    total: number;
  };
}

export interface MarketListResponse {
  markets: Market[];
  pagination: {
    cursor: string | null;
    has_more: boolean;
    total: number;
  };
}

export interface SignalListResponse {
  signals: Signal[];
  pagination: {
    cursor: string | null;
    has_more: boolean;
    total: number;
  };
}

export interface MarketDetailResponse {
  market: Market;
  observations: Observation[];
  signals: Signal[];
}

export interface HealthResponse {
  status: 'operational' | 'degraded' | 'unknown';
  timestamp: string;
  components?: Record<string, { status: string }>;
}

// Query params
export interface MarketQueryParams {
  cursor?: string;
  limit?: number;
  status?: string;
  category?: string;
  search?: string;
}

export interface SignalQueryParams {
  cursor?: string;
  limit?: number;
  market_id?: string;
  severity?: string;
  signal_type?: string;
  from?: string;
  to?: string;
}

// --- TASK 008 Enterprise Types ---

export type Severity = 'INFO' | 'WATCH' | 'SIGNIFICANT' | 'CRITICAL';

export type SignalType = 
  | 'NEW_MARKET' 
  | 'PROBABILITY_SHIFT' 
  | 'ACTIVITY_CHANGE' 
  | 'LIQUIDITY_CHANGE' 
  | 'MARKET_MOVEMENT';

export interface WatchlistSummary {
  id: string;
  name: string;
  description: string | null;
  created_at: string;
  updated_at: string;
  market_count: number;
  latest_signal_severity: Severity | null;
  latest_signal_at: string | null;
}

export interface Watchlist {
  id: string;
  name: string;
  description: string | null;
  created_at: string;
  updated_at: string;
}

export interface WatchlistMarket {
  id: string;
  title: string;
  category: string | null;
  status: string;
  yes_probability: string | null;
  no_probability: string | null;
  closes_at: string | null;
  latest_signal: SignalEvent | null;
  latest_observation: { id: number; observed_at: string; yes_probability: string | null } | null;
}

export interface WatchlistIntelligence {
  watchlist: Watchlist;
  market_count: number;
  markets: WatchlistMarket[];
  recent_signals: SignalEvent[];
  severity_distribution: Record<Severity, number>;
  latest_update: string | null;
}

export interface SignalEvent {
  id: number;
  market_id: string;
  source_market_id: string;
  signal_type: SignalType;
  severity: Severity;
  metric: string;
  previous_value: string | null;
  current_value: string | null;
  absolute_change: string | null;
  percentage_change: string | null;
  percentage_points: string | null;
  observation_window: string;
  observation_id: number;
  observed_at: string;
  fingerprint: string;
  source: string;
  created_at: string;
}

export interface RadarEvent extends SignalEvent {
  market_summary: { id: string; title: string; category: string | null; yes_probability: string | null; no_probability: string | null; status: string };
  explanation: string;
  deep_link: string;
}

export interface RadarPage {
  events: RadarEvent[];
  next_cursor: string | null;
  total: number;
}

export interface AlertRule {
  id: string;
  watchlist_id: string | null;
  market_id: string | null;
  name: string;
  enabled: boolean;
  signal_type: SignalType | null;
  minimum_severity: Severity | null;
  probability_change_threshold: string | null;
  activity_change_threshold: string | null;
  liquidity_change_threshold: string | null;
  cooldown_seconds: number;
  created_at: string;
  updated_at: string;
}

export interface AlertRuleInput {
  name: string;
  enabled?: boolean;
  watchlist_id?: string | null;
  market_id?: string | null;
  signal_type?: SignalType | null;
  minimum_severity?: Severity | null;
  probability_change_threshold?: string | null;
  activity_change_threshold?: string | null;
  liquidity_change_threshold?: string | null;
  cooldown_seconds?: number;
}

export interface AlertEvent {
  id: number;
  alert_rule_id: string;
  signal_event_id: number;
  market_id: string;
  status: 'PENDING' | 'DELIVERED' | 'SUPPRESSED' | 'FAILED';
  triggered_at: string;
  delivered_at: string | null;
}

export interface AppNotification {
  id: number;
  title: string;
  body: string;
  severity: Severity;
  market_id: string | null;
  read_at: string | null;
  created_at: string;
}

export interface NotificationsResponse {
  notifications: AppNotification[];
  next_cursor: string | null;
  total: number;
}
