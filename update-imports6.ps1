$files = @(
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\intelligence\tests\test_health.py'
)

foreach ($file in $files) {
    if (Test-Path $file) {
        $content = Get-Content $file -Raw
        $newContent = $content -replace 'prophet-intelligence', 'qevryn-intelligence'
        if ($content -ne $newContent) {
            Set-Content $file -Value $newContent
            Write-Host "Updated: $file"
        }
    }
}