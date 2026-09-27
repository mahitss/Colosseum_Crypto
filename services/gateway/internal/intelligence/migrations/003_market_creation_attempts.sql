-- Migration: 003_market_creation_attempts.sql
-- Market Studio creation attempts.
--
-- Security notes, enforced by what this table deliberately does NOT have:
--   * no private key, seed phrase or mnemonic column of any kind
--   * no signed transaction blob column. The signed payload is relayed
--     directly to the Solana RPC and discarded; only the resulting signature
--     is persisted. A signature is a public, non-secret value.
--
-- Money is stored exactly as Panta expresses it: USDC base units in integer
-- strings (6 decimals). NUMERIC is avoided for the quote amounts so we never
-- round-trip a value through a float or a lossy decimal scale.

CREATE TABLE market_creation_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Ownership / identity
    wallet_address TEXT NOT NULL,

    -- Panta quote + build references. createId is Panta's handle for one
    -- creation attempt and is the only authority for "did this reach Panta".
    create_id TEXT,
    build_fingerprint TEXT,
    expected_event_pda TEXT,

    -- Fee amounts, verbatim from Panta. Integer strings of USDC base units.
    payment_usdc TEXT,
    liquidity_injection_usdc TEXT,
    platform_revenue_usdc TEXT,

    market_type TEXT CHECK (market_type IS NULL OR market_type IN ('standard', 'breaking')),

    -- Draft integrity.
    -- draft_hash pins the exact content the user reviewed. Any material field
    -- change produces a different hash and therefore invalidates the prior
    -- quote/build rather than silently reusing it.
    draft_hash TEXT NOT NULL,
    draft_payload JSONB,

    -- Lifecycle. Note that 'confirmed' means Solana confirmed ONLY. The market
    -- is not live until 'registered' (Panta has indexed it), and it is not
    -- visible in Prophet until 'indexed'.
    status TEXT NOT NULL CHECK (status IN (
        'CREATED', 'SIGNED', 'BROADCASTING', 'SUBMITTED', 'CONFIRMING',
        'CONFIRMED', 'REGISTERING', 'REGISTERED', 'INDEXED',
        'FAILED', 'UNKNOWN'
    )) DEFAULT 'CREATED',

    -- Distinguishes "Solana confirmed but Panta registration failed" from a
    -- genuine end-to-end failure, so the UI never says "creation failed"
    -- when the money and the transaction actually succeeded.
    error_code TEXT,
    error_message TEXT,

    -- Public transaction outcome.
    solana_signature TEXT,

    -- Prophet-side identifiers, populated only after authoritative registration.
    market_id TEXT,
    registered_at TIMESTAMPTZ,
    indexed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT market_creation_attempts_signature_unique UNIQUE (solana_signature),
    -- Idempotency: the same (creation_attempt_id, draft_hash, wallet) triple is
    -- one logical creation. Deliberately NOT keyed on the question text, which
    -- is user-editable and therefore not a stable identity.
    CONSTRAINT market_creation_attempts_idempotency UNIQUE (id, draft_hash, wallet_address),
    CONSTRAINT market_creation_attempts_status_not_empty CHECK (length(trim(status)) > 0),
    -- Panta amounts are integer base-unit strings. Reject decimals, signs and
    -- exponents outright so no float or fractional value can ever be persisted.
    CONSTRAINT market_creation_attempts_payment_base_units
        CHECK (payment_usdc IS NULL OR payment_usdc ~ '^[0-9]+$'),
    CONSTRAINT market_creation_attempts_liquidity_base_units
        CHECK (liquidity_injection_usdc IS NULL OR liquidity_injection_usdc ~ '^[0-9]+$'),
    CONSTRAINT market_creation_attempts_revenue_base_units
        CHECK (platform_revenue_usdc IS NULL OR platform_revenue_usdc ~ '^[0-9]+$')
);

CREATE INDEX idx_market_creation_attempts_wallet ON market_creation_attempts(wallet_address);
CREATE INDEX idx_market_creation_attempts_create_id ON market_creation_attempts(create_id);
CREATE INDEX idx_market_creation_attempts_draft_hash ON market_creation_attempts(draft_hash);
CREATE INDEX idx_market_creation_attempts_status ON market_creation_attempts(status);
CREATE INDEX idx_market_creation_attempts_created ON market_creation_attempts(created_at DESC);
