# Prophet — FAQ

## What is Prophet?

Prophet is an **enterprise prediction intelligence platform** that turns prediction-market activity into continuously monitored, explainable signals. It sits on top of the Panta prediction-market protocol on Solana, adding deterministic signal generation, AI-grounded explanations, watchlists, alerts, and a human-controlled trading workflow.

---

## Why prediction markets?

Prediction markets aggregate dispersed information into calibrated probabilities. They are not perfect oracles, but they provide a **useful real-time signal of collective expectations** that is difficult to replicate with other methods. Prophet makes this signal continuous, structured, actionable, and explainable.

---

## What is Panta?

Panta is the prediction-market protocol on Solana. It provides:
- Live market catalog with real-time probabilities
- Primary-buy trading (pool-based liquidity)
- Market creation with fee quotes and registration
- On-chain settlement with verification

Prophet integrates with Panta's official REST API. Panta handles the market infrastructure; Prophet builds the intelligence layer on top.

---

## Why Solana?

- Sub-second finality
- Low, predictable transaction costs
- SPL token standard (USDC)
- Mature wallet ecosystem (Phantom, Solflare, Backpack, etc.)
- Mature RPC infrastructure

Panta chose Solana; Prophet builds on top.

---

## Why AI?

AI in Prophet **explains, never decides**:

| What AI Does | What AI Never Does |
|--------------|-------------------|
| Structures natural language → draft market | Creates markets |
| Explains signals with source citations | Decides alert triggers |
| Answers questions with source citations | Executes trades |
| Flags missing fields in drafts | Signs transactions |

The deterministic signal engine (Rust) and deterministic alert engine (Go) make all numerical decisions. AI only explains what the deterministic layer computed.

---

## Does AI trade automatically?

**No.** The user's wallet signs every transaction. The server never holds private keys, never signs, never broadcasts without explicit user approval. The trading flow is:

```
Quote → Build → User Signs in Wallet → Broadcast → Confirm → Report → Verify
```

---

## Does Prophet hold private keys?

**Never.** The server stores only the Panta server API key. It never receives wallet private keys, seed phrases, or signed transaction blobs. The user's wallet (Phantom, Solflare, etc.) signs transactions; the server broadcasts and reports.

---

## How are signals calculated?

**Deterministically.** A Rust engine uses fixed-point arithmetic (12 decimal places, `SCALE = 1_000_000_000_000`) — never float64. Five signal types:

| Signal | Trigger |
|--------|---------|
| `NEW_MARKET` | New market discovered in catalog |
| `PROBABILITY_SHIFT` | YES probability moved beyond threshold |
| `ACTIVITY_CHANGE` | Volume/trade count moved beyond threshold |
| `LIQUIDITY_CHANGE` | Liquidity moved beyond threshold |
| `MARKET_MOVEMENT` | Composite movement signal |

Severities: `INFO` < `WATCH` < `SIGNIFICANT` < `CRITICAL` — ranked by exact rational arithmetic (`math/big.Rat` in Go, `Fixed` in Rust).

---

## Can users create markets?

Yes, via **Market Studio** — a 9-step guided wizard:

1. **Describe** — Natural language → AI draft
2. **Clarify** — Ambiguity detection
3. **Review Draft** — All fields editable
4. **Resolution** — Criteria, sources, explicit confirmation
5. **Validate** — Deterministic re-validation (no LLM)
6. **Quote** — Exact Panta fee quote (USDC base units)
7. **Build** — Unsigned transaction from Panta
8. **Sign & Broadcast** — Wallet signs, server broadcasts
9. **Register** → Panta registration (only step that creates market)

**Gates:** Create button hidden until validation passes + human review + warning acknowledgment.

---

## How does trading work?

```
Quote → Build → Review → Wallet Signs → Broadcast → Solana Confirm → Panta Report → Panta Verify → Position Refresh
```

1. **Quote** — Read-only Panta quote (price, shares, cost, expiry)
2. **Build** — Unsigned transaction from Panta
3. **Review** — Wallet/network/market/side/amount validation
4. **Sign** — User signs in wallet (Phantom, Solflare, etc.)
5. **Broadcast** → Solana RPC
6. **Confirm** — Solana confirmation polling
7. **Report** → Signature reported to Panta
7. **Verify** → Panta verification (verified/pending/failed)
8. **Refresh** → Position update from Panta

**Critical:** Solana confirmed ≠ Trade complete. Only Panta verification = complete.

---

## How are signals calculated?

From raw Panta observations (probability, volume, liquidity) sampled per market per tick. The Rust engine computes:

| Signal | Trigger |
|--------|---------|
| `PROBABILITY_SHIFT` | YES probability moves beyond threshold |
| `ACTIVITY_CHANGE` | Volume/trade count moves beyond threshold |
| `LIQUIDITY_CHANGE` | Liquidity moves beyond threshold |
| `MARKET_MOVEMENT` | Composite probability movement |
| `NEW_MARKET` | New market appears in catalog |

Severity thresholds are configurable. All arithmetic uses fixed-point (12 decimals) or `math/big.Rat` — **never float64**.

---

## How do alerts work?

