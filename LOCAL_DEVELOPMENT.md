# QEVRYN Services - Local Development

## Quick Start

### Option 1: Batch Script (Easiest - Double Click)
```cmd
start_all_services.bat
```
This opens 3 separate command windows for each service.

### Option 2: PowerShell Script (Single Window)
```powershell
.\start_all_services.ps1
```
Runs all services in background, shows health checks, press Ctrl+C to stop all.

### Option 3: Manual (For Debugging)
Open 3 terminals and run:

**Terminal 1 - Panta Adapter:**
```cmd
cd services\panta-adapter
set PANTA_API_BASE_URL=https://live-api.panta.market/api/v1/
set PANTA_API_KEY=YOUR_PANTA_API_KEY_HERE
go run ./cmd/server
```

**Terminal 2 - Intelligence Service:**
```cmd
cd services\intelligence
set AI_PROVIDER=openrouter
set AI_API_KEY=YOUR_OPENROUTER_API_KEY_HERE
set AI_BASE_URL=https://openrouter.ai/api/v1
set AI_MODEL=openai/gpt-4o-mini  # Requires OpenRouter credits
set APP_ENV=development
set PORT=8001
python -m app.main
```

**Terminal 3 - Gateway:**
```cmd
cd services\gateway
set DATABASE_URL=postgresql://prophet:prophet@127.0.0.1:5432/prophet?sslmode=disable
set PANTA_API_KEY=YOUR_PANTA_API_KEY_HERE
set SOLANA_RPC_URL=https://mainnet.helius-rpc.com/?api-key=YOUR_HELIUS_API_KEY_HERE
set JWT_SECRET=YOUR_JWT_SECRET_HERE
set PANTA_ADAPTER_URL=http://127.0.0.1:8081/
set PANTA_API_URL=https://live-api.panta.market/api/v1/
set INTELLIGENCE_SERVICE_URL=http://localhost:8001
.\gateway.exe
```

## Service Endpoints

| Service | URL | Description |
|---------|-----|-------------|
| Gateway | http://localhost:8080 | Main API gateway |
| Panta Adapter | http://localhost:8081 | Panta markets proxy |
| Intelligence | http://localhost:8001 | AI market studio |

## Health Checks
```cmd
curl http://localhost:8080/health
curl http://localhost:8081/health
curl http://localhost:8001/health
```

## Test Market Studio
```cmd
curl -X POST http://localhost:8080/api/v1/market-studio/interpret ^
  -H "Content-Type: application/json" ^
  -d "{\"prompt\": \"Will Bitcoin reach $100k by end of 2024?\"}"
```

## Stop Services
```cmd
stop_all_services.bat
```

## Requirements
- Docker Desktop (for PostgreSQL & Redis)
- Go 1.21+
- Python 3.11+
- `gateway.exe` binary (run `go build -o gateway.exe .` in services/gateway)

## Troubleshooting

**Port already in use:**
```cmd
stop_all_services.bat
start_all_services.bat
```

**Panta authentication failed:**
- Check PANTA_API_KEY in start script matches your key
- Key should be `pk_live_...` (no duplicate `pk_live_` prefix)

**AI interpretation failed:**
- Check AI_API_KEY is valid OpenRouter key
- Verify model `apodex/apodex-1.1-mini:free` is available

**Database connection failed:**
- Ensure Docker containers are running: `docker ps`
- Check PostgreSQL on port 5432, Redis on 6379
