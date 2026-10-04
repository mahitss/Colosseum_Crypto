$files = @(
    'C:\Users\pc\OneDrive\Desktop\closseum hack\services\panta-adapter\cmd\smoke-test\main.go'
)

foreach ($file in $files) {
    if (Test-Path $file) {
        $content = Get-Content $file -Raw
        $newContent = $content -replace 'prophet/panta-adapter/', 'qevryn/panta-adapter/' -replace 'prophet/types', 'qevryn/types'
        if ($content -ne $newContent) {
            Set-Content $file -Value $newContent
            Write-Host "Updated: $file"
        }
    }
}