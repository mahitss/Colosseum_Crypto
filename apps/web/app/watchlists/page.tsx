'use client';

import { useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Plus, Trash2 } from 'lucide-react';

interface Watchlist {
  id: string;
  name: string;
  markets: string[];
}

export default function WatchlistsPage() {
  const [watchlists, setWatchlists] = useState<Watchlist[]>([]);
  const [newName, setNewName] = useState('');

  const createWatchlist = () => {
    if (!newName.trim()) return;
    setWatchlists([...watchlists, { id: Date.now().toString(), name: newName, markets: [] }]);
    setNewName('');
  };

  const deleteWatchlist = (id: string) => {
    setWatchlists(watchlists.filter(w => w.id !== id));
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Watchlists</h1>
        <p className="text-muted-foreground mt-2">Track markets you are interested in.</p>
      </div>

      {/* Create new watchlist */}
      <Card>
        <CardHeader>
          <CardTitle>Create Watchlist</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex gap-2">
            <Input
              placeholder="Watchlist name..."
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && createWatchlist()}
            />
            <Button onClick={createWatchlist}>
              <Plus className="h-4 w-4" /> Create
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Watchlist list */}
      {watchlists.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground">
            No watchlists yet. Create one to start tracking markets.
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {watchlists.map((wl) => (
            <Card key={wl.id}>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-lg">{wl.name}</CardTitle>
                <div className="flex items-center gap-2">
                  <Badge variant="secondary">{wl.markets.length} markets</Badge>
                  <Button variant="ghost" size="icon" onClick={() => deleteWatchlist(wl.id)}>
                    <Trash2 className="h-4 w-4 text-danger" />
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                {wl.markets.length === 0 ? (
                  <p className="text-sm text-muted-foreground">Add markets to this watchlist</p>
                ) : (
                  <div className="space-y-1">
                    {wl.markets.map((m) => (
                      <div key={m} className="text-sm">{m}</div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <p className="text-xs text-muted-foreground">Watchlists are stored locally in your browser.</p>
    </div>
  );
}