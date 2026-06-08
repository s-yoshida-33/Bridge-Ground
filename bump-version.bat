@echo off
setlocal enabledelayedexpansion

cd /d "%~dp0"

if "%~1"=="" (
    echo Usage: bump-version.bat ^<version^>
    echo Example: bump-version.bat 4.0.4
    echo.
    echo Updates version in:
    echo   internal\config\config.go
    echo   cmd\bridgeground\versioninfo.json
    echo   package.bat
    exit /b 1
)

set "VER=%~1"
for /f "tokens=1,2,3 delims=." %%a in ("%VER%") do (
    set "MAJOR=%%a"
    set "MINOR=%%b"
    set "PATCH=%%c"
)

if "!MAJOR!"=="" goto :bad_version
if "!MINOR!"=="" goto :bad_version
if "!PATCH!"=="" goto :bad_version

echo Bumping Bridge-Ground to v%VER%...

powershell -NoProfile -Command "(gc 'internal\config\config.go') -replace 'var Version = ""[^""]+""', 'var Version = ""%VER%""' | sc 'internal\config\config.go'"
if errorlevel 1 goto :error
echo   [OK] internal\config\config.go

powershell -NoProfile -Command "(gc 'cmd\bridgeground\versioninfo.json') -replace '""Major"": \d+','""Major"": !MAJOR!' -replace '""Minor"": \d+','""Minor"": !MINOR!' -replace '""Patch"": \d+','""Patch"": !PATCH!' -replace '""FileVersion"": ""\d+\.\d+\.\d+\.\d+""','""FileVersion"": ""%VER%.0""' -replace '""ProductVersion"": ""\d+\.\d+\.\d+\.\d+""','""ProductVersion"": ""%VER%.0""' | sc 'cmd\bridgeground\versioninfo.json'"
if errorlevel 1 goto :error
echo   [OK] cmd\bridgeground\versioninfo.json

powershell -NoProfile -Command "(gc 'package.bat') -replace 'set ""VERSION=[^""]+""','set ""VERSION=%VER%""' | sc 'package.bat'"
if errorlevel 1 goto :error
echo   [OK] package.bat

echo.
echo Done. Build with: package.bat
exit /b 0

:bad_version
echo ERROR: Invalid format. Use MAJOR.MINOR.PATCH  ^(e.g. 4.0.4^)
exit /b 1

:error
echo ERROR: Failed to update files. Make sure PowerShell is available.
exit /b 1
