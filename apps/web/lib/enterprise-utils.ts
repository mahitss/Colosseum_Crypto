import type { Severity, SignalType } from './api-types';

// Pure, testable formatting/derivation helpers. No React, no side effects.

/**
 * Format a decimal ratio string as a percentage with 1 decimal place.
 * "0.412" -> "41.2%", "0.5" -> "50.0%", null -> "—"
 * Does NOT use parseFloat on money/probability values.
 */
export function formatProbability(value: string | null): string {
  if (value === null || value === undefined || value === '') return '—';
  
  // Handle scientific notation edge case
  if (value.includes('e') || value.includes('E')) {
    const num = Number(value);
    if (Number.isNaN(num)) return '—';
    return `${(num * 100).toFixed(1)}%`;
  }
  
  // Find decimal point
  const dotIndex = value.indexOf('.');
  if (dotIndex === -1) {
    // Integer string like "1" -> "100.0%"
    const intPart = value.padStart(3, '0');
    return `${intPart.slice(0, -2)}.${intPart.slice(-2)[0]}%`;
  }
  
  const intPart = value.slice(0, dotIndex);
  const fracPart = value.slice(dotIndex + 1);
  
  // We need two digits of fraction to show 1 decimal in percentage
  // e.g., "0.412" -> intPart="0", fracPart="412" -> take "41" -> "41.2%"
  const firstTwo = (fracPart + '00').slice(0, 2);
  const percentage = `${intPart}.${firstTwo}`;
  
  // Remove leading zeros from integer part but keep at least one digit
  const normalized = percentage.replace(/^0+(\d)/, '$1');
  
  return `${normalized}%`;
}

/**
 * Format a signed change value with prefix.
 * unit: 'pp' = percentage points (multiply by 100), '%' = percent (as-is)
 * "0.166", 'pp' -> "+16.6pp"
 * "-0.12", 'pp' -> "−12.0pp"
 * "0.38", '%' -> "+38%"
 */
export function formatChange(value: string | null, unit: 'pp' | '%'): string {
  if (value === null || value === undefined || value === '') return '—';
  
  const isNegative = value.startsWith('-');
  const absValue = isNegative ? value.slice(1) : value;
  
  let formatted: string;
  if (unit === 'pp') {
    formatted = formatProbability(absValue).replace('%', '');
  } else {
    formatted = absValue;
  }
  
  const prefix = isNegative ? '−' : '+';
  const suffix = unit === 'pp' ? 'pp' : '%';
  return `${prefix}${formatted}${suffix}`;
}

/**
 * Human-readable severity label.
 */
export function formatSeverityLabel(severity: Severity): string {
  switch (severity) {
    case 'CRITICAL': return 'Critical';
    case 'SIGNIFICANT': return 'Significant';
    case 'WATCH': return 'Watch';
    case 'INFO': return 'Info';
    default: return severity;
  }
}

/**
 * Severity rank for sorting/comparison.
 */
export function severityRank(severity: Severity): number {
  switch (severity) {
    case 'CRITICAL': return 4;
    case 'SIGNIFICANT': return 3;
    case 'WATCH': return 2;
    case 'INFO': return 1;
    default: return 0;
  }
}

export const SEVERITY_ORDER: Severity[] = ['CRITICAL', 'SIGNIFICANT', 'WATCH', 'INFO'];

export type RadarTab = 'all' | 'CRITICAL' | 'SIGNIFICANT' | 'WATCH' | 'INFO';

export const RADAR_TABS: { id: RadarTab; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'CRITICAL', label: 'Critical' },
  { id: 'SIGNIFICANT', label: 'Significant' },
  { id: 'WATCH', label: 'Watch' },
  { id: 'INFO', label: 'Info' },
];

/**
 * Gate for "Create" actions. Returns true only when validation passes.
 * For alerts: name non-empty, scope valid (watchlist XOR market XOR global), cooldown >= 0.
 */
export function shouldShowCreateAlertButton(state: {
  name: string;
  watchlist_id?: string | null;
  market_id?: string | null;
  cooldown_seconds?: number;
}): boolean {
  if (!state.name?.trim()) return false;
  const hasWatchlist = !!state.watchlist_id;
  const hasMarket = !!state.market_id;
  if (hasWatchlist && hasMarket) return false; // must be exactly one or neither
  if (typeof state.cooldown_seconds === 'number' && state.cooldown_seconds < 0) return false;
  return true;
}

/**
 * Local filter for instant tab feedback. Server filter is authoritative.
 */
export function filterRadarEventsLocally(events: { severity: Severity }[], tab: RadarTab) {
  if (tab === 'all') return events;
  const minRank = severityRank(tab);
  return events.filter(e => severityRank(e.severity) >= minRank);
}