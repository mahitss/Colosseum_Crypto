# Architecture Decisions

## Monorepo layout

The repository uses a monorepo layout to keep the web application, API services, shared packages, and contracts in one cohesive workspace while allowing independent delivery and testing.

## Service boundaries

- Gateway is the main public-facing API and orchestration layer.
- Panta adapter isolates external Panta connectivity from the rest of the system.
- Intelligence service owns AI interaction concerns.
- Market engine owns deterministic signal computation.

## Runtime choices

- Next.js provides the frontend experience with App Router and TypeScript.
- Go powers the API orchestration layer and HTTP services.
- Python gives the team a clean AI and analysis environment via FastAPI.
- Rust is reserved for high-performance and deterministic computational components.

## Security

Environment variables and secrets are intentionally kept out of version control. All services are expected to read config from environment-backed settings.

## Validation

The project validates the repository foundation by running frontend builds, Go tests, Python tests, Rust tests, and Foundry tests in CI.
