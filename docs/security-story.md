# Security Story

## The Core Principle

> **"AI agents never receive private keys or autonomous financial authority."**

This is the single invariant that governs every security decision in QEVRYN.

---

## What This Means in Practice

### User-Controlled Wallet
- **Private keys never leave the user's device.** Phantom, Solflare, Backpack — the wallet lives in the browser extension or mobile app.
- **The server never sees private keys.** Not encrypted, not derived, not in memory, not in logs, not in the database.
- **Seed phrases never touch QEVRYN.** Not in transit, not at rest, never.

### Unsigned Transaction Architecture
```
Panta Build → Unsigned Transaction (base64) → User's Wallet → Signed Transaction → Solana RPC
```
1. Panta builds an **unsigned** transaction (base64 `VersionedTransaction`)
2. Gateway sends it to the browser
3. **User's wallet** prompts for signature
4. **User explicitly approves** in their wallet UI
4. Signed transaction → Solana RPC → Panta verification

**The server never signs.** It never could — it doesn't have the keys.

### Deterministic Validation (No AI in the Loop)
- **Market Studio:** AI *drafts* → Human reviews → Deterministic Go validation → Quote → Build → Sign
- **Alerts:** `MatchesRule()` is pure Go logic — `math/big.Rat` arithmetic, no LLM
- **Signals:** Rust engine, fixed-point arithmetic, deterministic fingerprinting
- **AI Copilot:** Tool-calling only — tools are `search_markets`, `get_signals`, `get_market_intelligence`. No `execute_trade`, `create_market`, `send_transaction`.

### Server-Side Secrets Only
| Secret | Location | Rotation |
|--------|----------|----------|
| `PANTA_API_KEY` | Gateway + Panta Adapter env | Manual / secret manager |
| `JWT_SECRET` | Gateway only | Manual / secret manager |
| `AI_API_KEY` | Intelligence service only | Manual / secret manager |
| `DATABASE_URL` | Gateway, Worker, Intelligence | Managed |
| `SOLANA_RPC_URL` | Gateway (trading) | Manual |

**Never in browser:** No `NEXT_PUBLIC_` secrets. No API keys in frontend bundles.

---

## Authorization Model

### Gateway: JWT + Ownership Checks
- JWT (HS256) with `user_id`, `email`, `role`, `exp`, `aud`
- Every HTTP handler resolves `user_id` via `ResolveUser` middleware
- Repository methods **always** scope by `user_id` in the same query:
  ```go
  // Good: ownership in WHERE clause
  SELECT * FROM watchlists WHERE id=$1 AND user_id=$2
  
  // Bad: separate check
  w := repo.GetWatchlist(id); if w.UserID != userID { return ErrNotFound }
  ```

### Resource Ownership
| Resource | Ownership Check |
|----------|-----------------|
| Watchlist | `user_id` on watchlist |
| Alert Rule | `user_id` on rule |
| Notification | `user_id` on notification |
| Trade Attempt | `wallet_address` on attempt |
| Market Creation | `wallet_address` on attempt |
| Alert Event | Via rule → `user_id` |

**No IDOR:** A user passing another user's watchlist ID gets `404` (not `403` — same as not found).

---

## Rate Limiting

| Layer | Algorithm | Limits |
|--------|-----------|--------|
| Gateway (per IP+path) | Token bucket + sliding window | 100 req/min general, 30/min AI, 20/min trading |
| Panta Adapter | Circuit breaker | 5 failures → open, 30s timeout, 2 successes → closed |
| Intelligence | Per-user token bucket | 20 req/min Copilot, 10/min Market Studio |

---

## Circuit Breaker (Panta Adapter)

```
Closed (normal) ──5 failures──▶ Open (30s) ──2 successes──▶ Closed
                              │
                              ▼
                         Half-Open (probe)
```

- Failure threshold: 5 consecutive errors
- Open state: immediate failures, no Panta calls
- Half-open: probe requests allowed, 2 successes → closed
- Metrics exposed: `circuit_breaker_state`, `circuit_breaker_failures`

---

## Financial Precision

| Value Type | Representation | Arithmetic |
|------------|----------------|------------|
| USDC amounts | `string` (base units, 6 decimals) | `string` concat / `math/big.Rat` |
| Probabilities | `string` (0.000000000000 - 1.000000000000) | `math/big.Rat` (Go), `decimal.Decimal` (Python), `Fixed` (Rust) |
| Percentages | `string` (basis points or decimal) | `math/big.Rat` |

