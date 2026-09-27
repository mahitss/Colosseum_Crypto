// Market Studio wizard logic — pure, no React, no network.
//
// Keeping this separate from the page component is deliberate: the rules that
// decide whether a Create Market action is allowed are the safety-critical part
// of this feature, so they need to be testable on their own.
//
// Run with: node --test scripts/market-studio.test.mjs

import type { CreationAttempt, MarketDraft, ValidationReport } from './market-studio-types';

export type StepId =
  | 'DESCRIBE'
  | 'CLARIFY'
  | 'DRAFT'
  | 'RESOLUTION'
  | 'VALIDATE'
  | 'QUOTE'
  | 'BUILD'
  | 'BROADCAST'
  | 'REGISTER';

// The nine steps, in order. The user may move backwards freely; forward
// movement is gated.
export const STEPS: ReadonlyArray<{ id: StepId; label: string }> = [
  { id: 'DESCRIBE', label: 'Describe' },
  { id: 'CLARIFY', label: 'Clarify' },
  { id: 'DRAFT', label: 'Review Draft' },
  { id: 'RESOLUTION', label: 'Resolution Rules' },
  { id: 'VALIDATE', label: 'Validate' },
  { id: 'QUOTE', label: 'Quote' },
  { id: 'BUILD', label: 'Build & Sign' },
  { id: 'BROADCAST', label: 'Broadcast' },
  { id: 'REGISTER', label: 'Register' },
];

export const STEP_INDEX: Record<StepId, number> = STEPS.reduce(
  (acc, step, index) => {
    acc[step.id] = index;
    return acc;
  },
  {} as Record<StepId, number>
);

export interface CreateActionGates {
  step: string;
  validation: ValidationReport | null;
  reviewed: boolean;
  warningsAcknowledged: boolean;
}

/**
 * Can the user act on this step?
 *
 * The Create Market action appears only at VALIDATE, and only when validation
 * has passed and every warning is visible. A user who has not reached review
 * must not be able to create anything.
 */
export function canShowCreateAction({
  step,
  validation,
  reviewed,
  warningsAcknowledged,
}: CreateActionGates): boolean {
  if (step !== 'VALIDATE') return false;
  if (!validation || !validation.valid) return false;
  // Warnings are never hidden. The action stays locked until the user has
  // actually seen them.
  if (hasWarnings(validation) && !warningsAcknowledged) return false;
  // The user must have reviewed the exact content being quoted.
  return Boolean(reviewed);
}

export function hasWarnings(validation: ValidationReport | null): boolean {
  return (validation?.issues ?? []).some((issue) => issue.severity === 'warning');
}

export function hasErrors(validation: ValidationReport | null): boolean {
  return (validation?.issues ?? []).some((issue) => issue.severity === 'error');
}

/**
 * The assistant asked for clarification, so the wizard is parked at CLARIFY.
 * This is the "Your question needs more detail" path.
 */
export function isAmbiguous(interpretation: { needs_clarification?: boolean } | null): boolean {
  return Boolean(interpretation?.needs_clarification);
}

// ---- Material-change detection ------------------------------------------- //
//
// After the user reaches final review, changing ANY material field must
// invalidate the quote and the built transaction. Presentation-only fields
// (outcome labels, notes) do not change what is being created, so they do not
// invalidate anything.
//
// This mirrors the server's draft fingerprint exactly. The server re-checks it
// on broadcast; this is the client-side early warning, not the authority.

export const MATERIAL_FIELDS = [
  'question',
  'resolution_criteria',
  'sources_of_truth',
  'category',
  'resolution_date',
  'end_date',
  'start_date',
  'image_url',
  'title',
  'description',
  'region',
  'market_type',
  'resolution_source_confirmed',
] as const satisfies ReadonlyArray<keyof MarketDraft>;

export const PRESENTATION_ONLY_FIELDS = ['outcome_yes', 'outcome_no', 'notes'] as const satisfies ReadonlyArray<keyof MarketDraft>;

