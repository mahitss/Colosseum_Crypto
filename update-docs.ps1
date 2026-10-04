$files = Get-ChildItem 'C:\Users\pc\OneDrive\Desktop\closseum hack\docs' -Filter *.md -File
foreach ($file in $files) {
    $content = Get-Content $file.FullName -Raw
    $newContent = $content -replace 'Prophet', 'QEVRYN' -replace 'prophet', 'qevryn'
    if ($content -ne $newContent) {
        Set-Content $file.FullName -Value $newContent
        Write-Host "Updated: $($file.Name)"
    }
}