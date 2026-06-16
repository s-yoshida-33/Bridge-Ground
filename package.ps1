$Version     = "4.0.6"
$PackageName = "BridgeGround_v$Version"

Set-Location $PSScriptRoot

Write-Host "Creating distribution package..."

if (Test-Path $PackageName) {
    Remove-Item -Recurse -Force $PackageName
}
New-Item -ItemType Directory -Path $PackageName | Out-Null

Write-Host "Building executable..."
go build -ldflags "-H windowsgui" -o "$PackageName\Bridge Ground.exe" ./cmd/bridgeground
if ($LASTEXITCODE -ne 0) {
    Write-Error "Build failed."
    exit 1
}

Write-Host "Copying configuration..."
Copy-Item config.json "$PackageName\"

Write-Host "Copying resources..."
Copy-Item -Recurse src "$PackageName\src"

Write-Host "Copying documentation..."
Copy-Item README.md "$PackageName\"

Write-Host ""
Write-Host "Package created: $PackageName"
Write-Host "You can zip this folder for distribution."
Write-Host ""
Read-Host "Press Enter to exit"
