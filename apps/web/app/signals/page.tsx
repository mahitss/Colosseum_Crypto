import { Metadata } from 'next';
import { Suspense } from 'react';
import { getRadar, type RadarPage } from '@/lib/enterprise-api';
import { SignalRadar } from './radar-content';
import { Skeleton } from '@/components/ui/skeleton';

export const revalidate = 30;

export const metadata: Metadata = {
  title: 'Signal Radar — QEVRYN',
  description: 'Live feed of detected signal events across prediction markets.',
};

const RADAR_PAGE_SIZE = 50;

function RadarSkeleton() {
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="space-y-2">
          <Skeleton className="h-5 w-40 bg-slate-800" />
          <Skeleton className="h-3 w-64 bg-slate-800/70" />
        </div>
        <Skeleton className="h-4 w-28 bg-slate-800" />
      </div>
      <Skeleton className="h-9 w-full max-w-lg bg-slate-800" />
      <Skeleton className="h-11 w-full bg-slate-800/60" />
      <div className="overflow-hidden rounded-lg border border-slate-800">
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} className="flex items-center gap-4 border-b border-slate-800/80 px-3 py-3 last:border-b-0">
            <Skeleton className="h-3 w-16 shrink-0 bg-slate-800" />
            <Skeleton className="h-3 w-20 shrink-0 bg-slate-800" />
            <Skeleton className="h-3 w-40 shrink-0 bg-slate-800" />
            <Skeleton className="h-3 flex-1 bg-slate-800/60" />
            <Skeleton className="h-3 w-12 shrink-0 bg-slate-800" />
          </div>
        ))}
      </div>
    </div>
  );
}

async function RadarLoader() {
  // The client component owns filtering and pagination, so it only needs the
  // first page seeded. A failure here is not fatal: the radar falls back to a
  // cold client fetch which surfaces the error in-place rather than failing the
  // whole route.
  let initialData: RadarPage | null = null;
  try {
    initialData = await getRadar({ limit: RADAR_PAGE_SIZE });
  } catch {
    initialData = null;
  }

  return <SignalRadar initialData={initialData} />;
}

export default function SignalsPage() {
  return (
    <Suspense fallback={<RadarSkeleton />}>
      <RadarLoader />
    </Suspense>
  );
}