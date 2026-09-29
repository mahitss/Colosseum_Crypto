# Prophet — Demo Script

## Demo Overview
**Duration:** ~3 minutes  
**Target Audience:** Technical judges, investors, potential users  
**Core Message:** "Prophet turns prediction-market noise into structured intelligence — with deterministic signals, grounded AI, and human-controlled trading."

---

## Pre-Demo Setup (5 min before)

### Infrastructure
```bash
# Terminal 1: Infrastructure
docker compose -f infrastructure/docker-compose.dev.yml up -d postgres redis

# Terminal 2: Market Engine (Linux/CI)
cd services/market-engine && cargo build --release

# Terminal 3: Panta Adapter
cd services/panta-adapter && go run ./cmd/server

# Terminal 4: Intelligence
cd services/intelligence && python -m app.main

# Terminal 5: Gateway
cd services/gateway && go run .

# Terminal 6: Worker
cd services/gateway && go run ./cmd/worker

# Terminal 7: Frontend
cd apps/web && npm run dev
```

### Environment Variables
```bash
export DATABASE_URL="postgresql://prophet:prophet@localhost:5432/prophet"
export PANTA_API_KEY="your-panta-api-key"
export PANTA_API_BASE_URL="https://live-api.panta.market/api/v1/"
export SOLANA_RPC_URL="https://api.mainnet-beta.solana.com"
export AI_API_KEY="your-openai-key"
export JWT_SECRET="$(openssl rand -hex 32)"
export MARKET_ENGINE_BIN="./services/market-engine/target/release/market-engine"
```

### Health Checks
```bash
curl -f http://localhost:8080/health/live
curl -f http://localhost:8080/health/ready
curl -f http://localhost:8081/health
curl -f http://localhost:8001/health
```

### Wallet
- Phantom or Solflare installed
- Connected to Mainnet
- SOL for fees, USDC for trading

### Browser
- Chrome/Edge/Firefox latest
- DevTools closed (or docked)
- Single tab open to `http://localhost:3000`
- No other Prophet tabs open

---

## Demo Script (3–5 minutes)

### 0:00 — OPEN COMMAND CENTER
**Action:** Open `http://localhost:3000`

**Say:** "Prophet is designed to answer one question: What does the market think happens next?"

**Show:**
- Page loads at `/` (Command Center)
- 4 KPI cards: Markets Tracked, Active Signals, Significant Changes, Markets Updated
- "Live" pulse indicator in top-right
- Signal Radar table with recent signals from Panta

---

### 0:20 — SHOW MARKET OVERVIEW
**Action:** Page loads at `/`

**Show:**
- Header: "Command Center" + "Prediction-market intelligence across the Panta ecosystem."
- 4 KPI cards: Markets Tracked, Active Signals, Significant Changes, Markets Updated
- Signal Radar table with real signals from Panta

**Say:** "This is the Command Center. Four KPIs — markets tracked, active signals, significant changes, last update. Below: live Signal Radar with real Panta data. Every row is a deterministic signal from our Rust engine."

**Click:** Any signal row → navigates to Market Detail

---

### 0:45 — OPEN SIGNAL RADAR
**Action:** Click "Signals" in sidebar → `/signals`

**Show:**
- Severity tabs: All / Critical / Significant / Watch / Info
- Filters: Market ID, Signal Type, Watchlist, Time Range
- Table with: Time, Severity bar, Market, Signal Type, Explanation, YES%, Time

**Say:** "This is the Signal Radar. Every row is a deterministic signal from our Rust engine — fixed-point arithmetic, 12 decimal places, no floating point. Severity tabs use minimum-severity semantics: 'Significant' shows Significant AND Critical. Keyset pagination means stable scrolling even as new signals land."

**Click:** Any row → navigates to Market Detail

---

### 1:15 — OPEN MARKET DETAIL
**Action:** Click a signal row → `/markets/{id}`

**Show:**
- Header: Title, status badge, category, close date
- 4 metric cards: YES%, NO%, Volume, Liquidity
- Trade Ticket (collapsed)
- Probability Chart (placeholder)
- Signal Timeline with severity badges
- Market Metadata

**Say:** "Every market shows real Panta data. The signal timeline shows deterministic signals from our Rust engine — fixed-point arithmetic, 12 decimal places, no floating point. Click any signal to see the explanation."

**Click:** A signal in timeline → expands with explanation

**Say:** "This explanation is deterministic — assembled from the signal's actual fields. No LLM generated this text."

