# Temperature Bridge Program

## Overview

Since Go cannot directly call C# libraries, we created a C# bridge program `TempBridge.exe` that uses `LibreHardwareMonitorLib` via NuGet to obtain accurate CPU and GPU temperature data.

The current bridge program uses `LibreHardwareMonitorLib >= 0.9.6`, which is based on `PawnIO` capabilities and no longer bundles `WinRing0` resources.

## Build Instructions

### Prerequisites

- Install [.NET 8.0 SDK](https://dotnet.microsoft.com/download/dotnet/8.0)
- Access to NuGet sources (`dotnet restore` will automatically fetch `LibreHardwareMonitorLib`)

### Windows Build

```bash
# Run from project root directory
build_bridge.bat
```

### Linux/Mac Build (Cross-compilation)

```bash
# Run from project root directory
chmod +x build_bridge.sh
./build_bridge.sh
```

### Manual Build

```bash
cd bridge/TempBridge
dotnet restore
dotnet publish TempBridge.csproj -c Release --self-contained false -o ../../build/bin/bridge
```

## How It Works

1. The Go program calls `TempBridge.exe`
2. The bridge program reads hardware temperatures via `LibreHardwareMonitorLib` from NuGet
3. The bridge program outputs temperature data in JSON format
4. The Go program parses and uses the JSON data

## Direct Launch Diagnostics

When running `TempBridge.exe` directly from the command line, the program enters diagnostic mode instead of named pipe mode:

```bash
cd bridge/TempBridge/bin/Release/net472
TempBridge.exe
```

Diagnostic mode outputs directly to the console:

- CPU/GPU/MAX temperatures
- Whether reading was successful
- Error messages
- Discovered temperature sensor names and readings

To force the original pipe mode, pass the `--pipe` parameter.

`--pipe` mode now uses a fixed named pipe and a global single-instance mutex; if a TempBridge instance is already listening on the system, a new process will not start a second listener but will attach to the existing instance.

## Output Format

```json
{
  "cpuTemp": 45,
  "gpuTemp": 38,
  "maxTemp": 45,
  "updateTime": 1692259200,
  "success": true,
  "error": ""
}
```

## Error Handling

If the bridge program is unavailable or fails, the Go program automatically falls back to alternative temperature reading methods:

1. Reading sensor data using `gopsutil`
2. Reading Windows system temperature via WMI
3. Reading NVIDIA GPU temperature using `nvidia-smi`

## Notes

- The bridge program requires administrator privileges to access all hardware sensors
- The first run may take some time to initialize hardware monitoring
- If you encounter permission issues, try running the main program as administrator
- Ensure `PawnIO` is installed on the system before running (TempBridge will report an error and exit at startup if not installed)
