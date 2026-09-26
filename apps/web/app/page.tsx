'use client';

import { useQuery } from '@tanstack/react-query';
import { getSummaryStats, getRecentSignals } from '@/lib/api-client';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from '@/components/ui/table';
import { Skeleton } from '@/components/ui/skeleton';
import { formatDistanceToNow } from 'date-fns';
import Link from 'next/link';

function CommandCenterContent() {
  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: ['summary-stats'],
    queryFn: getSummaryStats,
  });

  const { data: signals, isLoading: signalsLoading } = useQuery({
    queryKey: ['recent-signals'],
    queryFn: () => getRecentSignals(5),
  });

  if (statsLoading || signalsLoading) {
    return (
      <div className="space-y-6">
        <Skeleton variant="text" className="w-48 h-8" />
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} variant="card" />
          ))}
        </div>
        <Skeleton variant="card" className="h-64" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Command Center</h1>
        <p className="text-muted-foreground mt-2">Prediction-market intelligence across the Panta ecosystem.</p>
      </div>

      {/* Summary Strip */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">Markets Tracked</p>
            <p className="text-3xl font-bold mt-2">{stats?.marketsTracked ?? '—'}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">Active Signals</p>
            <p className="text-3xl font-bold mt-2">{stats?.activeSignals ?? '—'}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">Significant Changes</p>
            <p className="text-3xl font-bold mt-2">{stats?.significantChanges ?? '—'}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground">Markets Updated</p>
            <p className="text-lg font-semibold mt-2">{stats?.marketsUpdated ? formatDistanceToNow(new Date(stats.marketsUpdated)) + ' ago' : '—'}</p>
          </CardContent>
        </Card>
      </div>

      {/* Signal Radar */}
      <Card>
        <CardHeader>
          <CardTitle>Signal Radar</CardTitle>
          <CardDescription>Most recent significant signals from the Panta ecosystem</CardDescription>
        </CardHeader>
        <CardContent>
          {!signals || signals.signals.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">
              No signals yet. Data is being collected.
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Market</TableHead>
                  <TableHead>Signal</TableHead>
                  <TableHead>Severity</TableHead>
                  <TableHead>Previous → Current</TableHead>
                  <TableHead>Change</TableHead>
                  <TableHead>Time</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {signals.signals.map((signal) => (
                  <TableRow key={signal.id}>
                    <TableCell>{signal.market_id}</TableCell>
                    <TableCell className="font-medium">{signal.signal_type}</TableCell>
                    <TableCell>
                      <Badge variant={getSeverityVariant(signal.severity)}>
                        {signal.severity}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-col text-xs">
                        <span className="text-muted-foreground">{signal.previous_value ?? 'N/A'}</span>
                        <span className="text-muted-foreground">{signal.current_value ?? 'N/A'}</span>
                      </div>
                    </TableCell>
                    <TableCell className={signal.percentage_points && parseFloat(signal.percentage_points) > 0 ? 'text-success' : ''}>
                      {signal.percentage_points ?? '—'}
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {signal.timestamp ? formatDistanceToNow(new Date(signal.timestamp)) + ' ago' : '—'}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

export default function CommandCenterPage() {
  return <CommandCenterContent />;
}

function getSeverityVariant(severity: string): 'info' | 'warning' | 'success' | 'critical' {
  switch (severity.toUpperCase()) {
    case 'INFO':
      return 'info';
    case 'WATCH':
      return 'warning';
    case 'SIGNIFICANT':
      return 'success';
    case 'CRITICAL':
      return 'critical';
    default:
      return 'info';
  }
}
