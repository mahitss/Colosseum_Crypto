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

## Planned decomposition

- Web application: user-facing experience and dashboard surfaces
- Go gateway: API orchestration and service coordination
- Panta adapter: external Panta integration boundary
- Intelligence service: AI and market analysis workflows
- Market engine: deterministic signal generation and execution support
- Contracts: on-chain contracts and environment validation
