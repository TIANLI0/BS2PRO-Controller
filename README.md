# BS2PRO-Controller

> A third-party alternative controller for FlyDigi Space Station BS2/BS2PRO

A desktop application built with Wails + Go + Next.js for controlling FlyDigi Space Station BS2/BS2PRO cooler devices, providing fan control, temperature monitoring, and more.

## Features

- **Device Support**: Supports FlyDigi BS2 and BS2PRO coolers
- **Temperature Monitoring**: Real-time CPU/GPU temperature monitoring (supports multiple temperature data bridging methods)
- **Fan Control**:
  - Auto Mode: Automatically adjusts fan speed based on temperature
  - Learning Temperature Control: Continuously learns and fine-tunes curve offset based on target temperature
  - Manual Mode: Custom fixed fan speed
  - Curve Mode: Custom temperature-fan speed curve
- **Visual Dashboard**: Intuitive real-time temperature and fan speed display
- **System Tray**: Supports minimizing to system tray for background operation
- **Auto-start on Boot**: Can be set to auto-start and minimize on boot
- **Multi-process Architecture**: GUI and core services are separated for stability and reliability
- **LED Strip Configuration**: Supports complex LED strip control, thanks to community member @Whether

## System Architecture

The project uses a three-process architecture:

- **GUI Process** (`BS2PRO-Controller.exe`): Provides the user interface, built with the Wails framework
- **Core Service** (`BS2PRO-Core.exe`): Runs in the background, responsible for device communication and temperature monitoring
- **Temperature Bridge Process** (`TempBridge.exe`): A C# program that retrieves system temperature data

The three processes exchange data via IPC (Inter-Process Communication).

## Tech Stack

### Backend
- **Go 1.25+**: Primary development language
- **Wails v2**: Cross-platform desktop application framework
- **go-hid**: HID device communication
- **zap**: Logging

### Frontend
- **Next.js 16**: React framework
- **TypeScript**: Type safety
- **Tailwind CSS 4**: Styling framework
- **Recharts**: Chart visualization

### Temperature Bridge
- **C# .NET Framework 4.7.2**: Temperature data bridge program

## Development Environment Requirements

