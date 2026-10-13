import { getMarket } from '@/lib/api-client';
import { ApiError } from '@/lib/api-client';
import { notFound } from 'next/navigation';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from '@/components/ui/table';
import { formatDistanceToNow, format } from 'date-fns';
import { TradeTicket } from '@/components/trade-ticket';
import { cn } from '@/lib/cn';
import { ExternalLink, ChevronDown, ChevronUp, Activity } from 'lucide-react';

function formatVolume(volume: string): string {
  const num = parseFloat(volume);
  if (num >= 1e6) return `$${(num / 1e6).toFixed(1)}M`;
  if (num >= 1e3) return `$${(num / 1e3).toFixed(1)}K`;
  return `$${num.toFixed(0)}`;
}

/** Format a timestamp that the backend may legitimately not have. */
function formatDate(value: string | null | undefined): string {
  if (!value) return '—';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return '—';
  return format(parsed, 'MMM d, yyyy');
}

function formatAge(value: string | null | undefined): string | null {
  if (!value) return null;
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return null;
  return formatDistanceToNow(parsed) + ' ago';
}

export default async function MarketDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;

  let data;
  try {
    data = await getMarket(id);
  } catch (error) {
    // A market the backend does not know is a genuine 404, not a server
    // failure. Anything else (unreachable gateway, query failure) propagates
    // to Next's error boundary, which offers the user a retry.
    if (error instanceof ApiError && error.status === 404) {
      notFound();
    }
    throw error;
  }

  const { market, observations, signals } = data;

  const hasHistory = observations.length >= 2;
  const sortedSignals = [...signals].sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime());

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div>
        <div className="flex items-center gap-3 flex-wrap">
          {/* Panta legitimately serves markets whose title is empty; the
              description or image carries the content in that case. */}
          <h1 className="text-3xl font-bold tracking-tight">{market.title?.trim() || 'Untitled market'}</h1>
          <Badge variant={getStatusVariant(market.status)} className="text-sm">{market.status}</Badge>
        </div>
        {market.description && (
          <p className="text-muted-foreground mt-2">{market.description}</p>
        )}
        <div className="flex flex-wrap items-center gap-4 mt-3 text-sm text-muted-foreground">
          {market.category && <span>Category: {market.category}</span>}
          {market.closes_at && (
            <span>Closes: {formatDate(market.closes_at)}</span>
          )}
          {(() => {
            const age = formatAge(market.updated_at || market.created_at);
            return age ? (
              <span className="flex items-center gap-1 text-slate-500">
                <Activity className="w-3.5 h-3.5" />
                Last updated {age}
              </span>
            ) : null;
          })()}
        </div>
      </div>

      {/* Primary Metrics */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <Card className="border-slate-800/60 hover:border-emerald-500/30 transition-colors">
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">YES Probability</p>
            <p className="text-3xl font-bold mt-1 text-emerald-400 tabular-nums">
              {market.yes_probability ? `${parseFloat(market.yes_probability).toFixed(1)}%` : '—'}
            </p>
          </CardContent>
        </Card>
        <Card className="border-slate-800/60 hover:border-rose-500/30 transition-colors">
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">NO Probability</p>
            <p className="text-3xl font-bold mt-1 text-rose-400 tabular-nums">
              {market.no_probability ? `${parseFloat(market.no_probability).toFixed(1)}%` : '—'}
            </p>
          </CardContent>
        </Card>
        <Card className="border-slate-800/60 hover:border-slate-700/60 transition-colors">
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">Volume</p>
            <p className="text-2xl font-semibold mt-1 tabular-nums">
              {market.volume_usdc ? formatVolume(market.volume_usdc) : '—'}
            </p>
          </CardContent>
        </Card>
        <Card className="border-slate-800/60 hover:border-slate-700/60 transition-colors">
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">Liquidity</p>
            <p className="text-2xl font-semibold mt-1 tabular-nums">
              {market.liquidity ? formatVolume(market.liquidity) : '—'}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Trade Ticket */}
      <TradeTicket market={market} />

      {/* Probability Chart */}
      <Card className="border-slate-800/60">
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Probability Over Time</CardTitle>
              <CardDescription>Historical YES probability for this market</CardDescription>
            </div>
            <Badge variant="outline" className="text-xs">{observations.length} observations</Badge>
          </div>
        </CardHeader>
        <CardContent>
            {!hasHistory ? (
              <div className="text-center py-12 text-muted-foreground">
                <Activity className="w-8 h-8 mx-auto mb-3 text-slate-500" />
                <p>Historical data is being collected.</p>
              </div>
            ) : (
              <div className="h-72">
                {/* Recharts would render here - placeholder for now */}
                <div className="flex items-center justify-center h-full text-muted-foreground">
                  Chart: {observations.length} observations
                </div>
              </div>
            )}
          </CardContent>
        </Card>

      {/* Signal Timeline */}
      <Card className="border-slate-800/60">
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Signal Timeline</CardTitle>
              <CardDescription>Recent signals for this market</CardDescription>
            </div>
            <Badge variant="outline" className="text-xs">{signals.length} signals</Badge>
          </div>
        </CardHeader>
        <CardContent>
            {signals.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground">
                No signals yet for this market.
              </div>
            ) : (
              <div className="space-y-4">
                {sortedSignals.slice(0, 15).map((signal) => (
                  <div key={signal.id} className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-slate-800 bg-slate-800/40 px-4 py-3">
                    <div className="flex items-center gap-2 flex-wrap">
                      <Badge variant={getSeverityVariant(signal.severity)} className="text-xs">
                        {signal.severity}
                      </Badge>
                      <span className="text-sm font-medium text-foreground">
                        {signal.signal_type.replace(/_/g, ' ').toLowerCase()}
                      </span>
                      <span className="text-sm text-muted-foreground">
                        {signal.previous_value} → {signal.current_value}
                        {signal.percentage_points && ` (${signal.percentage_points})`}
                      </span>
                    </div>
                    <div className="text-sm text-muted-foreground flex items-center gap-1">
                      <span>{formatDistanceToNow(new Date(signal.timestamp))} ago</span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

      {/* Market Metadata */}
      <Card className="border-slate-800/60">
        <CardHeader>
          <CardTitle>Market Metadata</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <dt className="text-muted-foreground">Market ID</dt>
              <dd className="font-mono mt-1 text-sm">{market.source_market_id}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Source</dt>
              <dd className="mt-1 text-sm">{market.source}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Phase</dt>
              <dd className="mt-1 text-sm">{market.phase}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Resolution Status</dt>
              <dd className="mt-1 text-sm">{market.resolution_status ?? '—'}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Created</dt>
              <dd className="mt-1 text-sm">{formatDate(market.created_at)}</dd>
            </div>
            {market.closes_at && (
              <div>
                <dt className="text-muted-foreground">Closes</dt>
                <dd className="mt-1 text-sm">{formatDate(market.closes_at)}</dd>
              </div>
            )}
          </dl>
        </CardContent>
      </Card>
    </div>
  );
}

function getStatusVariant(status: string): 'default' | 'secondary' | 'success' | 'warning' | 'destructive' {
  switch (status.toUpperCase()) {
    case 'ACTIVE': return 'success';
    case 'CLOSED':
    case 'RESOLVED': return 'secondary';
    case 'PAUSED': return 'warning';
    default: return 'default';
  }
}

function getSeverityVariant(severity: string): 'info' | 'warning' | 'success' | 'critical' {
  switch (severity.toUpperCase()) {
    case 'INFO': return 'info';
    case 'WATCH': return 'warning';
    case 'SIGNIFICANT': return 'success';
    case 'CRITICAL': return 'critical';
    default: return 'info';
  }
}