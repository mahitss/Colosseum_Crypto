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