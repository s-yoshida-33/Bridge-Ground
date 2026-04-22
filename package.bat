@echo off
set "VERSION=4.0.1"
set "PACKAGE_NAME=BridgeGround_v%VERSION%"

echo Creating distribution package...

if exist "%PACKAGE_NAME%" rmdir /s /q "%PACKAGE_NAME%"
mkdir "%PACKAGE_NAME%"

echo Building executable...
go build -ldflags "-H windowsgui" -o "%PACKAGE_NAME%\Bridge Ground.exe" ./cmd/bridgeground

echo Copying configuration...
copy config.json "%PACKAGE_NAME%\" >nul

echo Copying resources...
xcopy /S /E /Y /I src "%PACKAGE_NAME%\src" >nul

echo Copying documentation...
copy README.md "%PACKAGE_NAME%\" >nul

echo.
echo Package created in directory: %PACKAGE_NAME%
echo You can zip this folder for distribution.
echo.
pause


