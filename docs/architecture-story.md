# Architecture Story

## Why These Technologies?

Every language in this stack exists for a real, concrete responsibility — not because "more languages = better."

---

## TypeScript / Next.js → Product & UI

**Why:** The user-facing product lives in the browser. Next.js 16 (App Router) gives us:
- Server components for fast initial loads
- Client components for interactivity (wallet, charts, real-time updates)
- TypeScript end-to-end with shared types from `packages/types`
- Built-in API routes for BFF pattern (Next.js API routes proxy to Go gateway)

**Why not Go templates / Python Jinja / Rust WASM?**
- Product velocity: React ecosystem, component libraries (Radix UI), wallet adapter ecosystem (Solana wallet adapters are React-first)
- TypeScript gives us compile-time contracts matching Go backend types via `packages/types`

**What we don't use it for:** No business logic, no signal computation, no AI orchestration, no database access.

---

## Go → API Boundary & Orchestration

**Why:** The Gateway is the system's front door. Go excels at:
- High-throughput HTTP serving (net/http, chi/stdlib)
- Concurrency for concurrent Panta requests, WebSocket upgrades (future)
- Rich stdlib: `net/http`, `crypto`, `encoding/json`, `time`, `context`
- Excellent PostgreSQL driver (`pgx/v5`) with prepared statements, connection pooling
- Excellent observability: `slog`, `pprof`, `expvar`, OpenTelemetry libraries
- Single binary deployment, fast startup, low memory

**What it owns:**
- HTTP API (market discovery, trading, market studio, watchlists, alerts, notifications)
- Authentication (JWT validation, user resolution)
- Rate limiting, circuit breaker, request ID propagation
- Request/response validation (strict decoding, no map[string]any)
- Orchestration: calls Panta Adapter, Intelligence, Market Engine

