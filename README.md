# QEVRYN

Enterprise Prediction Intelligence Platform

"See what the market thinks happens next."

---

## Overview

Qevryn transforms prediction-market activity into a structured intelligence layer. Instead of forcing users to manually browse fragmented prediction markets, Qevryn continuously organizes market probabilities, movements, activity changes, and liquidity shifts into deterministic signals, watchlists, alerts, and AI-grounded explanations.

Panta provides the underlying prediction-market infrastructure on Solana. Qevryn builds the intelligence and enterprise workflow layer on top of it.

---

## The Problem

Prediction markets generate high-value intelligence signals, but accessing them is fragmented and manual:

- **Fragmented markets** — Markets are scattered across venues with inconsistent APIs and data formats
- **Changing probabilities** — Probabilities shift continuously; manual monitoring misses critical inflection points
- **Difficult monitoring** — No unified view of market movements, activity changes, or liquidity shifts
- **No continuous signal layer** — Deterministic signals are not extracted from raw market data
- **Disconnected workflows** — Discovery, analysis, trading, and monitoring live in separate tools
- **Custody confusion** — Users often don't understand where their keys are held

---

## The Solution

**Panta provides the infrastructure.** Qevryn provides the intelligence layer.

```
┌─────────────────────────────────────────────────────────────────┐
│                        Panta Infrastructure                      │
│  (Prediction Markets on Solana)                                  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                        QEVRYN Platform                          │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌───────────┐  │
│  │ Market      │ │ Signal      │ │ Watchlists  │ │ Alerts &  │  │
│  │ Discovery   │ │ Radar       │ │ & Alerts    │ │ Notifications│  │
│  └─────────────┘ └─────────────┘ └─────────────┘ └───────────┘  │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌───────────┐  │
│  │ AI Copilot  │ │ Market      │ │ Trading     │ │ Portfolio │  │
│  │ (Grounded)  │ │ Studio      │ │ Workflow    │ │ & Positions│  │
│  └─────────────┘ └─────────────┘ └─────────────┘ └───────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Panta (Solana) + PostgreSQL                   │
└─────────────────────────────────────────────────────────────────┘
```

**Custody boundary:** Qevryn never holds private keys. Users sign transactions in their own wallet. The server never signs.

---

## The Problem with Prediction Markets Today

Prediction markets produce some of the highest-quality probabilistic forecasts available, but they're trapped in poor user experiences:

| Pain Point | Impact |
|------------|--------|
| **Fragmented venues** | Markets scattered across protocols; no unified catalog |
| **Manual monitoring** | Professionals manually refresh browser tabs |
| **No signal layer** | Raw price data ≠ actionable intelligence |
| **Alerting is manual** | No automated alerts for probability shifts, volume spikes, or liquidity changes |
| **Trading is disconnected** | Discovery, analysis, and execution are separate workflows |
| **Custody is opaque** | Users don't know who holds their keys |

---

## What Qevryn Does

### Market Discovery
Unified catalog of all Panta markets with search, filtering, and pagination. Real-time probability, volume, and liquidity data.

### Market Intelligence
Deterministic signals computed from raw observations:
- **Probability Shifts** — YES probability movements with exact percentage-point changes
- **Activity Changes** — Volume and trade count movements
- **Liquidity Changes** — Liquidity shifts in the market
- **Market Movements** — Composite movement signals
- **New Markets** — Newly discovered markets

All signals use **fixed-point decimal arithmetic** (12 decimal places) — never floating point.

### Signal Radar
Real-time feed of all deterministic signals with:
- Severity tabs: Critical / Significant / Watch / Info
- Filters: market, watchlist, signal type, time range, severity
- Keyset pagination for stable scrolling
- Deterministic explanations (no LLM generation)

### Watchlists & Alerts
- Create named watchlists of markets
- Configure alert rules with:
  - Scope: watchlist, specific market, or global
  - Signal type filter
  - Minimum severity threshold
  - Numeric thresholds (percentage points, % change)
  - Cooldown periods (up to 7 days)
- In-app notification inbox with read/unread state

