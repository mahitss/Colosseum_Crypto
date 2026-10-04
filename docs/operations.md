# QEVRYN Operations Manual

**Version:** 1.0  
**Date:** 2025-09-29

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Local Development](#local-development)
3. [Environment Configuration](#environment-configuration)
4. [Deployment](#deployment)
5. [Database Migrations](#database-migrations)
6. [Health Checks](#health-checks)
7. [Logs](#logs)
8. [Metrics](#metrics)
9. [Tracing](#tracing)
10. [Panta Outage Handling](#panta-outage-handling)
11. [Database Outage Handling](#database-outage-handling)
12. [Redis Outage Handling](#redis-outage-handling)
13. [AI Provider Outage Handling](#ai-provider-outage-handling)
14. [Incident Investigation](#incident-investigation)
15. [Failed Trade Investigation](#failed-trade-investigation)
16. [Failed Market Creation Investigation](#failed-market-creation-investigation)
17. [Rollback Procedures](#rollback-procedures)

---

## Architecture Overview

```
Browser (Next.js)
       |
       v
Go Gateway (port 8080)
       |
       v
Panta Adapter (port 8081)
       |
       v
Panta API + Solana
```

**Key Data Flows:**

1. **Market Reads**: Browser -> Gateway -> Panta Adapter -> Panta API
2. **Trading**: Browser -> Gateway -> Panta Adapter (quote/build) -> Wallet (sign) -> Solana RPC -> Panta Adapter (report/verify)
3. **Market Creation**: Browser -> Gateway -> Intelligence (AI draft) -> Gateway (validate/quote/build) -> Wallet (sign) -> Solana -> Panta (register) -> Gateway (index)
4. **Signal Generation**: Worker -> Panta Adapter -> Markets/Observations -> Market Engine (Rust) -> Signal Events -> Alert Evaluation -> Notifications

---

## Local Development

### Prerequisites

- Go 1.23+
- Python 3.13+
- Node.js 20+
- Rust 1.81+
- Docker & Docker Compose
- PostgreSQL 16 (via Docker)
- Redis 7 (via Docker)

### Quick Start

```bash
# 1. Start infrastructure
docker compose -f infrastructure/docker-compose.dev.yml up -d postgres redis

# 2. Export environment variables (see .env.example)
export DATABASE_URL="postgresql://QEVRYN:QEVRYN@localhost:5432/QEVRYN"
export PANTA_ADAPTER_URL="http://127.0.0.1:8081"
export PANTA_API_KEY="your-panta-api-key"
export SOLANA_RPC_URL="https://api.mainnet-beta.solana.com"
export AI_API_KEY="your-openai-api-key"
export JWT_SECRET="$(openssl rand -hex 32)"
export MARKET_ENGINE_BIN="./services/market-engine/target/release/market-engine"

# 3. Build market engine
cd services/market-engine && cargo build --release

# 4. Run services (each in separate terminal)
# Terminal 1: Panta Adapter
cd services/panta-adapter && go run ./cmd/server

# Terminal 2: Intelligence
cd services/intelligence && python -m app.main

# Terminal 3: Gateway
cd services/gateway && go run .

# Terminal 4: Worker (optional, for alerts)
cd services/gateway && go run ./cmd/worker

# Terminal 5: Frontend
cd apps/web && npm run dev
```

### Running Tests

```bash
# All Go tests
cd services/gateway && go test ./... -race
cd services/panta-adapter && go test ./...

# Python tests
cd services/intelligence && python -m pytest -v

# Frontend tests
cd apps/web && npm test

# Rust tests
cd services/market-engine && cargo test

# Smart contract tests
cd contracts/evm && forge test
```

---

## Environment Configuration

### Required Variables (All Services)

| Variable | Description | Example |
|----------|-------------|---------|
| DATABASE_URL | PostgreSQL connection string | postgresql://user:pass@host:5432/db |
| PANTA_API_KEY | Panta server API key (never commit!) | panta_sk_... |
| JWT_SECRET | JWT signing secret (32+ bytes) | openssl rand -hex 32 |

### Service-Specific Variables

| Service | Variable | Required | Default |
|---------|----------|----------|---------|
| Gateway | PORT | No | 8080 |
| Gateway | PANTA_ADAPTER_URL | No | http://127.0.0.1:8081 |
| Gateway | PANTA_API_URL | Trading only | https://live-api.panta.market/api/v1/ |
| Gateway | PANTA_API_KEY | Trading only | - |
| Gateway | SOLANA_RPC_URL | Trading only | - |
| Gateway | INTELLIGENCE_SERVICE_URL | No | http://localhost:8001 |
| Gateway | JWT_SECRET | Yes | - |
| Gateway | JWT_EXPIRY | No | 24h |
| Gateway | JWT_AUDIENCE | No | QEVRYN-api |
| Panta Adapter | PANTA_API_BASE_URL | No | https://live-api.panta.market/api/v1/ |
| Panta Adapter | PANTA_API_KEY | Yes | - |
| Panta Adapter | PANTA_API_TIMEOUT_SECONDS | No | 10 |
| Intelligence | APP_ENV | No | development |
| Intelligence | PORT | No | 8001 |
| Intelligence | AI_PROVIDER | No | openai |
| Intelligence | AI_API_KEY | For AI features | - |
| Intelligence | CORS_ALLOWED_ORIGINS | Production only | https://app.example.com |
| Worker | MARKET_ENGINE_BIN | Yes | /path/to/market-engine |
| Worker | QEVRYN_SYNC_INTERVAL_SECONDS | No | 300 |
| Worker | WORKER_ADVISORY_LOCK_KEY | Per deployment | 0x50524F5048455445 |

### Secrets Management

**Never commit secrets.** Use one of:
- Docker secrets: `echo "secret" | docker secret create panta_api_key -`
- HashiCorp Vault
- AWS Secrets Manager
- 1Password CLI: `op read op://vault/item/field`
- GitHub Actions secrets for CI/CD

---

## Deployment

### Production Deployment (Docker Compose)

```bash
# 1. Set all required environment variables
export POSTGRES_PASSWORD="secure-random-password"
export PANTA_API_KEY="panta_sk_..."
export SOLANA_RPC_URL="https://api.mainnet-beta.solana.com"
export AI_API_KEY="sk-..."
export JWT_SECRET="$(openssl rand -hex 32)"
export JWT_EXPIRY="24h"

# 2. Build and deploy
docker compose -f infrastructure/docker-compose.prod.yml build
docker compose -f infrastructure/docker-compose.prod.yml up -d

# 3. Verify health
curl -f http://localhost:8080/health/live
curl -f http://localhost:8080/health/ready
curl -f http://localhost:8081/health
curl -f http://localhost:8001/health
```

### Kubernetes Deployment (Future)

```yaml
# Key considerations:
# - Use StatefulSet for PostgreSQL
# - Use Deployment for stateless services
# - Configure liveness/readiness probes
# - Set resource requests/limits
# - Use ConfigMaps for non-secret config
# - Use Secrets for sensitive data
# - Configure PodDisruptionBudgets
# - Set up HorizontalPodAutoscaler for gateway
```

### Worker Deployment Notes

- **Single instance only**: Advisory lock prevents multiple workers
- **Lock key**: Must be unique per deployment sharing a database
- **Restart policy**: Always restart (`unless-stopped` or `Always`)
- **Resource limits**: 256Mi memory limit recommended

---

## Database Migrations

### Running Migrations

Migrations run automatically on service startup:

```bash
# Gateway runs migrations on startup
cd services/gateway && go run .

# Or run manually
cd services/gateway && go run ./cmd/migrate
```

### Migration Files

Located in `services/gateway/internal/intelligence/migrations/`:

| File | Description |
|------|-------------|
| 001_market_intelligence.sql | Core market, observation, signal tables |
| 002_trades.sql | Trade attempts, positions |
| 003_market_creation_attempts.sql | Market creation workflow |
| 004_watchlists_alerts.sql | Watchlists, signals, alerts, notifications |

### Adding New Migrations

1. Create `XXX_description.sql` in migrations folder
2. Use next sequential number
3. Test locally: `go run ./cmd/migrate`
4. Commit migration file
5. Deploy - runs automatically on startup

### Rollback

```bash
# Manual rollback (if needed)
psql $DATABASE_URL -c "DELETE FROM schema_migrations WHERE version = XXX;"
# Then manually run DOWN migration SQL
```

**Note**: Always test rollback in staging first.

---

## Health Checks

### Endpoints

| Service | Liveness | Readiness |
|---------|----------|-----------|
| Gateway | GET /health/live | GET /health/ready |
| Panta Adapter | GET /health | GET /health/panta |
| Intelligence | GET /health | GET /health |

### Liveness vs Readiness

| Probe | Purpose | Dependencies |
|-------|---------|--------------|
| Liveness | "Is process alive?" | None - just process check |
| Readiness | "Can serve traffic?" | DB, Panta Adapter, Intelligence |

### Kubernetes Probes

```yaml
livenessProbe:
  httpGet:
    path: /health/live
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 30

readinessProbe:
  httpGet:
    path: /health/ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

### Readiness Checks (Gateway)

- Database connectivity
- Panta Adapter reachable
- Intelligence service reachable

---

## Logs

### Log Format

All services use structured JSON logging via `slog`:

```json
{
  "time": "2026-09-29T12:34:56.789Z",
  "level": "INFO",
  "msg": "market sync finished",
  "request_id": "abc123...",
  "markets_fetched": 150,
  "signals_generated": 42,
  "duration_ms": 2341
}
```

### Key Log Fields

| Field | Description |
|-------|-------------|
| time | RFC3339 timestamp |
| level | DEBUG/INFO/WARN/ERROR |
| msg | Human-readable message |
| request_id | Correlation ID (X-Request-Id) |
| service | Service name |

### Log Levels

| Level | When to Use |
|-------|-------------|
| DEBUG | Detailed diagnostic (dev only) |
| INFO | Normal operations, key events |
| WARN | Recoverable issues, retries |
| ERROR | Failures requiring attention |

### Log Aggregation

```bash
# Docker Compose
docker compose logs -f gateway

# Kubernetes
kubectl logs -f deployment/gateway -c gateway

# With jq for parsing
docker compose logs gateway | jq 'select(.level=="ERROR")'
```

### Sensitive Data Policy

**NEVER LOG:**
- Authorization headers
- API keys (PANTA_API_KEY, AI_API_KEY)
- JWT tokens
- Wallet private keys/seed phrases
- Signed transaction blobs
- Database passwords

---

## Metrics

### Prometheus Endpoint

All services expose `/metrics` for Prometheus scraping.

### Key Metrics

#### Gateway (port 8080)

| Metric | Type | Description |
|--------|------|-------------|
| http_requests_total | Counter | Total HTTP requests by method, path, status |
| http_request_duration_seconds | Histogram | Request latency |
| panta_request_duration_seconds | Histogram | Panta API latency |
| panta_errors_total | Counter | Panta errors by operation |

#### Panta Adapter (port 8081)

| Metric | Type | Description |
|--------|------|-------------|
| panta_adapter_requests_total | Counter | Requests by operation, status |
| panta_adapter_latency_seconds | Histogram | Request latency |

#### Worker

| Metric | Type | Description |
|--------|------|-------------|
| worker_sync_duration_seconds | Histogram | Sync tick duration |
| worker_signals_generated_total | Counter | Signals generated per tick |
| worker_alerts_triggered_total | Counter | Alerts fired |
| worker_notifications_delivered_total | Counter | Notifications sent |
| worker_circuit_breaker_state | Gauge | 0=closed, 1=half-open, 2=open |

#### Intelligence (port 8001)

| Metric | Type | Description |
|--------|------|-------------|
| intelligence_requests_total | Counter | AI requests by type |
| intelligence_latency_seconds | Histogram | AI response latency |

### Prometheus Scraping

```yaml
scrape_configs:
  - job_name: 'QEVRYN'
    static_configs:
      - targets: ['gateway:8080', 'panta-adapter:8081', 'intelligence:8001']
```

### Alerting Rules (Example)

```yaml
groups:
- name: QEVRYN
  rules:
  - alert: HighErrorRate
    expr: rate(errors_total[5m]) > 0.1
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "High error rate on {{ $labels.service }}"

  - alert: WorkerDown
    expr: absent(worker_sync_duration_seconds) > 0
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "Worker not reporting sync metrics"

  - alert: HighPantaLatency
    expr: histogram_quantile(0.95, rate(panta_request_duration_seconds_bucket[5m])) > 10
    for: 5m
    labels:
      severity: warning
```

---

## Tracing

### OpenTelemetry Configuration

```bash
# Environment variables
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
OTEL_SERVICE_NAME=QEVRYN-gateway
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=production,service.version=1.0.0
```

### Trace Context Propagation

- W3C TraceContext headers (traceparent, tracestate)
- Propagated across: Gateway -> Panta Adapter -> Panta API
- Gateway -> Intelligence service
- Gateway -> Market Engine (via CLI)

### Jaeger/Tempo Query Examples

```bash
# Find slow requests
{service="QEVRYN-gateway"} | duration > 1s

# Find errors
{service="QEVRYN-gateway"} | level=error

# Trace a request
{service="QEVRYN-gateway"} | trace_id="abc123..."
```

---

## Panta Outage Handling

### Detection

- Health check `/health/panta` fails
- Increased `panta_errors_total` metric
- Circuit breaker opens (after 5 failures)

### Automatic Behavior

1. **Circuit Breaker Opens**: After 5 consecutive failures
   - Returns errors immediately without calling Panta
   - Prevents cascading timeouts
   
2. **Half-Open State**: After 30 seconds
   - Allows probe requests through
   - 2 successes -> Closed
   - 1 failure -> Open again

3. **Degraded Mode**: Gateway continues serving
   - Cached market data (if any)
   - Returns appropriate errors for write operations

### Manual Intervention

```bash
# Check circuit breaker state
curl http://localhost:8081/debug/circuit-breaker

# Force reset (if needed)
curl -X POST http://localhost:8081/debug/circuit-breaker/reset

# Check Panta status
curl -H "X-Api-Key: $PANTA_API_KEY" https://live-api.panta.market/api/v1/account/
```

### Communication

- Status page: Update `status.example.com`
- Alert on-call: PagerDuty/Slack
- User notification: Banner on UI

---

## Database Outage Handling

### Detection

- Readiness check `/health/ready` fails
- Increased connection errors in logs
- `pg_isready` fails

### Automatic Behavior

- Gateway returns 503 on `/health/ready`
- New requests get 503 immediately
- In-flight requests complete or timeout
- Worker pauses (DB connection fails)

### Recovery

```bash
# 1. Check PostgreSQL status
docker compose ps postgres
docker compose logs postgres

# 2. Restart if needed
docker compose restart postgres

# 3. Verify connectivity
pg_isready -h localhost -U QEVRYN -d QEVRYN

# 4. Restart services
docker compose restart gateway worker
```

### Data Integrity

- All writes use transactions
- Idempotency keys prevent duplicates
- No data loss on restart (ACID)

---

## Redis Outage Handling

### Current Usage

Redis is currently **optional** and not used by core services. Reserved for:
- Future caching layer
- Distributed rate limiting
- Session storage

### If Redis Fails

- No impact on current functionality
- Services log connection errors
- Graceful degradation

---

## AI Provider Outage Handling

### Detection

- Intelligence service returns 502/503
- Increased latency on `/v1/market-studio/interpret`
- Circuit breaker (if implemented) opens

### Fallback Behavior

1. **Market Studio Interpret**: Returns `needs_clarification=true` with deterministic fallback
2. **Market Studio Validate**: Works independently (no AI)
3. **Copilot**: Returns generic error, UI shows fallback message

### Configuration

```bash
# Disable AI features if provider unreliable
AI_API_KEY=""  # Disables AI features
```

---

## Incident Investigation

### General Approach

1. **Identify Scope**: Which service? Which endpoint? How many users?
2. **Check Health**: Liveness/readiness endpoints
3. **Check Logs**: Filter by `request_id`, `level=ERROR`
3. **Check Metrics**: Error rates, latency, saturation
4. **Check Traces**: Find slow/erroring requests
4. **Check Dependencies**: Panta, Database, AI provider

### Key Commands

```bash
# Recent errors
docker compose logs gateway | jq 'select(.level=="ERROR")' | head -20

# Specific request
docker compose logs gateway | jq 'select(.request_id=="abc123")'

# Metrics snapshot
curl -s http://localhost:8080/metrics | grep -E "http_requests_total|errors_total"

# Recent traces (if OTel configured)
# Use Jaeger/Tempo UI
```

### Common Issues

| Symptom | Likely Cause | Resolution |
|---------|--------------|------------|
| 503 on /health/ready | DB down | Check PostgreSQL |
| 502 on market studio | AI provider down | Check AI_API_KEY, fallback |
| 504 on trades | Panta timeout | Check Panta status, circuit breaker |
| High latency | DB load / Panta slow | Check metrics, scale DB |
| Worker not syncing | Advisory lock held | Check for duplicate worker |

---

## Failed Trade Investigation

### Trade Lifecycle States

```
IDLE -> QUOTING -> QUOTE_READY -> BUILDING -> READY_TO_SIGN -> SIGNING
  -> SIGNED -> BROADCASTING -> SUBMITTED -> CONFIRMING -> CONFIRMED
  -> REPORTING -> VERIFIED -> POSITION_REFRESHING -> COMPLETED
```

### Failure Points

| State | Common Failure | User Message |
|-------|----------------|--------------|
| QUOTING | Panta unavailable | "Unable to get quote" |
| BUILDING | Panta build failed | "Unable to build transaction" |
| BROADCASTING | RPC error | "Transaction broadcast failed" |
| CONFIRMING | Timeout/failed | "Transaction failed on-chain" |
| REPORTING | Panta report failed | "Confirmed on Solana. Panta verification pending." |

### Investigation Steps

```bash
# 1. Get trade attempt
curl -H "Authorization: Bearer $JWT" \
  http://localhost:8080/api/v1/trades/{attempt_id}

# 2. Check logs for attempt ID
docker compose logs gateway | jq 'select(.attempt_id=="...")'

# 3. Check Panta verification
curl -H "X-Api-Key: $PANTA_API_KEY" \
  https://live-api.panta.market/api/v1/transactions/{signature}/verify/

# 4. Check Solana explorer
# https://solscan.io/tx/{signature}
```

### Key Distinction

**Solana confirmed != Trade complete**. A trade is only complete after:
1. Solana confirms (CONFIRMED)
2. Panta reports (REPORTING -> VERIFIED)
3. Position refresh (COMPLETED)

---

## Failed Market Creation Investigation

### Creation Lifecycle

```
INTERPRET -> VALIDATE -> QUOTE -> BUILD -> SIGN -> BROADCAST -> CONFIRM -> REGISTER -> INDEX
```

### Failure Points

| Step | Error Code | User Message |
|------|------------|--------------|
| INTERPRET | AI_INTERPRETATION_FAILED | "AI interpretation failed" |
| VALIDATE | INVALID_DRAFT | "Draft invalid: ..." |
| QUOTE | QUOTE_FAILED | "Unable to get quote" |
| BUILD | BUILD_FAILED | "Unable to build transaction" |
| BROADCAST | BROADCAST_FAILED | "Transaction broadcast failed" |
| CONFIRM | CONFIRMATION_TIMEOUT / TRANSACTION_FAILED | "Transaction failed on-chain" |
| REGISTER | PANTA_REGISTRATION_FAILED | "Transaction confirmed on Solana. Panta registration is pending." |

### Critical Distinction

**Solana confirmed + Panta registration failed = NOT a failure**

The market exists on-chain but Panta hasn't indexed it yet. The UI must show:
> "Transaction confirmed on Solana. Panta registration is pending."

**Never show "creation failed" in this case.**

### Investigation

```bash
# 1. Get creation attempt
curl -H "Authorization: Bearer $JWT" \
  http://localhost:8080/api/v1/market-studio/attempts/{attempt_id}

# 2. Check logs
docker compose logs gateway | jq 'select(.attempt_id=="...")'

# 3. If REGISTERED but not INDEXED, wait and retry
# Poll GET /api/v1/market-studio/attempts/{id} until market_exists=true
```

---

## Rollback Procedures

### Application Rollback

```bash
# 1. Identify last good version
git log --oneline -10

# 2. Revert to previous version
git revert HEAD  # or git reset --hard <commit>

# 2. Rebuild and deploy
docker compose build
docker compose up -d

# 3. Verify health
curl -f http://localhost:8080/health/ready
```

### Database Rollback

```bash
# 1. Check migration history
psql $DATABASE_URL -c "SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 5;"

# 2. Remove migration record
psql $DATABASE_URL -c "DELETE FROM schema_migrations WHERE version = XXX;"

# 3. Run DOWN migration manually (if exists)
# Or manually reverse schema changes

# 3. Restart services
docker compose restart gateway
```

### Full System Rollback

```bash
# 1. Stop all services
docker compose down

# 2. Restore database from backup
pg_restore -d QEVRYN backup.dump

# 3. Deploy previous version
git checkout <previous-tag>
docker compose up -d
```

### Circuit Breaker Reset

```bash
# If circuit breaker stuck open
curl -X POST http://localhost:8081/debug/circuit-breaker/reset
```

---

## Appendix: Key Ports

| Service | Port | Protocol |
|---------|------|----------|
| Gateway | 8080 | HTTP |
| Panta Adapter | 8081 | HTTP |
| Intelligence | 8001 | HTTP |
| PostgreSQL | 5432 | TCP |
| Redis | 6379 | TCP |
| Solana RPC | 443 | HTTPS |

---

## Support Contacts

| Component | Contact | Escalation |
|-----------|---------|------------|
| Gateway | @backend-team | PagerDuty: QEVRYN-gateway |
| Panta Adapter | @backend-team | PagerDuty: QEVRYN-panta |
| Intelligence | @ml-team | PagerDuty: QEVRYN-intelligence |
| Worker | @backend-team | PagerDuty: QEVRYN-worker |
| Database | @dba-team | PagerDuty: QEVRYN-db |
| Panta API | support@panta.market | Email/Slack |
| AI Provider | OpenAI Support | API Dashboard |

---

*Document version 1.0 - Update on each deployment*
