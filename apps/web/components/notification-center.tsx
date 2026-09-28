'use client';

import * as React from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import { Bell, CheckCheck, X } from 'lucide-react';
import {
  getNotifications,
  getUnreadCount,
  markNotificationRead,
  markAllNotificationsRead,
  type AppNotification,
  type Severity,
} from '@/lib/enterprise-api';
import { formatSeverityLabel } from '@/lib/enterprise-utils';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/cn';

// Same visual language as the radar feed: a thin coloured bar carries severity,
// the text label is only the legend.
const SEVERITY_BAR: Record<Severity, string> = {
  CRITICAL: 'bg-rose-500',
  SIGNIFICANT: 'bg-amber-500',
  WATCH: 'bg-sky-500',
  INFO: 'bg-slate-500',
};

function relativeTime(iso: string): string {
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return '';
  const seconds = Math.max(0, Math.floor((Date.now() - then) / 1000));
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(then).toLocaleDateString();
}

export function NotificationCenter() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [open, setOpen] = React.useState(false);
  const containerRef = React.useRef<HTMLDivElement>(null);

  // The badge count comes from a real server query (GET /notifications/unread-count)
  // and is never derived from the list below: the list is capped at 20, so a
  // client-side tally would under-report and the badge would drift from truth.
  const { data: unread } = useQuery({
    queryKey: ['notifications', 'unread-count'],
    queryFn: getUnreadCount,
    refetchInterval: 30000,
  });
  const unreadCount = unread?.unread_count ?? 0;

  const { data: notifications, isLoading } = useQuery({
    queryKey: ['notifications'],
    queryFn: () => getNotifications(20),
    enabled: open,
  });

  React.useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('mousedown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open]);

  const markRead = useMutation({
    mutationFn: markNotificationRead,
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications', 'unread-count'] });
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
  });

  const markAllRead = useMutation({
    mutationFn: markAllNotificationsRead,
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications', 'unread-count'] });
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
  });

  const handleOpen = (notification: AppNotification) => {
    if (!notification.read_at) markRead.mutate(notification.id);
    setOpen(false);
    if (notification.market_id) router.push(`/markets/${notification.market_id}`);
  };

  const rows = notifications?.notifications ?? [];

  return (
    <div className="relative" ref={containerRef}>
      <button
        onClick={() => setOpen((v) => !v)}
        className="relative p-1.5 rounded-lg hover:bg-slate-800/50 text-slate-400 hover:text-white transition-colors"
        title="Notifications"
        aria-label={`Notifications (${unreadCount} unread)`}
        aria-expanded={open}
      >
        <Bell className="w-4 h-4" />
        {unreadCount > 0 && (
          <span className="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 px-1 rounded-full bg-rose-500 text-white text-[10px] font-semibold flex items-center justify-center">
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>

      {open && (
        <div className="absolute right-0 mt-2 w-96 z-50 rounded-xl border border-slate-800 bg-slate-900/95 backdrop-blur-sm shadow-2xl overflow-hidden animate-slide-in-from-top">
          <div className="flex items-center justify-between px-4 py-3 border-b border-slate-800/60">
            <span className="text-sm font-semibold text-white">Notifications</span>
            {unreadCount > 0 && (
              <button
                onClick={() => markAllRead.mutate()}
                disabled={markAllRead.isPending}
                className="flex items-center gap-1.5 text-xs text-slate-400 hover:text-white transition-colors disabled:opacity-50"
              >
                <CheckCheck className="w-3.5 h-3.5" />
                Mark all read
              </button>
            )}
          </div>

          <div className="max-h-96 overflow-y-auto">
            {isLoading ? (
              <div className="p-3 space-y-3">
                {[0, 1, 2, 3].map((i) => (
                  <div key={i} className="flex gap-2">
                    <Skeleton className="h-8 w-[3px] rounded-full" />
                    <div className="flex-1 space-y-1.5">
                      <Skeleton variant="text" />
                      <Skeleton className="h-3 w-1/3" />
                    </div>
                  </div>
                ))}
              </div>
            ) : rows.length === 0 ? (
              <div className="p-6 text-center">
                <p className="text-sm text-slate-400">
                  No alerts yet. Create a rule in Settings to get notified.
                </p>
              </div>
            ) : (
              rows.map((notification) => (
                <button
                  key={notification.id}
                  onClick={() => handleOpen(notification)}
                  className={cn(
                    'w-full text-left flex gap-2.5 px-4 py-3 border-b border-slate-800/60 transition-colors hover:bg-slate-800/50',
                    !notification.read_at && 'border-l-2 border-l-sky-500/60 bg-slate-800/30'
                  )}
                >
                  <span
                    className={cn('mt-0.5 h-8 w-[3px] shrink-0 rounded-full', SEVERITY_BAR[notification.severity as Severity])}
                    title={formatSeverityLabel(notification.severity)}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="flex items-baseline justify-between gap-2">
                      <span
                        className={cn(
                          'text-sm truncate',
                          notification.read_at ? 'text-slate-400' : 'text-white font-medium'
                        )}
                      >
                        {notification.title}
                      </span>
                      <span className="text-[11px] text-slate-500 shrink-0">
                        {relativeTime(notification.created_at)}
                      </span>
                    </span>
                    <span className="mt-0.5 block text-xs text-slate-400 leading-relaxed">
                      {notification.body}
                    </span>
                    <span className="mt-1 block text-[10px] uppercase tracking-wide text-slate-500">
                      {formatSeverityLabel(notification.severity)}
                    </span>
                  </span>
                </button>
              ))}
            )}
          </div>
        </div>
      )}
    </div>
  );
}