### AI Copilot (Grounded)
Natural-language queries over market data:
- "What changed significantly today?"
- "Which markets moved the most?"
- "Show me markets with probability below 40%"
- "Explain the biggest signal"

**Grounding guarantees:** Every AI response cites source data (markets, signals, observations). No invented numbers. Tool calls are logged and displayed.

### Market Studio (Creation)
9-step guided workflow:
1. **Describe** — Natural language → AI draft
2. **Clarify** — Ambiguity detection
2. **Review Draft** — All fields editable
3. **Resolution** — Criteria, sources, explicit confirmation
4. **Validate** — Deterministic re-validation (no LLM)
5. **Quote** — Exact Panta fee quote (USDC base units)
6. **Build** — Unsigned transaction from Panta
6. **Sign & Broadcast** — User signs in wallet; server never signs
7. **Broadcast** — Solana RPC submission
7. **Register** — Panta registration (only step that creates market)
8. **Live** — Market appears in Qevryn

*Gates:* Create button only appears after validation passes + user review + warning acknowledgment.

### Trading Workflow
1. **Quote** — Real Panta quote (read-only)
2. **Build** — Unsigned transaction from Panta
3. **Review** — Wallet/network/market/side/amount validation
7. **Sign** — User signs in wallet (Phantom, Solflare, etc.)
8. **Broadcast** — Solana RPC submission
9. **Confirm** — Solana confirmation polling
10. **Report** — Signature reported to Panta
11. **Verify** — Panta verification
11. **Refresh** — Position update

### Portfolio
Connected wallet positions with real-time values from Panta.

---

## Why Prediction Markets

Prediction markets aggregate dispersed information into calibrated probabilities. They are not perfect oracles, but they provide a **useful real-time signal of collective expectations** that is difficult to replicate with other methods.

Qevryn makes this signal:
- **Continuous** — Always on, always current
- **Structured** — Deterministic signals, not raw noise
- **Actionable** — Alerts, watchlists, trading workflow
- **Explainable** — Deterministic explanations, AI grounding

---

## Panta Integration

Panta is the prediction-market protocol on Solana. Qevryn integrates with Panta's official API:

| Feature | Panta Endpoint | Qevryn Implementation |
|---------|----------------|------------------------|
| Market Listing | `GET /api/v1/markets/` | Cursor-paginated catalog |
| Market Detail | `GET /api/v1/markets/{id}/` | Detail with spot prices |
| Market Creation - Quote | `POST /api/v1/markets/create/quote/` | Fee quote in USDC base units |
| Market Creation - Build | `POST /api/v1/markets/create/build/` | Unsigned transaction |
| Market Creation - Register | `POST /api/v1/markets/register/` | Only step that creates market |
| Primary Buy - Quote | `POST /api/v1/markets/primary-buy/quote/` | Human-readable USDC |
| Primary Buy - Build | `POST /api/v1/markets/primary-buy/build/` | Unsigned transaction |
| Primary Buy - Report | `POST /api/v1/markets/primary-buy/report/` | Report signature to Panta |
| Primary Buy - Verify | `GET /api/v1/transactions/{sig}/verify/` | Verification polling |
| Positions | `GET /api/v1/positions/?wallet=` | Wallet-scoped positions |

**Authentication:** `X-Api-Key` header (server-side only; never exposed to browser)

**Amount Format:** All fees in USDC base units (integer strings, 6 decimals). Never float64.

---

## Architecture

```
┌──────────────────┐
│   Browser        │
│   (Next.js)      │
└────────┬─────────┘
         │ HTTPS + JWT
         ▼
┌──────────────────┐
│   Go Gateway     │ ◄── Auth, Rate Limit, Circuit Breaker
│   (Port 8080)    │
└───────┬──────────┘
        │
    ┌───┼──────────────┐
    ▼   ▼              ▼
Panta    Python      Rust Engine
Adapter  Intelligence  (Signals)
(8081)   (8001)        (CLI)
   │         │            │
   ▼         ▼            ▼
Panta    OpenAI      Deterministic
API      API         Signals
        │
        ▼
   PostgreSQL + Redis
```

**Custody Boundary:** The server stores only the Panta server API key. It never receives wallet private keys, seed phrases, or signed transaction blobs. Users sign in their wallet; the server broadcasts and reports.

