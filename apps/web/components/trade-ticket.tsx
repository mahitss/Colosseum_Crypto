'use client';

import { useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';

interface TradeState {
  status: 'IDLE' | 'QUOTING' | 'QUOTE_READY' | 'BUILDING' | 'READY_TO_SIGN' | 'SIGNING' | 'SIGNED' | 'BROADCASTING' | 'SUBMITTED' | 'CONFIRMING' | 'CONFIRMED' | 'REPORTING' | 'VERIFIED' | 'COMPLETED' | 'FAILED' | 'CANCELLED';
  side?: 'YES' | 'NO';
  amount?: string;
  quote?: any;
  error?: string;
}

export function TradeTicket({ market }: { market: any }) {
  const [tradeState, setTradeState] = useState<TradeState>({ status: 'IDLE' });
  const [amount, setAmount] = useState('');
  const [showReview, setShowReview] = useState(false);

  const handleSideSelect = async (side: 'YES' | 'NO') => {
    setTradeState({ status: 'IDLE', side });
    setShowReview(false);
  };

  const handleReviewTrade = async () => {
    if (!amount || !tradeState.side) return;
    
    setTradeState({ ...tradeState, status: 'QUOTING', amount });
    
    try {
      // Call backend quote endpoint
      const quoteRes = await fetch('/api/v1/trades/quote', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          market_id: market.id,
          side: tradeState.side,
          amount_usdc: amount,
          wallet_pubkey: '', // Will be populated from wallet adapter
        }),
      });

      if (!quoteRes.ok) {
        setTradeState({ status: 'FAILED', error: 'Quote failed' });
        return;
      }

      const quote = await quoteRes.json();
      setTradeState({ status: 'QUOTE_READY', side: tradeState.side, amount, quote });
      setShowReview(true);
    } catch (err) {
      setTradeState({ status: 'FAILED', error: String(err) });
    }
  };

  const handleSignAndBuy = async () => {
    if (tradeState.status !== 'QUOTE_READY') return;

    setTradeState({ ...tradeState, status: 'BUILDING' });
    
    try {
      // Call backend build endpoint
      const buildRes = await fetch('/api/v1/trades/build', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          quote_reference: tradeState.quote?.quote_reference,
          wallet_pubkey: '', // From wallet adapter
        }),
      });

      if (!buildRes.ok) {
        setTradeState({ status: 'FAILED', error: 'Build failed' });
        return;
      }

      const build = await buildRes.json();
      
      // Next: request wallet signature
      // This must be explicit and user-initiated
      setTradeState({ status: 'READY_TO_SIGN', side: tradeState.side, amount });
      
      // In production: call wallet adapter to sign transaction
      // DO NOT sign automatically
      
    } catch (err) {
      setTradeState({ status: 'FAILED', error: String(err) });
    }
  };

  return (
    <div className="space-y-4">
      {/* Trade Ticket */}
      <Card>
        <CardHeader>
          <CardTitle>Buy {tradeState.side || 'Position'}</CardTitle>
          <CardDescription>Market: {market.title}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Side Selection */}
          {tradeState.status === 'IDLE' && (
            <div className="flex gap-4">
              <Button
                variant={tradeState.side === 'YES' ? 'default' : 'outline'}
                onClick={() => handleSideSelect('YES')}
              >
                YES
              </Button>
              <Button
                variant={tradeState.side === 'NO' ? 'default' : 'outline'}
                onClick={() => handleSideSelect('NO')}
              >
                NO
              </Button>
            </div>
          )}

          {/* Amount Input */}
          {tradeState.side && (
            <div>
              <label className="text-sm font-medium">Amount (USDC)</label>
              <Input
                type="number"
                placeholder="10.00"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                className="mt-2"
              />
            </div>
          )}

          {/* Status Display */}
          {tradeState.status !== 'IDLE' && (
            <div className="space-y-2">
              <Badge variant="secondary">{tradeState.status}</Badge>
              {tradeState.error && (
                <div className="text-sm text-red-500">{tradeState.error}</div>
              )}
            </div>
          )}

          {/* Review Button */}
          {tradeState.side && amount && tradeState.status === 'IDLE' && (
            <Button onClick={handleReviewTrade} className="w-full">
              Review Trade
            </Button>
          )}
        </CardContent>
      </Card>

      {/* Review Modal */}
      {showReview && tradeState.status === 'QUOTE_READY' && (
        <Card className="border-2 border-emerald-500">
          <CardHeader>
            <CardTitle>Review Trade</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Market:</span>
                <span>{market.title}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Side:</span>
                <Badge>{tradeState.side}</Badge>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Amount:</span>
                <span>{amount} USDC</span>
              </div>
              {tradeState.quote && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Total Cost:</span>
                  <span>{tradeState.quote.total_cost} USDC</span>
                </div>
              )}
            </div>

            <div className="pt-4 flex gap-2">
              <Button
                variant="outline"
                onClick={() => {
                  setShowReview(false);
                  setTradeState({ status: 'IDLE' });
                }}
              >
                Cancel
              </Button>
              <Button
                onClick={handleSignAndBuy}
                className="flex-1"
              >
                Sign & Buy
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
