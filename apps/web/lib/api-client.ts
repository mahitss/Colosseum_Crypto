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
import type {
  BuildResponse,
  BroadcastResponse,
  CreationAttempt,
  InterpretResponse,
  MarketDraft,
  MarketStudioErrorBody,
  MarketStudioErrorCode,
  QuoteResponse,
  RegisterResponse,
  ValidationReport,
} from './market-studio-types';

const API_BASE = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080';

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

/**
 * A Market Studio failure.
 *
 * `attempt` is the important part: when a step fails after an attempt already
 * exists, the gateway returns the authoritative state with it. Discarding that
 * would make the UI guess, and guessing "failed" when Solana actually
 * confirmed is the specific error this feature must never make.
 */
class MarketStudioError extends Error {
  constructor(
    public code: MarketStudioErrorCode | 'NETWORK_UNAVAILABLE' | 'INVALID_RESPONSE' | 'UNKNOWN',
    message: string,
    public attempt?: CreationAttempt,
    public cause?: unknown
  ) {
    super(message);
    this.name = 'MarketStudioError';
  }

  /** Solana settled the transaction, even if a later step failed. */
  get confirmedOnChain(): boolean {
    if (!this.attempt) return false;
    return ['CONFIRMED', 'REGISTERING', 'REGISTERED', 'INDEXED'].includes(
      this.attempt.status
    );
  }

  /** The wording to show. Prefers the server's own message. */
  get userMessage(): string {
    if (this.confirmedOnChain && !this.attempt?.market_exists) {
      return 'Transaction confirmed on Solana. Panta registration is pending.';
    }
    if (this.attempt?.message) return this.attempt.message;
    return this.message;
  }
}

export { ApiError, MarketStudioError };

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

// Market Studio
//
// These helpers are deliberately not built on fetchApi. A failed Market Studio
// step is not simply an HTTP error: the gateway attaches the creation attempt
// so the UI can show the real state. A confirmed Solana transaction whose Panta
// registration failed arrives as a 502 with a CONFIRMED attempt attached, and
// flattening that into a thrown string would push the UI toward saying
// "creation failed" — which would be false.

async function fetchMarketStudio<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const url = new URL(endpoint, API_BASE);

  let response: Response;
  try {
    response = await fetch(url.toString(), {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    });
  } catch (error) {
    throw new MarketStudioError('NETWORK_UNAVAILABLE', 'Could not reach the gateway.', undefined, error);
  }

  const text = await response.text();
  let body: MarketStudioErrorBody | T | null = null;
  if (text) {
    try {
      body = JSON.parse(text) as MarketStudioErrorBody | T;
    } catch {
      // A non-JSON body means the request never reached our handlers.
      throw new MarketStudioError(
        'INVALID_RESPONSE',
        `Unexpected response from the gateway (${response.status}).`,
        undefined
      );
    }
  }

  if (!response.ok) {
    const failure = (body ?? {}) as MarketStudioErrorBody;
    throw new MarketStudioError(
      // The gateway is the authority on error codes. An unrecognised code
      // still gets surfaced rather than being swallowed.
      (failure.error || 'UNKNOWN') as MarketStudioError['code'],
      failure.detail || `Request failed (${response.status}).`,
      failure.attempt
    );
  }

  return body as T;
}

/**
 * Turn a user description into a draft proposal, or ask for clarification.
 *
 * This never creates a market. The response must be shown to the user first.
 */
export async function interpretMarket(
  prompt: string
): Promise<InterpretResponse> {
  return fetchMarketStudio<InterpretResponse>('/api/v1/market-studio/interpret', {
    method: 'POST',
    body: JSON.stringify({ prompt }),
  });
}

/** The deterministic gate. The Create action is only enabled when this passes. */
export async function validateMarketDraft(
  draft: MarketDraft
): Promise<ValidationReport> {
  return fetchMarketStudio<ValidationReport>('/api/v1/market-studio/validate', {
    method: 'POST',
    body: JSON.stringify({ draft }),
  });
}

/** Ask Panta for the real fee quote. Nothing is created and nothing is charged. */
export async function quoteMarket(
  draft: MarketDraft,
  walletAddress: string
): Promise<QuoteResponse> {
  return fetchMarketStudio<QuoteResponse>('/api/v1/market-studio/quote', {
    method: 'POST',
    body: JSON.stringify({ draft, wallet_address: walletAddress }),
  });
}

/**
 * Build an unsigned transaction.
 *
 * The transaction is signed in the user's wallet, never on the server. The
 * returned draft hash pins the content that was reviewed.
 */
export async function buildMarket(
  attemptId: string,
  walletAddress: string
): Promise<BuildResponse> {
  return fetchMarketStudio<BuildResponse>('/api/v1/market-studio/build', {
    method: 'POST',
    body: JSON.stringify({
      creation_attempt_id: attemptId,
      wallet_address: walletAddress,
    }),
  });
}

/** Relay the wallet-signed transaction and wait for Solana confirmation. */
export async function broadcastMarket(
  attemptId: string,
  signedTransaction: string,
  walletAddress: string,
  draftHash: string
): Promise<BroadcastResponse> {
  return fetchMarketStudio<BroadcastResponse>('/api/v1/market-studio/broadcast', {
    method: 'POST',
    body: JSON.stringify({
      creation_attempt_id: attemptId,
      signed_transaction: signedTransaction,
      wallet_address: walletAddress,
      draft_hash: draftHash,
    }),
  });
}

/**
 * Register with Panta. This is the only step that makes a market exist.
 *
 * Idempotent: repeating the same createId and signature is safe.
 */
export async function registerMarket(
  attemptId: string
): Promise<RegisterResponse> {
  return fetchMarketStudio<RegisterResponse>('/api/v1/market-studio/register', {
    method: 'POST',
    body: JSON.stringify({ creation_attempt_id: attemptId }),
  });
}

/** Poll the authoritative attempt state, including after a page reload. */
export async function getMarketCreationAttempt(
  attemptId: string
): Promise<CreationAttempt> {
  return fetchMarketStudio<CreationAttempt>(
    `/api/v1/market-studio/attempts/${encodeURIComponent(attemptId)}`
  );
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