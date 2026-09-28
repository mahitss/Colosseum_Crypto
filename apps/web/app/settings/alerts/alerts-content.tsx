'use client';

import * as React from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Trash2, Plus, AlertTriangle, Pause, Play, Bell, ChevronDown, ChevronRight } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { EmptyState } from '@/components/ui/empty-state';
import { ErrorState } from '@/components/ui/error-state';
import type { AlertRule, AlertRuleInput, AlertEvent, WatchlistSummary, Severity, SignalType } from '@/lib/api-types';

const SEVERITIES: Severity[] = ['INFO', 'WATCH', 'SIGNIFICANT', 'CRITICAL'];

const SIGNAL_TYPES: { value: SignalType; label: string; blurb: string }[] = [
  { value: 'NEW_MARKET', label: 'New market', blurb: 'A market appears in the catalog.' },
  { value: 'PROBABILITY_SHIFT', label: 'Probability shift', blurb: 'The YES probability moved.' },
  { value: 'ACTIVITY_CHANGE', label: 'Activity change', blurb: 'Volume or trade count moved.' },
  { value: 'LIQUIDITY_CHANGE', label: 'Liquidity change', blurb: 'Liquidity in the market moved.' },
  { value: 'MARKET_MOVEMENT', label: 'Market movement', blurb: 'A composite movement signal.' },
];

const COOLDOWN_OPTIONS: { value: number; label: string }[] = [
  { value: 300, label: '5 minutes' },
  { value: 900, label: '15 minutes' },
  { value: 1800, label: '30 minutes' },
  { value: 3600, label: '1 hour' },
  { value: 21600, label: '6 hours' },
  { value: 86400, label: '1 day' },
];

export interface AlertsSettingsContentProps {
  initialRules: AlertRule[];
  initialEvents: AlertEvent[];
  watchlists: WatchlistSummary[];
}

