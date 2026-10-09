@echo off
REM QEVRYN Services Startup Script - Batch Version
REM Double-click to run, or run from cmd

echo Starting QEVRYN Services...

REM Kill any existing processes on our ports
for %%p in (8080 8081 8001 5432 6379) do (
    for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":%%p " ^| findstr "LISTENING"') do (
        echo Killing process on port %%p (PID: %%a)
        taskkill /F /PID %%a >nul 2>&1
    )
)

REM Start Docker containers
echo Starting PostgreSQL and Redis...
docker-compose -f infrastructure/docker-compose.dev.yml up -d

REM Wait for databases to be ready
timeout /t 3 /nobreak >nul

REM Start Panta Adapter in new window
echo Starting Panta Adapter on port 8081...
start "Panta Adapter" cmd /k "cd /d C:\Users\pc\OneDrive\Desktop\closseum hack\services\panta-adapter && set PANTA_API_BASE_URL=https://live-api.panta.market/api/v1/ && set PANTA_API_KEY=YOUR_PANTA_API_KEY_HERE && go run ./cmd/server"

REM Start Intelligence Service in new window
echo Starting Intelligence Service on port 8001...
start "Intelligence Service" cmd /k "cd /d C:\Users\pc\OneDrive\Desktop\closseum hack\services\intelligence && set AI_PROVIDER=openrouter && set AI_API_KEY=YOUR_OPENROUTER_API_KEY_HERE && set AI_BASE_URL=https://openrouter.ai/api/v1 && set AI_MODEL=openai/gpt-4o-mini && set APP_ENV=development && set PORT=8001 && python -m app.main"

REM Start Gateway in new window
echo Starting Gateway on port 8080...
start "Gateway" cmd /k "cd /d C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway && set DATABASE_URL=postgresql://prophet:prophet@127.0.0.1:5432/prophet?sslmode=disable && set PANTA_API_KEY=YOUR_PANTA_API_KEY_HERE && set SOLANA_RPC_URL=https://mainnet.helius-rpc.com/?api-key=YOUR_HELIUS_API_KEY_HERE && set JWT_SECRET=YOUR_JWT_SECRET_HERE && set PANTA_ADAPTER_URL=http://127.0.0.1:8081/ && set PANTA_API_URL=https://live-api.panta.market/api/v1/ && set INTELLIGENCE_SERVICE_URL=http://localhost:8001 && .\gateway.exe"

echo.
echo All services started in separate windows!
echo.
echo Gateway:      http://localhost:8080
echo Panta Adapter: http://localhost:8081
echo Intelligence:  http://localhost:8001
echo.
echo Close the windows to stop the services.
pause
