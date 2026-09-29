# Prophet — Demo Safety Checklist

## Pre-Demo Verification (T-30 min)

### Infrastructure
- [ ] PostgreSQL running (`docker compose -f infrastructure/docker-compose.dev.yml up -d postgres`)
- [ ] Redis running (`docker compose -f infrastructure/docker-compose.dev.yml up -d redis`)
- [ ] Market Engine binary built (`cd services/market-engine && cargo build --release`)
- [ ] `MARKET_ENGINE_BIN` points to built binary

### Environment Variables Set
```bash
export DATABASE_URL="postgresql://prophet:prophet@localhost:5432/prophet"
export PANTA_API_KEY="your-panta-api-key"
export PANTA_API_BASE_URL="https://live-api.panta.market/api/v1/"
export SOLANA_RPC_URL="https://api.mainnet-beta.solana.com"
export AI_API_KEY="your-openai-key"
export JWT_SECRET="$(openssl rand -hex 32)"
export MARKET_ENGINE_BIN="./services/market-engine/target/release/market-engine"
export PANTA_ADAPTER_URL="http://127.0.0.1:8081"
export PANTA_API_KEY="your-panta-api-key"
export PANTA_API_URL="https://live-api.panta.market/api/v1/"
export SOLANA_RPC_URL="https://api.mainnet-beta.solana.com"
export AI_API_KEY="your-openai-key"
export INTELLIGENCE_SERVICE_URL="http://localhost:8001"
export JWT_SECRET="your-jwt-secret"
export JWT_EXPIRY="24h"
export JWT_AUDIENCE="prophet-api"
export CORS_ALLOWED_ORIGINS="http://localhost:3000"
```

### Services Running (5 terminals)
- [ ] Terminal 1: Panta Adapter (`cd services/panta-adapter && go run ./cmd/server`)
- [ ] Terminal 2: Intelligence (`cd services/intelligence && python -m app.main`)
- [ ] Terminal 3: Gateway (`cd services/gateway && go run .`)
- [ ] Terminal 4: Worker (`cd services/gateway && go run ./cmd/worker`)
- [ ] Terminal 5: Frontend (`cd apps/web && npm run dev`)

### Health Checks
- [ ] `curl -f http://localhost:8080/health/live` → `{"status":"ok"}`
- [ ] `curl -f http://localhost:8080/health/ready` → `{"status":"ready"}`
- [ ] `curl -f http://localhost:8081/health` → `{"status":"ok"}`
- [ ] `curl -f http://localhost:8001/health` → `{"status":"ok","service":"prophet-intelligence"}`

### Wallet
- [ ] Phantom or Solflare installed
- [ ] Connected to Mainnet
- [ ] SOL balance > 0.01 SOL (for fees)
- [ ] USDC balance > 10 USDC (for trading demo)

### Browser
- [ ] Chrome/Edge/Firefox latest
- [ ] DevTools closed (or docked)
- [ ] Single tab open to `http://localhost:3000`
- [ ] No other Prophet tabs open
- [ ] Incognito/private window (clean state)

---

## Demo Safety Rules

### NEVER During Demo
- ❌ **Auto-sign** any transaction
- ❌ **Claim** a transaction succeeded before Panta verification
- ❌ Show **fake data** or simulated transactions
- ❌ **Bypass** wallet signing
- ❌ **Skip** validation steps in Market Studio
- ❌ **Pretend** a confirmed transaction = created market
- ❌ **Expose** API keys in browser DevTools
- ❌ **Skip** validation steps in Market Studio

### ALWAYS During Demo
- ✅ Show **real data** from Panta
- ✅ Let **user sign** in their wallet
- ✅ Show **"Transaction confirmed on Solana. Panta registration is pending."** when registration fails
- ✅ Explain **custody model**: "We never hold your keys"
- ✅ Show **deterministic validation** (no AI in validation)
- ✅ Highlight **"AI proposes, human disposes"**

---

## Critical Demo Flows — What Must Work

### 1. Signal Radar → Market Detail
- [ ] Command Center loads with real signals
- [ ] Click signal row → Market Detail opens
- [ ] Signal timeline shows deterministic explanations

### 2. Market Detail → Trade
- [ ] Trade Ticket opens
- [ ] Quote → Build → Wallet popup → **Cancel** → "User Rejected" state
- Explain: "We never auto-sign. User explicitly approves."

### 3. Market Studio
- [ ] Step 1: Describe → AI draft
- [ ] Step 2: Clarification (if triggered)
- [ ] Step 3: Draft review (all fields editable)
- [ ] Step 4: Resolution rules + source confirmation checkbox
- [ ] Step 5: Validate → green checkmarks
- [ ] Step 6: Quote (wallet connected) → fee breakdown
- [ ] Step 7: Build & Sign → Wallet popup → **Cancel** → "User Rejected"
- [ ] Step 8: Register → **Skip** (requires real tx)

### 5. AI Copilot
- Click suggested prompt → Response with source citations
- Click source badge → navigates to Market Detail

### 6. Alerts
- Create rule: "Big Moves" on Watchlist, Probability Shift, 10pp
- Trigger signal → Notification appears in bell

---

## Failure Scenarios — Prepared Responses

| Scenario | Response |
|----------|----------|
| Panta API down | "Circuit breaker opens after 5 failures. Graceful degradation — cached data shown with timestamp." |
| AI provider down | "Copilot shows 'unavailable'. Market Studio falls back to manual draft entry." |
| Wallet rejects signature | "User rejected. Nothing submitted. Explicit action required." |
| Solana confirms, Panta registration fails | "Shows 'Transaction confirmed on Solana. Panta registration is pending.' Never 'creation failed'." |
| Worker stuck | "Advisory lock prevents duplicates. Restart worker — auto-recovers." |
| Market Engine not built | "Must build on Linux/CI. Windows needs mingw. CI handles this." |

---

## Demo Day Checklist (Morning Of)

### T-60 min
- [ ] All services running
- [ ] Health checks pass
- [ ] Wallet connected with USDC/SOL
- [ ] Browser clean (incognito, single tab)
- [ ] Recording software ready (OBS)

### T-15 min
- [ ] Run through demo flow once (silent)
- [ ] Verify all health checks green
- [ ] Wallet connected, USDC/SOL visible
- [ ] Browser DevTools closed

### T-5 min
- [ ] Close unnecessary apps
- [ ] Disable notifications
- [ ] Water nearby
- [ ] Deep breath

---

## Post-Demo
- [ ] Stop all services gracefully
- [ ] `docker compose -f infrastructure/docker-compose.dev.yml down`
- [ ] Save demo recording
- [ ] Note any issues for post-demo fix

---

## Emergency Contacts
- Panta Support: support@panta.market
- OpenAI Status: status.openai.com
- Solana Status: status.solana.com
- Internal: Check service logs in respective terminals

---

## Golden Rules
1. **Never** claim a market exists before Panta registration
2. **Never** say "trade succeeded" before Panta verification
3. **Never** auto-sign or bypass wallet
4. **Always** say "confirmed on Solana, Panta registration pending" when registration fails
5. **Always** let user sign in their wallet
6. **Never** fabricate data for demo

---

*Checklist v1.0 — Review before every demo*