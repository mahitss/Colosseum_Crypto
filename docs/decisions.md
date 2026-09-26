# Architecture Decisions

## Monorepo layout

The repository uses a monorepo layout to keep the web application, API services, shared packages, and contracts in one cohesive workspace while allowing independent delivery and testing.

## Service boundaries

- Gateway is the main public-facing API and orchestration layer.
- Panta adapter isolates external Panta connectivity from the rest of the system.
- Intelligence service owns AI interaction concerns.
- Market engine owns deterministic signal computation.

## Panta integration

Panta communication is isolated in the Go adapter. The gateway calls an internal typed HTTP contract and returns shared Prophet domain objects rather than Panta JSON. Panta models and parsing stay in the adapter so upstream schema changes do not leak into the public API.

The adapter authenticates server-to-server with `X-Api-Key`, retries only network/timeout, 429, and 5xx failures with bounded exponential backoff, and respects `Retry-After` up to a bounded wait. Authentication, authorization, validation, and not-found errors are never retried; errors returned to callers omit raw upstream bodies and internal HTTP details.

The Panta key is server configuration, never browser configuration. The backend does not receive wallet private keys: users retain signing custody. Future transaction submission will handle unsigned Panta instructions and public transaction signatures only.

Money uses explicit string types: `HumanUSDC` for human-readable decimal operations and `USDCBaseUnits` for integer base-unit operations. No implicit conversions are provided.

## Runtime choices

- Next.js provides the frontend experience with App Router and TypeScript.
- Go powers the API orchestration layer and HTTP services.
- Python gives the team a clean AI and analysis environment via FastAPI.
- Rust is reserved for high-performance and deterministic computational components.

## Security

Environment variables and secrets are intentionally kept out of version control. All services are expected to read config from environment-backed settings.

## Validation

The project validates the repository foundation by running frontend builds, Go tests, Python tests, Rust tests, and Foundry tests in CI.
