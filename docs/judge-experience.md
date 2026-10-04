# QEVRYN — Judge Experience Document

## What Judges Will See in the First 30 Seconds

### Landing Page (`/`)
- **Command Center** loads in <2s
- 4 KPI cards animate in: Markets Tracked, Active Signals, Significant Changes, Markets Updated
- "Live" pulse indicator in top-right
- Signal Radar table with 5-10 real signals from Panta
- Each row clickable → Market Detail

### Visual Polish
- Dark theme (slate-950 background)
- Consistent spacing, typography, borders
- No placeholder text, no "lorem ipsum"
- Smooth hover/transition states
- Loading skeletons match final layout

---

## What Judges Can Click in 5 Minutes

### 1. Signal Radar → Market Detail (30s)
1. Click any signal row in Command Center table
2. → `/markets/{id}` opens
3. See: header, 4 metric cards, trade ticket, signal timeline, metadata

### 2. Signal Radar Page (1 min)
- Click "Signals" in sidebar
- Filter by: Severity tabs, Signal Type dropdown, Market ID, Watchlist, Time Range
- Click any row → Market Detail
- Pagination works (Load More)

### 3. Watchlists (1 min)
- `/watchlists` → Create "Test Watchlist"
- Go to Markets → Search "Bitcoin" → Add to watchlist
- Back to watchlist → See market with latest signal badge

### 4. Alerts (1 min)
- `/settings/alerts` → Create rule: "Big Moves" on "My Watchlist", Probability Shift, 10pp threshold
- See rule in list
- Check Alert Events tab (empty initially)

### 5. AI Copilot (30s)
- `/copilot` → Click "What changed significantly today?"
- Response appears with source citations
- Click source badge → Market Detail

### 6. Market Studio (2 min)
- `/studio` → Step 1: "Will ETH exceed $5,000 by Dec 31, 2026?"
- Click "Draft my market" → AI generates draft
- Step through: Draft → Resolution → Validate → Quote (wallet needed) → Build & Sign → Register
- Show: Create button only appears after validation + review + warning ack

### 6. Trading Flow (if wallet connected)
- Market Detail → Click "Trade"
- Select YES/NO, amount → Get Quote
- Build → Wallet popup → **Cancel** → Shows "User Rejected"
- Explain: Never auto-sign

---

## What Judges Can Verify in 10 Minutes

| Feature | Verification Method |
|---------|---------------------|
| Real Panta data | Market titles, probabilities match Panta API |
| Deterministic signals | Click signal → explanation matches exact numbers |
| AI grounding | Copilot responses cite sources; click source → Market Detail |
| Deterministic alerts | Create rule → trigger signal → notification appears |
| Market Studio gates | Create button hidden until validation + review + warnings |
| Trading custody | Wallet signs; server never signs |
| CONFIRMED ≠ VERIFIED | Show confirmed-but-unregistered state |
| Idempotency | Click Register twice → single registration |

---

## What Judges CANNOT Do (And That's Intentional)

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
Quote → Build → Wallet Sign → Broadcast → Confirm → Report → Verify → Position Refresh
```

---

## Data Flow Verification (For Technical Judges)

### Ask to See:
1. **Signal → Explanation** — Click signal → Explanation matches exact numbers
2. **Alert → Notification** — Create rule → Trigger signal → Notification appears
3. **Trade → Verify** → Quote → Build → Sign → Broadcast → Confirm → Report → Verify
4. **Market Creation** → Register → Market ID appears in QEVRYN
5. **CONFIRMED ≠ VERIFIED** — Show confirmed-but-unregistered state

### Database Queries Judges Can Run
```sql
-- Signal events with fingerprints
SELECT id, fingerprint, signal_type, severity, observed_at 
FROM signal_events ORDER BY observed_at DESC LIMIT 10;

-- Alert deduplication
SELECT alert_rule_id, dedupe_key, COUNT(*) 
FROM alert_events GROUP BY alert_rule_id, dedupe_key HAVING COUNT(*) > 1;

