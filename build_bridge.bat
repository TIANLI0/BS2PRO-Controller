@echo off
echo Building temperature bridge program...

set "ROOT=%~dp0"
set "PROJECT=%ROOT%bridge\TempBridge\TempBridge.csproj"
set "OUTDIR=%ROOT%build\bin\bridge"
set "BUILDROOT=%ROOT%build\bin"
set "PAWNIO_URL=https://github.com/namazso/PawnIO.Setup/releases/latest/download/PawnIO_setup.exe"
set "PAWNIO_OUT=%BUILDROOT%\PawnIO_setup.exe"

if not exist "%OUTDIR%" mkdir "%OUTDIR%"
if not exist "%BUILDROOT%" mkdir "%BUILDROOT%"

echo Restoring NuGet packages...
dotnet restore "%PROJECT%"
if errorlevel 1 goto :error

echo Building release version...
dotnet publish "%PROJECT%" -c Release --self-contained false -o "%OUTDIR%"
if errorlevel 1 goto :error

echo Downloading PawnIO installer...
powershell -NoProfile -ExecutionPolicy Bypass -Command "try { [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri '%PAWNIO_URL%' -OutFile '%PAWNIO_OUT%' -UseBasicParsing; exit 0 } catch { Write-Error $_; exit 1 }"
if errorlevel 1 goto :error

if not exist "%PAWNIO_OUT%" goto :error
echo PawnIO downloaded to: %PAWNIO_OUT%

echo Build completed, output directory: %OUTDIR%
goto :end

:error
echo Build failed! Please check the logs above.

:end
pause
