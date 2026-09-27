// Market Studio types.
//
// The draft is in Prophet's own vocabulary. Panta field names never leak into
// the UI: the gateway translates. All money fields are strings because Panta
// specifies USDC base units as integer strings and we never parse them as
// floats.

export const PANTA_CATEGORIES = [
  'sports',
  'crypto',
  'politics',
  'entertainment',
  'finance',
  'science',
  'world',
  'other',
] as const;

export type PantaCategory = (typeof PANTA_CATEGORIES)[number];

export type MarketType = 'standard' | 'breaking';

/** A user-editable market draft. Every field is reviewed before use. */
export interface MarketDraft {
  question: string;
  resolution_criteria: string;
  sources_of_truth: string[];
  category: string;
  resolution_date: string;
  end_date?: string;
  start_date?: string;
  image_url: string;
  title?: string;
  description?: string;
  region?: string;
  market_type?: MarketType;
  outcome_yes?: string;
  outcome_no?: string;
  notes?: string;
  /**
   * Set only when a human explicitly confirms the resolution source. The AI
   * must never set this on its own.
   */
  resolution_source_confirmed: boolean;
}

export type ValidationSeverity = 'error' | 'warning';

export interface ValidationIssue {
  field: string;
  code: string;
  message: string;
  severity: ValidationSeverity;
}

export interface ValidationReport {
  valid: boolean;
  issues: ValidationIssue[] | null;
  missing_fields: string[] | null;
  needs_clarification: boolean;
  draft_fingerprint?: string;
}

export interface InterpretResponse {
  draft: MarketDraft | null;
  needs_clarification: boolean;
  missing_fields: string[] | null;
  clarification: string | null;
  validation: ValidationReport | null;
  used_fallback: boolean;
}

/** Panta quote, passed through in Panta's own camelCase wire format. */
export interface CreateQuote {
  createId: string;
  expectedEventPda: string;
  /** USDC base units as an integer string, e.g. "50000000" = 50 USDC. */
  paymentUsdc: string;
  liquidityInjectionUsdc: string;
  platformRevenueUsdc: string;
  marketType: string;
  expiresAt: string;
  blockhashExpiryHintSec: number | null;
}

export interface CreateBuild {
  transaction: string;
  recentBlockhash: string;
  lastValidBlockHeight: number | null;
  blockhashExpiryHintSec: number | null;
  buildFingerprint: string;
  paymentUsdc: string;
  liquidityInjectionUsdc: string;
  platformRevenueUsdc: string;
  marketType: string;
  derived?: Record<string, unknown>;
  expiresAt: string;
}

/**
 * The full lifecycle of a creation attempt.
 *
 * Note the ordering: a Solana signature is NOT a created market. Only
 * REGISTERED and INDEXED mean a market exists.
 */
export type CreationStatus =
  | 'CREATED'
  | 'SIGNED'
  | 'BROADCASTING'
  | 'SUBMITTED'
  | 'CONFIRMING'
  | 'CONFIRMED'
  | 'REGISTERING'
  | 'REGISTERED'
  | 'INDEXED'
  | 'FAILED'
  | 'UNKNOWN';

/** Statuses at or beyond Solana confirmation. */
export const CONFIRMED_STATUSES: CreationStatus[] = [
  'CONFIRMED',
  'REGISTERING',
  'REGISTERED',
  'INDEXED',
];

/** Statuses that mean an authoritative market record exists at Panta. */
export const MARKET_EXISTS_STATUSES: CreationStatus[] = ['REGISTERED', 'INDEXED'];

export interface CreationAttempt {
  id: string;
  wallet_address: string;
  status: CreationStatus;
  create_id?: string;
  expected_event_pda?: string;
  payment_usdc?: string;
  liquidity_injection_usdc?: string;
  platform_revenue_usdc?: string;
  market_type?: string;
  draft_hash: string;
  solana_signature?: string;
  market_id?: string;
  error_code?: string;
  error_message?: string;
  /** Server-rendered status text. Display this rather than composing your own. */
  message: string;
  market_exists: boolean;
}

export interface QuoteResponse {
  attempt: CreationAttempt;
  quote: CreateQuote;
  validation: ValidationReport;
}

export interface BuildResponse {
  attempt: CreationAttempt;
  build: CreateBuild;
}

export interface BroadcastResponse {
  attempt: CreationAttempt;
  result: {
    signature: string;
    submit_status: string;
    confirm_status: string;
    confirmed: boolean;
    confirmations: number;
  };
}

export interface RegisterResponse {
  registered: boolean;
  market_id: string;
  message: string;
  attempt: CreationAttempt;
}

/** Error codes. The UI switches on these rather than parsing prose. */
export type MarketStudioErrorCode =
  | 'AI_INTERPRETATION_FAILED'
  | 'INVALID_DRAFT'
  | 'AMBIGUOUS_MARKET'
  | 'QUOTE_FAILED'
  | 'BUILD_FAILED'
  | 'WALLET_NOT_CONNECTED'
  | 'USER_REJECTED'
  | 'INVALID_TRANSACTION'
  | 'BROADCAST_FAILED'
  | 'CONFIRMATION_TIMEOUT'
  | 'PANTA_REGISTRATION_FAILED'
  | 'INDEXING_PENDING';

export interface MarketStudioErrorBody {
  error: string;
  detail?: string;
  /** Present whenever an attempt exists, so the true state can be shown. */
  attempt?: CreationAttempt;
}