/** Normalise a value so that formatting noise does not count as a change. */
function normalise(field: string, value: unknown): unknown {
  if (Array.isArray(value)) {
    // Source order is not meaningful, so it must not trigger invalidation.
    return [...value].map((v) => String(v).trim()).sort();
  }
  if (field === 'resolution_source_confirmed') return Boolean(value);
  if (typeof value === 'string') return value.trim();
  return value ?? null;
}

/** Field names whose value differs between two drafts. */
export function changedMaterialFields(
  before: MarketDraft | null | undefined,
  after: MarketDraft | null | undefined
): string[] {
  const changed: string[] = [];
  for (const field of MATERIAL_FIELDS) {
    const a = JSON.stringify(normalise(field, before?.[field]));
    const b = JSON.stringify(normalise(field, after?.[field]));
    if (a !== b) changed.push(field);
  }
  return changed;
}

/**
 * Has a material edit happened since the quote was taken?
 *
 * Returns the list of changed fields so the UI can say exactly what
 * invalidated the quote rather than a vague "something changed".
 */
export function invalidatesQuote(
  quotedDraft: MarketDraft | null,
  currentDraft: MarketDraft | null
): string[] {
  return changedMaterialFields(quotedDraft, currentDraft);
}

// ---- State classification ------------------------------------------------- //

export const TERMINAL_STATUSES = ['FAILED', 'INDEXED'];

/**
 * A signature alone is not success. Only REGISTERED and INDEXED mean a market
 * exists; CONFIRMED means only that Solana settled the transaction.
 */
export function marketExists(attempt: CreationAttempt | null | undefined): boolean {
  return ['REGISTERED', 'INDEXED'].includes(attempt?.status ?? '');
}

export function isConfirmedOnChain(attempt: CreationAttempt | null | undefined): boolean {
  return ['CONFIRMED', 'REGISTERING', 'REGISTERED', 'INDEXED'].includes(
    attempt?.status ?? ''
  );
}

/**
 * What may the UI honestly say right now?
 *
 * The ordering matters. "Transaction confirmed on Solana. Panta registration
 * is pending." must win over any generic failure text, because the
 * transaction really did succeed on chain even though Panta has not indexed
 * it. Reporting this as a failure would be false.
 */
export function statusMessage(attempt: CreationAttempt | null | undefined): string {
  if (!attempt) return '';
  if (attempt.market_exists) {
    return 'Market registered with Panta. It will appear in Prophet once indexing completes.';
  }
  if (isConfirmedOnChain(attempt)) {
    return 'Transaction confirmed on Solana. Panta registration is pending.';
  }
  if (attempt.status === 'FAILED') {
    return attempt.error_message || 'The attempt failed.';
  }
  if (attempt.status === 'UNKNOWN') {
    return 'The outcome is unknown. Check the transaction signature before retrying.';
  }
  return attempt.message || '';
}

/**
 * Retrying a broadcast is only safe when we are certain it did not land.
 * A submitted or confirmed transaction must never be re-sent, or the user
 * could be charged twice.
 */
export function canRetryBroadcast(attempt: CreationAttempt | null | undefined): boolean {
  if (!attempt) return false;
  if (attempt.solana_signature) return false;
  return ['FAILED', 'CREATED', 'UNKNOWN'].includes(attempt.status);
}

/** Registration may be retried safely: it is idempotent on createId+signature. */
export function canRetryRegistration(attempt: CreationAttempt | null | undefined): boolean {
  return isConfirmedOnChain(attempt) && !marketExists(attempt);
}

// ---- Amount formatting ---------------------------------------------------- //

/**
 * Render USDC base units for display without ever converting through a float.
 *
 * These strings come from Panta exactly as they were sent. Splitting on the
 * decimal point is a pure string operation, so the value shown is the value
 * that will be charged.
 */
export function formatBaseUnits(value: string | null | undefined): string {
  if (typeof value !== 'string' || !/^\d+$/.test(value)) return '—';
  const padded = value.padStart(7, '0');
  const whole = padded.slice(0, -6);
  const fraction = padded.slice(-6).replace(/0+$/, '');
  return fraction ? `${whole}.${fraction}` : whole;
}

/** Reject anything that is not an exact integer base-unit string. */
export function isValidBaseUnits(value: unknown): value is string {
  return typeof value === 'string' && /^\d+$/.test(value);
}
