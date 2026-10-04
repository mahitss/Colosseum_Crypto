$files = @(
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\main.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\cmd\worker\main.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\cmd\migrate\main.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\cmd\sync-markets\main.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\alerts.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\alerts_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\health.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\intelligence.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\markets.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\markets_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\market_studio.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\market_studio_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\trading.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\trading_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\watchlists.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\watchlists_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\market_studio.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\market_studio_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\trading.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\trading_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\watchlists.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\watchlists_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\market_studio.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\market_studio_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\trading.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\trading_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\watchlists.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\httpapi\watchlists_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\cmd\worker\main.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\cmd\migrate\main.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\cmd\sync-markets\main.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\panta-adapter\cmd\server\main.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\panta-adapter\internal\client\client.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\panta-adapter\internal\client\client_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\panta-adapter\internal\client\retry_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\panta-adapter\internal\markets\service.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\panta-adapter\internal\markets\service_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\panta-adapter\internal\transport\handler.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\panta-adapter\internal\transport\handler_test.go'
)

foreach ($file in $files) {
    if (Test-Path $file) {
        $content = Get-Content $file -Raw
        $newContent = $content -replace 'prophet/gateway/', 'qevryn/gateway/' -replace 'prophet/panta-adapter/', 'qevryn/panta-adapter/' -replace 'prophet/types', 'qevryn/types'
        if ($content -ne $newContent) {
            Set-Content $file -Value $newContent
            Write-Host "Updated: $file"
        }
    }
}