# Market Studio

Market Studio is the user-facing flow for creating a prediction market. An AI
assistant helps the user phrase a market, deterministic code checks it, and the
user reviews and signs every material decision themselves.

The central design rule: **the assistant proposes, the human disposes.** The
model is never a decision-maker about a market that real money will be settled
against, and it is never a signer.

## What the AI may and may not do

The assistant is allowed to:

- Turn a plain-language description into a structured draft.
- Propose a resolution rule and candidate sources.
- Explain what is missing and why.
- Ask for clarification.

The assistant is not allowed to:

- Create, sign, or submit a market on its own.
- Decide the resolution rule without human review.
- Invent an authoritative resolution source. If the source is ambiguous the
  draft is marked as requiring user confirmation, and creation is blocked until
  a human confirms it.
- Bypass validation, or modify a draft the user has already reviewed without
  re-presenting it.

The LLM is also never the final validator. See
[Two-tier validation](#two-tier-validation) below.

## Creation flow

```
User
  ↓ describe
AI Market Architect ────────→ clarification? → "Your question needs more detail"
  ↓ draft proposal                    (no market is created)
Deterministic Validation (Python, then re-checked in Go)
  ↓ valid + source confirmed
Panta Quote  ──── exact fees in USDC base units
  ↓
Panta Build  ──── unsigned transaction
  ↓
User's Wallet ──── the only place a signature is produced
  ↓ signed transaction
Gateway broadcasts to Solana
  ↓ confirmation
Panta Registration ──── the only step that makes a market exist
  ↓
QEVRYN Indexing ──── market becomes visible in the app
```

Only the **Panta Registration** step creates a market. A Solana signature by
itself means the transaction landed on chain, nothing more.

## API

All endpoints are on the gateway under `/api/v1/market-studio`.

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/interpret` | Description → draft proposal, or a clarification request |
| `POST` | `/validate` | Deterministic validation of a draft |
| `POST` | `/quote` | Exact fee quote from Panta |
| `POST` | `/build` | Unsigned transaction from Panta |
| `POST` | `/broadcast` | Relay the wallet-signed transaction, await confirmation |
| `POST` | `/register` | Register with Panta. The only step that creates a market |
| `GET` | `/attempts/{id}` | Authoritative state of a creation attempt |

### The Panta creation API

Verified against the official Panta documentation. Only fields that Panta
actually supports are sent; nothing is invented.

**`POST /api/v1/markets/create/quote/`**

Required: `wallet`, `question` (≤512), `resolutionRule` (≤2048),
`sourcesOfTruth` (non-empty, ≤20 entries), `category` (must be one of
`sports`, `crypto`, `politics`, `entertainment`, `finance`, `science`, `world`,
`other`), `startTime`, `endTime`, `resolutionTime` (unix seconds, with
`startTime < endTime <= resolutionTime`), and `imageUrl` (required, http/https,
≤2048, not localhost/private, not a data URL).

Optional: `marketType` (`standard` | `breaking`), `eventInProgress` (breaking
only), `title`, `description`, `region`, `oracle`.

Response: `createId`, `expectedEventPda`, `paymentUsdc`,
`liquidityInjectionUsdc`, `platformRevenueUsdc`, `marketType`, `expiresAt`,
`blockhashExpiryHintSec`.

**`POST /api/v1/markets/create/build/`**

Body: `createId` (required), `wallet` (optional, must match the quote's wallet).

Response: `transaction` (base64 `VersionedTransaction`), `recentBlockhash`,
`lastValidBlockHeight`, `blockhashExpiryHintSec`, `buildFingerprint`,
`paymentUsdc`, `liquidityInjectionUsdc`, `platformRevenueUsdc`, `marketType`,
`derived`, `expiresAt`.

**`POST /api/v1/markets/register/`**

Note the path: registration is *not* under `markets/create/`.

Body: `createId`, `signature` (base58). Verification is fail-closed. Repeating
the same `createId` and `signature` is idempotent.

Response: `createId`, `marketId` (the event PDA), `status: "registered"`,
`signature`, `category`, `title`, `images`.

### Money is never a float

Panta specifies all creation amounts in **USDC base units as integer strings**
(6 decimals). `"50000000"` is 50 USDC. The value is never parsed into a float
or a decimal number anywhere in the stack:

- Go carries every amount as `string`.
- `ParseBaseUnits` accepts integers only and rejects anything else.
- The database enforces it with `CHECK (col ~ '^[0-9]+$')`.
- The frontend formats by splitting the string, not by arithmetic.

Fees are never estimated or fabricated. The UI shows what Panta quoted, and the
build response is cross-checked against the quote.

## Two-tier validation

The validation split is deliberate and is the reason this feature is safe.

**Tier 1 — structural (Pydantic).** A generous envelope with an
`HARD_INPUT_CEILING` of 8192 characters. It exists to reject hostile or
absurdly large input, not to enforce business rules.

**Tier 2 — deterministic business rules (`validation.py`, and mirrored in
`validate.go`).** The exact Panta limits: question length, category allowlist,
image URL shape and host, date ordering, source count and specificity, and the
non-declarative-question rule.

Tier 1 must not pre-empt tier 2. The AI's honest output legitimately contains
blank fields — an unconfirmed resolution source has no URL yet — and those
blanks are *input to* the deterministic layer, not errors in it. An earlier
version validated the image URL inside Pydantic and crashed on a legitimately
empty field.

The Go gateway re-runs the deterministic check on the assistant's draft and
never trusts a `valid` flag arriving from the Python service.

The LLM is not consulted during validation at any point.

## The nine-step wizard

`apps/web/app/studio/page.tsx`, with the decision logic factored into
`apps/web/lib/market-studio.ts` so it can be unit tested.

1. **Describe** — free text.
2. **Clarify** — shown only when the question is too vague. Displays
   "Your question needs more detail before a market can be created." and stops.
3. **Review Draft** — every assistant-proposed field, editable.
4. **Resolution Rules** — criteria, sources, and the explicit source
   confirmation checkbox.
5. **Validate** — the deterministic report.
6. **Quote** — Panta's exact fee breakdown.
7. **Build & Sign** — the wallet prompts the user. The server never signs.
8. **Broadcast** — awaiting Solana confirmation.
9. **Register** — the final authoritative state.

Steps are selectable only up to the furthest step actually reached, so the user
can go back but cannot jump ahead past a gate.

### The Create action

The Create Market button is **not rendered at all** unless:

- the user is on the Validate step,
- deterministic validation passed,
- the user has reviewed the content, and
- if there are warnings, the user has explicitly acknowledged them.

Warnings are always displayed. They are never hidden, never collapsed away, and
never auto-cleared by an edit.

## State tracking

Eleven states are tracked, and the distinction between them is the point:

`CREATED` → `SIGNED` → `BROADCASTING` → `SUBMITTED` → `CONFIRMING` →
`CONFIRMED` → `REGISTERING` → `REGISTERED` → `INDEXED`, plus `FAILED` and
`UNKNOWN`.

- `CONFIRMED` means Solana settled the transaction. It does **not** mean a
  market exists.
- `REGISTERED` and `INDEXED` are the only states where a market exists. This is
  encoded in `Attempt.MarketExists()`.
- Success is never reported merely because a signature exists.
- No local market record is created before authoritative registration.

### The confirmed-but-unregistered case

If Solana confirms and Panta registration then fails, the response is a `502`
carrying the `CONFIRMED` attempt, and the UI says:

> Transaction confirmed on Solana. Panta registration is pending.

It never says "Market creation failed", because that would be false. The
registration is idempotent on `createId` + `signature`, so it can be retried
safely.

## Error codes

`AI_INTERPRETATION_FAILED`, `INVALID_DRAFT`, `AMBIGUOUS_MARKET`,
`QUOTE_FAILED`, `BUILD_FAILED`, `WALLET_NOT_CONNECTED`, `USER_REJECTED`,
`INVALID_TRANSACTION`, `BROADCAST_FAILED`, `CONFIRMATION_TIMEOUT`,
`PANTA_REGISTRATION_FAILED`, `INDEXING_PENDING`.

HTTP mapping is centralised in `marketStudioStatus`. Two mappings are load
bearing:

- `PANTA_REGISTRATION_FAILED` → **502**, not 4xx. The request was not rejected;
  the upstream failed *after* the chain settled. A 4xx would push the UI toward
  "creation failed".
- `DRAFT_CHANGED_AFTER_QUOTE` → **409**. The reviewed content no longer matches
  the quote, so the client must re-quote rather than retry blindly.

## Draft integrity and idempotency

A SHA-256 draft hash covers the **material** fields only. Outcome labels, notes
and source ordering are presentation-only and deliberately excluded, so
reordering sources or rewording a note does not throw away a valid quote.
Changing any material field after the quote invalidates both the quote and the
built transaction; the client shows which fields changed, and the server
re-checks the same hash on broadcast.

Idempotency is keyed on `creation_attempt_id` + `draft_hash` +
`wallet_address` — never on question text alone, since two markets may
legitimately share a question.

## Storage

The `market_creation_attempts` table records every attempt and its state
transitions. It deliberately has **no columns** for private keys, seed phrases,
or signed transaction blobs. Signed transactions exist only in transit between
the wallet and the broadcast endpoint and are never persisted.

## Testing

Automated tests never create a real market. Panta and Solana are stubbed at the
HTTP boundary, and the wallet is stubbed at the `signTransaction` hook.

```bash
# Wizard decision logic
cd apps/web && npm test

# Deterministic validation and the service pipeline
cd services/gateway && go test ./internal/marketstudio/ ./internal/httpapi/

# AI draft parsing, validation, and ambiguity handling
cd services/intelligence && python -m pytest -q
```

The two most important tests are
`TestConfirmedButRegistrationFailedIsNotReportedAsCreationFailure` in Go and
`registration is never reported as a creation failure` in the frontend suite.
Both exist to keep the honest-but-uncomfortable wording from being "simplified"
away by a future edit.

