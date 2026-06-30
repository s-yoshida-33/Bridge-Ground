param(
    [Parameter(Mandatory=$true)]
    [string]$Version
)

if ($Version -notmatch '^\d+\.\d+\.\d+$') {
    Write-Error "Invalid version format. Use MAJOR.MINOR.PATCH (e.g. 4.1.0)"
    exit 1
}

$parts = $Version.Split('.')
$major = [int]$parts[0]
$minor = [int]$parts[1]
$patch = [int]$parts[2]

Set-Location $PSScriptRoot

Write-Host "Bumping Bridge-Ground to v$Version..."

# internal/config/config.go
$f = 'internal\config\config.go'
(Get-Content $f) -replace 'var Version = "[^"]+"', ('var Version = "' + $Version + '"') |
    Set-Content $f
Write-Host "  [OK] $f"

# cmd/bridgeground/versioninfo.json
$f = 'cmd\bridgeground\versioninfo.json'
$c = Get-Content $f
$c = $c -replace '"Major": \d+',                            ('"Major": '           + $major)
$c = $c -replace '"Minor": \d+',                            ('"Minor": '           + $minor)
$c = $c -replace '"Patch": \d+',                            ('"Patch": '           + $patch)
$c = $c -replace '"FileVersion": "\d+\.\d+\.\d+\.\d+"',    ('"FileVersion": "'    + $Version + '.0"')
$c = $c -replace '"ProductVersion": "\d+\.\d+\.\d+\.\d+"', ('"ProductVersion": "' + $Version + '.0"')
$c | Set-Content $f
Write-Host "  [OK] $f"

# build/build-installer.ps1
$f = 'build\build-installer.ps1'
(Get-Content $f) -replace '\$Version\s*=\s*"[^"]+"', ('$Version    = "' + $Version + '"') |
    Set-Content $f
Write-Host "  [OK] $f"

# build/installer.iss
$f = 'build\installer.iss'
$c = Get-Content $f
$c = $c -replace '^AppVersion=.+',          ('AppVersion=' + $Version)
$c = $c -replace '^OutputBaseFilename=.+',  ('OutputBaseFilename=BridgeGroundSetup-x64-' + $Version)
$c | Set-Content $f
Write-Host "  [OK] $f"

Write-Host ""
Write-Host "Done. Build installer with: .\build\build-installer.ps1"
