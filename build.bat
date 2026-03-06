@echo off
setlocal enabledelayedexpansion
echo Building BS2PRO-Controller...

REM Extract version from wails.json
for /f "tokens=2 delims=:, " %%a in ('findstr /C:"\"productVersion\"" wails.json') do (
    set VERSION=%%a
    set VERSION=!VERSION:"=!
)

if "!VERSION!"=="" (
    echo WARNING: Could not extract version from wails.json, using dev
    set VERSION=dev
) else (
    echo Building version: !VERSION!
)

set LDFLAGS=-X github.com/TIANLI0/BS2PRO-Controller/internal/version.BuildVersion=!VERSION! -H=windowsgui

REM Build core service first
echo Building core service...
go-winres make --in cmd/core/winres/winres.json --out cmd/core/rsrc
go build -ldflags "!LDFLAGS!" -o build/bin/BS2PRO-Core.exe ./cmd/core/

REM Installer icon is still file-based; system notification icon is now embedded in BS2PRO-Core.exe
if not exist "build\windows\icon.ico" (
    echo WARNING: build\windows\icon.ico not found, executable/installer icon may be incorrect
)

REM Build main application with wails
echo Building main application...
wails build -nsis -ldflags "!LDFLAGS!"

REM Ensure core service is in the bin directory for installer
if exist "build\bin\BS2PRO-Core.exe" (
    echo Core service built successfully
) else (
    echo ERROR: Core service build failed!
    exit /b 1
)

echo Build completed successfully with version !VERSION!
endlocal