# QEVRYN Services Startup Script
# Run this in PowerShell as Administrator for best results

Write-Host "Starting QEVRYN Services..." -ForegroundColor Green

# Kill any existing processes on our ports
$ports = @(8080, 8081, 8001, 5432, 6379)
foreach ($port in $ports) {
    $process = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty OwningProcess
    if ($process) {
        Write-Host "Killing process on port $port (PID: $process)" -ForegroundColor Yellow
        Stop-Process -Id $process -Force -ErrorAction SilentlyContinue
    }
}

# Start Docker containers
Write-Host "Starting PostgreSQL and Redis..." -ForegroundColor Cyan
docker-compose -f infrastructure/docker-compose.dev.yml up -d

# Wait for databases to be ready
Start-Sleep -Seconds 3

# Start Panta Adapter
Write-Host "Starting Panta Adapter on port 8081..." -ForegroundColor Cyan
$pantaEnv = @{
    PANTA_API_BASE_URL = "https://live-api.panta.market/api/v1/"
    PANTA_API_KEY = "YOUR_PANTA_API_KEY_HERE"
}
$pantaProcess = Start-Process -FilePath "go" -ArgumentList "run", "./cmd/server" -WorkingDirectory "C:\Users\pc\OneDrive\Desktop\closseum hack\services\panta-adapter" -PassThru -EnvironmentVariable $pantaEnv -WindowStyle Hidden

# Start Intelligence Service
Write-Host "Starting Intelligence Service on port 8001..." -ForegroundColor Cyan
$intelEnv = @{
    AI_PROVIDER = "openrouter"
    AI_API_KEY = "YOUR_OPENROUTER_API_KEY_HERE"
    AI_BASE_URL = "https://openrouter.ai/api/v1"
    AI_MODEL = "apodex/apodex-1.1-mini:free"
    APP_ENV = "development"
    PORT = "8001"
}
$intelProcess = Start-Process -FilePath "python" -ArgumentList "-m", "app.main" -WorkingDirectory "C:\Users\pc\OneDrive\Desktop\closseum hack\services\intelligence" -PassThru -EnvironmentVariable $intelEnv -WindowStyle Hidden

# Start Gateway
Write-Host "Starting Gateway on port 8080..." -ForegroundColor Cyan
$gatewayEnv = @{
    DATABASE_URL = "postgresql://prophet:prophet@127.0.0.1:5432/prophet?sslmode=disable"
    PANTA_API_KEY = "YOUR_PANTA_API_KEY_HERE"
    SOLANA_RPC_URL = "https://mainnet.helius-rpc.com/?api-key=YOUR_HELIUS_API_KEY_HERE"
    JWT_SECRET = "YOUR_JWT_SECRET_HERE"
    PANTA_ADAPTER_URL = "http://127.0.0.1:8081/"
    PANTA_API_URL = "https://live-api.panta.market/api/v1/"
    INTELLIGENCE_SERVICE_URL = "http://localhost:8001"
}
$gatewayProcess = Start-Process -FilePath ".\gateway.exe" -WorkingDirectory "C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway" -PassThru -EnvironmentVariable $gatewayEnv -WindowStyle Hidden

Write-Host ""
Write-Host "Services started!" -ForegroundColor Green
Write-Host "Panta Adapter PID: $($pantaProcess.Id)" -ForegroundColor Gray
Write-Host "Intelligence Service PID: $($intelProcess.Id)" -ForegroundColor Gray
Write-Host "Gateway PID: $($gatewayProcess.Id)" -ForegroundColor Gray
Write-Host ""
Write-Host "Waiting for services to be ready..." -ForegroundColor Yellow
Start-Sleep -Seconds 5

# Test endpoints
Write-Host "Testing endpoints..." -ForegroundColor Cyan
try {
    $health = Invoke-RestMethod -Uri "http://127.0.0.1:8080/health" -Method GET
    Write-Host "Gateway: $($health.status)" -ForegroundColor Green
} catch {
    Write-Host "Gateway: Not ready yet" -ForegroundColor Red
}

try {
    $health = Invoke-RestMethod -Uri "http://127.0.0.1:8081/health" -Method GET
    Write-Host "Panta Adapter: $($health.status)" -ForegroundColor Green
} catch {
    Write-Host "Panta Adapter: Not ready yet" -ForegroundColor Red
}

try {
    $health = Invoke-RestMethod -Uri "http://127.0.0.1:8001/health" -Method GET
    Write-Host "Intelligence: $($health.status)" -ForegroundColor Green
} catch {
    Write-Host "Intelligence: Not ready yet" -ForegroundColor Red
}

Write-Host ""
Write-Host "Press Ctrl+C to stop all services" -ForegroundColor Yellow

# Keep script running and handle cleanup
try {
    while ($true) {
        Start-Sleep -Seconds 10
    }
} finally {
    Write-Host "Stopping services..." -ForegroundColor Yellow
    Stop-Process -Id $pantaProcess.Id -Force -ErrorAction SilentlyContinue
    Stop-Process -Id $intelProcess.Id -Force -ErrorAction SilentlyContinue
    Stop-Process -Id $gatewayProcess.Id -Force -ErrorAction SilentlyContinue
    Write-Host "All services stopped." -ForegroundColor Green
}
