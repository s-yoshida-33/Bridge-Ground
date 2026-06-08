@echo off
if "%~1"=="" (
    echo Usage: bump-version.bat ^<version^>
    echo Example: bump-version.bat 4.0.4
    exit /b 1
)
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0bump-version.ps1" -Version "%~1"
