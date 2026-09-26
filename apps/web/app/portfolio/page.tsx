'use client';

import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Wallet } from 'lucide-react';

export default function PortfolioPage() {
  const isConnected = false; // TODO: Connect to wallet state

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Portfolio</h1>
        <p className="text-muted-foreground mt-2">Your prediction market positions.</p>
      </div>

      {isConnected ? (
        <Card>
          <CardContent className="py-12">
            {/* Real positions would render here */}
            <p>Positions loaded from backend</p>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardContent className="py-16 text-center">
            <Wallet className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold">Connect your wallet to view positions.</h3>
            <p className="text-sm text-muted-foreground mt-2">
              Your prediction market positions will appear here once connected.
            </p>
          </CardContent>
        </Card>
      )}
    </div>
  );
}