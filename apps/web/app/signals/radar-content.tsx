'use client';

import * as React from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
  useInfiniteQuery,
  useQuery,
  type InfiniteData,
} from '@tanstack/react-query';
import { format, formatDistance } from 'date-fns';
import { ChevronDown, RefreshCw, RotateCcw, Search, X } from 'lucide-react';

import { getRadar, getWatchlists } from '@/lib/enterprise-api';
import type { RadarEvent, RadarPage, Severity, SignalType } from '@/lib/enterprise-api';
import {
  RADAR_TABS,
  formatProbability,
  formatSeverityLabel,
  type RadarTab,
} from '@/lib/enterprise-utils';
import { cn } from '@/lib/cn';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

const PAGE_SIZE = 50;
const REFETCH_INTERVAL_MS = 15_000;
const CLOCK_TICK_MS = 10_000;
const EM_DASH = '\u2014';

/**
 * Authoritative label map for the five signal types the gateway emits. Keying by
 * the `SignalType` union rather than a hand-written list means the select cannot
 * drift from the API contract: a new member of the union fails the build here
 * rather than silently rendering an unlabelled option.
 */
const SIGNAL_TYPE_LABELS: Record<SignalType, string> = {
  NEW_MARKET: 'New market',
  PROBABILITY_SHIFT: 'Probability shift',
  ACTIVITY_CHANGE: 'Activity change',
  LIQUIDITY_CHANGE: 'Liquidity change',
  MARKET_MOVEMENT: 'Market movement',
};

const SIGNAL_TYPES = Object.keys(SIGNAL_TYPE_LABELS) as SignalType[];

type TimeRange = '1h' | '24h' | '7d' | '30d' | 'all';

const TIME_RANGES: { id: TimeRange; label: string; windowMs: number | null }[] = [
  { id: '1h', label: 'Last hour', windowMs: 60 * 60 * 1000 },
  { id: '24h', label: 'Last 24h', windowMs: 24 * 60 * 60 * 1000 },
  { id: '7d', label: 'Last 7d', windowMs: 7 * 24 * 60 * 60 * 1000 },
  { id: '30d', label: 'Last 30d', windowMs: 30 * 24 * 60 * 60 * 1000 },
  { id: 'all', label: 'All time', windowMs: null },
];

// Severity is carried by a thin bar plus a muted label rather than a filled
// badge: a filled badge on every row turns the feed into a wall of colour and
// destroys the reading order. The bar is the signal, the label is the legend.
const SEVERITY_BAR: Record<Severity, string> = {
  CRITICAL: 'bg-rose-500',
  SIGNIFICANT: 'bg-amber-500',
  WATCH: 'bg-sky-500',
  INFO: 'bg-slate-500',
};

const SEVERITY_TEXT: Record<Severity, string> = {
  CRITICAL: 'text-rose-300',
  SIGNIFICANT: 'text-amber-300',
  WATCH: 'text-sky-300',
  INFO: 'text-slate-400',
};

// The gateway rejects a non-base58 market_id with a 400, so an in-flight
// half-typed address must not reach the wire as a filter.
const BASE58_ADDRESS = /^[1-9A-HJ-NP-Za-km-z]{32,44}$/;

const panelClass =
  'rounded-lg border border-slate-800 bg-slate-900/40';

const selectClass =
  'h-8 w-full appearance-none rounded-md border border-slate-700 bg-slate-900 py-0 pl-2.5 pr-7 text-[13px] text-slate-200 outline-none transition-colors hover:border-slate-600 focus:border-emerald-500/60 focus:ring-1 focus:ring-emerald-500/25';

/**
 * Narrow an arbitrary select value against the real option set. An unrecognised
 * value falls back to the unfiltered option rather than being forwarded to the
 * gateway, which rejects an unknown signal_type or an out-of-contract severity
 * with a 400.
 */
function toSignalType(value: string): SignalType | '' {
  return SIGNAL_TYPES.includes(value as SignalType) ? (value as SignalType) : '';
}

function toTimeRange(value: string): TimeRange {
  return TIME_RANGES.some((range) => range.id === value) ? (value as TimeRange) : 'all';
}

/** RFC3339 start of the selected window, or undefined for the unfiltered case. */
function timeRangeStart(range: TimeRange): string | undefined {
  const entry = TIME_RANGES.find((candidate) => candidate.id === range);
  if (!entry || entry.windowMs === null) return undefined;
  return new Date(Date.now() - entry.windowMs).toISOString();
}

