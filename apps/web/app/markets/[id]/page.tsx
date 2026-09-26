import { getMarket } from '@/lib/api-client';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from '@/components/ui/table';
import { formatDistanceToNow, format } from 'date-fns';

export default async function MarketDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const { market, observations, signals } = await getMarket(id);

  const hasHistory = observations.length >= 2;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <div className="flex items-center gap-3">
          <h1 className="text-3xl font-bold tracking-tight">{market.title}</h1>
          <Badge variant={getStatusVariant(market.status)}>{market.status}</Badge>
        </div>
        {market.description && (
          <p className="text-muted-foreground mt-2">{market.description}</p>
        )}
        <div className="flex items-center gap-4 mt-3 text-sm text-muted-foreground">
          {market.category && <span>Category: {market.category}</span>}
          {market.closes_at && (
            <span>Closes: {format(new Date(market.closes_at), 'MMM d, yyyy')}</span>
          )}
        </div>
      </div>

      {/* Primary Metrics */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">YES Probability</p>
            <p className="text-3xl font-bold mt-2 text-success">
              {market.yes_probability ? `${parseFloat(market.yes_probability).toFixed(1)}%` : '—'}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">NO Probability</p>
            <p className="text-3xl font-bold mt-2 text-danger">
              {market.no_probability ? `${parseFloat(market.no_probability).toFixed(1)}%` : '—'}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">Volume</p>
            <p className="text-2xl font-semibold mt-2">
              {market.volume_usdc ? formatVolume(market.volume_usdc) : '—'}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">Liquidity</p>
            <p className="text-2xl font-semibold mt-2">
              {market.liquidity ? formatVolume(market.liquidity) : '—'}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Probability Chart */}
      <Card>
        <CardHeader>
          <CardTitle>Probability Over Time</CardTitle>
          <CardDescription>
            Historical YES probability for this market
          </CardDescription>
        </CardHeader>
        <CardContent>
          {!hasHistory ? (
            <div className="text-center py-12 text-muted-foreground">
              Historical data is being collected.
            </div>
          ) : (
            <div className="h-64">
              {/* Recharts would render here - placeholder for now */}
              <div className="flex items-center justify-center h-full text-muted-foreground">
                Chart: {observations.length} observations
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Signal Timeline */}
      <Card>
        <CardHeader>
          <CardTitle>Signal Timeline</CardTitle>
          <CardDescription>Recent signals for this market</CardDescription>
        </CardHeader>
        <CardContent>
          {signals.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">
              No signals yet for this market.
            </div>
          ) : (
            <div className="space-y-4">
              {signals.slice(0, 10).map((signal) => (
                <div key={signal.id} className="flex items-start gap-4 pb-4 border-b last:border-0">
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{signal.signal_type}</span>
                      <Badge variant={getSeverityVariant(signal.severity)}>{signal.severity}</Badge>
                    </div>
                    <div className="text-sm text-muted-foreground mt-1">
                      {signal.previous_value} → {signal.current_value}
                      {signal.percentage_points && ` (${signal.percentage_points})`}
                    </div>
                  </div>
                  <div className="text-sm text-muted-foreground">
                    {formatDistanceToNow(new Date(signal.timestamp))} ago
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Market Metadata */}
      <Card>
        <CardHeader>
          <CardTitle>Market Metadata</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <dt className="text-muted-foreground">Market ID</dt>
              <dd className="font-mono mt-1">{market.source_market_id}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Source</dt>
              <dd className="mt-1">{market.source}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Phase</dt>
              <dd className="mt-1">{market.phase}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Resolution Status</dt>
              <dd className="mt-1">{market.resolution_status ?? '—'}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Created</dt>
              <dd className="mt-1">{format(new Date(market.created_at), 'MMM d, yyyy')}</dd>
            </div>
            {market.closes_at && (
              <div>
                <dt className="text-muted-foreground">Closes</dt>
                <dd className="mt-1">{format(new Date(market.closes_at), 'MMM d, yyyy')}</dd>
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

function formatVolume(volume: string): string {
  const num = parseFloat(volume);
  if (num >= 1e6) return `$${(num / 1e6).toFixed(1)}M`;
  if (num >= 1e3) return `$${(num / 1e3).toFixed(1)}K`;
  return `$${num.toFixed(0)}`;
}