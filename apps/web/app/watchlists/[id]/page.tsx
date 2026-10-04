import { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { getWatchlistIntelligence } from '@/lib/enterprise-api';
import { WatchlistDetailContent } from './watchlist-detail-content';

export const revalidate = 30;

interface Props {
  params: Promise<{ id: string }>;
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;
  const { watchlist } = await getWatchlistIntelligence(id);
  return { title: `${watchlist.name} - QEVRYN` };
}

export default async function WatchlistDetailPage({ params }: Props) {
  const { id } = await params;
  const intelligence = await getWatchlistIntelligence(id);
  if (!intelligence) notFound();
  return <WatchlistDetailContent initialData={intelligence} watchlistId={id} />;
}