**Never:** `float64`, `float32`, `double`, `float` for money/probability.

**Rust:** `Fixed(i128)` with `SCALE = 1_000_000_000_000` (12 decimal places)

---

## Idempotency (Database-Enforced)

| Operation | Idempotency Key | Mechanism |
|-----------|-----------------|-----------|
| Market observation | `(market_id, observed_at)` | `UNIQUE` + `ON CONFLICT DO NOTHING` |
| Signal event | `fingerprint` (SHA-256) | `UNIQUE (fingerprint)` |
| Alert event | `(alert_rule_id, dedupe_key)` | `UNIQUE (alert_rule_id, dedupe_key)` |
| Trade attempt | `trade_attempt_id` (UUID) | PK |
| Market creation | `create_id + signature` | `UNIQUE (create_id, signature)` |

**No application-level dedupe.** The database is the source of truth.

---

## Input Validation

| Layer | What |
|-------|------|
| Gateway (Go) | Strict JSON decoding (`DisallowUnknownFields`), struct validation, base58 address validation, decimal string validation |
| Intelligence (Python) | Pydantic models with strict config, `extra='forbid'` |
| Market Engine (Rust) | `Fixed::parse` validates decimal format, precision, range |
| Frontend | Zod schemas matching backend, TypeScript types from `packages/types` |

---

## AI Safety

| Guard | Implementation |
|-------|----------------|
| Tool allowlist | Only `search_markets`, `get_market_intelligence`, `get_signals`, `get_recent_changes`, `compare_markets` |
| Max tool calls | 8 per request |
| Max history | 10 messages |
| Request timeout | 30s |
| Prompt injection detection | Keyword/pattern detection in prompts, logged |
| Response size limit | 2000 tokens max |
| Tool timeout | 10s per tool |

**AI never:**
- Signs transactions
- Creates markets
- Modifies watchlists/alerts
- Executes trades
- Accesses database
- Reads environment variables
- Makes HTTP requests

---

## Financial Precision

| Operation | Implementation |
|-----------|----------------|
| USDC amounts | Base-unit integer strings (6 decimals) — `"50000000"` = 50 USDC |
| Probability math | `math/big.Rat` (Go), `decimal.Decimal` (Python), `Fixed` (Rust) |
| Percentage math | Exact rational → `big.Rat.FloatString(1)` for display |
| Rounding | `big.Rat.FloatString(1)` for % (1 decimal), `FloatString(0)` for magnitude |

---

## Logging Policy

| Logged | Never Logged |
|---------|--------------|
| Request ID, method, path, status, duration | Authorization headers |
| User ID (hashed) | API keys |
| Error code, kind | Private keys / seed phrases |
| Request/response size | Signed transaction blobs |
| Circuit breaker state | JWT tokens |

---

## Compliance Checklist

| Requirement | Status |
|-------------|--------|
| No private keys in code | ✅ |
| No private keys in logs | ✅ |
| No private keys in DB | ✅ |
| No float64 for money | ✅ |
| Idempotency keys on all writes | ✅ |
| Authorization on every handler | ✅ |
| Rate limiting on all public endpoints | ✅ |
| Circuit breaker on external deps | ✅ |
| Structured logging (no secrets) | ✅ |
| Request ID propagation | ✅ |
| AI tool allowlist | ✅ |
| AI max tool calls | ✅ |
| AI no autonomous actions | ✅ |

---

## Incident Response

| Scenario | Response |
|----------|----------|
| Panta API down | Circuit breaker opens → cached data served with timestamp |
| AI provider down | Copilot returns "unavailable" / Market Studio falls back to manual |
| Database down | Gateway `/health/ready` fails → 503; worker pauses |
| Worker crash | Advisory lock released → new worker acquires lock → resumes |
| AI prompt injection | Detected → logged → request rejected with generic error |
| Wallet rejects signature | UI shows "Transaction cancelled" — never "failed" |

---

## Security Contacts

| Role | Contact |
|----------|---------|
| Security issues | security@QEVRYN.example.com |
| Vulnerability disclosure | security@QEVRYN.example.com |
| On-call | PagerDuty: QEVRYN-oncall |

---

*Security is not a feature — it's the foundation. Every line of code is reviewed against these principles.*
