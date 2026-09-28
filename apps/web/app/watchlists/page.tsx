import { Suspense } from 'react';
import { getWatchlists } from '@/lib/enterprise-api';
import { WatchlistsContent } from './watchlists-content';

export const revalidate = 30;

async function getWatchlistsData() {
  try {
    return await getWatchlists();
  } catch (error) {
    console.error('Failed to fetch watchlists:', error);
    return [];
  }
}

export default async function WatchlistsPage() {
  const watchlists = await getWatchlistsData();

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Watchlists</h1>
        <p className="text-muted-foreground mt-2">Track markets and monitor signals across your watchlists.</p>
      </div>

      <Suspense fallback={<WatchlistsSkeleton />}>
        <WatchlistsContent initialWatchlists={watchlists} />
      </Suspense>
    </div>
  );
}

function WatchlistsSkeleton() {
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      {Array.from({ length: 4 }).map((_, i) => (
        <div key={i} className="animate-pulse">
          <div className="h-24 bg-muted rounded-xl" />
        </div>
      ))}
    </div>
  );
}
