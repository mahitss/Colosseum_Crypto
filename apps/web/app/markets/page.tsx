'use client';

import * as React from 'react';
import { useQuery } from '@tanstack/react-query';
import { getMarkets, searchMarkets } from '@/lib/api-client';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { EmptyState } from '@/components/ui/empty-state';
import { ErrorState } from '@/components/ui/error-state';
import { formatDistanceToNow } from 'date-fns';
import Link from 'next/link';
import { Search } from 'lucide-react';

export default function MarketsPage() {
  const [searchQuery, setSearchQuery] = React.useState('');
  const [cursor, setCursor] = React.useState<string | undefined>();
  const debouncedSearch = useDebounce(searchQuery, 300);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['markets', { cursor, search: debouncedSearch }],
    queryFn: () => getMarkets({ cursor, search: debouncedSearch, limit: 20 }),
  });

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Markets</h1>
          <p className="text-muted-foreground mt-2">
            Prediction markets from the Panta ecosystem
          </p>
        </div>
      </div>

      {/* Search */}
      <div className="flex items-center gap-4">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search markets..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-10"
          />
        </div>
      </div>

      {/* Market Table */}
      <Card>
        <CardHeader>
          <CardTitle>All Markets</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} variant="default" />
              ))}
            </div>
          ) : error ? (
            <ErrorState
              title="Unable to load markets"
              description="There was an error fetching market data. Please try again."
              onRetry={() => refetch()}
            />
          ) : data?.markets.length === 0 ? (
            <EmptyState
              title="No markets found"
              description={debouncedSearch ? `No markets match "${debouncedSearch}"` : 'No markets are currently available.'}
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Market</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">YES</TableHead>
                  <TableHead className="text-right">NO</TableHead>
                  <TableHead className="text-right">24H Change</TableHead>
                  <TableHead className="text-right">Volume</TableHead>
                  <TableHead className="text-right">Updated</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data?.markets.map((market) => (
                  <TableRow key={market.id} className="cursor-pointer">
                    <TableCell>
                      <Link href={`/markets/${market.id}`} className="block">
                        <div className="font-medium">{market.title}</div>
                        {market.category && (
                          <div className="text-xs text-muted-foreground">{market.category}</div>
                        )}
                      </Link>
                    </TableCell>
                    <TableCell>
                      <Badge variant={getStatusVariant(market.status)}>
                        {market.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right font-medium">
                      {market.yes_probability ? `${parseFloat(market.yes_probability).toFixed(1)}%` : '—'}
                    </TableCell>
                    <TableCell className="text-right">
                      {market.no_probability ? `${parseFloat(market.no_probability).toFixed(1)}%` : '—'}
                    </TableCell>
                    <TableCell className="text-right text-muted-foreground">
                      {/* Change would need additional API data */}
                      —
                    </TableCell>
                    <TableCell className="text-right text-muted-foreground">
                      {market.volume_usdc ? formatVolume(market.volume_usdc) : '—'}
                    </TableCell>
                    <TableCell className="text-right text-muted-foreground text-sm">
                      {formatDistanceToNow(new Date(market.updated_at || market.created_at))} ago
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}

          {/* Pagination */}
          {data?.pagination.has_more && (
            <div className="mt-4 flex justify-center">
              <Button
                variant="outline"
                onClick={() => setCursor(data.pagination.cursor || undefined)}
              >
                Load More
              </Button>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

// Hook for debouncing
function useDebounce<T>(value: T, delay: number): T {
  const [debouncedValue, setDebouncedValue] = React.useState(value);
  React.useEffect(() => {
    const handler = setTimeout(() => setDebouncedValue(value), delay);
    return () => clearTimeout(handler);
  }, [value, delay]);
  return debouncedValue;
}

function getStatusVariant(status: string): 'default' | 'secondary' | 'success' | 'warning' | 'destructive' {
  switch (status.toUpperCase()) {
    case 'ACTIVE':
      return 'success';
    case 'CLOSED':
    case 'RESOLVED':
      return 'secondary';
    case 'PAUSED':
      return 'warning';
    default:
      return 'default';
  }
}

function formatVolume(volume: string): string {
  const num = parseFloat(volume);
  if (num >= 1e6) return `$${(num / 1e6).toFixed(1)}M`;
  if (num >= 1e3) return `$${(num / 1e3).toFixed(1)}K`;
  return `$${num.toFixed(0)}`;
}