**What it does NOT do:**
- No signal computation (that's Rust)
- No AI orchestration (that's Python)
- No direct Panta API calls from browser (Panta Adapter isolates)

---

## Python → AI / Intelligence Orchestration

**Why:** The AI ecosystem lives in Python. Full stop.

**What it owns:**
- **Market Studio Agent:** Interprets natural language → structured draft → deterministic re-validation
- **AI Copilot:** Tool-calling agent with grounded tools (`search_markets`, `get_market_intelligence`, `get_signals`, `get_recent_changes`, `compare_markets`)
- **Deterministic validation** (shared logic with Go, but authoritative in Python for AI draft)
- **Prompt engineering:** System prompts, tool schemas, output parsing

**Why not Go?**
- No mature OpenAI SDK with tool calling / structured output
- Prompt engineering iteration speed matters
- Pydantic for schema validation of AI output
- LangChain/LangGraph ecosystem if needed later

**What it does NOT do:**
- No direct database access (gateway owns DB)
- No Panta API calls (gateway owns Panta boundary)
- No signal computation (Rust owns that)

---

## Rust → Deterministic Computation

**Why:** Signal generation requires:
- **Exact arithmetic** — no float64 rounding errors ever
- **Deterministic output** — same input → same output, always
- **Performance** — process thousands of markets per tick
- **Correctness** — fixed-point arithmetic with compile-time guarantees

**What it owns:**
- Signal engine: `generate_signals(EngineRequest) → Vec<Signal>`
- Fixed-point arithmetic: 12 decimal places, `SCALE = 1_000_000_000_000`
- Five signal types: `NEW_MARKET`, `PROBABILITY_SHIFT`, `ACTIVITY_CHANGE`, `LIQUIDITY_CHANGE`, `MARKET_MOVEMENT`
- Four severities: `INFO`, `WATCH`, `SIGNIFICANT`, `CRITICAL`
- Deterministic fingerprinting (SHA-256) for deduplication
- CLI interface: stdin JSON → stdout JSON (no HTTP, no DB, no network)

**Interface:**
```rust
pub fn generate_signals(request: EngineRequest) -> Result<Vec<Signal>, EngineError>
```

**Input:** `EngineRequest { config: SignalConfig, observations: Vec<ObservationInput> }`
**Output:** `Vec<Signal>` with deterministic fields

**Why not Go/Python?**
- Go: no fixed-point decimal in stdlib, easy to accidentally use float64
- Python: decimal module exists but slower, GIL limits throughput, easier to accidentally use float
- Rust: `Fixed` type enforces correctness at compile time; zero-cost abstractions; no GC pauses

---

## PostgreSQL → Durable State

**Why:** ACID, rich types, mature tooling, JSONB for flexible metadata, advisory locks for worker coordination.

**What it stores:**
- Markets, observations, signals (canonical)
- Signal events (durable event stream with fingerprint deduplication)
- Watchlists, watchlist_markets
- Alert rules, alert events, notifications
- Trade attempts, market creation attempts
- Schema migrations (embedded, versioned)

**Why not MongoDB / DynamoDB / SQLite?**
- ACID + JSONB + advisory locks + mature Go driver = right tool for this workload
- Advisory locks enable single-worker guarantee without external coordination service

---

## Redis → Short-Lived Coordination (Optional / Future)

Currently: **Not used in hot path.** Reserved for:
- Distributed rate limiting (if multi-instance gateway)
- Session caching (if we scale gateway horizontally)
- Distributed locks (if we need cross-instance coordination)

**Currently:** Gateway is single-instance (advisory lock on PG). Redis runs in dev for parity.

---

## Solana → Settlement Layer

**Why:** Panta chose Solana for:
- Sub-second finality
- Low transaction costs (predictable fees)
- SPL token standard (USDC)
- Mature wallet ecosystem (Phantom, Solflare, Backpack, etc.)
- RPC infrastructure

**What Prophet does on Solana:**
- Broadcasts user-signed transactions (via configured RPC)
- Polls for confirmation (`getSignatureStatuses`)
- Reports signature to Panta for verification
- Never holds private keys, never signs

---

## Panta → Prediction Market Infrastructure

**Why Panta?** It's the prediction-market protocol on Solana with:
- Live market catalog API
- Primary-buy trading (pool-based liquidity)
- Market creation with fee quotes + unsigned tx building + registration
- On-chain settlement with verification endpoint

**Why not build our own market protocol?**
- Panta solves the hard parts: liquidity bootstrapping, AMM math, on-chain settlement, dispute resolution
- Prophet's value is **intelligence on top**, not reinventing the market protocol
- Panta handles regulatory/compliance for the market layer

---

## Why Not One Service?

| If we combined... | Problem |
|-------------------|---------|
| Gateway + Intelligence | AI latency blocks HTTP requests; Python GIL blocks Go concurrency |
| Gateway + Market Engine | GC pauses / GC pauses in Go; Rust needs deterministic latency |
| Intelligence + Market Engine | Python can't do deterministic fixed-point; Rust can't do AI |
| Everything in Rust | No mature AI ecosystem, slower product velocity for UI/API |
| Everything in Go | No mature AI tooling, no fixed-point decimal in stdlib |
| Everything in Python | GIL kills throughput; float64 traps; no typed HTTP boundary |

**The split is the feature.** Each language does exactly one thing well.

---

## Data Flow Summary

```
User Action (Browser)
       │
       ▼
Next.js (UI, wallet, wallet signing)
       │
       ▼ HTTPS + JWT
Go Gateway (8080)
├── /api/v1/markets*      → Panta Adapter (8081) → Panta API
├── /api/v1/trades/*      → Panta Adapter → Solana RPC
├── /api/v1/market-studio/* → Intelligence (8001) + Panta Adapter
├── /api/v1/copilot/*     → Intelligence (8001)
├── /api/v1/watchlists*   → PostgreSQL
├── /api/v1/alerts*       → PostgreSQL
└── /api/v1/notifications* → PostgreSQL
       │
       ▼
Intelligence (8001)
├── /v1/market-studio/interpret  → OpenAI → structured draft
├── /v1/market-studio/validate   → deterministic (no LLM)
├── /v1/copilot/query            → tools → OpenAI → grounded response

Rust Market Engine (CLI)
  stdin: EngineRequest JSON
  stdout: Vec<Signal> JSON
  (no network, no DB, no config)

PostgreSQL (5432)
  markets, observations, signals, signal_events, watchlists, alerts, notifications, trades, market_creation_attempts

Redis (6379)
  (reserved for future: rate limiting, sessions, distributed locks)
```

---

## Why This Matters

Each language pays for its complexity:
- **TypeScript** pays for: UI complexity, wallet integration, reactive state
- **Go** pays for: HTTP boundary, auth, rate limiting, orchestration, PostgreSQL
- **Python** pays for: AI model interaction, prompt engineering, tool calling
- **Rust** pays for: numerical correctness, deterministic output, throughput

The boundaries are clean. Each service can be replaced, rewritten, or scaled independently. The contracts between them are explicit (HTTP + JSON, stdin/stdout JSON, SQL).

**This is not "microservices for the sake of microservices."** It's "right tool for each job, with clear contracts."