# QEVRYN — Judge Journey

## 60-Second First Impression

### Landing (`/`)
- **Command Center** loads in <2s
- 4 KPI cards: Markets Tracked, Active Signals, Significant Changes, Markets Updated
- "Live" pulse indicator top-right
- Signal Radar table with 5-10 real signals from Panta
- Click any row → Market Detail

**Time to value:** <5 seconds to see live market intelligence

---

## 5-Minute Judge Journey

### Minute 0-1: Command Center → Market Detail
1. Open `http://localhost:3000`
2. Click any signal row in Command Center
3. → `/markets/{id}` opens with:
   - Header: title, status badge, category, close date
   - 4 metric cards: YES%, NO%, Volume, Liquidity
   - Signal Timeline (expandable rows with severity badges)
   - Market Metadata table

**Verify:** Click a signal in timeline → explanation matches exact numbers

### 2. Signal Radar (1 min)
- Click "Signals" in sidebar
- Filter by: Severity tabs, Signal Type dropdown, Market ID, Watchlist, Time Range
- Click row → Market Detail
- Pagination / Load More works

### 3. Watchlists (1 min)
- `/watchlists` → Create "Test Watchlist"
- Go to Markets → Search "Bitcoin" → Add to watchlist
- Back to watchlist → market appears with latest signal badge

### 4. Alerts (1 min)
- `/settings/alerts` → Create rule: "Big Moves", Watchlist scope, Probability Shift, 10pp threshold
- Rule appears in list with enabled toggle
- Alert Events tab (empty initially)

### 6. AI Copilot (30s)
- `/copilot` → Click "What changed significantly today?"
- Response with source citations
- Click source badge → Market Detail

### 6. Market Studio (2 min)
- `/studio` → Step 1: "Will ETH exceed $5,000 by Dec 31, 2026?"
- "Draft my market" → AI draft → Step through: Draft → Resolution → Validate → Quote (wallet) → Build & Sign (Cancel) → Register

### 7. Trading (if wallet connected)
- Market Detail → Trade Ticket
- YES/NO, amount → Quote → Build → Wallet popup → **Cancel** → "User Rejected"
- **Key:** "We never auto-sign. User explicitly approves."

### 7. System Status
- `/settings` → System Status card → `/system-status`
- All services with real health checks

---

## What Judges CAN Verify in 10 Minutes

| Feature | Verification Method |
|---------|---------------------|
| Real Panta data | Market titles, probabilities match Panta API |
| Deterministic signals | Click signal → explanation matches exact numbers |
| AI grounding | Copilot cites sources; click source → Market Detail |
| Deterministic alerts | Create rule → trigger signal → notification appears |
| Market Studio gates | Create button hidden until validation + review + warnings |
| Trading custody | Wallet signs; server never signs |
| CONFIRMED ≠ VERIFIED | Show confirmed-but-unregistered state |
| Idempotency | Click Register twice → single registration |

---

## What Judges CANNOT Do (Intentional)

| Action | Why Blocked |
|---------|-------------|
| Auto-sign transaction | Custody model — user must sign |
| AI creates market | Human must review every field |
| AI decides alert trigger | Deterministic `MatchesRule` only |
| Auto-sign transaction | Custody violation |
| See private keys | Server never receives them |
| Trade without wallet | Wallet connection required |
| See fake data | All data from Panta or deterministic engine |

---

## Technical Deep Dive (If Asked)

### Signal Generation
```
Panta API → Adapter → Gateway → Market Engine (Rust CLI) → Signals → PostgreSQL
                                    ↓
                          Signal Fingerprint (SHA-256)
                          UNIQUE constraint → deduplication
```

### Alert Evaluation
```
Signal Event → Fingerprint → Insert (ON CONFLICT DO NOTHING)
    ↓
Enabled Rules for Market → MatchesRule (deterministic)
    ↓
Cooldown Check (DELIVERED since now()-cooldown)
    ↓
Dedupe Key (rule_id + fingerprint) → Insert ON CONFLICT DO NOTHING
    ↓
Notification → DELIVERED
```

### Market Creation
```
Describe → AI Draft → Human Review → Deterministic Validation → Panta Quote
    → Build Unsigned Tx → User Signs → Broadcast → Solana Confirm
    → Panta Register → Market Exists
```

### Trading
```
Quote → Build → Wallet Sign → Broadcast → Solana Confirm
  → Panta Report → Panta Verify → Position Refresh
```

### Key Distinction
**Solana confirmed ≠ Trade complete.** Trade complete = Panta verification.

### Idempotency
- `trade_attempt_id` primary key
- `solana_signature` UNIQUE
- `market_creation_attempts` + `create_id + signature` UNIQUE

---

## Quick Links for Judges

| Page | URL |
|------|-----|
| Command Center | `http://localhost:3000` |
| Markets | `http://localhost:3000/markets` |
| Market Detail | `http://localhost:3000/markets/{id}` |
| Signal Radar | `http://localhost:3000/signals` |
| Watchlists | `http://localhost:3000/watchlists` |
| Watchlist Detail | `http://localhost:3000/watchlists/{id}` |
| AI Copilot | `http://localhost:3000/copilot` |
| Market Studio | `http://localhost:3000/studio` |
| Alerts | `http://localhost:3000/settings/alerts` |
| System Status | `http://localhost:3000/system-status` |
| Health (Live) | `http://localhost:8080/health/live` |
| Health (Ready) | `http://localhost:8080/health/ready` |
| Panta Adapter Health | `http://localhost:8081/health` |
| Intelligence Health | `http://localhost:8001/health` |

---

## Red Flags to Avoid

| Red Flag | How We Avoid It |
|----------|-----------------|
| "AI creates markets" | Never — 9-step wizard with human gates |
| "Automated trading" | Never — user signs every transaction |
| "AI decides alerts" | Never — deterministic `MatchesRule` |
| "Server signs" | Never — wallet signs, server broadcasts |
| "Fake data in demo" | Never — all data from Panta or deterministic engine |
| "Server signs" | Never — wallet signs, server broadcasts |

---

## Quick Links for Judges

| Page | URL |
|------|-----|
| Command Center | `http://localhost:3000` |
| Markets | `http://localhost:3000/markets` |
| Market Detail | `http://localhost:3000/markets/{id}` |
| Signal Radar | `http://localhost:3000/signals` |
| Watchlists | `http://localhost:3000/watchlists` |
| Watchlist Detail | `http://localhost:3000/watchlists/{id}` |
| AI Copilot | `http://localhost:3000/copilot` |
| Market Studio | `http://localhost:3000/studio` |
| Alerts | `http://localhost:3000/settings/alerts` |
| System Status | `http://localhost:3000/system-status` |
| Gateway Health (Live) | `http://localhost:8080/health/live` |
| Gateway Health (Ready) | `http://localhost:8080/health/ready` |
| Panta Adapter | `http://localhost:8081/health` |
| Intelligence | `http://localhost:8001/health` |

---

*Judge Journey v1.0 — Designed for 10-minute evaluation*