export function AlertsSettingsContent({ initialRules, initialEvents, watchlists }: AlertsSettingsContentProps) {
  const queryClient = useQueryClient();
  const [showCreate, setShowCreate] = React.useState(false);
  const [expandedRule, setExpandedRule] = React.useState<string | null>(null);

  const rulesQuery = useQuery({
    queryKey: ['alert-rules'],
    queryFn: async () => {
      const response = await fetch('/api/alerts');
      if (!response.ok) throw new Error('Failed to fetch alert rules');
      return response.json() as Promise<AlertRule[]>;
    },
    initialData: initialRules,
    staleTime: 30_000,
  });

  const eventsQuery = useQuery({
    queryKey: ['alert-events'],
    queryFn: async () => {
      const response = await fetch('/api/alerts/events');
      if (!response.ok) throw new Error('Failed to fetch alert events');
      return response.json() as Promise<AlertEvent[]>;
    },
    initialData: initialEvents,
    staleTime: 30_000,
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['alert-rules'] });
    queryClient.invalidateQueries({ queryKey: ['alert-events'] });
  };

  const toggleMutation = useMutation({
    mutationFn: async ({ rule, enabled }: { rule: AlertRule; enabled: boolean }) => {
      const response = await fetch(`/api/alerts/${encodeURIComponent(rule.id)}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...toInput(rule), enabled }),
      });
      if (!response.ok) throw new Error('Failed to update alert rule');
      return response.json() as Promise<AlertRule>;
    },
    onSuccess: invalidate,
  });

  const deleteMutation = useMutation({
    mutationFn: async (id: string) => {
      const response = await fetch(`/api/alerts/${encodeURIComponent(id)}`, { method: 'DELETE' });
      if (!response.ok) throw new Error('Failed to delete alert rule');
    },
    onSuccess: invalidate,
  });

  const createMutation = useMutation({
    mutationFn: async (input: AlertRuleInput) => {
      const response = await fetch('/api/alerts', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(input),
      });
      if (!response.ok) {
        const body = await response.json().catch(() => null);
        throw new Error(body?.error?.message ?? 'Failed to create alert rule');
      }
      return response.json() as Promise<AlertRule>;
    },
    onSuccess: () => {
      invalidate();
      setShowCreate(false);
    },
  });

  const rules = rulesQuery.data ?? [];
  const events = eventsQuery.data ?? [];
  const enabledCount = rules.filter((r) => r.enabled).length;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Alert Rules</h1>
        <p className="text-muted-foreground mt-2">
          Rules are evaluated by the deterministic engine. A rule matches on structure, never on
          a model&rsquo;s judgement.
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">Total rules</p>
            <p className="text-3xl font-bold mt-2">{rules.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">Enabled</p>
            <p className="text-3xl font-bold mt-2 text-success">{enabledCount}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">Alerts fired</p>
            <p className="text-3xl font-bold mt-2">{events.length}</p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0">
          <div>
            <CardTitle>Rules</CardTitle>
            <CardDescription>Disabled rules are never evaluated.</CardDescription>
          </div>
          <Button onClick={() => setShowCreate((v) => !v)} size="sm" variant={showCreate ? 'secondary' : 'default'}>
            <Plus className="h-4 w-4 mr-2" />
            New rule
          </Button>
        </CardHeader>
        <CardContent className="space-y-4">
          {showCreate && (
            <RuleForm
              watchlists={watchlists}
              pending={createMutation.isPending}
              error={createMutation.error instanceof Error ? createMutation.error.message : null}
              onCancel={() => setShowCreate(false)}
              onSubmit={(input) => createMutation.mutate(input)}
            />
          )}

          {rulesQuery.isLoading && !rulesQuery.data ? (
            <div className="space-y-2">
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} className="h-14 w-full" />
              ))}
            </div>
          ) : rulesQuery.error ? (
            <ErrorState
              title="Could not load alert rules"
              description={rulesQuery.error instanceof Error ? rulesQuery.error.message : 'Unknown error'}
              onRetry={() => rulesQuery.refetch()}
            />
          ) : rules.length === 0 ? (
            <EmptyState
              icon={<Bell className="h-8 w-8" />}
              title="No alert rules yet"
              description="Create a rule to be notified when a market you care about changes in a way you care about."
            />
          ) : (
            <div className="space-y-2">
              {rules.map((rule) => (
                <RuleRow
                  key={rule.id}
                  rule={rule}
                  events={events.filter((e) => e.alert_rule_id === rule.id)}
                  expanded={expandedRule === rule.id}
                  onToggleExpand={() => setExpandedRule(expandedRule === rule.id ? null : rule.id)}
                  onToggleEnabled={(enabled) => toggleMutation.mutate({ rule, enabled })}
                  onDelete={() => deleteMutation.mutate(rule.id)}
                  pending={toggleMutation.isPending || deleteMutation.isPending}
                />
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Recent alert activity</CardTitle>
          <CardDescription>
            A <Badge variant="secondary">SUPPRESSED</Badge> alert was held back by a rule&rsquo;s
            cooldown. A <Badge variant="destructive">FAILED</Badge> alert will be retried.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {eventsQuery.isLoading && !eventsQuery.data ? (
            <Skeleton className="h-24 w-full" />
          ) : eventsQuery.error ? (
            <ErrorState
              title="Could not load alert activity"
              description={eventsQuery.error instanceof Error ? eventsQuery.error.message : 'Unknown error'}
              onRetry={() => eventsQuery.refetch()}
            />
          ) : events.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4">
              No alerts have fired yet. Alerts appear here once a rule matches a signal event.
            </p>
          ) : (
            <div className="space-y-1">
              {events.slice(0, 25).map((event) => (
                <div key={event.id} className="flex items-center gap-3 border-b border-slate-800/80 py-2 last:border-b-0 text-sm">
                  <Badge variant={statusVariant(event.status)}>{event.status}</Badge>
                  <span className="text-xs text-muted-foreground tabular-nums">
                    {new Date(event.triggered_at).toLocaleString()}
                  </span>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function statusVariant(status: AlertEvent['status']): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case 'DELIVERED':
      return 'default';
    case 'PENDING':
      return 'secondary';
    case 'FAILED':
      return 'destructive';
    default:
      return 'outline';
  }
}

// A failed alert will be retried, so the copy distinguishes the two states a
// user would otherwise conflate.

interface RuleFormProps {
  watchlists: WatchlistSummary[];
  pending: boolean;
  error: string | null;
  onSubmit: (input: AlertRuleInput) => void;
  onCancel: () => void;
}

function RuleForm({ watchlists, pending, error, onSubmit, onCancel }: RuleFormProps) {
  const [name, setName] = React.useState('');
  const [scope, setScope] = React.useState<'ALL' | 'WATCHLIST' | 'MARKET'>(
    watchlists.length > 0 ? 'WATCHLIST' : 'ALL',
  );
  const [watchlistId, setWatchlistId] = React.useState(watchlists[0]?.id ?? '');
  const [marketId, setMarketId] = React.useState('');
  const [signalType, setSignalType] = React.useState<SignalType | ''>('');
  const [minimumSeverity, setMinimumSeverity] = React.useState<Severity | ''>('SIGNIFICANT');
  const [probabilityThreshold, setProbabilityThreshold] = React.useState('');
  const [cooldown, setCooldown] = React.useState(1800);

  const canSubmit = name.trim().length > 0 && minimumSeverity !== '' && (scope !== 'WATCHLIST' || watchlistId !== '');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    onSubmit({
      name: name.trim(),
      enabled: true,
      watchlist_id: scope === 'WATCHLIST' ? watchlistId : null,
      market_id: scope === 'MARKET' ? marketId.trim() || null : null,
      signal_type: signalType === '' ? null : signalType,
      minimum_severity: minimumSeverity,
      probability_change_threshold: signalType === '' || signalType === 'PROBABILITY_SHIFT'
        ? probabilityThreshold.trim() || null
        : null,
      cooldown_seconds: cooldown,
    });
  };

  return (
    <form onSubmit={handleSubmit} className="rounded-lg border border-slate-800 bg-slate-900/40 p-4 space-y-4">
      <div className="space-y-2">
        <Label htmlFor="rule-name">Name</Label>
        <Input
          id="rule-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g. Significant moves on my watchlist"
          maxLength={120}
        />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="space-y-2">
          <Label htmlFor="rule-signal">When</Label>
          <select
            id="rule-signal"
            value={signalType}
            onChange={(e) => setSignalType(e.target.value as SignalType | '')}
            className="w-full h-9 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm"
          >
            <option value="">Any signal</option>
            {SIGNAL_TYPES.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          {signalType !== '' && (
            <p className="text-xs text-muted-foreground">
              {SIGNAL_TYPES.find((o) => o.value === signalType)?.blurb}
            </p>
          )}
        </div>

        <div className="space-y-2">
          <Label htmlFor="rule-scope">On</Label>
          <select
            id="rule-scope"
            value={scope}
            onChange={(e) => setScope(e.target.value as typeof scope)}
            className="w-full h-9 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm"
          >
            <option value="ALL">Every market</option>
            {watchlists.length > 0 && <option value="WATCHLIST">A watchlist</option>}
            <option value="MARKET">One market</option>
          </select>
        </div>

        <div className="space-y-2">
          <Label htmlFor="rule-severity">With minimum severity</Label>
          <select
            id="rule-severity"
            value={minimumSeverity}
            onChange={(e) => setMinimumSeverity(e.target.value as Severity | '')}
            className="w-full h-9 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm"
          >
            {SEVERITIES.map((severity) => (
              <option key={severity} value={severity}>
                {severity}
              </option>
            ))}
          </select>
        </div>
      </div>

      {scope === 'WATCHLIST' && (
        <div className="space-y-2">
          <Label htmlFor="rule-watchlist">Watchlist</Label>
          <select
            id="rule-watchlist"
            value={watchlistId}
            onChange={(e) => setWatchlistId(e.target.value)}
            className="w-full h-9 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm"
          >
            {watchlists.map((watchlist) => (
              <option key={watchlist.id} value={watchlist.id}>
                {watchlist.name}
              </option>
            ))}
          </select>
        </div>
      )}

      {scope === 'MARKET' && (
        <div className="space-y-2">
          <Label htmlFor="rule-market">Market address</Label>
          <Input
            id="rule-market"
            value={marketId}
            onChange={(e) => setMarketId(e.target.value)}
            placeholder="Panta market id"
          />
        </div>
      )}

      {(signalType === '' || signalType === 'PROBABILITY_SHIFT') && (
        <div className="space-y-2">
          <Label htmlFor="rule-threshold">Probability move threshold (percentage points, optional)</Label>
          <Input
            id="rule-threshold"
            type="number"
            min="0"
            step="0.1"
            value={probabilityThreshold}
            onChange={(e) => setProbabilityThreshold(e.target.value)}
            placeholder="e.g. 10 means alert on a 10pp move"
          />
          <p className="text-xs text-muted-foreground">
            Leave blank to alert on any qualifying severity. This applies only to probability
            signals; it is ignored for other signal types.
          </p>
        </div>
      )}

      <div className="space-y-2">
        <Label htmlFor="rule-cooldown">Cooldown</Label>
        <select
          id="rule-cooldown"
          value={cooldown}
          onChange={(e) => setCooldown(Number(e.target.value))}
          className="w-full h-9 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm"
        >
          {COOLDOWN_OPTIONS.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <p className="text-xs text-muted-foreground">
          After an alert fires, this rule stays quiet for the cooldown period.
        </p>
      </div>

      {error && (
        <p className="text-sm text-danger" role="alert">
          {error}
        </p>
      )}

      <div className="flex gap-2">
        <Button type="submit" disabled={!canSubmit || pending}>
          {pending ? 'Creating…' : 'Create rule'}
        </Button>
        <Button type="button" variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  );
}

interface RuleRowProps {
  rule: AlertRule;
  events: AlertEvent[];
  expanded: boolean;
  pending: boolean;
  onToggleExpand: () => void;
  onToggleEnabled: (enabled: boolean) => void;
  onDelete: () => void;
}

function RuleRow({ rule, events, expanded, pending, onToggleExpand, onToggleEnabled, onDelete }: RuleRowProps) {
  return (
    <div className="rounded-lg border border-slate-800">
      <div className="flex items-center gap-3 p-3">
        <button
          type="button"
          onClick={onToggleExpand}
          className="flex items-center gap-2 flex-1 text-left min-w-0"
          aria-expanded={expanded}
        >
          {expanded ? (
            <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
          ) : (
            <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
          )}
          <span className="font-medium truncate">{rule.name}</span>
          {!rule.enabled && <Badge variant="secondary">Disabled</Badge>}
        </button>
        <Button
          size="sm"
          variant="ghost"
          disabled={pending}
          onClick={() => onToggleEnabled(!rule.enabled)}
          aria-label={rule.enabled ? 'Disable rule' : 'Enable rule'}
        >
          {rule.enabled ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
        </Button>
        <Button
          size="sm"
          variant="ghost"
          disabled={pending}
          onClick={onDelete}
          aria-label="Delete rule"
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      </div>
      {expanded && (
        <div className="border-t border-slate-800 px-3 py-3 space-y-2 text-sm">
          <dl className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <div>
              <dt className="text-xs text-muted-foreground">Signal type</dt>
              <dd>{rule.signal_type ?? 'Any'}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">Minimum severity</dt>
              <dd>{rule.minimum_severity ?? 'Any'}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">Cooldown</dt>
              <dd>{formatCooldown(rule.cooldown_seconds)}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">Alerts fired</dt>
              <dd>{events.length}</dd>
            </div>
          </dl>
          {rule.probability_change_threshold && (
            <p className="text-xs text-muted-foreground">
              Probability threshold: {rule.probability_change_threshold} percentage points.
            </p>
          )}
        </div>
      )}
    </div>
  );
}

function formatCooldown(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
  if (seconds < 86400) return `${Math.round(seconds / 3600)}h`;
  return `${Math.round(seconds / 86400)}d`;
}

/** Converts a stored rule back into the input shape the API accepts. */
function toInput(rule: AlertRule): AlertRuleInput {
  return {
    name: rule.name,
    enabled: rule.enabled,
    watchlist_id: rule.watchlist_id,
    market_id: rule.market_id,
    signal_type: rule.signal_type,
    minimum_severity: rule.minimum_severity,
    probability_change_threshold: rule.probability_change_threshold,
    activity_change_threshold: rule.activity_change_threshold,
    liquidity_change_threshold: rule.liquidity_change_threshold,
    cooldown_seconds: rule.cooldown_seconds,
  };
}
