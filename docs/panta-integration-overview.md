# Panta Integration Overview

## Why Panta Was Selected

Panta is the leading prediction-market protocol on Solana, providing:
- Live market catalog with real-time probabilities
- Primary-buy trading (pool-based liquidity)
- Market creation with fee quotes and registration
- On-chain settlement with verification

**Key differentiators:**
- Live mainnet API with documented REST endpoints
- USDC base-unit amounts (integer strings, no float64)
- Unsigned transaction building for user custody
- Registration endpoint that finalizes market existence
- Verification endpoint for post-trade confirmation

---

## How Prophet Consumes Panta

### Flow 1: Market Discovery (Read Path)

```
Browser → Next.js → Go Gateway → Panta Adapter → Panta API
```

**Endpoints used:**
| Prophet Route | Adapter Route | Panta Route | Behavior |
|--------------|---------------|-------------|----------|
| `GET /api/v1/markets` | `GET /markets/` | `GET /api/v1/markets/` | Cursor-paginated catalog, optional `category`, `status`, `createdBy`, `cursor`, `limit` (1-50) |
| `GET /api/v1/markets/{id}` | `GET /markets/{id}/` | `GET /api/v1/markets/{marketId}/` | Catalog detail including spot price fields when RPC available |

**Adapter responsibilities:**
- Normalize Panta JSON → Prophet `types.Market`
- Validate response schema (reject malformed)
- Retry logic: 3 attempts, jittered exponential backoff, `Retry-After` respected (capped 30s)
- Circuit breaker: 5 failures → open, 30s timeout, 2 successes → closed
- Request ID propagation (`X-Request-Id`)

**Gateway responsibilities:**
- Rate limiting (token bucket + sliding window)
- JWT auth (except public market routes)
- Request ID propagation
- Request size limit (1MB)

---

### Flow 2: Market Creation (Write Path)

```
Next.js → Go Gateway → Market Studio → Panta Adapter → Panta API → Solana → Panta Registration → Prophet Indexing
```

**Endpoints used:**

| Step | Panta Endpoint | Request | Response |
|------|----------------|---------|----------|
| Quote | `POST /api/v1/markets/create/quote/` | `wallet`, `question`, `resolutionRule`, `sourcesOfTruth`, `category`, `startTime`, `endTime`, `resolutionTime`, `imageUrl`, `marketType?`, `eventInProgress?`, `title`, `description`, `region`, `oracle` | `createId`, `expectedEventPda`, `paymentUsdc`, `liquidityInjectionUsdc`, `platformRevenueUsdc`, `marketType`, `expiresAt`, `blockhashExpiryHintSec` |
| Build | `POST /api/v1/markets/create/build/` | `createId`, `wallet` | `transaction` (base64), `expectedWallet`, `expectedNetwork`, `buildReference`, `messageFormat`, `instructions[]` |
| Register | `POST /api/v1/markets/register/` | `createId`, `signature` (base58) | `createId`, `marketId`, `status: "registered"`, `signature`, `category`, `title`, `images` |

**Key constraints from Panta docs:**
- Amounts: USDC base units as integer strings (e.g., `"50000000"` = 50 USDC)
- Question ≤ 512 chars, resolution rule ≤ 2048 chars
- `sourcesOfTruth` ≤ 20 entries, non-empty array
- `startTime < endTime ≤ resolutionTime` (unix seconds)
- `imageUrl`: required, http/https, ≤2048 chars, no localhost/data URLs
- Category: one of `sports`, `crypto`, `politics`, `entertainment`, `finance`, `science`, `world`, `other`
- `marketType`: `standard` or `breaking` (default `standard`)

**Idempotency:**
- `createId` + `signature` idempotent on register
- `createId` used as idempotency key for quote/build

---

### Flow 3: Trading (Primary Buy)

```
User → Prophet UI → Gateway → Panta Adapter → Panta API
                           ↑                    ↓
                      User Wallet ←─────────────┘
```

**Endpoints used:**

| Step | Panta Endpoint | Request | Response |
|------|----------------|---------|----------|
| Quote | `POST /api/v1/markets/primary-buy/quote/` | `market_id`, `side`, `amount_usdc`, `wallet` | `quote_reference`, `price_per_share`, `total_cost`, `shares_received`, `expires_at` |
| Build | `POST /api/v1/markets/primary-buy/build/` | `quote_reference`, `wallet` | `transaction` (base64), `expected_wallet`, `expected_network`, `build_reference` |
| Broadcast | (Solana RPC) | Signed tx | Signature |
| Report | `POST /api/v1/markets/primary-buy/report/` | `market_id`, `side`, `amount_usdc`, `signature`, `quote_reference` | `order_reference` |
| Verify | `GET /api/v1/transactions/{sig}/verify/` | — | `status` (verified/pending/failed) |
| Positions | `GET /api/v1/positions/?wallet=` | — | `positions[]` |

**Amount formats:**
- Quote/build amounts: Human-readable USDC decimals (e.g., `"20.00"`)
- Creation amounts: USDC base units (integer strings, 6 decimals)

---

## Categorization: LIVE / VERIFIED vs IMPLEMENTED BUT NOT LIVE VERIFIED vs NOT IMPLEMENTED

### LIVE / VERIFIED (Production-Tested Against Real Panta)

