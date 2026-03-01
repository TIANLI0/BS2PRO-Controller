@echo off
echo 正在构建温度桥接程序...

set "ROOT=%~dp0"
set "PROJECT=%ROOT%bridge\TempBridge\TempBridge.csproj"
set "OUTDIR=%ROOT%build\bin\bridge"
set "BUILDROOT=%ROOT%build\bin"
set "PAWNIO_URL=https://github.com/namazso/PawnIO.Setup/releases/latest/download/PawnIO_setup.exe"
set "PAWNIO_OUT=%BUILDROOT%\PawnIO_setup.exe"

if not exist "%OUTDIR%" mkdir "%OUTDIR%"
if not exist "%BUILDROOT%" mkdir "%BUILDROOT%"

echo 还原NuGet包...
dotnet restore "%PROJECT%"
if errorlevel 1 goto :error

echo 编译发布版本...
dotnet publish "%PROJECT%" -c Release --self-contained false -o "%OUTDIR%"
if errorlevel 1 goto :error

echo 下载 PawnIO 安装器...
powershell -NoProfile -ExecutionPolicy Bypass -Command "try { [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri '%PAWNIO_URL%' -OutFile '%PAWNIO_OUT%' -UseBasicParsing; exit 0 } catch { Write-Error $_; exit 1 }"
if errorlevel 1 goto :error

if not exist "%PAWNIO_OUT%" goto :error
echo PawnIO 下载完成: %PAWNIO_OUT%

echo 构建完成，输出目录: %OUTDIR%
goto :end

:error
echo 构建失败，请查看上方日志。

:end
pause
