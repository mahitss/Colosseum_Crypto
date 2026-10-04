# QEVRYN — Pitch Document

## One-Liner

**QEVRYN is an enterprise prediction intelligence platform that turns prediction-market activity into continuously monitored, explainable signals.**

---

## 30-Second Pitch

"Prediction markets generate some of the highest-quality probabilistic forecasts available, but they're trapped in poor user experiences — fragmented venues, manual monitoring, no signal layer. Qevryn solves this by turning prediction-market activity into a structured intelligence layer: deterministic signals computed from raw market data, AI explanations grounded in actual data, watchlists and alerts that actually work, and a trading workflow where the user never surrenders custody. Panta provides the markets; Qevryn provides the intelligence."

---

## 60-Second Pitch

**The Problem:** Prediction markets generate some of the highest-quality probabilistic forecasts available, but they're trapped in poor user experiences — fragmented venues, manual monitoring, no signal layer, disconnected workflows, and opaque custody. Professionals manually refresh browser tabs. No signal layer exists. Trading, analysis, and monitoring are disconnected.

**The Insight:** Prediction markets already produce high-quality probabilistic forecasts. The problem isn't the data — it's the interface. What if the market data was continuously organized into deterministic signals, watchlists, alerts, and AI explanations, with a trading workflow where the user never surrenders custody?

**The Solution:** Qevryn is an enterprise prediction intelligence platform. Panta provides the market infrastructure on Solana. Qevryn builds the intelligence layer on top: deterministic signals computed via fixed-point arithmetic, a Signal Radar with severity-filtered streaming, watchlists and alerts with database-enforced idempotency, an AI Copilot that cites sources for every claim, a Market Studio where AI drafts but humans decide, and a trading workflow where the user's wallet signs every transaction.

**Panta Integration:** Panta is the prediction-market protocol on Solana. Qevryn integrates at the API boundary — market discovery, fee quotes, unsigned transaction building, registration — while keeping the server-side Panta API key isolated from the browser.

**Differentiation:** Three pillars that are hard to replicate together: (1) Deterministic signal engine in Rust using fixed-point arithmetic — no float64, ever. (2) AI that explains, never decides — every response cites sources, tools are logged, no autonomous actions. (3) Custody model where the server never sees private keys — the user's wallet signs, Panta verifies, Qevryn indexes.

---

## 2-Minute Pitch

**Problem:** Prediction markets generate high-quality probabilistic forecasts, but the user experience is broken. Markets are fragmented across venues. Probabilities shift continuously — manual monitoring misses inflection points. No signal layer extracts deterministic intelligence from raw price data. Trading, analysis, and monitoring are disconnected workflows. Users don't know who holds their keys.

**Insight:** The market data is high quality. The interface is the problem. What if the market data was continuously organized into deterministic signals, watchlists, alerts, and AI explanations, with a trading workflow where the user never surrenders custody?

**Product:** Qevryn is an enterprise prediction intelligence platform. Panta provides the market infrastructure on Solana. Qevryn builds the intelligence layer: deterministic signals computed via fixed-point arithmetic in Rust, a Signal Radar with severity-filtered streaming, watchlists and alerts with database-enforced idempotency, an AI Copilot that cites sources for every claim, a Market Studio where AI drafts but humans decide, and a trading workflow where the user's wallet signs every transaction.

**Architecture:** Next.js frontend → Go Gateway (orchestration, auth, rate limiting, circuit breaker) → Panta Adapter (typed boundary, circuit breaker, retries) → Panta API → Solana. Python Intelligence service for AI Market Architect and Copilot. Rust Market Engine for deterministic signal generation (fixed-point arithmetic, 12 decimal places, never float64). PostgreSQL for durable state, Redis for caching.

**Panta Integration:** Market discovery via paginated catalog. Market creation: quote → build → sign → broadcast → register → index. Trading: quote → build → wallet sign → broadcast → confirm → report → verify → refresh. All amounts in USDC base units (integer strings, 6 decimals). Never float64.

**Differentiation:** Three pillars that are hard to replicate together: (1) Deterministic signal engine in Rust using fixed-point arithmetic — no float64, ever. (2) AI that explains, never decides — every response cites sources, tools are logged, no autonomous actions. (3) Custody model where the server never sees private keys — the user's wallet signs, Panta verifies, Prophet indexes.

**Use Cases:** Professional traders monitoring probability shifts. Analysts tracking market movements. Teams creating custom prediction markets. Enterprises monitoring specific outcomes.

---

## Technical Differentiation

### Deterministic Signal Engine
- Rust implementation using fixed-point arithmetic (12 decimal places, SCALE = 1_000_000_000_000)
- Never float64 — all monetary and probability arithmetic uses `math/big.Rat` or integer base-unit strings
- Five signal types: NEW_MARKET, PROBABILITY_SHIFT, ACTIVITY_CHANGE, LIQUIDITY_CHANGE, MARKET_MOVEMENT
- Four severity levels: INFO, WATCH, SIGNIFICANT, CRITICAL
- Deterministic fingerprinting (SHA-256) for deduplication — excludes DB IDs and timestamps

### Panta-Native Integration
- Panta Adapter isolates Qevryn from Panta schema changes
- Circuit breaker (5 failures → open, 30s timeout, 2 successes → closed)
- Retries with jittered exponential backoff, Retry-After header respected
- Typed Go client with request/response validation
- Server-side Panta API key only; browser never sees it

### AI Grounding
- Copilot uses tools: `search_markets`, `get_market_intelligence`, `get_signals`, `get_recent_changes`, `compare_markets`
- Every response cites sources; tool calls logged
- Market Studio: AI drafts → deterministic re-validation (Go) → human review → quote → build → sign → broadcast → register
- AI never signs, creates, registers, or executes trades

