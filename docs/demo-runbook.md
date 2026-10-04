# QEVRYN - Demo Runbook

## Pre-Demo Checklist

### Infrastructure
- [ ] PostgreSQL running (`docker compose -f infrastructure/docker-compose.dev.yml up -d postgres`)
- [ ] Redis running (`docker compose -f infrastructure/docker-compose.dev.yml up -d redis`)
- [ ] Market Engine binary built (`cd services/market-engine && cargo build --release`)
- [ ] `MARKET_ENGINE_BIN` points to built binary

### Environment Variables
```bash
export DATABASE_URL="postgresql://QEVRYN:QEVRYN@localhost:5432/QEVRYN"
export PANTA_API_KEY="your-panta-api-key"
export PANTA_API_BASE_URL="https://live-api.panta.market/api/v1/"
export SOLANA_RPC_URL="https://api.mainnet-beta.solana.com"
export AI_API_KEY="your-openai-key"
export JWT_SECRET="$(openssl rand -hex 32)"
export JWT_EXPIRY="24h"
export JWT_AUDIENCE="QEVRYN-api"
export MARKET_ENGINE_BIN="./services/market-engine/target/release/market-engine"
export PANTA_ADAPTER_URL="http://127.0.0.1:8081"
export PANTA_API_KEY="your-panta-api-key"
export PANTA_API_URL="https://live-api.panta.market/api/v1/"
export SOLANA_RPC_URL="https://api.mainnet-beta.solana.com"
export AI_API_KEY="your-openai-key"
export INTELLIGENCE_SERVICE_URL="http://localhost:8001"
export JWT_SECRET="your-jwt-secret"
export JWT_EXPIRY="24h"
export JWT_AUDIENCE="QEVRYN-api"
export CORS_ALLOWED_ORIGINS="http://localhost:3000"
```

### Services Running
```bash
# Terminal 1: Panta Adapter
cd services/panta-adapter && go run ./cmd/server

# Terminal 2: Intelligence
cd services/intelligence && python -m app.main

# Terminal 3: Gateway
cd services/gateway && go run .

# Terminal 4: Worker
cd services/gateway && go run ./cmd/worker

# Terminal 5: Frontend
cd apps/web && npm run dev
```

### Health Checks
```bash
curl -f http://localhost:8080/health/live
curl -f http://localhost:8080/health/ready
curl -f http://localhost:8081/health
curl -f http://localhost:8001/health
```

### Wallet
- [ ] Phantom/Solflare installed
- [ ] Wallet connected to correct network (mainnet/devnet)
- [ ] SOL balance for transaction fees
- [ ] USDC balance for trading

### Browser
- [ ] Chrome/Edge/Firefox latest
- [ ] DevTools closed (or docked)
- [ ] Single tab open to `http://localhost:3000`
- [ ] No other QEVRYN tabs open

---

## Demo Flow (Click-by-Click)

### 1. Command Center (30s)
1. Open `http://localhost:3000`
2. Verify Command Center loads with 4 KPI cards
3. Point out "Live" indicator in top-right
4. Show Signal Radar table with recent signals
5. Click a signal row → navigates to Market Detail

### 2. Market Detail (45s)
1. On Market Detail page, show:
   - Header: title, status badge, category, close date
   - 4 metric cards: YES%, NO%, Volume, Liquidity
   - Trade Ticket (collapsed)
   - Probability Chart (placeholder)
   - Signal Timeline with severity badges
   - Market Metadata table
2. Click "Trade" button → opens Trade Ticket modal

### 3. Trading Flow (60s)
1. In Trade Ticket:
   - Select YES/NO
   - Enter amount (e.g., "10")
   - Click "Get Quote"
2. Show quote details: price, shares, total cost
3. Click "Build Transaction"
4. Show unsigned transaction details
5. Click "Sign & Buy" → Wallet popup appears
6. **DO NOT SIGN** - Click "Cancel" or "Reject"
7. Show "User Rejected" state
8. Explain: "We never auto-sign. User must explicitly approve."

### 4. Signal Radar (30s)
1. Navigate to `/signals`
2. Show severity tabs: All / Critical / Significant / Watch / Info
3. Demonstrate filters: Market, Signal Type, Severity, Time Range, Watchlist
4. Click a signal row → navigate to Market Detail
5. Show pagination / infinite scroll

### 5. Watchlists (45s)
1. Navigate to `/watchlists`
2. Click "Create Watchlist" → name it "Demo Watchlist"
2. Open watchlist → empty state
3. Go to Markets page → search for a market
3. Click "Add to Watchlist" → select "Demo Watchlist"
4. Return to Watchlist → shows market with latest signal
5. Click watchlist → detail view with severity distribution

