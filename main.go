package main

import (
	"context"
	"embed"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/TIANLI0/BS2PRO-Controller/internal/ipc"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

//go:embed all:frontend/dist
var assets embed.FS

var mainLogger *zap.SugaredLogger

func init() {
	logger, _ := zap.NewProduction()
	mainLogger = logger.Sugar()
}

var wailsContext *context.Context

// onSecondInstanceLaunch callback when a second instance is launched
func onSecondInstanceLaunch(secondInstanceData options.SecondInstanceData) {
	println("Second instance detected, args:", strings.Join(secondInstanceData.Args, ","))
	println("Working directory:", secondInstanceData.WorkingDirectory)

	if wailsContext != nil {
		runtime.WindowUnminimise(*wailsContext)
		runtime.WindowShow(*wailsContext)
		runtime.WindowSetAlwaysOnTop(*wailsContext, true)
		go func() {
			time.Sleep(1 * time.Second)
			runtime.WindowSetAlwaysOnTop(*wailsContext, false)
		}()

		runtime.EventsEmit(*wailsContext, "secondInstanceLaunch", secondInstanceData.Args)
	}
}

// ensureCoreServiceRunning ensures the core service is running
func ensureCoreServiceRunning() bool {
	// Detect if running in Wails binding generation mode
	exePath, err := os.Executable()
	if err == nil {
		tempDir := os.TempDir()
		if strings.HasPrefix(exePath, tempDir) {
			mainLogger.Info("Binding generation mode detected, skipping core service startup")
			return true
		}
	}

	// Check if core service is already running
	if ipc.CheckCoreServiceRunning() {
		mainLogger.Info("Core service is already running")
		return true
	}

	mainLogger.Info("Core service is not running, starting...")

	// Get core service path
	if err != nil {
		mainLogger.Errorf("Failed to get executable path: %v", err)
		return false
	}

	exeDir := filepath.Dir(exePath)
	corePath := filepath.Join(exeDir, "BS2PRO-Core.exe")

	// Check if core service executable exists
	if _, err := os.Stat(corePath); os.IsNotExist(err) {
		mainLogger.Errorf("Core service executable not found: %s", corePath)
		return false
	}

	// Start core service
	cmd := exec.Command(corePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x08000000, // CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW
	}

	if err := cmd.Start(); err != nil {
		mainLogger.Errorf("Failed to start core service: %v", err)
		return false
	}

	mainLogger.Infof("Core service started, PID: %d", cmd.Process.Pid)

	// Release process handle
	if cmd.Process != nil {
		cmd.Process.Release()
	}

	// Wait for core service to be ready
	for range 50 {
		time.Sleep(100 * time.Millisecond)
		if ipc.CheckCoreServiceRunning() {
			mainLogger.Info("Core service is ready")
			return true
		}
	}

	mainLogger.Warn("Timed out waiting for core service to be ready")
	return false
}

func main() {
	if !ensureCoreServiceRunning() {
		mainLogger.Warn("Warning: unable to start core service, GUI will run in limited functionality mode")
	}

	app := NewApp()

	windowStartState := options.Normal
	for _, arg := range os.Args {
		if arg == "--autostart" || arg == "/autostart" || arg == "-autostart" {
			windowStartState = options.Minimised
			break
		}
	}

	// Create application
	err := wails.Run(&options.App{
		Title:            "BS2PRO Controller",
		Width:            1024,
		Height:           768,
		WindowStartState: windowStartState,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},

		OnStartup: func(ctx context.Context) {
			wailsContext = &ctx
			app.startup(ctx)
		},
		OnBeforeClose: app.OnWindowClosing,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "d2111a29-a967-4e46-807f-2fb5fcff9ed4-gui",
			OnSecondInstanceLaunch: onSecondInstanceLaunch,
		},
		Windows: &windows.Options{
			WindowIsTranslucent: true,
		},
		Bind: []any{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