### Enterprise Monitoring
- Watchlists with market membership
- Alert rules scoped to watchlist, market, or global
- Severity thresholds + numeric thresholds per signal type
- Cooldown periods (up to 7 days)
- Database-enforced deduplication (UNIQUE constraints + ON CONFLICT DO NOTHING)
- In-app notification inbox with read/unread state

### Human-Controlled Financial Execution
- Quote → Build → Wallet Sign → Broadcast → Confirm → Report → Verify → Refresh
- User's wallet signs; server never holds private keys
- Draft hash prevents silent changes after quote
- Panta registration is the ONLY step that creates a market

### Auditable Workflows
- Deterministic signal fingerprinting (SHA-256)
- Alert deduplication via database UNIQUE constraints
- Draft fingerprinting (SHA-256) for quote/build integrity
- Trade attempt lifecycle with full audit trail
- Market creation attempts with full state history

---

## Future Roadmap

### Implemented (Current)
- [x] Market Discovery (Panta catalog)
- [x] Deterministic Signal Engine (5 signal types, 4 severities)
- [x] Signal Radar (filtering, pagination, severity tabs)
- [x] Watchlists (CRUD, market membership)
- [x] Alerts (rules, cooldown, dedupe, in-app notifications)
- [x] AI Copilot (grounded, tool-calling, source citations)
- [x] Market Studio (9-step wizard, AI draft, deterministic validation)
- [x] Trading Workflow (quote → build → sign → broadcast → confirm → report → verify)
- [x] Market Creation (quote → build → sign → broadcast → register)
- [x] Portfolio (positions from Panta)
- [x] Observability (structured logs, Prometheus metrics, OTel tracing)

### Planned (Post-Hackathon)
- [ ] WebSocket streaming for real-time Signal Radar updates
- [ ] Historical data ingestion (pre-ingestion period)
- [ ] Claims workflow (Panta settlement)
- [ ] Creator fees integration
- [ ] Trade attribution / on-chain analytics
- [ ] WebSocket push for Signal Radar
- [ ] Advanced charting (Recharts integration)
- [ ] Mobile-responsive optimizations
- [ ] Multi-language support
- [ ] Advanced alert compositions (AND/OR/NOT)
- [ ] Team/workspace collaboration features
- [ ] API keys for programmatic access
- [ ] Webhook notifications (email, Slack, Discord)
- [ ] Advanced charting with historical probability curves
- [ ] Market correlation analysis
- [ ] Portfolio analytics (PnL, win rate, Sharpe)

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

## Technical Specifications

### Signal Engine (Rust)
- Fixed-point arithmetic: 12 decimal places (SCALE = 1_000_000_000_000)
- 5 signal types: NEW_MARKET, PROBABILITY_SHIFT, ACTIVITY_CHANGE, LIQUIDITY_CHANGE, MARKET_MOVEMENT
- 4 severities: INFO, WATCH, SIGNIFICANT, CRITICAL
- SHA-256 fingerprinting for deduplication
- `math/big.Rat` for all probability arithmetic

### Panta Integration
- Market listing: `GET /api/v1/markets/` (cursor pagination, filters)
- Market detail: `GET /api/v1/markets/{id}/`
- Market creation: quote → build → register
- Trading: quote → build → sign → broadcast → confirm → report → verify
- Authentication: `X-Api-Key` header (server-side only)
- Amounts: USDC base units (integer strings, 6 decimals)

### Amount Types
- `HumanUSDC`: human-readable decimal string (e.g., "20.00")
- `USDCBaseUnits`: integer base-unit string (e.g., "50000000")
- Never parsed as float64

---

## Security Model

| Principle | Implementation |
|-----------|----------------|
| No private keys on server | Server stores only Panta API key |
| JWT auth | HS256, configurable expiry, audience validation |
| Rate limiting | Token bucket + sliding window per IP+path |
| Circuit breaker | Panta adapter (5 failures → open, 30s timeout) |
| Request size limit | 1MB |
| Structured logging | JSON, request IDs, no secrets |
| Parameterized SQL | pgx parameterized queries |
| Custody model | Server never holds private keys; user signs in wallet |

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
├── contracts/evm/              # Foundry Solidity 0.8.25
├── proto/                      # Protobuf (placeholder)
├── infrastructure/             # Docker Compose
├── docs/                       # Architecture, runbooks, API ref
├── Makefile                    # dev, test, lint, build
└── .env.example                # Environment template
```

---

## Local Development

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

# 2. Build market engine (Linux/CI recommended)
cd services/market-engine && cargo build --release

# 3. Run services (each in separate terminal)
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
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
curl http://localhost:8081/health
curl http://localhost:8001/health
```

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
| `AI_API_KEY` | Intelligence | AI features |
| `MARKET_ENGINE_BIN` | Worker | Yes |
| `CORS_ALLOWED_ORIGINS` | Intelligence | Production |
| `JWT_SECRET` | Gateway | Yes |
| `JWT_EXPIRY` | Gateway | No (default 24h) |
| `JWT_AUDIENCE` | Gateway | No (default: qevryn-api) |

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
| `docs/production-readiness.md` | Production audit |
| `docs/production-checklist.md` | Production checklist |
| `docs/security.md` | Security model |
| `docs/operations.md` | Operations runbook |
| `docs/demo-runbook.md` | Demo script + failure plan |
| `docs/data-lineage.md` | Data flow documentation |
| `docs/test-matrix.md` | Test coverage matrix |
| `docs/production-readiness.md` | Production audit |
| `docs/production-checklist.md` | Production checklist |

---

## License

Proprietary — All rights reserved.

---

*QEVRYN — Enterprise Prediction Intelligence Platform*  
*See what the market thinks happens next.*