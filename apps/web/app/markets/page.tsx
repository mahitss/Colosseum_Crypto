'use client';

import * as React from 'react';
import { useQuery } from '@tanstack/react-query';
import { getMarkets } from '@/lib/api-client';
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
import { Search, Filter, ChevronDown, ChevronUp } from 'lucide-react';
import { cn } from '@/lib/cn';

export default function MarketsPage() {
  const [searchQuery, setSearchQuery] = React.useState('');
  const [cursor, setCursor] = React.useState<string | undefined>();
  const [statusFilter, setStatusFilter] = React.useState<string>('');
  const [categoryFilter, setCategoryFilter] = React.useState<string>('');
  const [sortBy, setSortBy] = React.useState<'updated' | 'volume' | 'probability'>('updated');
  const [sortDir, setSortDir] = React.useState<'asc' | 'desc'>('desc');
  const debouncedSearch = useDebounce(searchQuery, 300);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['markets', { cursor, search: debouncedSearch, status: statusFilter, category: categoryFilter, sortBy, sortDir }],
    queryFn: () => getMarkets({ cursor, search: debouncedSearch, status: statusFilter, category: categoryFilter, sortBy, sortDir, limit: 20 }),
  });

  const handleSort = (field: 'updated' | 'volume' | 'probability') => {
    if (sortBy === field) {
      setSortDir(sortDir === 'asc' ? 'desc' : 'asc');
    } else {
      setSortBy(field);
      setSortDir('desc');
    }
  };

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div className="min-w-0">
          <h1 className="text-3xl font-bold tracking-tight">Markets</h1>
          <p className="text-muted-foreground mt-1">Prediction markets from the Panta ecosystem</p>
        </div>
      </div>

      {/* Filters & Search */}
      <Card className="border-slate-800/60">
        <CardContent className="pt-6 pb-4">
          <div className="flex flex-col sm:flex-row gap-4 items-end">
            {/* Search */}
            <div className="relative flex-1 sm:max-w-md">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-slate-500" />
              <Input
                placeholder="Search markets..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-10"
              />
            </div>

            {/* Status Filter */}
            <div className="sm:w-40">
              <label htmlFor="status-filter" className="sr-only">Status</label>
              <select
                id="status-filter"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                className="w-full h-10 appearance-none rounded-md border border-slate-700 bg-slate-900 py-0 pl-3 pr-8 text-sm text-slate-100 outline-none transition-colors hover:border-slate-600 focus:border-emerald-500/60 focus:ring-1 focus:ring-emerald-500/25"
              >
                <option value="">All Statuses</option>
                <option value="ACTIVE">Active</option>
                <option value="CLOSED">Closed</option>
                <option value="RESOLVED">Resolved</option>
                <option value="PAUSED">Paused</option>
              </select>
            </div>

            {/* Category Filter */}
            <div className="sm:w-40">
              <label htmlFor="category-filter" className="sr-only">Category</label>
              <select
                id="category-filter"
                value={categoryFilter}
                onChange={(e) => setCategoryFilter(e.target.value)}
                className="w-full h-10 appearance-none rounded-md border border-slate-700 bg-slate-900 py-0 pl-3 pr-8 text-sm text-slate-100 outline-none transition-colors hover:border-slate-600 focus:border-emerald-500/60 focus:ring-1 focus:ring-emerald-500/25"
              >
                <option value="">All Categories</option>
                <option value="crypto">Crypto</option>
                <option value="sports">Sports</option>
                <option value="politics">Politics</option>
                <option value="tech">Technology</option>
                <option value="finance">Finance</option>
                <option value="entertainment">Entertainment</option>
                <option value="other">Other</option>
              </select>
            </div>

            {/* Sort */}
            <div className="flex items-center gap-2 sm:ml-auto">
              <label htmlFor="sort-by" className="sr-only">Sort by</label>
              <select
                id="sort-by"
                value={`${sortBy}:${sortDir}`}
                onChange={(e) => {
                  const [field, dir] = e.target.value.split(':');
                  setSortBy(field as 'updated' | 'volume' | 'probability');
                  setSortDir(dir as 'asc' | 'desc');
                }}
                className="h-10 appearance-none rounded-md border border-slate-700 bg-slate-900 py-0 pl-3 pr-8 text-sm text-slate-100 outline-none transition-colors hover:border-slate-600 focus:border-emerald-500/60 focus:ring-1 focus:ring-emerald-500/25"
              >
                <option value="updated:desc">Updated ↓</option>
                <option value="updated:asc">Updated ↑</option>
                <option value="volume:desc">Volume ↓</option>
                <option value="volume:asc">Volume ↑</option>
                <option value="probability:desc">Probability ↓</option>
                <option value="probability:asc">Probability ↑</option>
              </select>
              <Button variant="ghost" size="sm" onClick={() => { setSearchQuery(''); setStatusFilter(''); setCategoryFilter(''); setSortBy('updated'); setSortDir('desc'); }} className="h-10">
                Clear filters
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Market Table */}
      <Card className="border-slate-800/60">
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <CardTitle>All Markets</CardTitle>
            <div className="flex items-center gap-2 text-sm text-slate-500">
              {data?.pagination?.total !== undefined && (
                <span className="font-mono tabular-nums text-slate-400">
                  {data.pagination.total.toLocaleString()} total
                </span>
              )}
            </div>
          </CardHeader>
        </CardHeader>
        <CardContent className="pb-4">
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
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="cursor-pointer select-none" onClick={() => handleSort('updated')}>
                      <div className="flex items-center gap-1">
                        Market
                        {sortBy === 'updated' && (sortDir === 'desc' ? <ChevronDown className="h-3 w-3" /> : <ChevronUp className="h-3 w-3" />)}
                      </div>
                    </TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="cursor-pointer select-none" onClick={() => handleSort('probability')}>
                      <div className="flex items-center gap-1 justify-end">
                        YES
                        {sortBy === 'probability' && (sortDir === 'desc' ? <ChevronDown className="h-3 w-3" /> : <ChevronUp className="h-3 w-3" />)}
                      </div>
                    </TableHead>
                    <TableHead className="cursor-pointer select-none" onClick={() => handleSort('probability')}>
                      <div className="flex items-center gap-1 justify-end">
                        NO
                        {sortBy === 'probability' && (sortDir === 'desc' ? <ChevronDown className="h-3 w-3" /> : <ChevronUp className="h-3 w-3" />)}
                      </div>
                    </TableHead>
                    <TableHead className="cursor-pointer select-none" onClick={() => handleSort('volume')}>
                      <div className="flex items-center gap-1 justify-end">
                        Volume
                        {sortBy === 'volume' && (sortDir === 'desc' ? <ChevronDown className="h-3 w-3" /> : <ChevronUp className="h-3 w-3" />}
                      </div>
                    </TableHead>
                    <TableHead>Updated</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data?.markets.map((market) => (
                    <TableRow key={market.id} className="cursor-pointer hover:bg-slate-800/40 transition-colors" onClick={() => window.location.href = `/markets/${market.id}`}>
                      <TableCell className="max-w-md">
                        <Link href={`/markets/${market.id}`} className="block">
                          <div className="font-medium truncate">{market.title}</div>
                          {market.category && (
                            <div className="text-xs text-muted-foreground">{market.category}</div>
                          )}
                        </Link>
                      </TableCell>
                      <TableCell>
                        <Badge variant={getStatusVariant(market.status)} className="text-xs">{market.status}</Badge>
                      </TableCell>
                      <TableCell className="text-right font-medium tabular-nums">
                        {market.yes_probability ? `${parseFloat(market.yes_probability).toFixed(1)}%` : '—'}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {market.no_probability ? `${parseFloat(market.no_probability).toFixed(1)}%` : '—'}
                      </TableCell>
                      <TableCell className="text-right text-muted-foreground tabular-nums">
                        {market.volume_usdc ? formatVolume(market.volume_usdc) : '—'}
                      </TableCell>
                      <TableCell className="text-right text-muted-foreground text-sm">
                        {formatDistanceToNow(new Date(market.updated_at || market.created_at))} ago
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </>
          )}

          {/* Pagination */}
          {data?.pagination?.has_more && (
            <div className="mt-4 flex justify-center">
              <Button
                variant="outline"
                onClick={() => setCursor(data.pagination.cursor || undefined)}
                disabled={isLoading}
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
    case 'ACTIVE': return 'success';
    case 'CLOSED':
    case 'RESOLVED': return 'secondary';
    case 'PAUSED': return 'warning';
    default: return 'default';
  }
}

function formatVolume(volume: string): string {
  const num = parseFloat(volume);
  if (num >= 1e6) return `$${(num / 1e6).toFixed(1)}M`;
  if (num >= 1e3) return `$${(num / 1e3).toFixed(1)}K`;
  return `$${num.toFixed(0)}`;
}