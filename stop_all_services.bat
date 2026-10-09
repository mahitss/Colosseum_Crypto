@echo off
REM QEVRYN Services Stop Script

echo Stopping QEVRYN Services...

REM Kill processes on our ports
for %%p in (8080 8081 8001) do (
    for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":%%p " ^| findstr "LISTENING"') do (
        echo Killing process on port %%p (PID: %%a)
        taskkill /F /PID %%a >nul 2>&1
    )
)

REM Stop Docker containers
echo Stopping Docker containers...
docker-compose -f infrastructure/docker-compose.dev.yml down

echo All services stopped.
pause