# QEVRYN — Screenshot Checklist for Submission

## Overview

This document lists the recommended screenshots for the hackathon submission. Each screenshot should demonstrate a real, working feature — no mocked data, no placeholder UI.

---

## Required Screenshots

### 1. Command Center (`/`)
**Viewport:** 1440×900 (desktop)
**What to capture:**
- Full viewport showing header with "Command Center" title
- 4 KPI cards: Markets Tracked, Active Signals, Significant Changes, Markets Updated
- "Live" pulse indicator in top-right
- Signal Radar table with ≥5 real signals
- Severity badges visible (Critical, Significant, Watch, Info)
- Table columns: Market, Signal, Severity, Previous → Current, Change, Time

**What NOT to show:**
- Placeholder data
- "Loading..." states
- Empty states

**Purpose:** Prove the Command Center loads real Panta data and displays live signals.

---

### 2. Signal Radar (`/signals`)
**Viewport:** 1440×900
**What to capture:**
- Full page with severity tabs (All / Critical / Significant / Watch / Info)
- Filter bar: Market ID, Signal Type, Watchlist, Time Range
- Live refresh indicator (pulsing green dot)
- Table with ≥10 signals showing: Time, Severity bar, Market, Signal Type, Explanation, YES%, Time
- Clickable row → navigates to Market Detail

**Filters to demonstrate:**
- Severity tab: "Critical" → shows only Critical
- Market ID filter: type partial ID → filters
- Time range: "Last 24h"

**What NOT to show:**
- Empty state
- Loading skeletons (unless capturing load state intentionally)

---

### 3. Market Detail (`/markets/{id}`)
**Viewport:** 1440×900
**What to capture:**
- Header: Market title, status badge, category, close date
- 4 metric cards: YES% (green), NO% (red), Volume, Liquidity
- Trade Ticket (collapsed or expanded)
- Probability Chart placeholder (or "Chart: N observations")
- Signal Timeline: ≥5 signals with severity badges, explanations
- Market Metadata table (ID, source, phase, resolution status, created date)

**Key elements visible:**
- Real market title (not "Market 123")
- Real probabilities (e.g., "63.4%", not "50.0%")
- Real signal explanations with exact numbers
- Market ID is a real base58 Panta market ID

---

### 4. AI Copilot (`/copilot`)
**Viewport:** 1440×900
**What to capture:**
- Empty state with suggested prompts
- User message: "What changed significantly today?"
- Assistant response with **source citations** (badges)
- Click a source badge → navigates to Market Detail
- Second query: "Which markets moved the most today?"

**Key elements visible:**
- Source citation badges (clickable)
- Evidence cards with market title, signal type, severity
- No "AI generated this" — grounded responses only

---

### 5. Watchlist (`/watchlists` and `/watchlists/[id]`)
**Viewport:** 1440×900
**Screenshots needed:**
1. **List view** (`/watchlists`): Cards with market count, latest signal severity badge, "Create Watchlist" button
2. **Create flow:** Modal open with name + description
3. **Detail view** (`/watchlists/{id}`): Severity distribution bar, market table with latest signals, remove buttons

**Key elements:**
- Real watchlist name
- Real market count
- Real severity distribution (not all zeros)
- Real latest signal timestamps

---

### 6. Alerts (`/settings/alerts`)
**Viewport:** 1440×900
**Screenshots needed:**
1. **List view:** Empty state or existing rules
2. **Create rule modal:** Name, scope (watchlist/market/global), signal type, severity, threshold, cooldown
3. **Created rule** in list with enabled toggle
- Alert Events tab (shows "No events yet" or real events)

**Key elements:**
- Form validation (required fields, numeric thresholds)
- Scope selection: watchlist / market / global
- Cooldown input with validation

---

### 7. Trading Flow (3 screens)
**Screen 1 — Quote (`/markets/{id}` → Trade Ticket):**
- Market header with YES/NO toggle
- Amount input (USDC)
- "Get Quote" → shows quote with price, shares, total cost, expiry

**Screen 2 — Build:**
- Build button → unsigned transaction details
- Expected wallet/network validation

**Screen 3 — Sign & Broadcast:**
- Wallet popup (Phantom/Solflare) — **do not sign**
- Show "User Rejected" state after clicking Cancel
- **Caption:** "We never auto-sign. User explicitly approves."

---

### 8. Market Studio (3 screens)
**Screen 1 — Describe + Draft:**
- Input: "Will Bitcoin exceed $150,000 before Dec 31, 2026?"
- "Draft my market" → AI generates draft
- Step 2: Clarification (if triggered) or Step 3: Draft review

**Screen 2 — Resolution + Validate:**
- Resolution criteria + source input + confirmation checkbox
- Validate button → green checkmarks + warnings

**Screen 3 — Quote + Build & Sign:**
- Quote card with USDC base-unit breakdown
- "Build & Sign in Wallet" → wallet popup → **Cancel** → "User Rejected" state
- **Critical:** Show "User Rejected" state, explain "We never auto-sign"

---

### 9. System Status
**Page:** `/system-status`
Show all services with real health checks:
- Gateway, Panta Adapter, Intelligence, Database, Redis
- Real latency numbers, status badges

---

## What NOT to Capture

| ❌ Don't Capture | Why |
|------------------|-----|
| Placeholder data | Misrepresents product |
| "Loading..." states | Not a feature |
| Fake transaction confirmations | Misleading |
| Fake wallet signatures | Security violation |
| Mock AI responses | Misrepresents AI grounding |
| Fake transaction confirmations | Misleading |
| Placeholder charts | Not implemented |
| Empty states as "features" | Misleading |

---

## Technical Specifications

| Parameter | Value |
|-----------|-------|
| Format | PNG or WebP |
| Resolution | 1440×900 minimum (desktop) |
| Format | PNG preferred (lossless) |
| Naming | `feature-name-desktop.png` |
| Annotations | Red circles/arrows for key UI elements |

---

## Mobile Viewports (Optional)

| Viewport | Pages |
|-----------|-------|
| 390×844 (iPhone 12) | Command Center, Signal Radar, Market Detail |
| 768×1024 (iPad) | Watchlist Detail, Market Studio |

---

## File Naming Convention

```
docs/screenshots/
├── command-center-desktop.png
├── signal-radar-desktop.png
├── market-detail-desktop.png
├── ai-copilot-desktop.png
├── watchlist-list-desktop.png
├── watchlist-detail-desktop.png
├── alerts-desktop.png
├── trade-quote-desktop.png
├── trade-build-desktop.png
├── trade-sign-rejected-desktop.png
├── studio-describe-desktop.png
├── studio-validate-desktop.png
├── studio-quote-desktop.png
├── studio-sign-cancelled-desktop.png
└── system-status-desktop.png
```

---

## What Judges Will Look For

| Screenshot | Proves |
|------------|--------|
| Command Center | Real Panta data loads |
| Signal Radar | Deterministic signals, severity filtering |
| Market Detail | Real probabilities, real signals |
| AI Copilot | Source citations, grounded responses |
| Watchlists | CRUD + real-time signal badges |
| Alerts | Deterministic rules, cooldown, dedupe |
| Trading | Custody boundary (cancel = no tx) |
| Market Studio | Gates enforced, AI draft → human review |
| System Status | Real dependency health |

---

*Checklist v1.0 — Capture before submission*
