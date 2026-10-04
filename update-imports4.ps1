$files = @(
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\ratelimit\middleware.go'
)

foreach ($file in $files) {
    if (Test-Path $file) {
        $content = Get-Content $file -Raw
        $newContent = $content -replace 'prophet/gateway/', 'qevryn/gateway/' -replace 'prophet/types', 'qevryn/types'
        if ($content -ne $newContent) {
            Set-Content $file -Value $newContent
            Write-Host "Updated: $file"
        }
    }
}