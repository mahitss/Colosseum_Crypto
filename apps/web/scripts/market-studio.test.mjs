// Market Studio wizard logic tests — pure logic, no network, no wallet.
// Run with: node --test scripts/market-studio.test.mjs
//
// The TS source in lib/market-studio.ts is mirrored here because there is no TS
// runner in the web app (see package.json — only next/eslint). If you change one,
// change the other; the assertions below encode the safety rules, not the syntax.
import { test } from 'node:test';
import assert from 'node:assert/strict';

import {
  STEPS,
  MATERIAL_FIELDS,
  canShowCreateAction,
  hasWarnings,
  hasErrors,
  isAmbiguous,
  changedMaterialFields,
  invalidatesQuote,
  marketExists,
  isConfirmedOnChain,
  statusMessage,
  canRetryBroadcast,
  canRetryRegistration,
  formatBaseUnits,
  isValidBaseUnits,
} from '../lib/market-studio.ts';

const PASSING = { valid: true, issues: [], missing_fields: [], needs_clarification: false };
const FAILING = {
  valid: false,
  issues: [{ field: 'image_url', code: 'IMAGE_URL_REQUIRED', message: 'Required', severity: 'error' }],
  missing_fields: ['image_url'],
  needs_clarification: false,
};
const PASSING_WITH_WARNING = {
  valid: true,
  issues: [{ field: 'sources_of_truth', code: 'SOURCE_MAYBE_UNVERIFIED', message: 'Check', severity: 'warning' }],
  missing_fields: [],
  needs_clarification: false,
};

const BASE_DRAFT = {
  question: 'Will BTC close above 100000 on 2026-12-31?',
  resolution_criteria: 'Resolves YES if the daily close is above 100000.',
  sources_of_truth: ['https://example.com/a', 'https://example.com/b'],
  category: 'crypto',
  resolution_date: '2026-12-31',
  image_url: 'https://example.com/a.png',
  resolution_source_confirmed: true,
};

// ---- The nine steps ------------------------------------------------------- //

test('the wizard has nine steps in a fixed order', () => {
  assert.equal(STEPS.length, 9);
  assert.equal(new Set(STEPS.map((s) => s.id)).size, 9);
  assert.equal(STEPS[0].id, 'DESCRIBE');
  assert.equal(STEPS[STEPS.length - 1].id, 'REGISTER');
});

// ---- Create Market is gated ---------------------------------------------- //

test('create action is hidden before the user reaches final review', () => {
  for (const step of ['DESCRIBE', 'CLARIFY', 'DRAFT', 'RESOLUTION', 'QUOTE', 'BUILD', 'BROADCAST', 'REGISTER']) {
    assert.equal(
      canShowCreateAction({ step, validation: PASSING, reviewed: true, warningsAcknowledged: true }),
      false,
      `create action must not appear at step ${step}`
    );
  }
});

test('create action is hidden while validation fails', () => {
  assert.equal(
    canShowCreateAction({ step: 'VALIDATE', validation: FAILING, reviewed: true, warningsAcknowledged: true }),
    false
  );
  assert.equal(
    canShowCreateAction({ step: 'VALIDATE', validation: null, reviewed: true, warningsAcknowledged: true }),
    false
  );
});

test('create action requires the user to have reviewed the content', () => {
  assert.equal(
    canShowCreateAction({ step: 'VALIDATE', validation: PASSING, reviewed: false, warningsAcknowledged: true }),
    false
  );
});

test('create action stays locked until warnings have been acknowledged', () => {
  assert.equal(
    canShowCreateAction({ step: 'VALIDATE', validation: PASSING_WITH_WARNING, reviewed: true, warningsAcknowledged: false }),
    false,
    'warnings must be seen before the action is available'
  );
  assert.equal(
    canShowCreateAction({ step: 'VALIDATE', validation: PASSING_WITH_WARNING, reviewed: true, warningsAcknowledged: true }),
    true
  );
});

test('a valid draft with no warnings needs no acknowledgement', () => {
  assert.equal(hasWarnings(PASSING), false);
  assert.equal(
    canShowCreateAction({ step: 'VALIDATE', validation: PASSING, reviewed: true, warningsAcknowledged: false }),
    true
  );
});

test('errors and warnings are distinguished', () => {
  assert.equal(hasErrors(FAILING), true);
  assert.equal(hasWarnings(FAILING), false);
  assert.equal(hasWarnings(PASSING_WITH_WARNING), true);
  assert.equal(hasErrors(PASSING_WITH_WARNING), false);
});

// ---- Ambiguity ----------------------------------------------------------- //

test('an ambiguous question asks for clarification instead of drafting', () => {
  assert.equal(isAmbiguous({ needs_clarification: true }), true);
  assert.equal(isAmbiguous({ needs_clarification: false }), false);
  assert.equal(isAmbiguous(null), false);
});

// ---- Material change invalidates the quote -------------------------------- //

test('an unchanged draft does not invalidate the quote', () => {
  assert.deepEqual(invalidatesQuote(BASE_DRAFT, { ...BASE_DRAFT }), []);
});