---

## What Qevryn Does NOT Do

| Feature | Status | Reason |
|---------|--------|--------|
| Automated trading | ❌ | Custody model prohibits |
| Automated market creation | ❌ | Human review required at every gate |
| AI decides alert triggers | ❌ | Deterministic `MatchesRule` only |
| AI generates market explanations without sources | ❌ | Grounded in tool results only |
| Float64 for money | ❌ | Base-unit strings + `math/big.Rat` |
| Auto-trading bots | ❌ | Custody model violation |
| Claims/settlement | ❌ | Out of scope |
| Creator fees | ❌ | Not implemented |
| Trade attribution | ❌ | Not implemented |

---

## Quick Start

### Prerequisites
- Go 1.23+
- Python 3.13+
- Node.js 20+
- Rust 1.81+
- Docker (PostgreSQL + Redis)
- Panta API key
- Solana RPC URL
- OpenAI API key (for AI features)

### Quick Start

```bash
# 1. Start infrastructure
docker compose -f infrastructure/docker-compose.dev.yml up -d postgres redis

# 2. Export environment variables
export DATABASE_URL="postgresql://prophet:prophet@localhost:5432/prophet"
export PANTA_API_KEY="your-panta-api-key"
export PANTA_API_BASE_URL="https://live-api.panta.market/api/v1/"
export SOLANA_RPC_URL="https://api.mainnet-beta.solana.com"
export AI_API_KEY="your-openai-key"
export JWT_SECRET="$(openssl rand -hex 32)"
export MARKET_ENGINE_BIN="./services/market-engine/target/release/market-engine"

# 3. Build market engine (Linux/CI recommended)
cd services/market-engine && cargo build --release

# 4. Run services (each in separate terminal)
# Terminal 1: Panta Adapter
cd services/panta-adapter && go run ./cmd/server

# Terminal 2: Intelligence
cd services/intelligence && python -m app.main

# Terminal 3: Gateway
cd services/gateway && go run .

# Terminal 4: Worker (alert ingestion)
cd services/gateway && go run ./cmd/worker

# Terminal 5: Frontend
cd apps/web && npm run dev
```

### Verify Health
```bash
curl http://localhost:8080/health/live   # Gateway liveness
curl http://localhost:8080/health/ready  # Gateway readiness
curl http://localhost:8081/health        # Panta Adapter
curl http://localhost:8001/health        # Intelligence
```

---

## Demo Flow (Hackathon)

1. **Command Center** — Market overview + live Signal Radar
2. **Market Detail** — Probability, signals, metadata
3. **Signal Radar** — Filter by severity, type, watchlist, time
3. **Watchlist** → Create → Add markets → Alert on "Significant" shifts
4. **Alert** → Create rule → Trigger signal → Notification appears
5. **Copilot** → "What changed significantly today?" → Grounded response
4. **Market Studio** — Describe → AI Draft → Validate → Quote → Build → Sign → Broadcast → Register
5. **Trade** — Quote → Build → Sign in Wallet → Broadcast → Confirm → Report → Verify
6. **System Status** — Dependency health dashboard

---

## Demo Safety

- **No real transactions** in automated tests
- **Demo mode** flag available for deterministic fixtures
- **Never** auto-sign transactions
- **Never** claim "trade succeeded" before Panta verification
- **Never** fabricate data for demo

---

## Testing

```bash
# Go
cd services/gateway && go test ./... -race
cd services/panta-adapter && go test ./...

# Python
cd services/intelligence && python -m pytest -v

# Frontend
cd apps/web && npm test && npm run build

# Rust
cd services/market-engine && cargo test

# Solidity
cd contracts/evm && forge test

# All
make test
make lint
make build
```

---

## Repository Structure

