# Panta Integration

## Configuration

The adapter reads these environment variables:

- `PANTA_API_BASE_URL`: defaults to `https://live-api.panta.market/api/v1/`; the trailing slash is normalized and retained.
- `PANTA_API_KEY`: server-side key sent only as `X-Api-Key`. Required for Panta requests; never commit or expose it.
- `PANTA_API_TIMEOUT_SECONDS`: per-attempt request timeout; defaults to `10` and accepts `1` through `120`.
- `PANTA_ADAPTER_URL`: gateway-to-adapter URL; defaults to `http://127.0.0.1:8081`.

Export the key in the server shell or inject it through a secret manager. Do not place it in frontend environment variables, source files, URLs, or logs.

## Local setup

Start the adapter:

```powershell
$env:PANTA_API_KEY = "<your server-side key>"
$env:PANTA_API_BASE_URL = "https://live-api.panta.market/api/v1/"
$env:PANTA_API_TIMEOUT_SECONDS = "10"
Set-Location services/panta-adapter
go run ./cmd/server
```

Start the gateway in another shell:

```powershell
$env:PANTA_ADAPTER_URL = "http://127.0.0.1:8081"
Set-Location services/gateway
go run .
```

Run the read-only live smoke test after exporting the adapter environment variables:

```powershell
Set-Location services/panta-adapter
go run ./cmd/smoke-test
```

Or run `make panta-smoke` from the repository root. The command fails clearly when the key is missing. It requests one market page with limit 1 and prints only each returned market's ID, title, and phase. It never creates markets, trades, signs, submits transactions, or prints the key.

## Supported operations

| Qevryn route | Adapter route | Panta route | Behavior |
| --- | --- | --- | --- |
| `GET /api/v1/markets` | `GET /markets/` | `GET /api/v1/markets/` | Cursor-paginated catalog, optional `category`, `status`, `createdBy`, `cursor`, and `limit` (1-50; Panta default 20). |
| `GET /api/v1/markets/{id}` | `GET /markets/{id}/` | `GET /api/v1/markets/{marketId}/` | Catalog detail including spot price fields when Panta's RPC lookup is available. |
| Not exposed | `GET /health/panta` | `GET /api/v1/account/` | Authenticated, read-only connectivity diagnostic. |

The catalog list does not live-query chain prices; price fields are null there. Detail may include `yesPrice`, `noPrice`, and primary/secondary prices when RPC data is available. `nextCursor` is opaque and is passed back unchanged.

## Models, auth, and errors

Panta JSON models are confined to `services/panta-adapter/internal/client`. The adapter validates/decode bounds the response and maps each record through `internal/markets` to shared `qevryn/types.Market`. The gateway consumes and returns only Qevryn domain types.

The server authenticates with `X-Api-Key` and forwards a generated/caller request ID as `X-Request-Id`. Logs contain the request path, status, duration, attempt, and safe error kind only; credentials and response bodies are not logged. Adapter errors are typed and caller responses are sanitized.

Retries are bounded to at most three attempts by default. Network errors/timeouts, HTTP 429, and HTTP 5xx may retry with jittered exponential backoff. `Retry-After` is respected but capped at 30 seconds. HTTP 400/401/403/404 and malformed successful payloads are not retried.

## Amount types

The shared Go types keep formats distinct: `HumanUSDC` is a human-readable decimal string such as `"20.00"`; `USDCBaseUnits` is an integer base-unit string such as `"50000000"`. This read-only task does not convert or use either for trading.

## Custody and limitations

The backend never receives a user's private key or seed phrase. When trading is implemented later, Panta will build unsigned transactions/instructions, the user wallet will sign, the transaction will be broadcast through the chosen Solana RPC, and QEVRYN will report only the resulting signature to Panta. This flow is not implemented here.

No Redis cache, trading, wallet signing, AI, market creation, order submission, positions, or claims are included. Market detail prices can be absent when Panta's RPC data is unavailable. Automated tests use `httptest`; the live smoke test is manual and requires a valid key.

## Official references

- [Panta documentation index](https://docs.panta.market/llms.txt)
- [List markets](https://docs.panta.market/api-reference/markets/list)
- [Get market](https://docs.panta.market/api-reference/markets/get)
- [Authentication](https://docs.panta.market/guides/authentication)
- [Errors and rate limits](https://docs.panta.market/guides/errors)