test('changing any material field invalidates the quote', () => {
  for (const field of MATERIAL_FIELDS) {
    const edited = { ...BASE_DRAFT };
    if (field === 'resolution_source_confirmed') edited[field] = false;
    else if (field === 'sources_of_truth') edited[field] = ['https://example.com/c'];
    else edited[field] = 'changed';
    const changed = changedMaterialFields(BASE_DRAFT, edited);
    assert.ok(changed.includes(field), `changing ${field} must invalidate the quote`);
  }
});

test('presentation-only edits do not invalidate the quote', () => {
  const edited = { ...BASE_DRAFT, outcome_yes: 'Yes', outcome_no: 'No', notes: 'internal only' };
  assert.deepEqual(changedMaterialFields(BASE_DRAFT, edited), []);
});

test('source order alone does not invalidate the quote', () => {
  const reordered = { ...BASE_DRAFT, sources_of_truth: [...BASE_DRAFT.sources_of_truth].reverse() };
  assert.deepEqual(changedMaterialFields(BASE_DRAFT, reordered), []);
});

test('whitespace-only differences are not material', () => {
  assert.deepEqual(changedMaterialFields(BASE_DRAFT, { ...BASE_DRAFT, question: `  ${BASE_DRAFT.question}  ` }), []);
});

// ---- State honesty -------------------------------------------------------- //

test('a signature alone does not mean a market exists', () => {
  const attempt = { status: 'SUBMITTED', solana_signature: 'sig-1', market_exists: false };
  assert.equal(marketExists(attempt), false);
  assert.equal(isConfirmedOnChain(attempt), false);
});

test('only REGISTERED and INDEXED mean a market exists', () => {
  for (const status of ['CREATED', 'SIGNED', 'BROADCASTING', 'SUBMITTED', 'CONFIRMING', 'CONFIRMED', 'REGISTERING', 'REGISTERED', 'INDEXED', 'FAILED', 'UNKNOWN']) {
    const expected = status === 'REGISTERED' || status === 'INDEXED';
    assert.equal(marketExists({ status }), expected, `status ${status}`);
  }
});

test('a confirmed but unregistered attempt says registration is pending', () => {
  const attempt = { status: 'CONFIRMED', solana_signature: 'sig-1', market_exists: false };
  assert.equal(
    statusMessage(attempt),
    'Transaction confirmed on Solana. Panta registration is pending.'
  );
});

test('registration is never reported as a creation failure', () => {
  const attempt = {
    status: 'CONFIRMED',
    market_exists: false,
    error_code: 'PANTA_REGISTRATION_FAILED',
    error_message: 'panta rejected the registration',
  };
  const message = statusMessage(attempt);
  assert.match(message, /Panta registration is pending/);
  assert.doesNotMatch(message, /creation failed/i);
});

test('a registered market reports that it will be indexed', () => {
  assert.match(
    statusMessage({ status: 'REGISTERED', market_exists: true }),
    /registered with Panta/i
  );
});

test('an unknown outcome is not reported as a failure', () => {
  const message = statusMessage({ status: 'UNKNOWN', market_exists: false });
  assert.match(message, /unknown/i);
  assert.doesNotMatch(message, /creation failed/i);
});

test('a genuine failure reports the error', () => {
  assert.match(
    statusMessage({ status: 'FAILED', market_exists: false, error_message: 'quote rejected' }),
    /quote rejected/
  );
});

test('an absent attempt produces no message', () => {
  assert.equal(statusMessage(null), '');
  assert.equal(statusMessage(undefined), '');
});

// ---- Retry safety --------------------------------------------------------- //

test('a transaction with a signature is never re-broadcast', () => {
  for (const status of ['BROADCASTING', 'SUBMITTED', 'CONFIRMING', 'CONFIRMED', 'REGISTERING', 'REGISTERED']) {
    assert.equal(
      canRetryBroadcast({ status, solana_signature: 'sig-1' }),
      false,
      `status ${status} must not be re-broadcast`
    );
  }
});

test('a broadcast that clearly never landed may be retried', () => {
  assert.equal(canRetryBroadcast({ status: 'FAILED' }), true);
  assert.equal(canRetryBroadcast({ status: 'CREATED' }), true);
  assert.equal(canRetryBroadcast(null), false);
});

test('registration can be retried after a confirmed transaction', () => {
  assert.equal(canRetryRegistration({ status: 'CONFIRMED' }), true);
  assert.equal(canRetryRegistration({ status: 'REGISTERED' }), false);
  assert.equal(canRetryRegistration({ status: 'SUBMITTED' }), false);
});

// ---- Money is never a float ---------------------------------------------- //

test('USDC base units are formatted as exact decimal strings', () => {
  assert.equal(formatBaseUnits('50000000'), '50');
  assert.equal(formatBaseUnits('1'), '0.000001');
  assert.equal(formatBaseUnits('1234567'), '1.234567');
  assert.equal(formatBaseUnits('1000000'), '1');
});

test('a non-integer amount is refused rather than guessed at', () => {
  assert.equal(formatBaseUnits('50.5'), '—');
  assert.equal(formatBaseUnits('-1'), '—');
  assert.equal(formatBaseUnits(''), '—');
  assert.equal(formatBaseUnits(null), '—');
  assert.equal(isValidBaseUnits('50000000'), true);
  assert.equal(isValidBaseUnits('50.00'), false);
  assert.equal(isValidBaseUnits('1e6'), false);
});