-- Market creation states
SELECT status, market_id, solana_signature, panta_reference 
FROM market_creation_attempts ORDER BY created_at DESC LIMIT 10;

-- Trade lifecycle
SELECT status, solana_signature, panta_reference, verified_at 
FROM trade_attempts ORDER BY created_at DESC LIMIT 10;
```

---

## What NOT to Test (Not Implemented)

| Feature | Status |
|---------|--------|
| Claims/settlement | Not implemented |
| Creator fees | Not implemented |
| Trade attribution | Not implemented |
| WebSocket streaming | Not implemented |
| Automated trading | Intentionally absent |
| AI market creation | Intentionally absent |
| Claims/settlement API | Not implemented |

---

## Known Limitations (Honest)

| Limitation | Impact | Mitigation |
|------------|--------|------------|
| Market Engine Windows build | CI handles; local dev needs WSL/Linux | CI builds on Linux |
| Frontend ESLint warnings | zod/v4 dep issue | Pre-existing, non-blocking |
| CSP/HSTS headers | Not implemented | Post-demo |
| Market Engine Windows build | Missing mingw | CI/Linux builds |
| 1 flaky Go test | Pre-existing | Expected behavior |
| 1 Python test (date validation) | Pre-existing | Validation logic difference |

---

## Judge Q&A Cheat Sheet

| Question | Answer |
|----------|--------|
| "How is this different from [competitor]?" | "Deterministic signals + grounded AI + user custody. No float64 for money." |
| "How do you handle Panta downtime?" | "Circuit breaker + graceful degradation. Cached data with timestamp." |
| "Can AI create markets?" | "No. AI drafts only. Human reviews every field. Panta registers." |
| "How do you handle Panta API changes?" | "Adapter isolates schema. Versioned endpoints. Contract tests." |
| "What's your moat?" | "Deterministic signals + grounded AI + custody model. Hard to replicate all three." |
| "How do you make money?" | "Enterprise subscriptions for advanced alerts, API access, white-label." |
| "What if Panta goes down?" | "Circuit breaker opens. Graceful degradation. Cached data shown with timestamp." |
| "Can AI create markets?" | "No. AI drafts only. Human reviews every field. Panta registers." |
| "How do you handle Panta API changes?" | "Adapter isolates schema. Versioned endpoints. Contract tests." |
| "What's your moat?" | "Deterministic signals + grounded AI + custody model. Hard to replicate all three." |

---

## Red Flags to Avoid

| Red Flag | How We Avoid It |
|----------|-----------------|
| "AI creates markets" | Never — 9-step wizard with human gates |
| "Automated trading" | Never — user signs every transaction |
| "AI decides alerts" | Never — deterministic `MatchesRule` |
| "Fake data in demo" | Never — all data from Panta or deterministic engine |
| "Server signs" | Never — wallet signs, server broadcasts |
| "Fake volume/users" | Never — all real Panta data |
| "Guaranteed returns" | Never — "useful signal" language only |

---

## Quick Links for Judges

| Link | Purpose |
|--------|---------|
| `http://localhost:3000` | Command Center |
| `http://localhost:3000/markets` | Markets |
| `http://localhost:3000/signals` | Signal Radar |
| `http://localhost:3000/watchlists` | Watchlists |
| `http://localhost:3000/watchlists/{id}` | Watchlist Detail |
| `http://localhost:3000/studio` | Market Studio |
| `http://localhost:3000/copilot` | AI Copilot |
| `http://localhost:3000/settings/alerts` | Alerts |
| `http://localhost:3000/system-status` | System Status |
| `http://localhost:8080/health/live` | Gateway Liveness |
| `http://localhost:8080/health/ready` | Gateway Readiness |
| `http://localhost:8081/health` | Panta Adapter |
| `http://localhost:8001/health` | Intelligence |

---

## Contact During Demo

- **Technical issues:** Check service logs in terminals
- **Panta issues:** Check `services/panta-adapter` logs
- **AI issues:** Check `services/intelligence` logs
- **Worker issues:** Check `services/gateway/cmd/worker` logs

---

*Prepared for Colosseum Hackathon 2024 — QEVRYN Team*
