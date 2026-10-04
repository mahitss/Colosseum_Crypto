# Architecture

## Core platform flow

The project is organized around a clear request flow that preserves separation of responsibilities.

### Prediction data and blockchain

Next.js
    ↓
Go Gateway
    ↓
Panta Adapter
    ↓
Panta API
    ↓
Solana

The frontend sits in the Next.js application and communicates with the Go gateway. The Go gateway provides the public entry point for the web app and orchestrates downstream integrations, while the Panta adapter handles Panta-specific communication and translation. The Panta API is the upstream market and prediction service, and Solana is the underlying blockchain layer that supports on-chain activity.

For market reads, the request path is Browser → Qevryn Web → Go Gateway → Panta Adapter → Panta API. The gateway calls the adapter over an internal typed HTTP interface. Panta authentication, retry policy, Panta response models, and response validation live only in `services/panta-adapter`. Valid upstream records are mapped into shared Qevryn market types before returning to the gateway. The browser never receives or uses the server-side Panta API key.

The adapter sends `X-Api-Key` and a correlation `X-Request-Id` to Panta. It does not log request headers or response bodies. `/health` checks local configuration only; `/health/panta` makes an authenticated read-only `GET /account/` request.

### Intelligence and AI

Go Gateway
    ↓
Python Intelligence
    ↓
AI providers

The gateway is also responsible for routing requests that require market intelligence and analysis to the Python intelligence service. That service can orchestrate requests to external AI providers while keeping model interaction isolated from the main API boundary.

### Signal processing

Go Gateway
    ↓
Rust Market Engine

The Rust market engine handles deterministic signal computation and high-performance market-processing workflows. It is intentionally separate from the API layer so that signal logic can be developed and tested independently.

## Market creation

Next.js
    ↓
Go Gateway
    ↓
Python Intelligence (AI Market Architect, draft proposal only)
    ↓
Go Gateway (deterministic validation, quote, build)
    ↓
Panta API  →  Solana  →  Panta registration
    ↓
    Qevryn indexing

Market creation is a distinct flow from trading. The user describes a market in
prose, the AI Market Architect in `services/intelligence/app/market_studio/`
turns it into a structured draft, and deterministic validation decides whether
that draft is acceptable. The assistant never creates, signs, or submits
anything, and market-creation logic is deliberately kept out of the read-only
Copilot agent.

The Panta integration for creation lives in `services/gateway/internal/marketstudio/`
rather than in the Panta adapter, because creation is a write path with
attempts, idempotency, and state tracking, while the adapter handles market
reads. Fee amounts are carried as USDC base-unit integer strings end to end and
are never parsed as floating point, because that is the format Panta specifies
and rounding would change what the user is charged.

A Solana signature is not a created market. A market exists only after Panta
registration succeeds, and a confirmed transaction whose registration failed is
reported as pending registration rather than as a failure. See
[docs/market-studio.md](./market-studio.md) for the full flow, the nine-step
wizard, and the error semantics.

## User-owned monitoring surface

Watchlists, the signal radar, alert rules and the in-app notification inbox are
the user-owned half of the platform. They are served by
`services/gateway/internal/httpapi/{watchlists,alerts}.go` over tables created
by migration `004_watchlists_alerts.sql`, and they require no credential beyond
`DATABASE_URL` — unlike trading and market creation, which stay gated on
Panta and Solana configuration.

Ingestion is deliberately **not** part of the gateway. `services/gateway/cmd/worker`
is a separate binary that polls Panta, persists observations, asks the
deterministic engine for signals, bridges those signals into a fingerprinted
event stream, evaluates alert rules, and delivers notifications. It is separate
because the gateway serves user requests while the worker polls a third-party
API, and those have completely different failure and scaling characteristics.

Two properties are load-bearing and worth stating at the architecture level:

- **Deduplication lives in the database.** Three `UNIQUE` constraints on
  `market_observations`, `signal_events` and `alert_events`, every insert
  `ON CONFLICT DO NOTHING`. A crashed-and-retried tick converges on exactly the
  state an uninterrupted tick would have produced, which is also what makes the
  worker's deliberately overlapping read window safe.
- **No model participates in deciding whether an alert fires.** Matching is a
  plain predicate, explanations are assembled mechanically from the fields that
  are actually present, and all probability and money arithmetic uses
  `math/big.Rat` so a threshold comparison cannot flip on floating-point
  rounding at the boundary.

See [docs/watchlists-alerts.md](./watchlists-alerts.md) for the data model, the
evaluation order, the worker's operational limits, and the API surface.

## Custody boundary

The backend stores only the Panta server API key in its runtime environment. It
never receives wallet private keys or seed phrases.

Trading requests an unsigned transaction from Panta, has the user's wallet sign
it, broadcasts through a Solana RPC, and reports only the resulting signature
back to Panta. Market creation follows the same custody rule: the gateway
obtains an unsigned transaction from Panta and the user's wallet produces the
signature. No server-side signing key exists anywhere in the system, and the
`market_creation_attempts` table has no column for a private key, seed phrase,
or signed transaction blob.

## Planned decomposition

- Web application: user-facing experience and dashboard surfaces
- Go gateway: API orchestration and service coordination
- Panta adapter: external Panta integration boundary
- Intelligence service: AI and market analysis workflows
- Market engine: deterministic signal generation and execution support
- Contracts: on-chain contracts and environment validation
