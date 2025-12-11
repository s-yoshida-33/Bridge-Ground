@echo off
if not exist bridge-ground.exe (
    echo Building BridgeGround...
    go build -o bridge-ground.exe ./cmd/bridgeground
)
echo Starting BridgeGround...
start bridge-ground.exe

