# QEVRYN Trading E2E Test Mode

Automated tests must **never** spend real funds or touch mainnet.

This project provides a deterministic test mode for the trading transaction
lifecycle without pretending those transactions are production transactions.

## How It Works

### Go Gateway (httptest-backed)

The Go test suite covers the complete trade lifecycle using:

| Component | Test double | What it verifies |
|-----------|-------------|------------------|
| Panta API | `httptest.Server` returning fixture JSON | quote 200/400/401/429/500, malformed responses, build, report, verify, positions |
| Solana RPC | `httptest.Server` returning fixture JSON-RPC | `sendRawTransaction`, confirmation polling (confirmed/failed/timeout), invalid encodings, retry behavior |
| HTTP layer | `fakeTradeService` (in-memory) | request validation, DTO shaping, error status codes |

Deterministic fixtures:
- A fixed fake signature string is used everywhere (never a real mainnet signature).
- A fixed base64 placeholder is used for the "signed transaction".
- No network egress is ever attempted during tests.

### Frontend

The `/markets/[id]` page renders the `TradeTicket` component which drives a
client-side state machine. To exercise every state without real funds:

1. Start the gateway with `PANTA_API_URL`, `PANTA_API_KEY`, `SOLANA_RPC_URL`
   pointing at a **mock endpoint** (e.g. a local `httptest`-style stub), or run
   the gateway with only the mocked endpoints reachable.
2. Use a **burner/test wallet** (never a funded mainnet wallet).
3. Drive the UI through the states and assert each rendered state.

### UI States That Can Be Verified

- `WALLET_NOT_CONNECTED` — disconnect the wallet, attempt a trade.
- `QUOTING` / `QUOTE_READY` — quote request with a mock-gated gateway.
- `BUILDING` / `READY_TO_SIGN` — build with mock Panta.
- `SIGNING` / `USER_REJECTED` — reject the wallet signature prompt.
- `BROADCASTING` / `SUBMITTED` — broadcast via mocked RPC.
- `CONFIRMING` / `CONFIRMED` / `CONFIRMATION_TIMEOUT` — mocked confirmation.
- `REPORTING` / `VERIFIED` / `PANTA_REPORT_FAILED` — mocked report.
- `POSITION_REFRESHING` / `COMPLETED` — position endpoint returns fixtures.
- `PANTA_REPORT_FAILED` after `CONFIRMED` — asserts the "Transaction confirmed
  on Solana. Panta verification is pending." message (the critical distinction).

## Rules

- Real transactions require explicit manual user action and a user wallet
  signature. They are never part of CI.
- No automated test may call a funded wallet.
- No automated test may broadcast to mainnet.
- The signing prompt is always user-initiated in tests, exactly as in production.
