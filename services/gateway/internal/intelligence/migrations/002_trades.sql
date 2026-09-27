-- Migration: 002_trades.sql
-- Create trade_attempts table for transaction lifecycle tracking

CREATE TABLE trade_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_address TEXT NOT NULL,
    market_id TEXT NOT NULL,
    side TEXT NOT NULL CHECK (side IN ('YES', 'NO')),
    amount_usdc NUMERIC(38, 12) NOT NULL,
    
    -- References
    quote_reference TEXT,
    solana_signature TEXT UNIQUE,
    panta_reference TEXT,
    
    -- Lifecycle
    status TEXT NOT NULL CHECK (status IN (
        'IDLE', 'QUOTING', 'QUOTE_READY', 'BUILDING', 'READY_TO_SIGN',
        'SIGNING', 'SIGNED', 'BROADCASTING', 'SUBMITTED', 'CONFIRMING',
        'CONFIRMED', 'REPORTING', 'VERIFIED', 'POSITION_REFRESHING',
        'COMPLETED', 'CANCELLED', 'FAILED', 'UNKNOWN'
    )) DEFAULT 'IDLE',
    
    -- Error tracking
    error_code TEXT,
    error_message TEXT,
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    confirmed_at TIMESTAMP WITH TIME ZONE,
    verified_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT valid_amount CHECK (amount_usdc > 0),
    CONSTRAINT unique_trade_attempt UNIQUE (wallet_address, market_id, side, amount_usdc, created_at)
);

CREATE INDEX idx_trade_attempts_wallet ON trade_attempts(wallet_address);
CREATE INDEX idx_trade_attempts_market ON trade_attempts(market_id);
CREATE INDEX idx_trade_attempts_signature ON trade_attempts(solana_signature);
CREATE INDEX idx_trade_attempts_status ON trade_attempts(status);
CREATE INDEX idx_trade_attempts_created ON trade_attempts(created_at DESC);
