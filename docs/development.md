# Development Guide

## Getting started

1. Copy `.env.example` to `.env` and set the required environment values.
2. Start the local infrastructure services:
   - PostgreSQL
   - Redis
3. Install frontend dependencies and start the web app.
4. Run the service and package test suites as needed.

## Local infrastructure

Use Docker Compose from the infrastructure directory:

```bash
docker compose -f infrastructure/docker-compose.dev.yml up -d postgres redis
```

## Common commands

```bash
make dev
make test
make lint
make build
```

## Working conventions

- Keep business logic out of this foundation task.
- Maintain strict TypeScript and clean Go/Python/Rust conventions.
- Do not add secrets to the repository.
- Keep all integrations behind explicit service boundaries.
