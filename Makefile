.PHONY: dev test lint build panta-smoke

DEV_COMPOSE := infrastructure/docker-compose.dev.yml

help:
	@echo "Available targets: dev, test, lint, build, panta-smoke"

default: help

dev:
	docker compose -f $(DEV_COMPOSE) up -d postgres redis
	cd apps/web && npm run dev

test:
	cd apps/web && npm run build
	cd packages/types && go test ./...
	cd services/gateway && go test ./...
	cd services/panta-adapter && go test ./...
	cd services/intelligence && python -m pytest -q
	cargo test --manifest-path services/market-engine/Cargo.toml
	cd contracts/evm && forge test

lint:
	cd apps/web && npm run lint
	cd services/gateway && go vet ./... && go test ./...
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

panta-smoke:
	cd services/panta-adapter && go run ./cmd/smoke-test
