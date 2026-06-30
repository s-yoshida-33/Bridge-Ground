param(
    [string]$IsccPath = "C:\Program Files (x86)\Inno Setup 6\ISCC.exe"
)

$Version    = "4.0.8"
$ScriptDir  = Split-Path -Parent $MyInvocation.MyCommand.Definition
$RepoRoot   = Split-Path -Parent $ScriptDir
$DistDir    = Join-Path $ScriptDir "dist"
$ReleaseDir = Join-Path $RepoRoot "release"

Set-Location $RepoRoot

# Clean staging directory
if (Test-Path $DistDir) {
    Remove-Item -Recurse -Force $DistDir
}
New-Item -ItemType Directory -Path $DistDir | Out-Null

# Ensure release directory exists
if (-not (Test-Path $ReleaseDir)) {
    New-Item -ItemType Directory -Path $ReleaseDir | Out-Null
}

Write-Host "Building bridge-ground.exe (v$Version)..."
$env:GOARCH = "amd64"
$env:GOOS   = "windows"
go build -ldflags "-H windowsgui" -o "$DistDir\bridge-ground.exe" ./cmd/bridgeground
if ($LASTEXITCODE -ne 0) {
    Write-Error "Build failed."
    exit 1
}

Write-Host "Copying src/..."
Copy-Item -Recurse (Join-Path $RepoRoot "src") (Join-Path $DistDir "src")

Write-Host "Running Inno Setup..."
if (-not (Test-Path $IsccPath)) {
    Write-Error "ISCC.exe not found at: $IsccPath"
    Write-Host "Install Inno Setup 6 from https://jrsoftware.org/isinfo.php"
    Write-Host "Or specify the path: -IsccPath `"C:\path\to\ISCC.exe`""
    exit 1
}

$IssFile = Join-Path $ScriptDir "installer.iss"
& $IsccPath $IssFile
if ($LASTEXITCODE -ne 0) {
    Write-Error "Inno Setup compilation failed."
    exit 1
}

Write-Host ""
Write-Host "Done! Installer: $ReleaseDir\BridgeGroundSetup-x64-$Version.exe"