1. User creates rule: scope (watchlist/market/global), signal type, min severity, thresholds, cooldown
2. Worker ingests signals → persists signal events (fingerprint deduplication)
3. For each new signal event: load enabled rules for market → `MatchesRule` (pure predicate)
3. If match: check cooldown (last `DELIVERED` since `now()-cooldown`)
4. Insert alert event with dedupe key `(alert_rule_id, fingerprint)` — `ON CONFLICT DO NOTHING`
5. If inserted → create notification → mark `DELIVERED`

**No LLM decides.** `MatchesRule` is a pure predicate: enabled → scope → signal type → severity → numeric thresholds.

---

## Can I use Prophet without AI?

Yes. All deterministic features work without an AI key:
- Market discovery, Signal Radar, Watchlists, Alerts, Portfolio
- Market Studio validation (deterministic re-validation runs without AI)
- Trading workflow

AI features (Copilot, Market Studio interpret) require `AI_API_KEY`.

---

## What happens if Panta goes down?

- **Circuit breaker** opens after 5 failures (30s timeout)
- Gateway serves cached market data with timestamp banner
- Signal Radar shows cached signals with timestamp
- Trading buttons disabled with "Panta unavailable" tooltip
- Worker pauses ingestion, retries with exponential backoff

---

## What happens if AI provider goes down?

- Copilot returns "AI temporarily unavailable"
- Market Studio `/interpret` falls back to deterministic fallback (manual draft entry)
- `/validate` endpoint works independently (no AI)
- Market Studio still functional for manual draft + validation

---

## What if Solana confirms but Panta registration fails?

**This is not a failure.** The UI shows:
> "Transaction confirmed on Solana. Panta registration is pending."

The market exists on-chain. Panta registration is idempotent — retry button available. Never reported as "creation failed."

---

## What are the current limitations?

| Limitation | Status |
|------------|--------|
| Market Engine Windows build | Requires mingw (CI/Linux works) |
| Frontend ESLint warnings | Pre-existing (zod/v4) |
| CSP/HSTS headers | Not implemented (post-demo) |
| Claims/settlement workflow | Not implemented |
| Creator fees | Not implemented |
| Trade attribution | Not implemented |
| WebSocket streaming | Not implemented |

---

## What is the custody model?

**Server never holds keys.** User's wallet (Phantom, Solflare, etc.) signs transactions. Server broadcasts signed transaction, reports signature to Panta. No private keys, seed phrases, or signed blobs ever touch the server.

---

## How are amounts handled?

**Never float64.** All amounts as strings:
- **USDC base units** (6 decimals): `"50000000"` = 50 USDC
- **Human display:** formatted from base units at UI layer
- **Arithmetic:** `math/big.Rat` (Go), `decimal.Decimal` (Python), `Fixed` (Rust)

---

## How is idempotency handled?

**Database-level, not application-level:**

| Operation | Idempotency Key | Mechanism |
|-----------|-----------------|-----------|
| Observation | `(market_id, observed_at)` | `UNIQUE` + `ON CONFLICT DO NOTHING` |
| Signal event | `fingerprint` (SHA-256) | `UNIQUE (fingerprint)` |
| Alert event | `(alert_rule_id, dedupe_key)` | `UNIQUE (alert_rule_id, dedupe_key)` |
| Trade attempt | `trade_attempt_id` (UUID) | PK |
| Market creation | `create_id + signature` | `UNIQUE (create_id, signature)` |

---

## How does AI grounding work?

1. User asks question
2. Copilot detects intent → calls allowed tools (`search_markets`, `get_signals`, `get_market_intelligence`, `compare_markets`, `get_recent_changes`)
3. Tool returns real Prophet data
4. AI synthesizes answer **citing sources**
5. Source citations are clickable → navigate to Market Detail

**AI never invents numbers.** If data missing, it says so.

---

## What's the difference between CONFIRMED and VERIFIED?

| State | Meaning |
|-------|---------|
| `CONFIRMED` | Solana confirmed the transaction |
| `VERIFIED` | Panta verified the trade |

**CONFIRMED ≠ VERIFIED.** A trade can be `CONFIRMED` (on-chain) but `VERIFIED` pending. UI shows: "Transaction confirmed on Solana. Panta verification pending."

---

## What happens if Panta goes down?

- Circuit breaker opens after 5 failures (30s timeout)
- Half-open after 30s for probe requests
- Gateway serves cached market data with timestamp banner
- Trading buttons disabled with "Panta unavailable" tooltip
- Worker pauses ingestion, retries with exponential backoff

---

## Can I run this locally?

Yes. See [Quick Start](README.md#quick-start) in README. Requires:
- Go 1.23+, Python 3.13+, Node.js 20+, Rust 1.81+
- Docker (PostgreSQL + Redis)
- Panta API key, Solana RPC URL, OpenAI API key

---

## What's not implemented (yet)?

| Feature | Status |
|---------|--------|
| Claims/settlement | Not implemented |
| Creator fees | Not implemented |
| Trade attribution | Not implemented |
| WebSocket streaming | Not implemented |
| Automated trading | Intentionally absent (custody) |
| AI market creation | Intentionally absent |
| Claims/settlement API | Not implemented |
| Breaking markets | Partial (type supported) |
| Advanced alert compositions | Not implemented |

---

*FAQ v1.0 — Updated with each release*