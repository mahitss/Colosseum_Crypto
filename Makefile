.PHONY: dev test lint build panta-smoke worker worker-build market-engine-cross

DEV_COMPOSE := infrastructure/docker-compose.dev.yml

help:
	@echo "Available targets: dev, test, lint, build, panta-smoke, worker, worker-build, market-engine-cross"

default: help

dev:
	docker compose -f $(DEV_COMPOSE) up -d postgres redis
	cd apps/web && npm run dev

test:
	cd apps/web && node --test "scripts/*.test.mjs"
	cd apps/web && npm run build
	cd packages/types && go test ./...
	cd services/gateway && go test ./...
	cd services/panta-adapter && go test ./...
	cd services/intelligence && python -m pytest -q
	cargo test --manifest-path services/market-engine/Cargo.toml
	cd contracts/evm && forge test

lint:
	cd apps/web && npm run lint
	cd services/gateway && go vet ./... && go test ./... && go build -o bin/worker ./cmd/worker
	cd services/panta-adapter && go vet ./... && go test ./...
	cd services/intelligence && python -m compileall app
	cargo clippy --manifest-path services/market-engine/Cargo.toml --all-targets -- -D warnings

build:
	cd apps/web && npm run build
	cd services/gateway && go build ./...
	cd services/panta-adapter && go build ./...
	cd services/intelligence && python -m compileall app
	cargo build --manifest-path services/market-engine/Cargo.toml
	cd contracts/evm && forge build

# worker-build compiles the background worker and the deterministic engine it
# shells out to. These are the two binaries that make up the ingestion chain;
# the worker is useless without the engine, so they are built together.
worker-build:
	cd services/gateway && go build -o bin/worker ./cmd/worker
	cargo build --release --manifest-path services/market-engine/Cargo.toml

# market-engine-cross builds the market-engine for Windows using Docker cross-compilation.
# Use this on Windows/macOS/Linux when MSVC/Visual Studio is not available locally.
# Requires Docker. Outputs to services/market-engine/target/x86_64-pc-windows-gnu/release/market-engine.exe
market-engine-cross:
	docker build -f services/market-engine/Dockerfile.cross --target builder -t market-engine-builder .
	docker create --name market-engine-builder-bin market-engine-builder
	docker cp market-engine-builder-bin:/app/target/x86_64-pc-windows-gnu/release/market-engine.exe services/market-engine/target/x86_64-pc-windows-gnu/release/market-engine.exe
	docker rm market-engine-builder-bin

# worker runs the alert-ingestion worker in the foreground. It requires
# DATABASE_URL and MARKET_ENGINE_BIN; see docs/watchlists-alerts.md.
worker: worker-build
	cd services/gateway && go run ./cmd/worker

panta-smoke:
	cd services/panta-adapter && go run ./cmd/smoke-test