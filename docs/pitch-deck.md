# QEVRYN — One-Pager

## The Problem

Prediction markets generate high-quality probabilistic forecasts, but they're trapped in poor user experiences:

- **Fragmented venues** — Markets scattered across protocols with inconsistent APIs
- **Manual monitoring** — Professionals manually refresh browser tabs
- **No signal layer** — Raw price data ≠ actionable intelligence
- **Disconnected workflows** — Discovery, analysis, trading, monitoring are separate
- **Custody confusion** — Users don't know who holds their keys

---

## The Solution: Qevryn

**Enterprise Prediction Intelligence Platform**

Qevryn turns prediction-market activity into a structured intelligence layer.

| Panta Provides | Qevryn Adds |
|----------------|--------------|
| Market infrastructure | Unified market catalog |
| Raw price data | Deterministic signals |
| Transaction endpoints | Watchlists, alerts, AI explanations |
| | Human-controlled trading workflow |
| | Market creation workflow |

---

## Core Differentiators

| Feature | How Qevryn Does It |
|---------|---------------------|
| **Deterministic Signals** | Fixed-point arithmetic (12 decimals), never float64 |
| **AI Grounding** | Every AI response cites sources; no invented numbers |
| **Custody Model** | Server never holds keys; user signs in wallet |
| **Deterministic Alerts** | `math/big.Rat` arithmetic; no LLM decides triggers |
| **Idempotency** | Database-level UNIQUE constraints + ON CONFLICT DO NOTHING |
| **Market Creation** | 9-step wizard: AI draft → deterministic validation → human review → quote → build → sign → broadcast → register |

---

## Technical Architecture

```
Browser (Next.js) → Go Gateway → Panta Adapter → Panta API → Solana
                     ↓
              Python Intelligence → AI Providers
                     ↓
              Rust Market Engine (deterministic signals)
                     ↓
              PostgreSQL + Redis
```

---

## Key Differentiators

| Feature | Typical Prediction Market App | Qevryn |
|---------|-------------------------------|---------|
| Signal Generation | Manual / None | Deterministic (Rust, fixed-point) |
| Alert Logic | LLM-based or none | Deterministic (math/big.Rat) |
| AI Explanations | Hallucination-prone | Source-cited, tool-grounded |
| Market Creation | Manual / None | 9-step wizard with gates |
| Trading | Custodial or unclear | User signs; server never holds keys |
| Signal Generation | Manual / Heuristic | Deterministic (fixed-point Rust) |
| Idempotency | Application-level | Database UNIQUE + ON CONFLICT DO NOTHING |

---

## What Qevryn Actually Built

| Component | Status |
|-----------|--------|
| Market Discovery | ✅ Unified catalog with search/filter/pagination |
| Signal Engine | ✅ 5 deterministic signal types (Rust, fixed-point) |
| Signal Radar | ✅ Filterable, paginated, severity-tabs |
| Watchlists | ✅ CRUD + market membership |
| Alerts | ✅ Rules + cooldown + dedupe + in-app notifications |
| AI Copilot | ✅ Grounded in tools, source-cited |
| Market Studio | ✅ 9-step wizard with gates |
| Trading Workflow | ✅ Quote → Build → Sign → Broadcast → Confirm → Report → Verify |
| Market Creation | 9-step wizard with AI draft + deterministic validation |
| Portfolio | ✅ Positions from Panta |
| Observability | Structured logs, Prometheus metrics, OpenTelemetry |

---

## What Qevryn Does NOT Do

| Feature | Status | Reason |
|---------|--------|--------|
| Automated trading | ❌ | Custody model prohibits |
| AI decides alerts | ❌ | Deterministic rules only |
| AI executes trades | ❌ | Custody violation |
| AI creates markets | ❌ | Human review required |
| Float64 for money | ❌ | Base-unit strings + math/big.Rat |
| Claims/settlement | ❌ | Out of scope |

---

## Technical Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Next.js    │────▶│  Go Gateway │────▶│Panta Adapter│
│  (Port 3000)│     │   (8080)    │     │   (8081)    │
└─────────────┘     └──────┬──────┘     └──────┬──────┘
                           │                   │
                    ┌──────┴──────┐      ┌─────┴─────┐
                    ▼             ▼      ▼           ▼
              Python Intel    Rust Engine   Panta API
              (FastAPI)       (Rust CLI)    (HTTPS)
                  │             │            │
                  └──────┬──────┘            │
                         ▼                   ▼
                  PostgreSQL + Redis       Solana
                       │                    │
                       └────────┬────────────┘
                                ▼
                         User Wallet (Signs)
```

**Custody:** Server stores only Panta API key. Never receives private keys, seed phrases, or signed transaction blobs.

---

## Demo Readiness

| Area | Status |
|------|--------|
| Market Discovery | ✅ Real Panta data |
| Signal Radar | ✅ Live filters, pagination |
| Watchlists | ✅ CRUD + alerts |
| Alerts | ✅ Rules + cooldown + dedupe |
| Notifications | ✅ In-app inbox |
| AI Copilot | ✅ Grounded in tools |
| Market Studio | ✅ 9-step wizard with gates |
| Trading | ✅ Quote → Build → Sign → Broadcast → Verify |
| Market Creation | ✅ 9-step wizard |
| Portfolio | ✅ Positions from Panta |
| System Status | ✅ Dependency health |
| Observability | Structured logs, Prometheus, OTel |

---

## What We Don't Have (Honest)

- Market Engine builds on Windows (CI handles this)
- Frontend build warnings (missing optional wallet adapter packages)
- CSP/HSTS headers (post-demo)
- CSP/HSTS headers (post-demo)
- Playwright E2E tests (manual demo instead)

---

## Judge Quick-Start

```bash
# 1. Infrastructure
docker compose -f infrastructure/docker-compose.dev.yml up -d postgres redis

# 2. Build market engine (Linux)
cd services/market-engine && cargo build --release

# 3. Export env vars (see .env.example)
export DATABASE_URL="postgresql://prophet:prophet@localhost:5432/prophet"
export PANTA_API_KEY="your-key"
export PANTA_API_BASE_URL="https://live-api.panta.market/api/v1/"
export SOLANA_RPC_URL="https://api.mainnet-beta.solana.com"
export AI_API_KEY="sk-..."
export JWT_SECRET="$(openssl rand -hex 32)"
export MARKET_ENGINE_BIN="./services/market-engine/target/release/market-engine"

# 4. Run services (5 terminals)
# 1. cd services/panta-adapter && go run ./cmd/server
# 2. cd services/intelligence && python -m app.main
# 3. cd services/gateway && go run .
# 4. cd services/gateway && go run ./cmd/worker
# 4. cd apps/web && npm run dev

# 5. Open http://localhost:3000
```

---

## Contact

**QEVRYN Team** — Colosseum Hackathon 2024

*Enterprise Prediction Intelligence Platform*  
*See what the market thinks happens next.*