export interface SignalRadarProps {
  initialData: RadarPage | null;
}

export function SignalRadar({ initialData }: SignalRadarProps) {
  const router = useRouter();

  const [tab, setTab] = React.useState<RadarTab>('all');
  const [marketIdInput, setMarketIdInput] = React.useState('');
  const [signalType, setSignalType] = React.useState<SignalType | ''>('');
  const [watchlistId, setWatchlistId] = React.useState('');
  const [timeRange, setTimeRange] = React.useState<TimeRange>('all');

  const debouncedMarketId = useDebouncedValue(marketIdInput, 350);
  const trimmedMarketId = debouncedMarketId.trim();
  const marketIdRejected = trimmedMarketId !== '' && !BASE58_ADDRESS.test(trimmedMarketId);
  const marketId = marketIdRejected ? '' : trimmedMarketId;

  const severity: Severity | undefined = tab === 'all' ? undefined : tab;

  const hasTimeFilter = timeRange !== 'all';

  const { data: watchlists } = useQuery({
    queryKey: ['watchlists'],
    queryFn: getWatchlists,
  });

  const query = useInfiniteQuery({
    // Keyed on the window selector, never on the resolved timestamp: the start
    // of the window is recomputed inside the fetcher so a feed left open keeps
    // returning a moving "last 24h" instead of freezing on the instant the
    // filter was picked, without the key churning on every poll.
    queryKey: ['radar', { severity: severity ?? null, signalType, marketId, watchlistId, timeRange }],
    queryFn: ({ pageParam }) =>
      getRadar({
        ...(severity ? { severity } : {}),
        ...(signalType ? { signal_type: signalType } : {}),
        ...(marketId ? { market_id: marketId } : {}),
        ...(watchlistId ? { watchlist_id: watchlistId } : {}),
        ...(timeRangeStart(timeRange) ? { from: timeRangeStart(timeRange) } : {}),
        limit: PAGE_SIZE,
        ...(pageParam ? { cursor: pageParam } : {}),
      }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.next_cursor ?? undefined,
    initialData: initialData
      ? ({
          pages: [initialData],
          pageParams: [undefined],
        } as InfiniteData<RadarPage, string | undefined>)
      : undefined,
    // Matched to the polling interval so the server-rendered first page is used
    // for one full cycle before the first client refetch, instead of being
    // thrown away by a duplicate request seconds after hydration.
    staleTime: REFETCH_INTERVAL_MS,
    refetchInterval: REFETCH_INTERVAL_MS,
  });

  const {
    data,
    isPending,
    isError,
    isFetching,
    isFetchingNextPage,
    hasNextPage,
    fetchNextPage,
    refetch,
    dataUpdatedAt,
  } = query;

  const events = React.useMemo(
    () => data?.pages.flatMap((page) => page.events) ?? [],
    [data],
  );

  // `total` is the count for the active filter, not a per-severity breakdown,
  // so it cannot be reused as a tab count. Tab counts are therefore counted off
  // the events actually in hand, and labelled as such in the UI.
  const serverTotal = data?.pages[0]?.total ?? null;

  const tabCounts = React.useMemo(() => {
    const counts: Record<Severity, number> = {
      CRITICAL: 0,
      SIGNIFICANT: 0,
      WATCH: 0,
      INFO: 0,
    };
    for (const event of events) {
      counts[event.severity] += 1;
    }
    return counts;
  }, [events]);

  const hasActiveFilters = Boolean(severity || signalType || marketId || watchlistId || hasTimeFilter);
  const hasLoadedEvents = events.length > 0;

  const now = useClockTick(CLOCK_TICK_MS);
  const lastUpdatedLabel = dataUpdatedAt
    ? formatDistance(dataUpdatedAt, now, { addSuffix: true })
    : null;

  const resetFilters = () => {
    setTab('all');
    setMarketIdInput('');
    setSignalType('');
    setWatchlistId('');
    setTimeRange('all');
  };

  const openMarket = (event: RadarEvent) => {
    if (!event.deep_link) return;
    router.push(event.deep_link);
  };

  const onRowKeyDown = (event: React.KeyboardEvent<HTMLTableRowElement>, row: RadarEvent) => {
    if (event.key !== 'Enter' && event.key !== ' ') return;
    event.preventDefault();
    openMarket(row);
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-xl font-semibold tracking-tight text-white">Signal Radar</h1>
          <p className="mt-1 text-[13px] text-slate-500">
            Deterministic signal events across tracked prediction markets.
          </p>
        </div>

        <div className="flex items-center gap-2">
          {serverTotal !== null && hasActiveFilters && (
            <span className="font-mono text-[11px] tabular-nums text-slate-500">
              {serverTotal.toLocaleString()} matching
            </span>
          )}
          <span
            className="inline-flex items-center gap-1.5 rounded-md border border-slate-800 bg-slate-900/60 px-2 py-1"
            title="The feed refreshes every 15 seconds"
          >
            <span className="relative flex h-1.5 w-1.5">
              {isFetching && (
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-60" />
              )}
              <span
                className={cn(
                  'relative inline-flex h-1.5 w-1.5 rounded-full',
                  isError && hasLoadedEvents ? 'bg-amber-500' : 'bg-emerald-400',
                )}
              />
            </span>
            <span className="font-mono text-[11px] text-slate-400">
              {lastUpdatedLabel ? `live \u00b7 updated ${lastUpdatedLabel}` : 'connecting'}
            </span>
          </span>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => refetch()}
            disabled={isFetching}
            aria-label="Refresh radar"
            className="h-7 w-7 text-slate-400 hover:bg-slate-800 hover:text-white"
          >
            <RefreshCw className={cn('h-3.5 w-3.5', isFetching && 'animate-spin')} />
          </Button>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <div
          role="tablist"
          aria-label="Filter by severity"
          className="flex flex-wrap items-center gap-0.5 rounded-lg border border-slate-800 bg-slate-900/60 p-0.5"
        >
          {RADAR_TABS.map((entry) => {
            const isActive = entry.id === tab;
            const count = entry.id === 'all' ? events.length : tabCounts[entry.id];
            return (
              <button
                key={entry.id}
                type="button"
                role="tab"
                aria-selected={isActive}
                onClick={() => setTab(entry.id)}
                className={cn(
                  'inline-flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-[13px] font-medium transition-colors',
                  isActive
                    ? 'bg-slate-800 text-white'
                    : 'text-slate-400 hover:bg-slate-800/50 hover:text-slate-200',
                )}
              >
                <span>{entry.label}</span>
                <span
                  className={cn(
                    'font-mono text-[11px] tabular-nums',
                    isActive ? 'text-slate-400' : 'text-slate-600',
                  )}
                >
                  {count}
                </span>
              </button>
            );
          })}
        </div>
        <span className="text-[11px] text-slate-600">counts reflect loaded events</span>
      </div>

      <div className={cn(panelClass, 'flex flex-wrap items-center gap-2 p-2')}>
        <div className="relative w-full sm:w-60">
          <label htmlFor="radar-market-id" className="sr-only">
            Filter by market id
          </label>
          <Input
            id="radar-market-id"
            value={marketIdInput}
            onChange={(event) => setMarketIdInput(event.target.value)}
            placeholder="Market id (base58)"
            spellCheck={false}
            autoComplete="off"
            className="h-8 border-slate-700 bg-slate-900 pl-7 pr-7 font-mono text-[12px] text-slate-100 placeholder:font-sans placeholder:text-slate-600 focus-visible:ring-1 focus-visible:ring-emerald-500/30"
          />
          <Search className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-600" />
          {marketIdInput && (
            <button
              type="button"
              onClick={() => setMarketIdInput('')}
              aria-label="Clear market id"
              className="absolute right-1.5 top-1/2 -translate-y-1/2 rounded p-0.5 text-slate-500 transition-colors hover:bg-slate-800 hover:text-slate-200"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>

        <FilterSelect
          label="Signal type"
          value={signalType}
          onValueChange={(value) => setSignalType(toSignalType(value))}
          className="w-full sm:w-44"
        >
          <option value="">All signal types</option>
          {SIGNAL_TYPES.map((type) => (
            <option key={type} value={type}>
              {SIGNAL_TYPE_LABELS[type]}
            </option>
          ))}
        </FilterSelect>

        <FilterSelect
          label="Watchlist"
          value={watchlistId}
          onValueChange={setWatchlistId}
          className="w-full sm:w-44"
        >
          <option value="">All watchlists</option>
          {(watchlists ?? []).map((watchlist) => (
            <option key={watchlist.id} value={watchlist.id}>
              {watchlist.name}
            </option>
          ))}
        </FilterSelect>

        <FilterSelect
          label="Time range"
          value={timeRange}
          onValueChange={(value) => setTimeRange(toTimeRange(value))}
          className="w-full sm:w-36"
        >
          {TIME_RANGES.map((range) => (
            <option key={range.id} value={range.id}>
              {range.label}
            </option>
          ))}
        </FilterSelect>

        <div className="ml-auto flex items-center gap-2">
          {marketIdRejected && (
            <span className="text-[11px] text-rose-400">
              Not a base58 market address
            </span>
          )}
          {hasActiveFilters && (
            <Button
              variant="ghost"
              size="sm"
              onClick={resetFilters}
              className="h-7 gap-1.5 px-2 text-[12px] text-slate-400 hover:bg-slate-800 hover:text-white"
            >
              <RotateCcw className="h-3 w-3" />
              Reset
            </Button>
          )}
        </div>
      </div>

      {isError && hasLoadedEvents && (
        <div className="rounded-md border border-amber-500/25 bg-amber-500/5 px-3 py-2 text-[12px] text-amber-300">
          Refresh failed. Showing the last successful update.
        </div>
      )}

      <div className="overflow-hidden rounded-lg border border-slate-800 bg-slate-900/30">
        {isPending && !hasLoadedEvents ? (
          <RadarRowsSkeleton />
        ) : isError && !hasLoadedEvents ? (
          <FeedPanel
            title="Unable to load the radar"
            description="The signal feed could not be reached. Check the gateway connection and retry."
            action={
              <Button
                variant="outline"
                size="sm"
                onClick={() => refetch()}
                className="mt-4 h-8 border-slate-700 bg-slate-900 px-3 text-[12px] text-slate-200 hover:bg-slate-800 hover:text-white"
              >
                Retry
              </Button>
            }
          />
        ) : !hasLoadedEvents ? (
          hasActiveFilters ? (
            <FeedPanel
              title="No events match these filters"
              description="Nothing in the feed fits this combination of severity, signal type, market, watchlist and time range. Widen the time range, clear the market or watchlist filter, or switch back to All."
            />
          ) : (
            <FeedPanel
              title="No signal events yet"
              description="The radar is empty because no signal has been ingested, not because anything is filtered out. Events appear here as soon as the intelligence pipeline writes its first detection."
            />
          )
        ) : (
          <>
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="h-8 border-b border-slate-800 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                    Time
                  </TableHead>
                  <TableHead className="h-8 border-b border-slate-800 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                    Severity
                  </TableHead>
                  <TableHead className="h-8 border-b border-slate-800 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                    Market
                  </TableHead>
                  <TableHead className="h-8 border-b border-slate-800 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                    Signal
                  </TableHead>
                  <TableHead className="h-8 min-w-[20rem] border-b border-slate-800 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                    Explanation
                  </TableHead>
                  <TableHead className="h-8 border-b border-slate-800 px-3 text-right text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                    YES
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {events.map((event) => (
                  <TableRow
                    key={event.id}
                    tabIndex={0}
                    onClick={() => openMarket(event)}
                    onKeyDown={(keyEvent) => onRowKeyDown(keyEvent, event)}
                    className="cursor-pointer border-b border-slate-800/70 transition-colors hover:bg-slate-800/40 focus-visible:bg-slate-800/40 focus-visible:outline-none"
                  >
                    <TableCell className="p-0 px-3 py-2 align-middle">
                      <div className="font-mono text-[12px] tabular-nums text-slate-300">
                        {formatDistance(new Date(event.observed_at), now, { addSuffix: true })}
                      </div>
                      <div className="font-mono text-[10.5px] tabular-nums text-slate-600">
                        {format(new Date(event.observed_at), 'HH:mm:ss')}
                      </div>
                    </TableCell>

                    <TableCell className="p-0 px-3 py-2 align-middle">
                      <span className="inline-flex items-center gap-2">
                        <span
                          aria-hidden
                          className={cn('h-3.5 w-[3px] shrink-0 rounded-full', SEVERITY_BAR[event.severity])}
                        />
                        <span
                          className={cn('text-[13px] font-medium', SEVERITY_TEXT[event.severity])}
                        >
                          {formatSeverityLabel(event.severity)}
                        </span>
                      </span>
                    </TableCell>

                    <TableCell className="max-w-[22rem] p-0 px-3 py-2 align-middle">
                      <Link
                        href={event.deep_link}
                        onClick={(clickEvent) => clickEvent.stopPropagation()}
                        className="block truncate text-[13px] font-medium text-slate-100 transition-colors hover:text-emerald-300"
                        title={event.market_summary.title}
                      >
                        {event.market_summary.title}
                      </Link>
                      <div className="mt-0.5 truncate font-mono text-[10.5px] text-slate-600">
                        {event.market_summary.id}
                      </div>
                    </TableCell>

                    <TableCell className="p-0 px-3 py-2 align-middle">
                      <div className="text-[13px] text-slate-300">
                        {SIGNAL_TYPE_LABELS[event.signal_type]}
                      </div>
                      {event.metric && (
                        <div className="mt-0.5 font-mono text-[10.5px] uppercase tracking-wide text-slate-600">
                          {event.metric}
                        </div>
                      )}
                    </TableCell>

                    <TableCell className="min-w-[20rem] p-0 px-3 py-2 align-middle">
                      {event.explanation ? (
                        <p className="text-[13px] leading-relaxed text-slate-200">
                          {event.explanation}
                        </p>
                      ) : (
                        <span className="text-[13px] text-slate-600">{EM_DASH}</span>
                      )}
                    </TableCell>

                    <TableCell className="p-0 px-3 py-2 text-right align-middle font-mono text-[13px] tabular-nums text-slate-200">
                      {formatProbability(event.market_summary.yes_probability)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>

            {hasNextPage && (
              <div className="flex items-center justify-center border-t border-slate-800 px-3 py-2.5">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => fetchNextPage()}
                  disabled={isFetchingNextPage}
                  className="h-8 border-slate-700 bg-slate-900 px-3 text-[12px] text-slate-300 hover:bg-slate-800 hover:text-white"
                >
                  {isFetchingNextPage ? 'Loading' : 'Load more'}
                </Button>
              </div>
            )}
          </>
        )}
      </div>

      {hasLoadedEvents && (
        <div className="flex flex-wrap items-center justify-between gap-2 text-[11px] text-slate-600">
          <span className="font-mono tabular-nums">
            {events.length.toLocaleString()} loaded
            {serverTotal !== null ? ` of ${serverTotal.toLocaleString()}` : ''}
          </span>
          <span>Ordered by observed_at, newest first. Click a row to open the market.</span>
        </div>
      )}
    </div>
  );
}

interface FilterSelectProps
  extends Omit<React.SelectHTMLAttributes<HTMLSelectElement>, 'onChange'> {
  label: string;
  onValueChange: (value: string) => void;
}

function FilterSelect({
  label,
  className,
  children,
  onValueChange,
  ...props
}: FilterSelectProps) {
  return (
    <div className={cn('relative', className)}>
      <label className="sr-only">{label}</label>
      <select
        {...props}
        onChange={(event) => onValueChange(event.target.value)}
        className={cn(selectClass, className)}
      >
        {children}
      </select>
      <ChevronDown className="pointer-events-none absolute right-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-600" />
    </div>
  );
}

function FeedPanel({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center px-6 py-14 text-center">
      <h3 className="text-[15px] font-semibold text-slate-200">{title}</h3>
      <p className="mt-2 max-w-md text-[13px] leading-relaxed text-slate-500">{description}</p>
      {action}
    </div>
  );
}

function RadarRowsSkeleton() {
  return (
    <div>
      {Array.from({ length: 10 }).map((_, index) => (
        <div
          key={index}
          className="flex items-center gap-4 border-b border-slate-800/70 px-3 py-2.5 last:border-b-0"
        >
          <Skeleton className="h-3 w-16 shrink-0 bg-slate-800" />
          <Skeleton className="h-3 w-20 shrink-0 bg-slate-800" />
          <Skeleton className="h-3 w-44 shrink-0 bg-slate-800" />
          <Skeleton className="h-3 w-28 shrink-0 bg-slate-800/70" />
          <Skeleton className="h-3 flex-1 bg-slate-800/50" />
          <Skeleton className="h-3 w-10 shrink-0 bg-slate-800" />
        </div>
      ))}
    </div>
  );
}

function useDebouncedValue<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = React.useState(value);

  React.useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);

  return debounced;
}

function useClockTick(intervalMs: number): number {
  const [tick, setTick] = React.useState(() => Date.now());

  React.useEffect(() => {
    const timer = setInterval(() => setTick(Date.now()), intervalMs);
    return () => clearInterval(timer);
  }, [intervalMs]);

  return tick;
}

export default SignalRadar;