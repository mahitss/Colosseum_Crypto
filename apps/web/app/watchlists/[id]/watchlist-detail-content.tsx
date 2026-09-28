'use client';

import * as React from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import Link from 'next/link';
import { formatDistanceToNow } from 'date-fns';
import { ArrowLeft, RefreshCw, X, Clock } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { EmptyState } from '@/components/ui/empty-state';
import { ErrorState } from '@/components/ui/error-state';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/ui/table';
import { getWatchlistIntelligence, removeMarketFromWatchlist } from '@/lib/enterprise-api';
import type { WatchlistIntelligence, Severity } from '@/lib/enterprise-api';
import { formatProbability, formatSeverityLabel, SEVERITY_ORDER } from '@/lib/enterprise-utils';

const SEVERITY_COLORS: Record<Severity, string> = {
  CRITICAL: 'bg-danger',
  SIGNIFICANT: 'bg-warning',
  WATCH: 'bg-info',
  INFO: 'bg-success',
};

const SEVERITY_VARIANTS: Record<Severity, 'critical' | 'warning' | 'info' | 'success'> = {
  CRITICAL: 'critical',
  SIGNIFICANT: 'warning',
  WATCH: 'info',
  INFO: 'success',
};

export interface WatchlistDetailContentProps {
  initialData: WatchlistIntelligence;
  watchlistId: string;
}

export function WatchlistDetailContent({ initialData, watchlistId }: WatchlistDetailContentProps) {
  const queryClient = useQueryClient();

  const { data, isLoading, error, refetch, isFetching } = useQuery({
    queryKey: ['watchlist-intel', watchlistId],
    queryFn: () => getWatchlistIntelligence(watchlistId),
    initialData,
    refetchInterval: 30_000,
  });

  const removeMutation = useMutation({
    mutationFn: (marketId: string) => removeMarketFromWatchlist(watchlistId, marketId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['watchlist-intel', watchlistId] });
    },
  });

  const handleRemove = (marketId: string) => {
    if (window.confirm('Remove this market from the watchlist?')) {
      removeMutation.mutate(marketId);
    }
  };

  const relative = (timestamp: string | null): string => {
    if (!timestamp) return 'Never';
    try {
      return formatDistanceToNow(new Date(timestamp), { addSuffix: true });
    } catch {
      return 'Unknown';
    }
  };

  if (isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-1/3" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (error) {
    return (
      <ErrorState
        title="Unable to load watchlist"
        description="There was an error fetching this watchlist intelligence. Please try again."
        onRetry={() => refetch()}
      />
    );
  }

  const { watchlist, markets, recent_signals, severity_distribution, latest_update } = data;
  const totalSignals = SEVERITY_ORDER.reduce((sum, s) => sum + (severity_distribution[s] ?? 0), 0);
  const recent = recent_signals.slice(0, 10);

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <Link
            href="/watchlists"
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
            Back to watchlists
          </Link>
          <h1 className="mt-2 text-2xl font-bold text-foreground truncate">{watchlist.name}</h1>
          {watchlist.description && (
            <p className="mt-1 text-sm text-muted-foreground">{watchlist.description}</p>
          )}
          <div className="mt-2 flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            <span className="font-medium text-foreground">{data.market_count}</span>
            <span>{data.market_count === 1 ? 'market' : 'markets'}</span>
            <span className="inline-flex items-center gap-1">
              <Clock className="h-3.5 w-3.5" />
              Updated {relative(latest_update)}
            </span>
          </div>
        </div>
        <Button
          variant="outline"
          size="icon"
          onClick={() => refetch()}
          disabled={isFetching}
          aria-label="Refresh"
          className="self-start"
        >
          <RefreshCw className={`h-4 w-4 ${isFetching ? 'animate-spin' : ''}`} />
        </Button>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Severity distribution</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {totalSignals === 0 ? (
            <div className="h-2 w-full rounded-full bg-surface" />
          ) : (
            <div className="flex h-2 w-full overflow-hidden rounded-full bg-surface">
              {SEVERITY_ORDER.map((severity) => {
                const count = severity_distribution[severity] ?? 0;
                if (count === 0) return null;
                return (
                  <div
                    key={severity}
                    className={SEVERITY_COLORS[severity]}
                    style={{ width: `${(count / totalSignals) * 100}%` }}
                  />
                );
              })}
            </div>
          )}
          <div className="flex flex-wrap gap-x-4 gap-y-1">
            {SEVERITY_ORDER.map((severity) => (
              <div key={severity} className="flex items-center gap-1.5 text-sm text-muted-foreground">
                <span className={`h-2.5 w-2.5 rounded-full ${SEVERITY_COLORS[severity]}`} />
                <span className="font-medium text-foreground">{severity_distribution[severity] ?? 0}</span>
                <span>{formatSeverityLabel(severity)}</span>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Markets</CardTitle>
        </CardHeader>
        <CardContent>
          {markets.length === 0 ? (
            <EmptyState
              title="No markets in this watchlist"
              description="Add markets to this watchlist to start tracking their signals."
              icon={<X className="h-12 w-12 text-muted-foreground/50" />}
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Market</TableHead>
                  <TableHead>Category</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">YES</TableHead>
                  <TableHead className="text-right">NO</TableHead>
                  <TableHead>Latest Signal</TableHead>
                  <TableHead className="w-10" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {markets.map((market) => (
                  <TableRow key={market.id}>
                    <TableCell className="max-w-md">
                      <Link
                        href={`/markets/${market.id}`}
                        className="font-medium text-foreground hover:text-primary transition-colors"
                        title={market.title}
                      >
                        {market.title.length > 60 ? `${market.title.slice(0, 60)}…` : market.title}
                      </Link>
                    </TableCell>
                    <TableCell>
                      {market.category ? (
                        <Badge variant="secondary">{market.category}</Badge>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{market.status}</Badge>
                    </TableCell>
                    <TableCell className="text-right font-mono text-sm">
                      {formatProbability(market.yes_probability)}
                    </TableCell>
                    <TableCell className="text-right font-mono text-sm">
                      {formatProbability(market.no_probability)}
                    </TableCell>
                    <TableCell>
                      {market.latest_signal ? (
                        <Badge variant={SEVERITY_VARIANTS[market.latest_signal.severity]}>
                          {formatSeverityLabel(market.latest_signal.severity)}
                        </Badge>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="text-muted-foreground hover:text-danger hover:bg-danger/10"
                        onClick={() => handleRemove(market.id)}
                        disabled={removeMutation.isPending}
                        aria-label="Remove market"
                      >
                        <X className="h-4 w-4" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {recent.length > 0 && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Recent signals</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {recent.map((signal) => (
              <div
                key={signal.id}
                className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border bg-surface/40 px-3 py-2"
              >
                <div className="flex items-center gap-2">
                  <Badge variant={SEVERITY_VARIANTS[signal.severity]}>
                    {formatSeverityLabel(signal.severity)}
                  </Badge>
                  <span className="text-sm text-foreground">
                    {signal.signal_type.replace(/_/g, ' ').toLowerCase()}
                  </span>
                </div>
                <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                  <Clock className="h-3 w-3" />
                  {relative(signal.observed_at)}
                </span>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}

export default WatchlistDetailContent;
