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

For market reads, the request path is Browser → Prophet Web → Go Gateway → Panta Adapter → Panta API. The gateway calls the adapter over an internal typed HTTP interface. Panta authentication, retry policy, Panta response models, and response validation live only in `services/panta-adapter`. Valid upstream records are mapped into shared Prophet market types before returning to the gateway. The browser never receives or uses the server-side Panta API key.

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

## Custody boundary

The backend stores only the Panta server API key in its runtime environment. It never receives wallet private keys or seed phrases. Future transaction flows will request unsigned transactions/instructions from Panta, have the user's wallet sign them, broadcast through a Solana RPC, and report only the resulting signature to Panta. Trading and signing are not implemented in this task.

## Planned decomposition

- Web application: user-facing experience and dashboard surfaces
- Go gateway: API orchestration and service coordination
- Panta adapter: external Panta integration boundary
- Intelligence service: AI and market analysis workflows
- Market engine: deterministic signal generation and execution support
- Contracts: on-chain contracts and environment validation
