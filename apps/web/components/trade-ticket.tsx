'use client';

import { useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { useWallet } from '@/components/wallet-provider';

// Explicit trade lifecycle states — never reduced to loading=true/false.
type TradeStatus =
  | 'IDLE'
  | 'QUOTING'
  | 'QUOTE_READY'
  | 'BUILDING'
  | 'READY_TO_SIGN'
  | 'SIGNING'
  | 'SIGNED'
  | 'BROADCASTING'
  | 'SUBMITTED'
  | 'CONFIRMING'
  | 'CONFIRMED'
  | 'REPORTING'
  | 'VERIFIED'
  | 'POSITION_REFRESHING'
  | 'COMPLETED'
  | 'CANCELLED'
  | 'FAILED'
  | 'UNKNOWN';

interface TradeError {
  code: string;
  message: string;
}

const STEP_LABELS: Partial<Record<TradeStatus, string>> = {
  QUOTING: 'Requesting quote',
  QUOTE_READY: 'Quote received',
  BUILDING: 'Building transaction',
  READY_TO_SIGN: 'Ready to sign',
  SIGNING: 'Waiting for your signature',
  SIGNED: 'Transaction signed',
  BROADCASTING: 'Submitting to Solana',
  SUBMITTED: 'Submitted to Solana',
  CONFIRMING: 'Confirming on Solana',
  CONFIRMED: 'Confirmed on Solana',
  REPORTING: 'Reporting to Panta',
  VERIFIED: 'Verified by Panta',
  POSITION_REFRESHING: 'Refreshing position',
  COMPLETED: 'Trade confirmed',
};

const SUCCESS_STEPS: TradeStatus[] = ['QUOTE_READY', 'BUILDING', 'SIGNED', 'SUBMITTED', 'CONFIRMED', 'VERIFIED', 'POSITION_REFRESHING', 'COMPLETED'];

export function TradeTicket({ market }: { market: any }) {
  const wallet = useWallet();
  const [status, setStatus] = useState<TradeStatus>('IDLE');
  const [side, setSide] = useState<'YES' | 'NO' | null>(null);
  const [amount, setAmount] = useState('');
  const [tradeAttemptID, setTradeAttemptID] = useState('');
  const [quote, setQuote] = useState<any>(null);
  const [signature, setSignature] = useState<string | null>(null);
  const [position, setPosition] = useState<any>(null);
  const [error, setError] = useState<TradeError | null>(null);

  const { connected, publicKey, signTransaction, available } = wallet;
  const walletAddress = publicKey ?? '';
  const shortAddress = walletAddress ? `${walletAddress.slice(0, 3)}...${walletAddress.slice(-3)}` : '';

  const transition = (next: TradeStatus) => {
    setStatus(next);
    setError(null);
  };

  const handleQuote = async () => {
    if (!connected || !publicKey) {
      setError({ code: 'WALLET_NOT_CONNECTED', message: 'Connect wallet to trade.' });
      return;
    }
    if (!side || !amount) {
      setError({ code: 'INVALID_REQUEST', message: 'Select a side and enter an amount.' });
      return;
    }
    transition('QUOTING');
    try {
      const res = await fetch('/api/v1/trades/quote', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ market_id: market.id, side, amount_usdc: amount, wallet_pubkey: publicKey }),
      });
      if (!res.ok) {
        setError({ code: 'QUOTE_FAILED', message: 'Quote failed. Try again.' });
        setStatus('FAILED');
        return;
      }
      const data = await res.json();
      setQuote(data);
      setTradeAttemptID(data.trade_attempt_id);
      transition('QUOTE_READY');
    } catch {
      setError({ code: 'QUOTE_FAILED', message: 'Quote failed. Try again.' });
      setStatus('FAILED');
    }
  };

  const handleSignAndBuy = async () => {
    if (!connected || !publicKey || !signTransaction) {
      setError({ code: 'WALLET_NOT_CONNECTED', message: 'Connect wallet to trade.' });
      setStatus('FAILED');
      return;
    }
    if (!side || !amount || !tradeAttemptID) {
      setError({ code: 'INVALID_REQUEST', message: 'Missing trade parameters.' });
      setStatus('FAILED');
      return;
    }
    setError(null);
    transition('BUILDING');

    try {
      // STEP 2 — Build unsigned transaction from Panta.
      const buildRes = await fetch('/api/v1/trades/build', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ quote_reference: quote?.quote_reference, wallet_pubkey: publicKey }),
      });
      if (!buildRes.ok) {
        setError({ code: 'BUILD_FAILED', message: 'Could not build transaction.' });
        setStatus('FAILED');
        return;
      }
      const built = await buildRes.json();

      // STEP 3 — Validate transaction context before requesting a signature.
      const validationRes = await fetch('/api/v1/trades/validate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          trade_attempt_id: tradeAttemptID,
          wallet_pubkey: publicKey,
          market_id: market.id,
          side,
          amount_usdc: amount,
          expected_wallet: built.expected_wallet,
          expected_network: built.expected_network,
          actual_network: process.env.NEXT_PUBLIC_SOLANA_NETWORK || 'mainnet-beta',
        }),
      });
      if (!validationRes.ok) {
        setError({ code: 'INVALID_TRANSACTION', message: 'Transaction context validation failed.' });
        setStatus('FAILED');
        return;
      }

      // STEP 4 — Explicit user signature (user clicked "Sign & Buy").
      transition('READY_TO_SIGN');
      let signedTx: { serialize(opt?: unknown): Uint8Array };
      try {
        signedTx = await signTransaction(Buffer.from(built.transaction_data, 'base64'));
      } catch (err: any) {
        if (err?.code === 4001 || err?.message?.includes('rejected')) {
          setError({ code: 'USER_REJECTED', message: 'You rejected the signature request.' });
        } else {
          setError({ code: 'SIGNING_FAILED', message: err?.message || 'Signature failed.' });
        }
        setStatus('CANCELLED');
        return;
      }
      transition('SIGNED');

      // STEP 5 — Broadcast through configured server-side Solana RPC.
      transition('BROADCASTING');
      const broadcastRes = await fetch('/api/v1/trades/broadcast', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          trade_attempt_id: tradeAttemptID,
          signed_tx: Buffer.from(signedTx.serialize({ requireAllSignatures: false })).toString('base64'),
          wallet_pubkey: publicKey,
        }),
      });
      if (!broadcastRes.ok) {
        setError({ code: 'BROADCAST_FAILED', message: 'Transaction could not be broadcast.' });
        setStatus('FAILED');
        return;
      }
      const broadcast = await broadcastRes.json();
      setSignature(broadcast.signature);
      transition('SUBMITTED');

      // STEP 6 — Confirmation on Solana.
      transition('CONFIRMING');
      const confirmRes = await fetch('/api/v1/trades/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ trade_attempt_id: tradeAttemptID, signature: broadcast.signature }),
      });
      if (!confirmRes.ok) {
        setError({
          code: 'CONFIRMATION_TIMEOUT',
          message: 'Transaction submitted but confirmation timed out. It may still settle.',
        });
        setStatus('UNKNOWN');
        return;
      }
      const confirmed = await confirmRes.json();
      transition('CONFIRMED');

      // STEP 7 — Report + verify with Panta.
      transition('REPORTING');
      const reportRes = await fetch('/api/v1/trades/report', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ trade_attempt_id: tradeAttemptID, signature: broadcast.signature }),
      });
      if (!reportRes.ok) {
        // Critical: do NOT tell the user the trade failed if confirmed on Solana.
        setError({
          code: 'PANTA_REPORT_FAILED',
          message: 'Transaction confirmed on Solana. Panta verification is pending.',
        });
        setStatus('CONFIRMED');
        return;
      }
      transition('VERIFIED');

      // STEP 8 — Refresh position from authoritative Panta data.
      transition('POSITION_REFRESHING');
      const posRes = await fetch(`/api/v1/trades/positions/${encodeURIComponent(publicKey)}`);
      if (posRes.ok) {
        const pos = await posRes.json();
        setPosition(pos.positions?.[0] ?? null);
      }
      transition('COMPLETED');
    } catch (err: any) {
      setError({ code: 'TRADE_FAILED', message: err?.message || 'Trade failed.' });
      setStatus('FAILED');
    }
  };

  // Honest states: wallet unavailable vs disconnected.
  if (!available) {
    return (
      <Card>
        <CardContent className="py-8 text-center">
          <p className="text-muted-foreground">Wallet connection is unavailable in this environment.</p>
        </CardContent>
      </Card>
    );
  }

  if (!connected || !publicKey) {
    return (
      <Card>
        <CardContent className="py-8 text-center">
          <p className="text-muted-foreground">Connect wallet to trade.</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Trade</CardTitle>
          <CardDescription>{market.title}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Side selection */}
          <div className="flex gap-4">
            <Button variant={side === 'YES' ? 'default' : 'outline'} onClick={() => { setSide('YES'); setStatus('IDLE'); setQuote(null); }} disabled={status !== 'IDLE'}>
              YES
            </Button>
            <Button variant={side === 'NO' ? 'default' : 'outline'} onClick={() => { setSide('NO'); setStatus('IDLE'); setQuote(null); }} disabled={status !== 'IDLE'}>
              NO
            </Button>
          </div>

          {/* Amount */}
          {side && status === 'IDLE' && (
            <div>
              <label className="text-sm font-medium text-muted-foreground">Amount (USDC)</label>
              <Input
                type="number"
                inputMode="decimal"
                min="0.01"
                step="0.01"
                placeholder="10.00"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                className="mt-2"
              />
            </div>
          )}

          {/* Quote */}
          {side && amount && status === 'IDLE' && (
            <Button onClick={handleQuote} className="w-full">
              Review Trade
            </Button>
          )}

          {/* Quote result */}
          {quote && status === 'QUOTE_READY' && (
            <div className="rounded-lg border border-slate-800 p-4 space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">Side</span>
                <Badge variant={side === 'YES' ? 'success' : 'destructive'}>{side}</Badge>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Amount</span>
                <span>{amount} USDC</span>
              </div>
              {quote.price_per_share && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Price / share</span>
                  <span>{quote.price_per_share}</span>
                </div>
              )}
              {quote.shares_received && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Expected shares</span>
                  <span>{quote.shares_received}</span>
                </div>
              )}
              <div className="flex justify-between pt-2 border-t border-slate-800">
                <span className="text-muted-foreground">Wallet</span>
                <span className="font-mono">{shortAddress}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Network</span>
                <span>Solana</span>
              </div>

              <div className="pt-3 flex gap-2">
                <Button variant="outline" onClick={() => { setStatus('IDLE'); setQuote(null); setTradeAttemptID(''); }}>
                  Cancel
                </Button>
                <Button onClick={handleSignAndBuy} className="flex-1">
                  Sign & Buy
                </Button>
              </div>
            </div>
          )}

          {/* Progress */}
          {status !== 'IDLE' && status !== 'QUOTE_READY' && (
            <div className="space-y-2 text-sm">
              {SUCCESS_STEPS.map((step) => {
                const currentIdx = SUCCESS_STEPS.indexOf(status as TradeStatus);
                const stepIdx = SUCCESS_STEPS.indexOf(step);
                const done = stepIdx < currentIdx;
                const active = step === status;
                const label = STEP_LABELS[step] || step;
                return (
                  <div key={step} className="flex items-center gap-2">
                    <span className={`w-4 h-4 rounded-full border flex items-center justify-center text-[10px] ${done ? 'bg-emerald-500 border-emerald-500 text-white' : active ? 'border-emerald-500 text-emerald-500 animate-pulse' : 'border-slate-700'}`}>
                      {done ? '✓' : active ? '◉' : '○'}
                    </span>
                    <span className={done ? 'text-emerald-400' : active ? 'text-white' : 'text-slate-500'}>
                      {label}
                    </span>
                  </div>
                );
              })}
            </div>
          )}

          {/* Signature result */}
          {signature && (status === 'CONFIRMED' || status === 'VERIFIED' || status === 'POSITION_REFRESHING' || status === 'COMPLETED') && (
            <div className="text-xs text-muted-foreground break-all font-mono">
              Signature: {signature}
            </div>
          )}

          {/* Position */}
          {position && status === 'COMPLETED' && (
            <div className="rounded-lg border border-emerald-800 p-4">
              <p className="text-sm font-semibold text-emerald-400">Position updated</p>
              <p className="text-xs text-muted-foreground mt-1">
                {position.side} · {position.quantity} shares{position.value_usdc ? ` · ${position.value_usdc} USDC` : ''}
              </p>
            </div>
          )}

          {/* Error */}
          {error && (
            <div className="rounded-lg border border-red-900 bg-red-950/40 p-4">
              <p className="text-sm font-medium text-red-400">{error.code.replace(/_/g, ' ')}</p>
              <p className="text-xs text-red-300 mt-1">{error.message}</p>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}