| Feature | Status | Evidence |
|---------|--------|----------|
| Market listing (paginated) | ✅ LIVE | Smoke test passes, real Panta data in UI |
| Market detail (with spot prices) | ✅ LIVE | Real Panta data displayed |
| Market creation quote | ✅ LIVE | Real Panta fee quotes returned |
| Market creation build | ✅ LIVE | Unsigned tx returned, base64 decoded |
| Market creation register | ⚠️ NOT LIVE VERIFIED | Requires real Solana tx + Panta registration |
| Trading quote | ✅ LIVE | Real quotes from Panta |
| Trading build | ⚠️ NOT LIVE VERIFIED | Requires wallet + real Panta build |
| Trading broadcast | ⚠️ NOT LIVE VERIFIED | Requires wallet + Solana RPC |
| Trading report/verify | ⚠️ NOT LIVE VERIFIED | Requires real signature |
| Positions | ✅ LIVE | Real Panta positions returned |

### IMPLEMENTED BUT NOT LIVE VERIFIED

| Feature | Status | Blocker |
|---------|--------|---------|
| Market creation register | Implemented | Requires real Solana tx + Panta registration |
| Trading build | Implemented | Requires wallet + Panta |
| Trading broadcast | Implemented | Requires wallet + Solana RPC |
| Trading report | Implemented | Requires real signature |
| Trading verify | Implemented | Requires real signature |
| Market creation full flow | Implemented | End-to-end needs live Panta + wallet |

### NOT IMPLEMENTED

| Feature | Status | Reason |
|---------|--------|--------|
| WebSocket price streaming | Not implemented | Out of scope |
| Breaking markets | Partial | Market type supported, not tested live |
| Claims/settlement | Not implemented | Out of scope |
| Creator fees | Not implemented | Not implemented |
| Trade attribution | Not implemented | Not implemented |
| WebSocket streaming | Not implemented | Out of scope |
| Claims/settlement | Not implemented | Out of scope |

---

## Amount Handling

| Context | Format | Example |
|---------|--------|---------|
| Market listing (UI) | Human-readable USDC | `"20.00"` |
| Market creation fees | USDC base units (6 decimals) | `"50000000"` = 50 USDC |
| Trading amounts | Human-readable USDC | `"20.00"` |
| Internal storage | `string` (base units) | Never float64 |

**Rule:** Never parse monetary values as `float64`. Use `string` throughout, `math/big.Rat` for arithmetic.

---

## Error Handling

| Error Type | Adapter Behavior | Gateway Response |
|------------|------------------|------------------|
| 401/403 | Auth failure | 401 |
| 404 | Not found | 404 |
| 429 | Retry with `Retry-After` (capped 30s) | 429 + `Retry-After` |
| 5xx | Retry (3 attempts, jittered backoff) | 502/503/504 |
| Malformed 200 | Validate → reject | 502 |
| Network error | Retry (3 attempts) | 503 |

**Never retried:** 400, 401, 403, 404, malformed success responses.

---

## Amount Handling Rules

| Rule | Implementation |
|------|----------------|
| Never float64 | All amounts as `string` |
| Base units | USDC = 6 decimals (`"50000000"` = 50 USDC) |
| Human display | Format from base units at UI layer |
| Arithmetic | `math/big.Rat` (Go) / `decimal.Decimal` (Python) |
| Validation | `ParseBaseUnits` rejects non-integer strings |

---

## Panta API Reference (Verified)

### Market Discovery
```
GET /api/v1/markets/                    # List markets
GET /api/v1/markets/{marketId}/         # Get market detail
```

### Market Creation
```
POST /api/v1/markets/create/quote/     # Fee quote
POST /api/v1/markets/create/build/     # Unsigned transaction
POST /api/v1/markets/register/         # Register (creates market)
```

### Primary Buy (Trading)
```
POST /api/v1/markets/primary-buy/quote/
POST /api/v1/markets/primary-buy/build/
POST /api/v1/markets/primary-buy/report/
GET  /api/v1/transactions/{sig}/verify/
GET  /api/v1/positions/?wallet=
```

### Account
```
GET /api/v1/account/                    # Auth check (used for /health/panta)
```

---

## Live vs Not Live Summary

| Category | Status |
|----------|--------|
| Market Discovery | ✅ LIVE |
| Market Detail | ✅ LIVE |
| Market Creation (quote/build) | ✅ LIVE (quote/build) / ⚠️ NOT LIVE VERIFIED (register) |
| Trading (quote/build) | ⚠️ NOT LIVE VERIFIED |
| Trading (broadcast/confirm/report/verify) | ⚠️ NOT LIVE VERIFIED |
| Positions | ✅ LIVE |
| Signal Engine | ✅ LIVE (deterministic, no Panta needed) |
| Watchlists/Alerts | ✅ LIVE (no Panta dependency) |
| AI Copilot | ⚠️ REQUIRES OPENAI KEY |
| Market Studio | ✅ IMPLEMENTED (registration not live verified) |

---

## Key Takeaway

**LIVE:** Market discovery, detail, positions, signals, watchlists, alerts
**IMPLEMENTED BUT NOT LIVE VERIFIED:** Market creation registration, trading broadcast/confirm/report/verify
**NOT IMPLEMENTED:** Claims, creator fees, trade attribution, WebSocket streaming

The boundary is: anything requiring a real wallet signature + Solana transaction + Panta registration is "not live verified" because it requires mainnet funds and live Panta interaction.