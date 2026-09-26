.PHONY: dev test lint build

DEV_COMPOSE := infrastructure/docker-compose.dev.yml

help:
	@echo "Available targets: dev, test, lint, build"

default: help

dev:
	docker compose -f $(DEV_COMPOSE) up -d postgres redis
	cd apps/web && npm install && npm run dev

test:
	cd apps/web && npm install && npm run build
	cd services/gateway && go test ./...
	cd services/panta-adapter && go test ./...
	cd services/intelligence && python -m pip install -r requirements.txt >/dev/null && pytest -q
	cargo test --manifest-path services/market-engine/Cargo.toml
	cd contracts/evm && forge test

lint:
	cd apps/web && npm install && npm run lint
	cd services/gateway && go test ./...
	cd services/panta-adapter && go test ./...
	cd services/intelligence && python -m pip install -r requirements.txt >/dev/null && python -m compileall app
	cargo test --manifest-path services/market-engine/Cargo.toml -- --nocapture

build:
	cd apps/web && npm install && npm run build
	cd services/gateway && go build ./...
	cd services/panta-adapter && go build ./...
	cd services/intelligence && python -m pip install -r requirements.txt >/dev/null && python -m compileall app
	cargo build --manifest-path services/market-engine/Cargo.toml
	cd contracts/evm && forge build
