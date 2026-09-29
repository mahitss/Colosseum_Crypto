# Prophet — Demo Script (Click-by-Click)

## Demo Overview
**Duration:** ~10 minutes  
**Audience:** Technical judges, investors, potential users  
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

### Environment
```bash
export DATABASE_URL="postgresql://prophet:prophet@localhost:5432/prophet"
export PANTA_API_KEY="your-panta-key"
export PANTA_API_BASE_URL="https://live-api.panta.market/api/v1/"
export SOLANA_RPC_URL="https://api.mainnet-beta.solana.com"
export AI_API_KEY="sk-..."
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

### Wallet Setup
- Phantom or Solflare installed
- Connected to Mainnet
- SOL for fees, USDC for trading

---

## Demo Script (10 minutes)

### 0. Opening (30s)
**Say:** "Prophet turns prediction-market noise into structured intelligence. Panta provides the markets; Prophet provides the intelligence layer — deterministic signals, grounded AI, human-controlled trading."

**Action:** Open http://localhost:3000 → Command Center loads

---

### 1. Command Center (60s)
**Action:** Page loads at `/`

**Show:**
- Header: "Command Center" + "Live" pulse indicator
- 4 KPI cards: Markets Tracked, Active Signals, Significant Changes, Markets Updated
- Signal Radar table below with real signals

**Say:** "This is the Command Center. Four KPIs — markets tracked, active signals, significant changes, last update. Below: live Signal Radar with real Panta data. Every row is a deterministic signal from our Rust engine."

**Click:** Any signal row → navigates to Market Detail

---

### 2. Market Detail (90s)
**Action:** Click a signal row → `/markets/{id}`

**Show:**
- Header: Title, status badge, category, close date
- 4 metric cards: YES%, NO%, Volume, Liquidity (tabular numbers)
- Trade Ticket (collapsed)
- Probability Chart (placeholder)
- Signal Timeline — expandable rows with severity badges
- Market Metadata (ID, source, phase, resolution status, created date)

**Say:** "Every market shows real Panta data. The signal timeline shows deterministic signals from our Rust engine — fixed-point arithmetic, 12 decimal places, no floating point. Click any signal to see the explanation."

**Click:** A signal in timeline → expands with explanation

**Say:** "This explanation is deterministic — assembled from the signal's actual fields. No LLM generated this text."

---

### 3. Signal Radar (60s)
**Navigate to:** `/signals` (or click "Signal Radar" in sidebar)

**Show:**
- Header with severity tabs: All / Critical / Significant / Watch / Info
- Filter bar: Market ID, Signal Type, Watchlist, Time Range
- Live refresh indicator (15s)
- Table with: Time, Severity bar, Market, Signal Type, Explanation, YES%, Time

**Demonstrate:**
1. Click "Critical" tab → filters to Critical only
2. Type market ID in search → filters to that market
3. Change time range to "Last 24h"
4. Click a row → navigates to Market Detail

**Say:** "Every filter is server-side. The severity tabs use minimum-severity semantics — 'Significant' shows Significant AND Critical. Keyset pagination means stable scrolling even as new signals arrive."

---

### 4. Watchlists (60s)
**Navigate to:** `/watchlists`

**Show:**
- Empty state or existing watchlists
- Click "Create Watchlist" → name "My Crypto Watchlist"
- Click the watchlist → detail view
- Show empty state → "Add markets"

**Navigate to:** `/markets` → Search "Bitcoin" → Click "Add to Watchlist" → Select "Demo Watchlist"

**Back to:** `/watchlists` → Click the watchlist → Shows market with latest signal + severity

**Say:** "Watchlists scope alerts. You only get alerts for markets you care about."

---

### 5. Alerts (60s)
**Navigate to:** `/settings/alerts`

**Show:**
- Empty state or existing rules
- Click "Create Alert Rule"
- Fill form:
  - Name: "Big Probability Shifts"
  - Scope: "My Crypto Watchlist" (dropdown)
  - Signal Type: "Probability Shift"
  - Minimum Severity: "Significant"
  - Probability Change Threshold: "10" (percentage points)
  - Cooldown: "3600" (1 hour)
- Save → Rule appears in list
- Click "Alert Events" tab → empty (no events yet)

**Say:** "Alerts are deterministic. Cooldown prevents spam. Dedupe keys prevent duplicates. If Panta registration fails after Solana confirms, we show 'Registration pending' — never 'failed'."

---

### 6. AI Copilot (60s)
**Navigate to:** `/copilot`

**Show:**
- Suggested prompts buttons
- Type: "What changed significantly today?"
- Send → Shows thinking animation → Response with source citations
- Click a source badge → Opens Market Detail
- Ask: "Which markets moved the most today?"
- Response with market links
- Ask: "Explain the biggest signal today"
- Shows deterministic explanation

**Say:** "Every AI response cites sources — markets, signals, observations. No invented numbers. If data is missing, it says so."

---

### 6. Market Studio (90s)
**Navigate to:** `/studio`

**Step 1 — Describe:**
- Type: "Will Bitcoin exceed $150,000 before December 31, 2026, according to CoinGecko?"
- Click "Draft my market"
- AI returns draft with clarification questions if needed

**Step 2-4:** Review Draft → Resolution Rules
- Show draft fields editable
- Resolution criteria: "Bitcoin daily close > $150,000 on Dec 31, 2026"
- Add source: "https://www.coingecko.com/en/coins/bitcoin"
- Check "I confirm this resolution source"

**Step 5 — Validate:**
- Click "Re-run validation"
- Shows green checkmarks + any warnings
- Click "I have reviewed this market"

**Step 6 — Quote:**
- Connect wallet (Phantom)
- Click "Get Quote"
- Show fee breakdown in USDC base units (integer strings)

**Step 7 — Build & Sign:**
- Click "Build & Sign in Wallet"
- Wallet popup appears → **Click Cancel**
- Show "User Rejected" state

**Say:** "We never auto-sign. User explicitly approves. If they cancel, nothing happens."

---

### 7. System Status (30s)
Navigate to `/settings` → Click "System Status" card → `/system-status`

Show all services with real health checks:
- Gateway, Panta Adapter, Intelligence, Database, Redis

---

### 10. System Status (30s)
**Navigate to:** `/settings` → Click "System Status"

**Show:** All dependency health checks with real checks

---

## Closing (30s)

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
| AI down | "Copilot shows 'unavailable'. Market Studio falls back to manual." |
| Wallet rejects | "User rejected. Nothing submitted. Explicit action required." |
| Solana confirms, Panta fails | "Shows 'Transaction confirmed on Solana. Panta registration pending.' Never 'failed'." |
| Worker crashes | "Advisory lock prevents duplicates. Restart auto-recovers." |

---

## Backup Plan (If Live Fails)

1. **Recorded demo video** — 10 min walkthrough
2. **Screenshots** — Key screens in `docs/demo-screenshots/`
3. **Local recording** — OBS recording of full flow
4. **Static screenshots** in presentation deck

---

## Post-Demo Q&A Prep

| Likely Question | Answer |
|-----------------|--------|
| "How is this different from [competitor]?" | "Deterministic signals, grounded AI, user custody. No float64." |
| "How do you handle Panta downtime?" | "Circuit breaker + graceful degradation. Cached data shown with timestamp." |
| "Can AI create markets?" | "No. AI drafts only. Human reviews every field. Panta registers." |
| "How do you handle Panta API changes?" | "Adapter isolates schema. Versioned endpoints. Contract tests." |
| "What's your moat?" | "Deterministic signals + grounded AI + custody model. Hard to replicate all three." |
| "How do you make money?" | "Enterprise subscriptions for advanced alerts, API access, white-label." |

---

## End of Demo Script