### Required Software
- **Go 1.21+**: [Download](https://golang.org/dl/)
- **Node.js 18+**: [Download](https://nodejs.org/)
- **Bun**: Fast JavaScript runtime [Installation Guide](https://bun.sh/)
- **Wails CLI**: Install with `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **.NET SDK 8.0+**: [Download](https://dotnet.microsoft.com/download)
- **go-winres**: Windows resource tool `go install github.com/tc-hib/go-winres@latest`

### Optional Software
- **NSIS 3.x**: For generating installers [Download](https://nsis.sourceforge.io/)

## Quick Start

### 1. Clone the Project

```bash
git clone https://github.com/TIANLI0/BS2PRO-Controller.git
cd BS2PRO-Controller
```

### 2. Install Dependencies

#### Install Go Dependencies
```bash
go mod tidy
```

#### Install Frontend Dependencies
```bash
cd frontend
bun install
cd ..
```

### 3. Run in Development Mode

```bash
# Start Wails development mode (with hot reload)
wails dev
```

### 4. Build Production Version

#### Build Temperature Bridge Program
```bash
build_bridge.bat
```

#### Build Complete Application
```bash
build.bat
```

After building, the executables are located in the `build/bin/` directory:
- `BS2PRO-Controller.exe` - GUI main program
- `BS2PRO-Core.exe` - Core service
- `bridge/TempBridge.exe` - Temperature bridge program

The installer is located in the `build/bin/` directory:
- `BS2PRO-Controller-amd64-installer.exe` - Windows installer

## Project Structure

```
BS2PRO-Controller/
├── main.go                 # GUI main program entry point
├── app.go                  # GUI application logic
├── wails.json             # Wails configuration file
├── build.bat              # Windows build script
├── build_bridge.bat       # Bridge program build script
├── cmd/
│   └── core/              # Core service program
│       ├── main.go        # Service entry point
│       └── app.go         # Service logic
│
├── internal/              # Internal packages
│   ├── autostart/         # Auto-start on boot management
│   ├── bridge/            # Temperature bridge communication
│   ├── config/            # Configuration management
│   ├── device/            # HID device communication
│   ├── ipc/               # Inter-process communication
│   ├── logger/            # Logging module
│   ├── temperature/       # Temperature monitoring
│   ├── tray/              # System tray
│   ├── types/             # Type definitions
│   └── version/           # Version information
│
├── bridge/
│   └── TempBridge/        # C# temperature bridge program
│       └── Program.cs     # Bridge program source code
│
├── frontend/              # Next.js frontend
│   ├── src/
│   │   ├── app/
│   │   │   ├── components/    # React components
│   │   │   ├── services/      # API services
│   │   │   └── types/         # TypeScript types
│   │   └── ...
│   └── package.json
│
└── build/                 # Build output directory
```

## Usage Guide

### First Run

1. Run `BS2PRO-Controller.exe` to launch the program
2. The program will automatically start the core service `BS2PRO-Core.exe`
3. Connect your BS2/BS2PRO device (USB connection)
4. The program will automatically detect and connect to the device

### Fan Control Modes

#### Auto Mode
- Automatically adjusts fan speed based on current temperature
- Suitable for daily use

#### Manual Mode
- Set a fixed fan speed level (levels 0-9)
- Suitable for specific use cases

#### Curve Mode
- Custom temperature-fan speed curve
- Multiple control points can be added
- Achieves fine-grained temperature control

### Temperature Monitoring

The program supports multiple temperature monitoring methods:

1. **TempBridge**: Retrieves system temperature via the C# bridge program


### System Tray

- Click the tray icon to open the main window
- Right-click menu provides quick actions
- Supports minimizing to tray for background operation

## Configuration File

The configuration file is located at `%APPDATA%\BS2PRO-Controller\config.json`

Main configuration options:
```json
{
  "autoStart": false,           // Auto-start on boot
  "minimizeToTray": true,       // Minimize to tray on close
  "temperatureSource": "auto",  // Temperature data source
  "updateInterval": 1000,       // Update interval (milliseconds)
  "fanCurve": [...],           // Fan curve
  "fanMode": "auto"            // Fan mode
}
```

## Log Files

Log files are located in the `build/bin/logs/` directory:
- `core_YYYYMMDD.log` - Core service log
- `gui_YYYYMMDD.log` - GUI program log

## FAQ

### Device won't connect?
1. Make sure the BS2/BS2PRO device is properly connected to the computer
2. Check that the device driver is installed correctly
3. Try unplugging and re-plugging the device
4. Check the log files for specific errors

### Temperature not showing?
1. Check the temperature data source settings
2. If using TempBridge, make sure the files in the `bridge` directory are complete
3. If using AIDA64/HWiNFO, make sure the software is running and shared memory is enabled

### Auto-start on boot not working?
1. Run the program as administrator and reconfigure the setting
2. Check the registry entry: `HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run`

## Build Instructions

### Version Number Management

The version number is defined in the `info.productVersion` field of `wails.json`. The build script automatically reads and embeds it into the program.

### LDFLAGS

Version information is injected during build:
```bash
-ldflags "-X github.com/TIANLI0/BS2PRO-Controller/internal/version.BuildVersion=VERSION -H=windowsgui"
```

### Generating the Installer

Running `build.bat` will automatically generate an NSIS installer (NSIS must be installed).

## Contributing

Issues and Pull Requests are welcome!

1. Fork this project
2. Create a feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Author

- **TIANLI0** - [GitHub](https://github.com/TIANLI0)
- Email: wutianli@tianli0.top

## Acknowledgements

- [Wails](https://wails.io/) - Excellent Go desktop application framework
- [Next.js](https://nextjs.org/) - React application framework
- FlyDigi - BS2/BS2PRO hardware devices

## Disclaimer

This is a third-party open-source project and is not affiliated with FlyDigi. Any issues arising from the use of this software are the sole responsibility of the user.

---

If this project is helpful to you, please give it a Star!
