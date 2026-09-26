# Prophet

Prophet is a prediction intelligence platform built to unify market discovery, deterministic signals, AI analysis, and blockchain-facing workflows.

## Repository layout

- apps/web: Next.js frontend
- services/gateway: Go gateway service
- services/panta-adapter: Go Panta integration boundary
- services/intelligence: FastAPI intelligence service
- services/market-engine: Rust signal engine
- contracts/evm: Foundry Solidity contracts
- packages/ui, packages/types, packages/sdk: shared reusable packages
- proto: protobuf definitions and generated code
- infrastructure: Docker and infrastructure definitions
- docs: architecture and development documentation
- tests: shared testing assets

## Local tasks

```bash
make dev
make test
make lint
make build
```

## Environment

Use `.env.example` as the list of supported environment variables. Export values in the shell for the service being run; do not commit secrets.
