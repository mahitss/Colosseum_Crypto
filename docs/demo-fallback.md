# Prophet — Demo Fallback Plan

## Principle

**Never fake:**
- Transaction confirmation
- Panta verification
- Market creation
- Wallet signatures
- Market data
- Signal generation
- Alert triggering

If a component fails, acknowledge it honestly and fall back gracefully.

---

## Failure Scenarios & Fallbacks

### 1. Panta API Unavailable

**Symptom:**
- `/health/ready` returns 503
- Market data stale
- Signal Radar empty/stale
- Trading quotes fail

**Fallback:**
- Circuit breaker opens after 5 failures (30s timeout)
- Gateway returns cached market data with timestamp banner: "Market data may be stale — Panta unavailable"
- Signal Radar shows cached signals with timestamp
- Trading buttons disabled with "Panta unavailable" tooltip
- Status page shows "Panta: Degraded"

**Communication:**
> "Panta is experiencing issues. Showing cached data from [timestamp]. Trading disabled until restored."

---

### 2. AI Provider Unavailable (OpenAI)

**Symptom:**
- `/copilot` returns 502/503
- Market Studio `/interpret` fails
- Copilot returns generic error

**Fallback:**
- Copilot UI shows "AI temporarily unavailable" with retry button
- Market Studio `/interpret` returns structured error: "AI interpretation unavailable. You can still manually create a market draft."
- Deterministic validation (`/validate`) still works — no AI needed
- Market Studio falls back to manual draft entry

**Communication:**
> "AI service temporarily unavailable. Market Studio available in manual mode. Deterministic validation still active."

---

### 3. Wallet Unavailable / Rejected

**Symptom:**
- Wallet popup doesn't open
- User clicks "Reject" in wallet
- Wallet disconnected mid-flow

**Fallback:**
- Wallet popup: Show "Please connect Phantom or Solflare" with install links
- User rejects: Show "Transaction cancelled. Nothing was submitted." — **never** "failed"
- Disconnected mid-flow: Preserve draft state, show "Wallet disconnected. Reconnect to continue."

**Never say:** "Transaction failed" when user cancelled.

---

### 4. Database Unavailable

**Symptom:**
- `/health/ready` returns 503
- Queries timeout
- Worker pauses

**Fallback:**
- Gateway returns 503 on `/health/ready`
- New requests get 503 immediately
- In-flight requests complete or timeout
- Worker pauses (pauses sync loop, keeps advisory lock)
- Auto-reconnect with exponential backoff

**Communication:**
> "Database temporarily unavailable. In-flight requests completing. Please retry in a moment."

---

### 5. AI Provider Returns Malformed Response

**Symptom:**
- Copilot returns malformed JSON
- Market Studio interpret returns invalid JSON
- Tool calls malformed

**Fallback:**
- Catch parse errors, return structured error: "AI returned unexpected format. Please rephrase."
- Market Studio: Show "AI returned unexpected format. Please rephrase or edit manually."
- Log full response for debugging (never expose to user)

---

### 5. Solana RPC Unavailable

**Symptom:**
- Broadcast fails
- Confirmation polling times out
- Transaction status unknown

**Fallback:**
- Broadcast: "Solana RPC unavailable. Try again in a moment."
- Confirmation: Poll with exponential backoff (2s, 4s, 8s, max 60s)
- After timeout: "Confirmation timed out. Check Solana explorer: [signature link]"
- Never claim transaction failed — only "confirmation pending"

---

### 6. Panta Registration Fails After Solana Confirmation

**Scenario:** Transaction confirmed on Solana, but Panta `/register` fails

**Response:**
- Status: `CONFIRMED` (Solana confirmed)
- Panta registration: `REGISTERING` → `FAILED`
- UI shows: **"Transaction confirmed on Solana. Panta registration is pending."**
- **Never** show "Market creation failed"
- Retry button: "Retry Panta Registration" (idempotent on createId + signature)
- Polling: Auto-retry registration every 30s for 5 minutes

---

### 6. Worker Crashes / Stuck

**Symptom:**
- No new signals for > 10 minutes
- Alert events stop
- Worker logs show panic or stall