---

### 1:45 — AI COPILOT
**Navigate to:** `/copilot`

**Show:**
- Suggested prompts
- Type: "What changed significantly today?"
- Response with source citations
- Click source badge → navigates to Market Detail

**Say:** "Every AI response cites sources — markets, signals, observations. No invented numbers. If data is missing, it says so."

---

### 2:15 — WATCHLISTS
**Navigate to:** `/watchlists`

**Show:**
- Create "Test Watchlist"
- Add markets from Markets page
- View watchlist detail with severity distribution bar

**Say:** "Watchlists scope alerts. You only get alerts for markets you care about."

---

### 6. ALERTS (60s)
**Navigate to:** `/settings/alerts`

**Show:**
- Create rule: "Big Moves" on "My Watchlist", Probability Shift, 10pp threshold
- Show rule in list
- Check Alert Events tab (empty initially)

**Say:** "Alerts are deterministic. Cooldown prevents spam. Dedupe keys prevent duplicates. If Panta registration fails after Solana confirms, we show 'Registration pending' — never 'failed'."

---

### 7. MARKET STUDIO (90s)
**Navigate to:** `/studio`

**Step 1 — Describe:** Type "Will Bitcoin exceed $150,000 before December 31, 2026?" → Click "Draft my market"

**Step 2-4:** Draft → Resolution → Validate
- Show draft fields editable
- Resolution criteria + source confirmation checkbox
- Validate → green checkmarks + warnings

**Step 6 — Quote:** Connect wallet → "Get Quote" → fee breakdown in USDC base units

**Step 7 — Build & Sign:**
- Click "Build & Sign in Wallet"
- Wallet popup appears → **Click Cancel**
- Show "User Rejected" state

**Say:** "We never auto-sign. User explicitly approves. If they cancel, nothing happens."

---

### 3:10 — SYSTEM STATUS
Navigate to `/settings` → Click "System Status" → `/system-status`

Show all services with real health checks.

---

### 3:10 — CLOSE

**Say:** "Prophet is production-ready. All core workflows work end-to-end. The code is open for review. We're ready for mainnet."

**Open:** GitHub repo, docs, demo runbook

---

## Key Talking Points (Memorize)

| Topic | Key Phrase |
|-------|------------|
| Deterministic signals | "Fixed-point arithmetic, 12 decimals, never float64" |
| AI grounding | "Every response cites sources. No invented numbers." |
| Custody | "Server never holds keys. User signs in wallet." |
| Alert determinism | "math/big.Rat arithmetic. No LLM decides triggers." |
| Market creation | "9-step wizard. AI drafts, human decides, Panta registers." |
| Trading | "Quote → Build → Sign → Broadcast → Confirm → Report → Verify" |
| CONFIRMED vs VERIFIED | "Solana confirmed ≠ market exists. Only Panta registration = market." |
| Idempotency | "Database UNIQUE constraints + ON CONFLICT DO NOTHING" |

---

## Failure Responses (Memorize)

| Failure | Response |
|---------|----------|
| Panta down | "Circuit breaker opens after 5 failures. Graceful degradation." |
| AI down | "Copilot returns 'unavailable'. Market Studio falls back to manual." |
| Wallet rejects | "User rejected. Nothing submitted. Explicit action required." |
| Solana confirms, Panta fails | "Transaction confirmed on Solana. Panta registration is pending." |
| Worker crashes | "Advisory lock prevents duplicates. Restart auto-recovers." |

---

## Backup Plan (If Live Fails)

1. **Recorded demo video** — 10 min walkthrough
2. **Screenshots** — Key screens in `docs/demo-screenshots/`
3. **Local recording** — OBS recording of full flow
4. **Static screenshots** in presentation deck

---

## Post-Demo Q&A Prep

| Question | Answer |
|----------|--------|
| "How is this different from [competitor]?" | "Deterministic signals + grounded AI + user custody. No float64." |
| "How do you handle Panta downtime?" | "Circuit breaker + graceful degradation. Cached data with timestamp." |
| "Can AI create markets?" | "No. AI drafts only. Human reviews every field. Panta registers." |
| "How do you handle Panta API changes?" | "Adapter isolates schema. Versioned endpoints. Contract tests." |
| "What's your moat?" | "Deterministic signals + grounded AI + custody model. Hard to replicate all three." |
| "How do you make money?" | "Enterprise subscriptions for advanced alerts, API access, white-label." |

---

*Demo Script v1.0 — Review before every demo*