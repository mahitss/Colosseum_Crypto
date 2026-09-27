'use client';

import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Wallet, RefreshCw } from 'lucide-react';
import { useWallet } from '@/components/wallet-provider';
import { useQuery } from '@tanstack/react-query';

interface Position {
  market_id: string;
  market_title?: string;
  side: string;
  quantity: string;
  value_usdc?: string;
  status: string;
}

export default function PortfolioPage() {
  const { available, connected, publicKey } = useWallet();
  const walletAddress = publicKey ?? '';

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['positions', walletAddress],
    queryFn: async (): Promise<{ positions: Position[] }> => {
      const res = await fetch(`/api/v1/trades/positions/${encodeURIComponent(walletAddress)}`);
      if (!res.ok) throw new Error('Failed to load positions');
      return res.json();
    },
    enabled: connected && !!walletAddress,
  });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Portfolio</h1>
          <p className="text-muted-foreground mt-2">Your prediction market positions.</p>
        </div>
        {connected && (
          <Button variant="outline" size="sm" onClick={() => refetch()} className="gap-2">
            <RefreshCw className="w-4 h-4" /> Refresh
          </Button>
        )}
      </div>

      {!available ? (
        <Card>
          <CardContent className="py-12 text-center">
            <p className="text-muted-foreground">
              Wallet connection is unavailable in this environment.
            </p>
          </CardContent>
        </Card>
      ) : !connected ? (
        <Card>
          <CardContent className="py-16 text-center">
            <Wallet className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold">Connect your wallet to view positions.</h3>
            <p className="text-sm text-muted-foreground mt-2">
              Your prediction market positions will appear here once connected.
            </p>
          </CardContent>
        </Card>
      ) : isLoading ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground">
            Loading positions...
          </CardContent>
        </Card>
      ) : error ? (
        <Card>
          <CardContent className="py-12 text-center">
            <p className="text-red-400">Unable to load positions.</p>
            <Button variant="outline" size="sm" className="mt-4" onClick={() => refetch()}>
              Retry
            </Button>
          </CardContent>
        </Card>
      ) : !data || data.positions.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground">
            No positions yet.
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>Positions</CardTitle>
            <CardDescription>Real position data from Panta</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {data.positions.map((pos, idx) => (
              <div
                key={`${pos.market_id}-${pos.side}-${idx}`}
                className="flex items-center justify-between rounded-lg border border-slate-800 p-4"
              >
                <div className="min-w-0">
                  <p className="font-medium truncate">{pos.market_title || pos.market_id}</p>
                  <p className="text-xs text-muted-foreground mt-1">
                    {pos.market_id}
                  </p>
                </div>
                <div className="flex items-center gap-6 text-right">
                  <div>
                    <p className="text-xs text-muted-foreground">Side</p>
                    <p className={`text-sm font-semibold ${pos.side === 'YES' ? 'text-emerald-400' : 'text-red-400'}`}>
                      {pos.side}
                    </p>
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground">Qty</p>
                    <p className="text-sm font-medium">{pos.quantity}</p>
                  </div>
                  {pos.value_usdc && (
                    <div>
                      <p className="text-xs text-muted-foreground">Value</p>
                      <p className="text-sm font-medium">{pos.value_usdc} USDC</p>
                    </div>
                  )}
                  <div>
                    <p className="text-xs text-muted-foreground">Status</p>
                    <p className="text-sm">{pos.status}</p>
                  </div>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}