**Fallback:**
- Advisory lock prevents duplicate workers
- Restart worker: `go run ./cmd/worker`
- Auto-recovers on restart (advisory lock re-acquired)
- Overlapping read window catches missed signals

---

### 7. Market Engine Binary Missing / Fails

**Symptom:**
- Worker logs: "market engine binary not found" or "engine failed"
- No new signals generated

**Fallback:**
- Error: "Deterministic engine unavailable. Signals paused."
- Market data still flows (Panta → observations)
- Alert evaluation pauses (no new signals to evaluate)
- Fix: `cd services/market-engine && cargo build --release` (Linux/CI)

---

### 8. AI Returns Malformed / Hallucinated Response

**Symptom:**
- Copilot returns nonsense
- Market Studio draft has invalid fields
- Source citations missing

**Fallback:**
- Catch parse errors → "AI returned unexpected format. Please rephrase."
- Market Studio: Show raw AI output in collapsible "Raw Output" section
- Validation runs regardless — invalid draft fails validation
- Log full AI response for debugging (never expose to user)

---

### 6. Panta Returns 429 / Rate Limited

**Symptom:**
- Adapter returns 429
- `Retry-After` header present

**Fallback:**
- Respect `Retry-After` header (capped at 30s)
- Show user: "Panta rate limited. Retrying in [n]s..."
- Circuit breaker tracks failures independently
- After 5 failures → circuit opens, returns cached data

---

### 8. Wallet Disconnects Mid-Flow

**Scenario:** User connects wallet, gets quote, builds, then disconnects wallet before signing

**Fallback:**
- Draft state preserved in localStorage
- On reconnect: "Wallet reconnected. Your draft is preserved."
- Quote expires per Panta expiry — re-quote if expired
- Draft hash invalidates on any material edit

---

### 9. Panta Returns Malformed Response

**Symptom:**
- Panta returns 200 but invalid JSON
- Missing expected fields
- Schema mismatch

**Fallback:**
- Adapter validates response schema
- On failure: "Panta returned unexpected response. Retrying..."
- Log full response for debugging
- Circuit breaker tracks as failure

---

### 9. User Navigates Away Mid-Flow

**Scenario:** User closes tab during Market Studio, Trading, or Alert creation

**Fallback:**
- Draft state in localStorage (Market Studio)
- Trade attempt persisted in DB (server-side)
- On return: "You have an in-progress [market draft / trade]. Continue?"

---

## Demo Mode

If `NEXT_PUBLIC_DEMO_MODE=true`:
- Banner: "**DEMO MODE** — Data is simulated"
- All Panta calls return deterministic fixtures
- Market Engine returns fixed signals
- AI returns canned responses
- Wallet signing simulated (no real wallet needed)
- Panta calls return static fixtures
- **Never** claim demo data is real

**Banner:**
```
┌─────────────────────────────────────────────┐
│  ⚠️  DEMO MODE — Data is simulated          │
│  No real transactions will be submitted.    │
└─────────────────────────────────────────────┘
```

---

## Never Fake

| Never Fake | Instead |
|------------|---------|
| Transaction confirmation | Show "Pending confirmation..." |
| Panta verification | Show "Verifying with Panta..." |
| Market creation | Show "Registering with Panta..." |
| Wallet signature | Show actual wallet popup |
| Market data | Show "No data" or cached with timestamp |
| Signal generation | Show "No signals yet" |
| Alert trigger | Show "No alerts yet" |

---

## Demo Mode Flag

```bash
export NEXT_PUBLIC_DEMO_MODE=true
```

When enabled:
- All Panta calls return deterministic fixtures
- Market Engine returns fixed signals
- AI returns canned responses
- Wallet signing simulated
- Banner: "**DEMO MODE** — No real transactions will be submitted"

**Never** claim demo data is real.

---

## Post-Demo Cleanup

- Stop all services gracefully
- `docker compose -f infrastructure/docker-compose.dev.yml down`
- Clear browser cookies/localStorage for clean state
- Reset demo wallet if used

---

*Fallback Plan v1.0 — Review before every demo*