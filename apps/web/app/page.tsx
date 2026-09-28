'use client';

import { useQuery } from '@tanstack/react-query';
import { getSummaryStats, getRecentSignals } from '@/lib/api-client';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from '@/components/ui/table';
import { Skeleton } from '@/components/ui/skeleton';
import { formatDistanceToNow } from 'date-fns';
import Link from 'next/link';
import { cn } from '@/lib/cn';
import { ExternalLink, TrendingUp, Activity, AlertTriangle } from 'lucide-react';

function CommandCenterContent() {
  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: ['summary-stats'],
    queryFn: getSummaryStats,
    refetchInterval: 30000,
  });

  const { data: signals, isLoading: signalsLoading } = useQuery({
    queryKey: ['recent-signals'],
    queryFn: () => getRecentSignals(5),
    refetchInterval: 15000,
  });

  if (statsLoading || signalsLoading) {
    return (
      <div className="space-y-6 animate-fade-in">
        <div>
          <Skeleton variant="text" className="w-48 h-8" />
          <p className="text-sm text-muted-foreground mt-1">Loading command center...</p>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} variant="card" className="h-24" />
          ))}
        </div>
        <Skeleton variant="card" className="h-64" />
      </div>
    );
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div>
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold tracking-tight">Command Center</h1>
            <p className="text-muted-foreground mt-1">Prediction-market intelligence across the Panta ecosystem.</p>
          </div>
          <div className="flex items-center gap-2">
            <span className="hidden sm:inline-flex items-center gap-1.5 text-xs text-slate-400">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse-soft" />
              <span className="text-slate-500">Live</span>
            </span>
          </div>
        </div>
      </div>

      {/* Summary Strip */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <Card className="border-slate-800/60 hover:border-emerald-500/30 transition-colors">
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-muted-foreground">Markets Tracked</p>
                <p className="text-3xl font-bold mt-1 tabular-nums">{stats?.marketsTracked ?? '—'}</p>
              </div>
              <div className="w-10 h-10 rounded-lg bg-emerald-500/15 flex items-center justify-center">
                <TrendingUp className="w-5 h-5 text-emerald-400" />
              </div>
            </div>
          </CardContent>
        </Card>
        <Card className="border-slate-800/60 hover:border-amber-500/30 transition-colors">
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-muted-foreground">Active Signals</p>
                <p className="text-3xl font-bold mt-1 tabular-nums">{stats?.activeSignals ?? '—'}</p>
              </div>
              <div className="w-10 h-10 rounded-lg bg-amber-500/15 flex items-center justify-center">
                <Activity className="w-5 h-5 text-amber-400" />
              </div>
            </div>
          </CardContent>
        </Card>
        <Card className="border-slate-800/60 hover:border-rose-500/30 transition-colors">
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-muted-foreground">Significant Changes</p>
                <p className="text-3xl font-bold mt-1 tabular-nums">{stats?.significantChanges ?? '—'}</p>
              </div>
              <div className="w-10 h-10 rounded-lg bg-rose-500/15 flex items-center justify-center">
                <AlertTriangle className="w-5 h-5 text-rose-400" />
              </div>
            </div>
          </CardContent>
        </Card>
        <Card className="border-slate-800/60 hover:border-slate-700/60 transition-colors">
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-muted-foreground">Markets Updated</p>
                <p className="text-lg font-semibold mt-1">{stats?.marketsUpdated ? formatDistanceToNow(new Date(stats.marketsUpdated)) + ' ago' : '—'}</p>
              </div>
              <div className="w-10 h-10 rounded-lg bg-slate-700/50 flex items-center justify-center text-slate-500">
                <span className="text-xs">LIVE</span>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Signal Radar */}
      <Card className="border-slate-800/60">
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-base">Signal Radar</CardTitle>
              <CardDescription>Most recent significant signals from the Panta ecosystem</CardDescription>
            </div>
            <Link href="/signals" className="text-sm text-muted-foreground hover:text-white transition-colors flex items-center gap-1">
              View all
              <ExternalLink className="w-3 h-3" />
            </Link>
          </CardHeader>
          <CardContent>
            {!signals || signals.signals.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground">
                <div className="w-12 h-12 mx-auto mb-3 rounded-full bg-slate-800/50 flex items-center justify-center">
                  <Activity className="w-6 h-6 text-slate-500" />
                </div>
                <p className="text-sm">No signals yet. Data is being collected.</p>
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="h-8 border-b border-slate-800 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                      Market
                    </TableHead>
                    <TableHead className="h-8 border-b border-slate-800 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                      Signal
                    </TableHead>
                    <TableHead className="h-8 border-b border-slate-800 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                      Severity
                    </TableHead>
                    <TableHead className="h-8 border-b border-slate-800 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                      Previous → Current
                    </TableHead>
                    <TableHead className="h-8 border-b border-slate-800 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                      Change
                    </TableHead>
                    <TableHead className="h-8 border-b border-slate-800 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                      Time
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {signals.signals.map((signal) => (
                    <TableRow key={signal.id} className="cursor-pointer hover:bg-slate-800/40 transition-colors" onClick={() => signal.market_id && (window.location.href = `/markets/${signal.market_id}`)}>
                      <TableCell className="font-medium text-foreground">{signal.market_id}</TableCell>
                      <TableCell className="font-medium">{signal.signal_type.replace(/_/g, ' ')}</TableCell>
                      <TableCell>
                        <Badge variant={getSeverityVariant(signal.severity)} className="gap-1">
                          {signal.severity}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-col text-xs">
                          <span className="text-muted-foreground">{signal.previous_value ?? 'N/A'}</span>
                          <span className="text-muted-foreground">{signal.current_value ?? 'N/A'}</span>
                        </div>
                      </TableCell>
                      <TableCell className={signal.percentage_points && parseFloat(signal.percentage_points) > 0 ? 'text-emerald-400 font-medium' : 'text-rose-400 font-medium'}>
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