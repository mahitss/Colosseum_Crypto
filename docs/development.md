# Development Guide

## Getting started

1. Use `.env.example` as the list of supported variables and export the values needed by each service in your shell.
2. Install the frontend dependencies once with `npm ci` from `apps/web`.
3. Install the Python service dependencies with `python -m pip install -r requirements.txt` from `services/intelligence`.
4. Export `PANTA_API_KEY` in your shell for authenticated adapter operations; never add it to `.env.example`, source control, browser code, or logs.
5. Start the local infrastructure services:
   - PostgreSQL
   - Redis
6. Run the service and package test suites as needed.

## Local infrastructure

Use Docker Compose from the infrastructure directory:

```bash
docker compose -f infrastructure/docker-compose.dev.yml up -d postgres redis
```

Export the variables in your shell before starting a service; services do not automatically load a root `.env` file. Secrets should not be committed.

## Common commands

```bash
make dev
make test
make lint
make build
make panta-smoke
```

The Panta adapter listens on port `8081` by default and the gateway on `8080`. Set `PANTA_ADAPTER_URL` for the gateway when the adapter is elsewhere.

## Working conventions

- Keep business logic out of this foundation task.
- Maintain strict TypeScript and clean Go/Python/Rust conventions.
- Do not add secrets to the repository.
- Keep all integrations behind explicit service boundaries.