```
.
├── apps/web                    # Next.js 16 frontend
├── services/
│   ├── gateway/                # Go 1.23 gateway + worker
│   ├── panta-adapter/          # Go 1.23 Panta boundary
│   ├── intelligence/           # FastAPI 0.115 + OpenAI
│   └── market-engine/          # Rust 2021 signal engine
├── contracts/evm/              # Foundry Solidity 0.8.25
├── packages/
│   ├── types/                  # Shared Go types
│   ├── ui/                     # (placeholder)
│   └── sdk/                    # (placeholder)
├── contracts/evm/              # Foundry Solidity
├── proto/                      # Protobuf (placeholder)
├── infrastructure/             # Docker Compose
├── docs/                       # Architecture, runbooks, API ref
├── Makefile                    # dev, test, lint, build
└── .env.example                # Environment template
```

---

## Security

- **No private keys** — Server never holds keys; user signs in wallet
- **JWT auth** — HS256, configurable expiry, audience validation
- **Rate limiting** — Token bucket + sliding window per IP+path
- **Circuit breaker** — Panta adapter (5 failures → open, 30s timeout)
- **Request size limit** — 1MB body limit
- **Structured logging** — JSON, request IDs, no secrets in logs
- **Parameterized SQL** — pgx parameterized queries throughout

---

## Deployment

```bash
# Production
docker compose -f infrastructure/docker-compose.prod.yml up -d

# Health checks
curl http://gateway:8080/health/live
curl http://gateway:8080/health/ready
curl http://panta-adapter:8081/health
curl http://intelligence:8001/health
```

### Environment Variables

| Variable | Service | Required |
|----------|---------|----------|
| `DATABASE_URL` | Gateway, Worker, Intelligence | Yes |
| `PANTA_API_KEY` | Panta Adapter, Gateway | Yes |
| `PANTA_API_BASE_URL` | Panta Adapter | No (default) |
| `PANTA_API_URL` | Gateway (trading) | Trading only |
| `SOLANA_RPC_URL` | Gateway (trading) | Trading only |
| `JWT_SECRET` | Gateway | Yes |
| `AI_API_KEY` | Intelligence | AI features |
| `MARKET_ENGINE_BIN` | Worker | Yes |
| `CORS_ALLOWED_ORIGINS` | Intelligence | Production |

---

## Documentation

| Document | Description |
|----------|-------------|
| `docs/architecture.md` | System architecture |
| `docs/panta-integration.md` | Panta API integration |
| `docs/trading-flow.md` | Trading lifecycle |
| `docs/market-studio.md` | Market creation flow |
| `docs/watchlists-alerts.md` | Watchlists, alerts, notifications |
| `docs/production-readiness.md` | Production audit |
| `docs/security.md` | Security model |
| `docs/operations.md` | Operations runbook |
| `docs/demo-runbook.md` | Demo script + failure plan |
| `docs/data-lineage.md` | Data flow documentation |
| `docs/test-matrix.md` | Test coverage matrix |

---

## Current Limitations

| Limitation | Status | Impact |
|------------|--------|--------|
| Market Engine Windows build | Requires mingw (CI/Linux works) | Local Windows dev needs WSL or mingw |
| Frontend ESLint config | zod/v4 module resolution error | Pre-existing, non-blocking |
| Frontend build warnings | Missing optional wallet adapter packages | Install packages or mark optional |
| CSP/HSTS headers | Not implemented | Post-demo item |
| 1 flaky Go test | `TestAlertRulesRefuseExecutableContent/sql_in_a_name` | Pre-existing, expected behavior |
| 1 Python test | `test_validation_rejects_today_resolution_date` | Pre-existing, validation logic difference |

---

## What Qevryn Does NOT Do

| Feature | Status | Reason |
|---------|--------|--------|
| Automated trading | ❌ | Custody model prohibits |
| Automated market creation | ❌ | Human review required at every gate |
| AI decides alert triggers | ❌ | Deterministic `MatchesRule` only |
| AI generates market explanations without sources | ❌ | Grounded in tool results only |
| Float64 for money | ❌ | Base-unit strings + `math/big.Rat` |
| Auto-trading bots | ❌ | Custody model violation |
| Claims/settlement | ❌ | Out of scope |
| Creator fees | ❌ | Not implemented |
| Trade attribution | ❌ | Not implemented |

---

## License

Proprietary — All rights reserved.

---

*QEVRYN — Enterprise Prediction Intelligence Platform*  
*See what the market thinks happens next.*