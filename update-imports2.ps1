$files = @(
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\auth\middleware.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\intelligence\normalize.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\intelligence\normalize_test.go',
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\gateway\internal\intelligence\sync.go'
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