import { Metadata } from 'next';
import { getAlertEvents, getAlertRules, getWatchlists } from '@/lib/enterprise-api';
import { AlertsSettingsContent } from './alerts-content';

export const metadata: Metadata = {
  title: 'Alert Rules — Prophet',
  description: 'Configure deterministic alert rules and inspect recent alert activity.',
};

export default async function AlertSettingsPage() {
  // Each source is independent: a user with no watchlists still has a working
  // alerts page, and a transient failure in one must not blank the whole page.
  const [rules, events, watchlists] = await Promise.all([
    getAlertRules().catch(() => []),
    getAlertEvents(25).catch(() => []),
    getWatchlists().catch(() => []),
  ]);

  return <AlertsSettingsContent initialRules={rules} initialEvents={events} watchlists={watchlists} />;
}