### 6. Alerts (45s)
1. Navigate to `/settings/alerts`
2. Click "Create Alert Rule"
3. Fill: Name="Big Moves", Severity="Significant", Signal Type="Probability Shift", Threshold="10pp"
4. Save → appears in list
5. Show Alert Events tab (empty initially)
6. Show Notifications bell in topbar (empty)

### 6. AI Copilot (60s)
1. Navigate to `/copilot`
2. Click suggested prompt: "What changed significantly today?"
3. Show response with source citations
4. Click a source badge → navigates to market
4. Ask: "Which markets moved the most today?"
5. Show response with market links
6. Ask: "Explain the biggest signal"
6. Show deterministic explanation

### 7. Market Studio (90s)
1. Navigate to `/studio`
2. **Step 1 - Describe**: Enter "Will Bitcoin exceed $150,000 before December 31, 2026?"
2. Click "Draft my market" → AI generates draft
3. **Step 2** - Review draft (skip if complete)
3. **Step 3** - Resolution Rules:
   - Criteria: "Bitcoin daily close > $150,000 on CoinGecko"
   - Add source: "https://www.coingecko.com/en/coins/bitcoin"
   - Check "I confirm this resolution source"
4. Click "Continue to validation" → **Step 4: Validate**
4. Click "Re-run validation" → shows green checkmarks
5. **Step 5 - Validate**: Click "I have reviewed this market"
5. **Step 6 - Quote**: Connect wallet → Click "Get Quote"
5. Show fee breakdown (USDC base units)
6. **Step 7 - Build & Sign**: Click "Build & Sign in Wallet"
   - **DO NOT SIGN** - Click "Cancel" in wallet
   - Show "User Rejected" state
6. Explain: "Only the user can sign. Server never holds keys."
7. **Step 9 - Register**: (Skip - requires real transaction)

### 9. System Status (15s)
1. Navigate to `/settings`
2. Show System Status card
2. Click "System Status" → `/system-status`
3. Show all services: Gateway, Panta Adapter, Intelligence, Database

---

## Failure Plan

### If Panta API Unavailable
- **Symptom**: `/health/ready` returns 503, market data stale
- **Action**: Show "Market data temporarily unavailable" banner
- **Fallback**: Show cached data with timestamp, disable trading buttons

### If AI Provider Unavailable
- **Symptom**: Copilot returns "Error processing query", Market Studio interpret fails
- **Action**: Show "AI temporarily unavailable" in chat/studio
- **Fallback**: Market Studio still works with manual draft entry

### If Wallet Unavailable
- **Symptom**: "Connect Wallet" button shows, no wallet detected
- **Action**: Show "Connect Phantom/Solflare" with install links
- **Never**: Attempt to sign on behalf of user

### If Database Unavailable
- **Symptom**: `/health/ready` returns 503, queries fail
- **Action**: Show "Service temporarily unavailable" page
- **Recovery**: Auto-reconnect on restart, worker pauses gracefully

### If Market Data Stale
- **Symptom**: Signal Radar shows old timestamps (>1hr)
- **Action**: Show "Data may be stale" indicator, manual refresh button
- **Recovery**: Worker auto-recovers on next tick

### If Worker Stuck
- **Symptom**: No new signals > 10 minutes
- **Action**: Check worker logs, restart worker process
- **Recovery**: Advisory lock prevents duplicate workers

---

## Critical Reminders

### NEVER Do During Demo
- ❌ Sign a transaction automatically
- ❌ Claim a transaction succeeded before Panta verification
- ❌ Show fake data or simulated transactions
- ❌ Bypass wallet signing
- ❌ Expose API keys in browser DevTools
- ❌ Skip validation steps in Market Studio
- ❌ Pretend a confirmed transaction = created market

### ALWAYS Do During Demo
- ✅ Show real data from Panta
- ✅ Let user sign in their wallet
- ✅ Show "Transaction confirmed on Solana. Panta registration is pending." when registration fails
- ✅ Explain custody model: "We never hold your keys"
- ✅ Show deterministic validation (no AI in validation)
- ✅ Highlight "AI proposes, human disposes"

### If Something Breaks
1. Acknowledge: "That's a real production issue"
2. Show error handling: "Here's how we handle it..."
3. Fall back to working feature
3. Don't improvise fake data

---

## Post-Demo Cleanup
- [ ] Close all terminals
- [ ] Stop Docker containers: `docker compose -f infrastructure/docker-compose.dev.yml down`
- [ ] Clear browser cookies/localStorage for clean state
- [ ] Reset demo wallet if used

---

## Emergency Contacts
- Panta Support: support@panta.market
- OpenAI Status: status.openai.com
- Solana Status: status.solana.com
- Internal: Check `services/gateway/cmd/worker` logs

---

*Demo Runbook v1.0 - Update before each demo*
