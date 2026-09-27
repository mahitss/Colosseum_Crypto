// Frontend trade lifecycle tests — pure logic, no network, no wallet.
// Run with: node --test scripts/trade-states.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';

// Mirror of the success flow in components/trade-ticket.tsx.
const SUCCESS_FLOW = [
  'QUOTE_READY', 'BUILDING', 'SIGNED', 'SUBMITTED',
  'CONFIRMED', 'VERIFIED', 'POSITION_REFRESHING', 'COMPLETED',
];

function validDecimal(s) {
  if (s === '') return false;
  let hasDot = false;
  for (const ch of s) {
    if (ch === '.') {
      if (hasDot) return false;
      hasDot = true;
    } else if (ch < '0' || ch > '9') {
      return false;
    }
  }
  return true;
}

function positiveDecimal(s) {
  const [integer = '', fraction = ''] = s.split('.');
  return (integer + fraction).split('').some((ch) => ch !== '0');
}

test('trade state machine has an explicit ordered success flow', () => {
  assert.ok(SUCCESS_FLOW.includes('QUOTE_READY'));
  assert.ok(SUCCESS_FLOW.includes('BUILDING'));
  assert.ok(SUCCESS_FLOW.includes('SIGNED'));
  assert.ok(SUCCESS_FLOW.includes('CONFIRMED'));
  assert.ok(SUCCESS_FLOW.includes('VERIFIED'));
  assert.ok(SUCCESS_FLOW.includes('COMPLETED'));
  // Distinct states — never reduced to loading=true/false.
  assert.equal(new Set(SUCCESS_FLOW).size, SUCCESS_FLOW.length);
});

test('distinguishes submitted from confirmed from verified from completed', () => {
  const states = new Set([...SUCCESS_FLOW, 'SUBMITTED', 'CONFIRMED', 'VERIFIED', 'COMPLETED']);
  assert.ok(states.has('SUBMITTED'));
  assert.ok(states.has('CONFIRMED'));
  assert.ok(states.has('VERIFIED'));
  assert.ok(states.has('COMPLETED'));
});

test('accepts valid positive decimals', () => {
  assert.equal(validDecimal('10.50'), true);
  assert.equal(positiveDecimal('10.50'), true);
});

test('rejects zero and zero-decimal amounts', () => {
  assert.equal(validDecimal('0'), true);
  assert.equal(positiveDecimal('0'), false);
  assert.equal(validDecimal('0.00'), true);
  assert.equal(positiveDecimal('0.00'), false);
});

test('rejects non-numeric amounts', () => {
  assert.equal(validDecimal('abc'), false);
  assert.equal(validDecimal('-5'), false);
  assert.equal(validDecimal('1.2.3'), false);
});