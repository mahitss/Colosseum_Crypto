# Prophet Security Documentation

**Version:** 1.0  
**Date:** 2025-09-29

---

## Table of Contents

1. [Trust Boundaries](#trust-boundaries)
2. [Secret Management](#secret-management)
3. [Wallet Custody Model](#wallet-custody-model)
4. [Authentication](#authentication)
5. [Authorization](#authorization)
6. [AI Boundaries](#ai-boundaries)
7. [Transaction Signing Model](#transaction-signing-model)
8. [Data Handling](#data-handling)
9. [Logging Policy](#logging-policy)
10. [Network Security](#network-security)
10. [Incident Response](#incident-response)

---

## Trust Boundaries

```
┌─────────────────────────────────────────────────────────────────┐
│                        BROWSER (UNTRUSTED)                      │
│  - User's wallet (Phantom, Solflare, etc.)                      │
│  - User-provided input (market descriptions, trade params)      │
│  - JWT in Authorization header                                  │
└──────────────────────────────┬──────────────────────────────────┘
                               │ HTTPS + JWT
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      GO GATEWAY (TRUSTED)                       │
│  - JWT validation & user context resolution                     │
│  - Rate limiting, request size limits                           │
│  - Input validation & sanitization                              │
│  - Ownership checks on all user resources                       │
│  - Orchestrates downstream services                             │
│  - NEVER handles private keys                                   │
└──────────────────────────────┬──────────────────────────────────┘
                               │ Internal network
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
       ┌─────────────┐ ┌──────────────┐ ┌──────────────┐
       │ PANTA       │ │ INTELLIGENCE │ │ MARKET ENGINE  │
       │ ADAPTER     │ │ SERVICE      │ │ (RUST CLI)     │
       │             │ │              │ │                │
       │ - Panta     │ │ - AI Market  │ │ - Deterministic│
       │   API calls │ │   Architect  │ │   signal calc  │
       │ - Circuit   │ │ - Copilot    │ │ - No network   │
       │   breaker   │ │   (read-only)│ │ - Stdin/Stdout │
       └─────────────┘ └──────────────┘ └──────────────┘
              │                │                │
              ▼                ▼                ▼
       ┌──────────────────────────────────────────────────┐
       │              POSTGRESQL (TRUSTED)                 │
       │ - User data (watchlists, alerts, notifications)  │
       │ - Market data, observations, signals             │
       │ - Trade attempts, creation attempts              │
       │ - Alert events, notifications                    │
       └──────────────────────────────────────────────────┘
```

### Key Principles

1. **Browser is untrusted**: All input validated server-side
2. **Gateway is the trust anchor**: Validates JWT, enforces ownership
3. **Internal services trust Gateway**: No independent auth
4. **Database is source of truth**: All durable state in PostgreSQL
5. **Panta API is external**: Circuit breaker, no blind retries

---

## Secret Management

### Classification

| Secret Type | Examples | Storage | Rotation |
|-------------|----------|---------|----------|
| **Critical** | `PANTA_API_KEY`, `JWT_SECRET`, `AI_API_KEY` | Vault/Secrets Manager | 90 days |
| **High** | `DATABASE_URL`, `SOLANA_RPC_URL` | Vault/Secrets Manager | 180 days |
| **Medium** | `PANTA_API_BASE_URL`, `INTELLIGENCE_SERVICE_URL` | ConfigMap/Env | N/A |

### Handling Rules

1. **Never in code**: No hardcoded secrets
2. **Never in logs**: Structured logging excludes secrets
3. **Never in frontend**: No `NEXT_PUBLIC_` secrets
4. **Never in Docker images**: Multi-stage builds, runtime injection
5. **Never in git**: `.gitignore` covers `.env*`, `*.key`, `*.pem`

### Injection Methods

| Environment | Method |
|-------------|--------|
| Local dev | `.env` file (gitignored), sourced in shell |
| Docker Compose | `environment:` from host env |
| Kubernetes | `Secrets` + `envFrom` or `valueFrom` |
| Docker Swarm | `docker secret create` + `secrets:` in compose |

### Rotation Procedure

```bash
# 1. Generate new secret
NEW_JWT_SECRET=$(openssl rand -hex 32)

# 2. Update in secret manager
# (Vault/AWS/GCP/1Password)

# 3. Rolling restart
docker compose up -d --force-recreate gateway

# 4. Verify
curl -H "Authorization: Bearer $NEW_TOKEN" http://localhost:8080/api/v1/watchlists
```

---

## Wallet Custody Model

### Core Principle

**The server NEVER touches private keys.**

```
┌─────────────┐     ┌──────────────┐     ┌──────────────┐
│   USER      │     │   GATEWAY    │     │    PANTA     │
│  (WALLET)   │     │   (SERVER)   │     │    API       │
└──────┬──────┘     └──────┬───────┘     └──────┬───────┘
       │                   │                     │
       │ 1. Request quote  │                     │
       ├──────────────────▶│                     │
       │                   │ 2. Returns quote    │
       │◀──────────────────┤ (quote_reference)   │
       │                   │                     │
       │ 3. Request build  │                     │
       ├──────────────────▶│                     │
       │                   │ 4. Returns unsigned │
       │◀──────────────────┤ tx + build_ref      │
       │                   │                     │
       │ 5. User reviews   │                     │
       │    & signs in     │                     │
       │    wallet         │                     │
       │                   │                     │
       │ 6. Submit signed  │                     │
       │    tx + build_ref │                     │
       ├──────────────────▶│                     │
       │                   │ 7. Broadcasts to    │
       │                   │    Solana RPC       │
       │                   ├────────────────────▶│
       │                   │                     │
       │                   │ 8. Returns sig      │
       │                   │◀────────────────────┤
       │                   │                     │
       │ 9. Reports to     │                     │
       │    Panta          │                     │
       ├──────────────────▶│                     │
       │                   │ 10. Reports result  │
       │◀──────────────────┤                     │
       │                   │                     │
```

### What Server NEVER Does

- ❌ Generate private keys/mnemonics
- ❌ Store private keys (encrypted or not)
- ❌ Sign transactions
- ❌ Hold custody of user funds
- ❌ Auto-sign on user's behalf
- ❌ Accept private keys via API

### What Server DOES

- ✅ Request unsigned transactions from Panta
- ✅ Validate transaction parameters
- ✅ Broadcast signed transactions via Solana RPC
- ✅ Report signatures to Panta
- ✅ Verify transaction status with Panta
- ✅ Track trade state in database

### Trade State Machine

```
IDLE -> QUOTING -> QUOTE_READY -> BUILDING -> READY_TO_SIGN -> SIGNING
  -> SIGNED -> BROADCASTING -> SUBMITTED -> CONFIRMING -> CONFIRMED
  -> REPORTING -> VERIFIED -> POSITION_REFRESHING -> COMPLETED

Any state -> FAILED, CANCELLED, UNKNOWN
```

---

## Authentication

### JWT Implementation

- **Algorithm**: HS256 (symmetric, rotating secret)
- **Claims**: `user_id` (sub), `email`, `role`, `exp`, `iat`, `jti`
- **Expiry**: 24 hours (configurable via `JWT_EXPIRY`)
- **Audience**: `prophet-api` (configurable via `JWT_AUDIENCE`)

### Flow

```
1. User authenticates via external provider (OAuth/OIDC)
   OR development mode uses fixed "local-user"
   
2. Gateway generates JWT with user_id
   
3. Client includes JWT in Authorization header:
   Authorization: Bearer <jwt>
   
4. Gateway validates on each request:
   - Signature verification
   - Expiry check
   - Issuer/audience validation
   - Sets user_id in request context
   
5. Handlers use ResolveUser() to get user_id
```

### Development Mode

```bash
# No auth provider configured
# Gateway uses DefaultUserResolver -> "local-user"
# All requests scoped to "local-user"
```

### Production Requirements

- External OIDC provider (Auth0, Clerk, Supabase, etc.)
- JWKS endpoint for key rotation
- Refresh token flow for long sessions
- Short-lived access tokens (15-30 min)

---

## Authorization

### Model: Ownership-Based

Every user-owned resource is scoped by `user_id`:

```go
// Example: Watchlist access
func (r *Repository) GetWatchlist(ctx context.Context, userID, watchlistID string) (*Watchlist, error) {
    // Single query with ownership check
    err := r.pool.QueryRow(ctx, `
        SELECT ... FROM watchlists 
        WHERE id=$1 AND user_id=$2`, watchlistID, userID).Scan(...)
    // Returns ErrWatchlistNotFound if not owned
}
```

### Protected Resources

| Resource | Ownership | Access Control |
|----------|-----------|----------------|
| Watchlists | `user_id` on watchlist | All CRUD scoped by user_id |
| Alert Rules | `user_id` on rule | All CRUD scoped by user_id |
| Notifications | `user_id` on notification | List/read scoped by user_id |
| Trade Attempts | `wallet_address` | Scoped by wallet in request |
| Creation Attempts | `wallet_address` | Scoped by wallet in request |

### IDOR Prevention

```go
// WRONG: Trust user input
func BadHandler(w http.ResponseWriter, r *http.Request) {
    watchlistID := r.FormValue("watchlist_id")  // User controlled!
    repo.GetWatchlist(ctx, watchlistID)         // No ownership check
}

// CORRECT: Resolve from authenticated context
func GoodHandler(w http.ResponseWriter, r *http.Request) {
    userID := ResolveUser(r)  // From JWT, validated
    watchlistID := r.PathValue("id")
    repo.GetWatchlist(ctx, userID, watchlistID)  // Ownership enforced in SQL
}
```

### Alert Rules Scope

```sql
-- Alert rule can target:
-- 1. Specific market (market_id)
-- 2. Watchlist (watchlist_id) 
-- 3. All user's markets (both NULL)

-- CHECK constraint enforces mutual exclusion:
CONSTRAINT alert_rules_valid CHECK (
    (watchlist_id IS NOT NULL AND market_id IS NULL) OR
    (watchlist_id IS NULL AND market_id IS NOT NULL) OR
    (watchlist_id IS NULL AND market_id IS NULL)
)
```

---

## AI Boundaries

### Market Studio (AI Market Architect)

**What AI Does:**
- Structures natural language into structured draft
- Identifies missing fields
- Suggests categories, dates, outcomes
- Flags vague/ambiguous prompts

**What AI NEVER Does:**
- ❌ Create markets
- ❌ Sign transactions
- ❌ Broadcast to blockchain
- ❌ Register with Panta
- ❌ Modify draft after user review without explicit action
- ❌ Execute trades
- ❌ Modify user's watchlists/alerts
- ❌ Access private keys

### Copilot (Read-Only Analysis)

**What AI Does:**
- Answers questions about markets
- Explains signals
- Compares markets
- Searches market data

**What AI NEVER Does:**
- ❌ Execute trades
- ❌ Create/modify watchlists
- ❌ Create/modify alerts
- ❌ Execute SQL
- ❌ Execute shell commands
- ❌ Access environment variables
- ❌ Access secrets
- ❌ Make HTTP requests to arbitrary URLs

### Implementation Safeguards

```python
# 1. Tool allowlist
ALLOWED_TOOLS = {
    "search_markets": search_markets,
    "get_signals": get_signals,
    "get_market_detail": get_market_detail,
}

# 2. Argument schemas (Pydantic)
class SearchMarketsArgs(BaseModel):
    query: str = Field(max_length=200)
    limit: int = Field(ge=1, le=50)

# 3. Max tool calls per request
MAX_TOOL_CALLS = 5

# 4. Request timeout
REQUEST_TIMEOUT = 30  # seconds

# 5. Response size limit
MAX_RESPONSE_TOKENS = 2000

# 6. Prompt injection defenses
SYSTEM_PROMPT = """You are a read-only assistant. 
Never execute actions. Never access external systems.
If asked to do something outside your tools, refuse."""
```

### Deterministic Validation Gate

```python
# AI output ALWAYS validated before any action
def interpret(prompt: str) -> Interpretation:
    draft = ai_agent.interpret(prompt)
    if draft:
        validation = validate_draft(draft, prompt=prompt)
        if not validation.valid:
            draft = None  # Discard invalid draft
    return Interpretation(draft=draft, validation=validation)

# Create action ONLY enabled when validate_draft() returns valid=true
```

---

## Transaction Signing Model

### End-to-End Flow

```
1. USER selects market, side, amount in UI
         │
         ▼
2. GATEWAY requests quote from PANTA
   POST /api/v1/trades/quote
   {market_id, side, amount_usdc, wallet_pubkey}
         │
         ▼
3. PANTA returns quote with quote_reference
   {quote_reference, price, shares, expires_at}
         │
         ▼
4. GATEWAY requests build from PANTA
   POST /api/v1/trades/build
   {quote_reference, wallet_pubkey}
         │
         ▼
5. PANTA returns unsigned transaction
   {transaction_data, build_reference, expected_wallet}
         │
         ▼
6. FRONTEND validates:
   - expected_wallet == connected_wallet
   - transaction_data is valid base64
   - build_reference matches quote
         │
         ▼
7. USER signs in wallet (Phantom/Solflare)
   - User explicitly clicks "Sign"
   - Wallet shows amount, market, side
         │
         ▼
8. FRONTEND submits signed tx
   POST /api/v1/trades/broadcast
   {trade_attempt_id, signed_transaction, wallet_pubkey, draft_hash}
         │
         ▼
8. GATEWAY broadcasts via Solana RPC
   sendRawTransaction(signed_tx)
         │
         ▼
9. GATEWAY polls for confirmation
   getSignatureStatuses(signature)
         │
         ▼
10. GATEWAY reports to PANTA
    POST /api/v1/trades/report
    {trade_attempt_id, signature}
         │
         ▼
11. PANTA verifies
    GET /transactions/{signature}/verify
         │
         ▼
12. GATEWAY refreshes positions
    GET /api/v1/trades/positions/{wallet}
```

### Critical Security Invariants

| Invariant | Enforcement |
|-----------|-------------|
| Server never signs | No signing keys in code/config |
| User must explicitly sign | Wallet UI requires click |
| Amount/market/side immutable after quote | `quote_reference` binds params |
| Build validates wallet match | `expected_wallet` == connected |
| Draft hash prevents silent changes | `draft_hash` in broadcast |
| Solana confirmed != Trade complete | Must verify with Panta |

### Idempotency Keys

| Operation | Key | Storage |
|-----------|-----|---------|
| Trade attempt | `trade_attempt_id` (UUID) | `trade_attempts.id` |
| Quote | `quote_reference` | `trade_attempts.quote_reference` |
| Build | `build_reference` | `trade_attempts.build_reference` |
| Broadcast | `solana_signature` | `trade_attempts.solana_signature` (UNIQUE) |
| Report | `trade_attempt_id` + `signature` | `trade_attempts.panta_reference` |

---

## Data Handling

### Personal Data

| Data Type | Classification | Retention | Access |
|-----------|----------------|-----------|--------|
| User ID (UUID) | Pseudonymous | Account lifetime | Owner + Admins |
| Email | PII | Account lifetime | Owner + Admins |
| Wallet address | Pseudonymous | Account lifetime | Owner + Admins |
| Trade history | Financial | 7 years (regulatory) | Owner |
| Alert preferences | Preference | Account lifetime | Owner |

### Data Minimization

- Only collect what's needed for features
- No tracking/analytics without consent
- No third-party sharing
- Pseudonymize where possible

### Data Flow

```
User Input -> Validation -> Sanitization -> Database
                │
                ▼
        Parameterized Queries (pgx)
                │
                ▼
        Output Encoding (JSON)
                │
                ▼
        Response to Client
```

### Encryption

| State | Method |
|-------|--------|
| In transit | TLS 1.2+ (HTTPS/WSS) |
| At rest | PostgreSQL TDE / Volume encryption |
| Secrets | Vault/Secrets Manager (not in DB) |

### Data Retention

| Data | Retention | Deletion |
|------|-----------|----------|
| Trade attempts | 7 years | Auto-archive after completion |
| Market observations | 2 years | Rolling window |
| Signal events | 2 years | Rolling window |
| Alert events | 1 year | Auto-delete |
| Notifications | 1 year | Auto-delete on read |
| Logs | 90 days | Log rotation |

---

## Logging Policy

### Structured Logging

```json
{
  "time": "2026-09-29T12:34:56.789Z",
  "level": "INFO",
  "msg": "market sync finished",
  "request_id": "abc123...",
  "service": "gateway",
  "markets_fetched": 150,
  "signals_generated": 42,
  "duration_ms": 2341
}
```

### Required Fields

| Field | Description |
|-------|-------------|
| `time` | RFC3339 timestamp |
| `level` | DEBUG/INFO/WARN/ERROR |
| `msg` | Human-readable message |
| `request_id` | Correlation ID (X-Request-Id) |
| `service` | Service name |

### Log Levels

| Level | When |
|-------|-------|
| DEBUG | Detailed diagnostic (dev only) |
| INFO | Normal operations, state changes |
| WARN | Recoverable issues, retries, near-limits |
| ERROR | Failures requiring attention |

### What to Log

| Event | Level | Fields |
|-------|-------|--------|
| Request start | INFO | method, path, request_id |
| Request complete | INFO | status, duration_ms |
| Request error | ERROR | error_code, message, duration |
| Trade state change | INFO | attempt_id, from_state, to_state |
| Alert triggered | INFO | alert_rule_id, signal_event_id, user_id |
| Notification sent | INFO | notification_id, user_id, channel |
| Circuit breaker state change | WARN | from_state, to_state |
| Auth failure | WARN | reason (expired, invalid, missing) |

### What NOT to Log

| Category | Examples |
|----------|----------|
| Secrets | API keys, JWT secrets, DB passwords |
| Tokens | JWTs, refresh tokens, session IDs |
| Private keys | Wallet private keys, seed phrases |
| Signed tx | Raw signed transaction blobs |
| Auth headers | Authorization, Cookie headers |
| PII in bulk | Full email lists, wallet lists |

### Log Retention

| Environment | Retention | Storage |
|-------------|-----------|---------|
| Development | 7 days | Local/Console |
| Staging | 30 days | Loki/CloudWatch |
| Production | 90 days | Loki/CloudWatch/S3 |

---

## Network Security

### Transport Security

| Connection | Protocol | Verification |
|------------|----------|--------------|
| Browser -> Gateway | HTTPS (TLS 1.2+) | Valid cert, HSTS |
| Gateway -> Panta Adapter | HTTP (internal) | mTLS (future) |
| Gateway -> Intelligence | HTTP (internal) | mTLS (future) |
| Gateway -> Panta API | HTTPS | Valid cert, pinned (future) |
| Gateway -> Solana RPC | HTTPS | Valid cert |
| Gateway -> PostgreSQL | TLS | Verify cert (future) |
| Worker -> PostgreSQL | TLS | Verify cert (future) |

### Firewall Rules

```
# Gateway (8080)
ALLOW 443/TCP from Internet -> Gateway
ALLOW 8080/TCP from Load Balancer -> Gateway

# Panta Adapter (8081)
DENY from Internet
ALLOW 8081/TCP from Gateway -> Panta Adapter

# Intelligence (8001)
DENY from Internet
ALLOW 8001/TCP from Gateway -> Intelligence

# PostgreSQL (5432)
DENY from Internet
ALLOW 5432/TCP from Gateway/Worker -> PostgreSQL

# Redis (6379)
DENY from Internet
ALLOW 6379/TCP from Gateway/Worker -> Redis
```

### Rate Limiting

| Endpoint | Limit | Window |
|----------|-------|--------|
| `/api/v1/market-studio/interpret` | 10 req/min | Per IP |
| `/api/v1/trades/quote` | 30 req/min | Per IP |
| `/api/v1/trades/build` | 30 req/min | Per IP |
| `/api/v1/trades/broadcast` | 20 req/min | Per IP |
| `/api/v1/copilot/query` | 20 req/min | Per IP |
| General API | 100 req/min | Per IP |

---

## Incident Response

### Severity Levels

| Level | Definition | Response Time | Escalation |
|-------|------------|---------------|------------|
| SEV-1 | Total outage, data loss | 15 min | Page on-call, CTO |
| SEV-2 | Major feature down | 1 hour | Page on-call, Team lead |
| SEV-3 | Minor feature degraded | 4 hours | Team lead |
| SEV-4 | Minor issue, workaround exists | Next business day | Team |

### Runbook: SEV-1 Total Outage

1. **Acknowledge**: Page on-call within 5 min
2. **Assess**: Check all health endpoints
3. **Communicate**: Status page, Slack #incidents
4. **Mitigate**: Identify root cause, apply fix
4. **Verify**: All health checks pass
5. **Communicate**: Resolution, RCA timeline
5. **Postmortem**: Within 48 hours

### Key Runbooks

| Scenario | Runbook |
|----------|---------|
| Database down | `docs/runbooks/database-down.md` |
| Panta API down | `docs/runbooks/panta-down.md` |
| Worker stuck | `docs/runbooks/worker-stuck.md` |
| High error rate | `docs/runbooks/high-errors.md` |
| AI provider down | `docs/runbooks/ai-down.md` |

---

*Document version 1.0 - Review quarterly*