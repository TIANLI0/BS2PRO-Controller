@echo off
echo 正在构建温度桥接程序...

set "ROOT=%~dp0"
set "PROJECT=%ROOT%bridge\TempBridge\TempBridge.csproj"
set "OUTDIR=%ROOT%build\bin\bridge"

if not exist "%OUTDIR%" mkdir "%OUTDIR%"

echo 还原NuGet包...
dotnet restore "%PROJECT%"
if errorlevel 1 goto :error

echo 编译发布版本...
dotnet publish "%PROJECT%" -c Release --self-contained false -o "%OUTDIR%"
if errorlevel 1 goto :error

echo 构建完成，输出目录: %OUTDIR%
goto :end

:error
echo 构建失败，请查看上方日志。

:end
pause
