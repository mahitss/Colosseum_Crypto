'use client';

import { useSearchParams } from 'next/navigation';
import { Suspense } from 'react';
import { getSignals } from '@/lib/api-client';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from '@/components/ui/table';
import { Skeleton } from '@/components/ui/skeleton';
import { EmptyState } from '@/components/ui/empty-state';
import { ErrorState } from '@/components/ui/error-state';
import { useQuery } from '@tanstack/react-query';
import { formatDistanceToNow } from 'date-fns';

function SignalsContent() {
  const params = useSearchParams();
  const severity = params.get('severity') || undefined;
  const signalType = params.get('signal_type') || undefined;
  const marketId = params.get('market_id') || undefined;

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['signals', { severity, signal_type: signalType, market_id: marketId }],
    queryFn: () => getSignals({ severity, signal_type: signalType, market_id: marketId, limit: 50 }),
  });

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Signals</CardTitle>
          <CardDescription>
            Explorer
            {severity && ` – ${severity}`}
            {signalType && ` – ${signalType}`}
            {marketId && ` – Market ${marketId}`}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (<Skeleton key={i} />))}
            </div>
          ) : error ? (
            <ErrorState title="Unable to load signals" description="Please try again" onRetry={refetch} />
          ) : !data || data.signals.length === 0 ? (
            <EmptyState title="No signals" description="There are no matching signals at this time." />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Type</TableHead>
                  <TableHead>Market</TableHead>
                  <TableHead>Previous → Current</TableHead>
                  <TableHead>Change</TableHead>
                  <TableHead>Severity</TableHead>
                  <TableHead>Time</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.signals.map((signal) => (
                  <TableRow key={signal.id}>
                    <TableCell>{signal.signal_type}</TableCell>
                    <TableCell>{signal.market_id}</TableCell>
                    <TableCell className="font-medium">
                      {signal.previous_value ?? '—'} → {signal.current_value ?? '—'}
                    </TableCell>
                    <TableCell>{signal.percentage_points ?? '—'}</TableCell>
                    <TableCell>
                      <Badge variant={getSeverityVariant(signal.severity)}>{signal.severity}</Badge>
                    </TableCell>
                    <TableCell>{formatDistanceToNow(new Date(signal.timestamp))}</TableCell>
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

export default function SignalsPage() {
  return (
    <Suspense fallback={<Skeleton variant="default" />}>
      <SignalsContent />
    </Suspense>
  );
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
