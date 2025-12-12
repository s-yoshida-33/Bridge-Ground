@echo off
if not exist build mkdir build

echo Building BridgeGround...
go build -o "build/Bridge Ground.exe" ./cmd/bridgeground

echo Copying resources to build folder...
copy /Y config.json build\ >nul
xcopy /S /E /Y /I src build\src >nul

echo Starting BridgeGround...
cd build
start "" "Bridge Ground.exe"
