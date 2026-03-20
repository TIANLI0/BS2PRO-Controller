package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/TIANLI0/BS2PRO-Controller/internal/autostart"
	"github.com/TIANLI0/BS2PRO-Controller/internal/bridge"
	"github.com/TIANLI0/BS2PRO-Controller/internal/config"
	"github.com/TIANLI0/BS2PRO-Controller/internal/device"
	hotkeysvc "github.com/TIANLI0/BS2PRO-Controller/internal/hotkey"
	"github.com/TIANLI0/BS2PRO-Controller/internal/ipc"
	"github.com/TIANLI0/BS2PRO-Controller/internal/logger"
	"github.com/TIANLI0/BS2PRO-Controller/internal/notifier"
	"github.com/TIANLI0/BS2PRO-Controller/internal/smartcontrol"
	"github.com/TIANLI0/BS2PRO-Controller/internal/temperature"
	"github.com/TIANLI0/BS2PRO-Controller/internal/tray"
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
	"github.com/TIANLI0/BS2PRO-Controller/internal/version"
)

//go:embed icon.ico
var iconData []byte

// CoreApp is the core application structure
type CoreApp struct {
	ctx context.Context

	// Managers
	deviceManager    *device.Manager
	bridgeManager    *bridge.Manager
	tempReader       *temperature.Reader
	configManager    *config.Manager
	trayManager      *tray.Manager
	hotkeyManager    *hotkeysvc.Manager
	notifier         *notifier.Manager
	autostartManager *autostart.Manager
	logger           *logger.CustomLogger
	ipcServer        *ipc.Server

	// State
	isConnected        bool
	monitoringTemp     bool
	currentTemp        types.TemperatureData
	lastDeviceMode     string
	userSetAutoControl bool
	isAutoStartLaunch  bool
	debugMode          bool

	// Monitoring related
	guiLastResponse   int64
	guiMonitorEnabled bool
	healthCheckTicker *time.Ticker
	cleanupChan       chan bool
	quitChan          chan bool

	// Synchronization
	mutex                 sync.RWMutex
	stopMonitoring        chan bool
	manualGearLevelMemory map[string]string
}

// NewCoreApp creates a new core application instance
func NewCoreApp(debugMode, isAutoStart bool) *CoreApp {
	// Initialize logging system
	installDir := config.GetInstallDir()
	customLogger, err := logger.NewCustomLogger(debugMode, installDir)
	if err != nil {
		// If initialization fails, cannot log, exit directly
		panic(fmt.Sprintf("Failed to initialize logging system: %v", err))
	} else {
		customLogger.Info("Core service started")
		customLogger.Info("Install directory: %s", installDir)
		customLogger.Info("Debug mode: %v", debugMode)
		customLogger.Info("Auto-start mode: %v", isAutoStart)
		customLogger.CleanOldLogs()
	}

	// Create managers
	bridgeMgr := bridge.NewManager(customLogger)
	deviceMgr := device.NewManager(customLogger)
	tempReader := temperature.NewReader(bridgeMgr, customLogger)
	configMgr := config.NewManager(installDir, customLogger)
	trayMgr := tray.NewManager(customLogger, iconData)
	autostartMgr := autostart.NewManager(customLogger)

	app := &CoreApp{
		ctx:                context.Background(),
		deviceManager:      deviceMgr,
		bridgeManager:      bridgeMgr,
		tempReader:         tempReader,
		currentTemp:        types.TemperatureData{BridgeOk: true},
		configManager:      configMgr,
		trayManager:        trayMgr,
		autostartManager:   autostartMgr,
		logger:             customLogger,
		isConnected:        false,
		monitoringTemp:     false,
		stopMonitoring:     make(chan bool, 1),
		lastDeviceMode:     "",
		userSetAutoControl: false,
		isAutoStartLaunch:  isAutoStart,
		debugMode:          debugMode,
		guiLastResponse:    time.Now().Unix(),
		cleanupChan:        make(chan bool, 1),
		quitChan:           make(chan bool, 1),
		guiMonitorEnabled:  true,
		manualGearLevelMemory: map[string]string{
			"Silent":      "Mid",
			"Standard":    "Mid",
			"Performance": "Mid",
			"Overclock":   "Mid",
		},
	}
	app.notifier = notifier.NewManager(customLogger, iconData)
	app.hotkeyManager = hotkeysvc.NewManager(customLogger, app.handleHotkeyAction)

	return app
}

// Start starts the core service
func (a *CoreApp) Start() error {
	a.logInfo("=== BS2PRO Core Service Starting ===")
	a.logInfo("Version: %s", version.Get())
	a.logInfo("Install directory: %s", config.GetInstallDir())
	a.logInfo("Debug mode: %v", a.debugMode)
	a.logInfo("Current working directory: %s", config.GetCurrentWorkingDir())

	// Detect if this is an auto-start launch
	a.isAutoStartLaunch = autostart.DetectAutoStartLaunch(os.Args)
	a.logInfo("Auto-start mode: %v", a.isAutoStartLaunch)

	// Load configuration
	a.logInfo("Loading configuration file")
	cfg := a.configManager.Load(a.isAutoStartLaunch)
	if normalizedLight, changed := normalizeLightStripConfig(cfg.LightStrip); changed {
		cfg.LightStrip = normalizedLight
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("Failed to save default light strip config: %v", err)
		}
	}
	if normalizedSmart, changed := smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode); changed {
		cfg.SmartControl = normalizedSmart
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("Failed to save default smart control config: %v", err)
		}
	}
	if normalizeHotkeyConfig(&cfg) {
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("Failed to save default hotkey config: %v", err)
		}
	}
	if normalizeCurveProfilesConfig(&cfg) {
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("Failed to save default fan curve profile config: %v", err)
		}
	}
	if normalizeManualGearMemoryConfig(&cfg) {
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("Failed to save default gear memory config: %v", err)
		}
	}
	a.syncManualGearLevelMemory(cfg)
	a.logInfo("Configuration loaded, config path: %s", cfg.ConfigPath)

	// Sync debug mode from config
	if cfg.DebugMode {
		a.debugMode = true
		if a.logger != nil {
			a.logger.SetDebugMode(true)
		}
		a.logInfo("Synced debug mode from config: enabled")
	}

	// Check and sync Windows auto-start status
	a.logInfo("Checking Windows auto-start status")
	actualAutoStart := a.autostartManager.CheckWindowsAutoStart()
	if actualAutoStart != cfg.WindowsAutoStart {
		cfg.WindowsAutoStart = actualAutoStart
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("Failed to save config when syncing Windows auto-start status: %v", err)
		} else {
			a.logInfo("Synced Windows auto-start status: %v", actualAutoStart)
		}
	}

	// Initialize HID
	a.logInfo("Initializing HID library")
	if err := a.deviceManager.Init(); err != nil {
		a.logError("Failed to initialize HID library: %v", err)
		return err
	}
	a.logInfo("HID library initialized successfully")

	// Set device callbacks
	a.deviceManager.SetCallbacks(a.onFanDataUpdate, a.onDeviceDisconnect)

	// Start IPC server
	a.logInfo("Starting IPC server")
	a.ipcServer = ipc.NewServer(a.handleIPCRequest, a.logger)
	if err := a.ipcServer.Start(); err != nil {
		a.logError("Failed to start IPC server: %v", err)
		return err
	}

	// Initialize system tray
	a.logInfo("Initializing system tray")
	a.initSystemTray()
	a.applyHotkeyBindings(cfg)

	// Start health monitoring
	if cfg.GuiMonitoring {
		a.logInfo("Starting health monitoring")
		a.safeGo("startHealthMonitoring", func() {
			a.startHealthMonitoring()
		})
	}

	a.logInfo("=== BS2PRO Core Service Started ===")

	// Start temperature monitoring immediately after launch (decoupled from smart control toggle)
	a.safeGo("startTemperatureMonitoring@Start", func() {
		a.startTemperatureMonitoring()
	})

	// Try to connect device
	a.safeGo("delayedConnectDevice", func() {
		if a.isAutoStartLaunch {
			// Wait longer during auto-start to allow device firmware to finish initialization
			a.logInfo("Auto-start mode: waiting for device initialization (3 seconds)")
			time.Sleep(3 * time.Second)
		} else {
			time.Sleep(1 * time.Second)
		}
		a.ConnectDevice()
	})

	return nil
}

// Stop stops the core service
func (a *CoreApp) Stop() {
	a.logInfo("Core service stopping...")
	a.stopTemperatureMonitoring()
	if a.hotkeyManager != nil {
		a.hotkeyManager.Stop()
	}

	// Clean up resources
	a.cleanup()

	// Stop all monitoring
	a.DisconnectDevice()

	// Stop bridge program
	a.bridgeManager.Stop()

	// Stop IPC server
	if a.ipcServer != nil {
		a.ipcServer.Stop()
	}

	// Stop tray
	a.trayManager.Quit()

	a.logInfo("Core service stopped")
}

// initSystemTray initializes the system tray
func (a *CoreApp) initSystemTray() {
	a.trayManager.SetCallbacks(
		a.onShowWindowRequest,
		a.onQuitRequest,
		func() bool {
			cfg := a.configManager.Get()
			newState := !cfg.AutoControl
			a.SetAutoControl(newState)
			return newState
		},
		func(profileID string) string {
			profile, err := a.SetActiveFanCurveProfile(profileID)
			if err != nil {
				a.logError("Tray failed to set fan curve profile: %v", err)
				return ""
			}
			return profile.Name
		},
		func() ([]tray.CurveOption, string) {
			cfg := a.configManager.Get()
			options := make([]tray.CurveOption, 0, len(cfg.FanCurveProfiles))
			for _, p := range cfg.FanCurveProfiles {
				if p.ID == "" {
					continue
				}
				name := p.Name
				if strings.TrimSpace(name) == "" {
					name = "Default"
				}
				options = append(options, tray.CurveOption{ID: p.ID, Name: name})
			}
			return options, cfg.ActiveFanCurveProfileID
		},
		func() tray.Status {
			a.mutex.RLock()
			defer a.mutex.RUnlock()
			cfg := a.configManager.Get()
			fanData := a.deviceManager.GetCurrentFanData()
			var currentRPM uint16
			if fanData != nil {
				currentRPM = fanData.CurrentRPM
			}
			curveOptions := make([]tray.CurveOption, 0, len(cfg.FanCurveProfiles))
			for _, p := range cfg.FanCurveProfiles {
				if p.ID == "" {
					continue
				}
				name := p.Name
				if strings.TrimSpace(name) == "" {
					name = "Default"
				}
				curveOptions = append(curveOptions, tray.CurveOption{ID: p.ID, Name: name})
			}

			return tray.Status{
				Connected:            a.isConnected,
				CPUTemp:              a.currentTemp.CPUTemp,
				GPUTemp:              a.currentTemp.GPUTemp,
				CurrentRPM:           currentRPM,
				AutoControlState:     cfg.AutoControl,
				ActiveCurveProfileID: cfg.ActiveFanCurveProfileID,
				CurveProfiles:        curveOptions,
			}
		},
	)
	a.trayManager.Init()
}

// onShowWindowRequest handles show window request callback
func (a *CoreApp) onShowWindowRequest() {
	a.logInfo("Received show window request")

	// Notify all connected GUI clients to show window
	if a.ipcServer != nil && a.ipcServer.HasClients() {
		a.ipcServer.BroadcastEvent("show-window", nil)
	} else {
		// No GUI connection, launch GUI
		a.logInfo("No GUI connection, attempting to launch GUI")
		if err := launchGUI(); err != nil {
			a.logError("Failed to launch GUI: %v", err)
		}
	}
}

// onQuitRequest handles quit request callback
func (a *CoreApp) onQuitRequest() {
	a.logInfo("Received quit request")

	// Notify all GUI clients to quit
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent("quit", nil)
	}

	// Send quit signal
	select {
	case a.quitChan <- true:
	default:
	}
}

// handleIPCRequest handles IPC requests
func (a *CoreApp) handleIPCRequest(req ipc.Request) ipc.Response {
	switch req.Type {
	// Device related
	case ipc.ReqConnect:
		success := a.ConnectDevice()
		return a.successResponse(success)

	case ipc.ReqDisconnect:
		a.DisconnectDevice()
		return a.successResponse(true)

	case ipc.ReqGetDeviceStatus:
		status := a.GetDeviceStatus()
		return a.dataResponse(status)

	case ipc.ReqGetCurrentFanData:
		data := a.deviceManager.GetCurrentFanData()
		return a.dataResponse(data)

	// Config related
	case ipc.ReqGetConfig:
		cfg := a.configManager.Get()
		return a.dataResponse(cfg)

	case ipc.ReqUpdateConfig:
		var cfg types.AppConfig
		if err := json.Unmarshal(req.Data, &cfg); err != nil {
			return a.errorResponse("Failed to parse config: " + err.Error())
		}
		if err := a.UpdateConfig(cfg); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqSetFanCurve:
		var curve []types.FanCurvePoint
		if err := json.Unmarshal(req.Data, &curve); err != nil {
			return a.errorResponse("Failed to parse fan curve: " + err.Error())
		}
		if err := a.SetFanCurve(curve); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqGetFanCurve:
		curve := a.configManager.Get().FanCurve
		return a.dataResponse(curve)

	case ipc.ReqGetFanCurveProfiles:
		return a.dataResponse(a.GetFanCurveProfiles())

	case ipc.ReqSetActiveFanCurveProfile:
		var params ipc.SetActiveFanCurveProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		profile, err := a.SetActiveFanCurveProfile(params.ID)
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(profile)

	case ipc.ReqSaveFanCurveProfile:
		var params ipc.SaveFanCurveProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		profile, err := a.SaveFanCurveProfile(params)
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(profile)

	case ipc.ReqDeleteFanCurveProfile:
		var params ipc.DeleteFanCurveProfileParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		if err := a.DeleteFanCurveProfile(params.ID); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqExportFanCurveProfiles:
		code, err := a.ExportFanCurveProfiles()
		if err != nil {
			return a.errorResponse(err.Error())
		}
		return a.dataResponse(code)

	case ipc.ReqImportFanCurveProfiles:
		var params ipc.ImportFanCurveProfilesParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		if err := a.ImportFanCurveProfiles(params.Code); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	// Control related
	case ipc.ReqSetAutoControl:
		var params ipc.SetAutoControlParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		if err := a.SetAutoControl(params.Enabled); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqSetManualGear:
		var params ipc.SetManualGearParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		success := a.SetManualGear(params.Gear, params.Level)
		return a.successResponse(success)

	case ipc.ReqGetAvailableGears:
		gears := types.GearCommands
		return a.dataResponse(gears)

	case ipc.ReqSetCustomSpeed:
		var params ipc.SetCustomSpeedParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		if err := a.SetCustomSpeed(params.Enabled, params.RPM); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqSetGearLight:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		success := a.SetGearLight(params.Enabled)
		return a.successResponse(success)

	case ipc.ReqSetPowerOnStart:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		success := a.SetPowerOnStart(params.Enabled)
		return a.successResponse(success)

	case ipc.ReqSetSmartStartStop:
		var params ipc.SetStringParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		success := a.SetSmartStartStop(params.Value)
		return a.successResponse(success)

	case ipc.ReqSetBrightness:
		var params ipc.SetIntParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		success := a.SetBrightness(params.Value)
		return a.successResponse(success)

	case ipc.ReqSetLightStrip:
		var params ipc.SetLightStripParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		if err := a.SetLightStrip(params.Config); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	// Temperature related
	case ipc.ReqGetTemperature:
		a.mutex.RLock()
		temp := a.currentTemp
		a.mutex.RUnlock()
		return a.dataResponse(temp)

	case ipc.ReqTestTemperatureReading:
		temp := a.tempReader.Read()
		return a.dataResponse(temp)

	case ipc.ReqTestBridgeProgram:
		data := a.bridgeManager.GetTemperature()
		return a.dataResponse(data)

	case ipc.ReqGetBridgeProgramStatus:
		status := a.bridgeManager.GetStatus()
		return a.dataResponse(status)

	// Auto-start related
	case ipc.ReqSetWindowsAutoStart:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		if err := a.SetWindowsAutoStart(params.Enabled); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqCheckWindowsAutoStart:
		enabled := a.autostartManager.CheckWindowsAutoStart()
		return a.dataResponse(enabled)

	case ipc.ReqIsRunningAsAdmin:
		isAdmin := a.autostartManager.IsRunningAsAdmin()
		return a.dataResponse(isAdmin)

	case ipc.ReqGetAutoStartMethod:
		method := a.autostartManager.GetAutoStartMethod()
		return a.dataResponse(method)

	case ipc.ReqSetAutoStartWithMethod:
		var params ipc.SetAutoStartWithMethodParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		if err := a.autostartManager.SetAutoStartWithMethod(params.Enable, params.Method); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	// Window related
	case ipc.ReqShowWindow:
		a.onShowWindowRequest()
		return a.successResponse(true)

	case ipc.ReqHideWindow:
		// GUI handles hiding itself
		return a.successResponse(true)

	case ipc.ReqQuitApp:
		a.safeGo("onQuitRequest", func() {
			a.onQuitRequest()
		})
		return a.successResponse(true)

	// Debug related
	case ipc.ReqGetDebugInfo:
		info := a.GetDebugInfo()
		return a.dataResponse(info)

	case ipc.ReqSetDebugMode:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("Failed to parse parameters: " + err.Error())
		}
		if err := a.SetDebugMode(params.Enabled); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqUpdateGuiResponseTime:
		atomic.StoreInt64(&a.guiLastResponse, time.Now().Unix())
		return a.successResponse(true)

	// System related
	case ipc.ReqPing:
		return a.dataResponse("pong")

	case ipc.ReqIsAutoStartLaunch:
		return a.dataResponse(a.isAutoStartLaunch)

	default:
		return a.errorResponse(fmt.Sprintf("Unknown request type: %s", req.Type))
	}
}

// Response helper methods
func (a *CoreApp) successResponse(success bool) ipc.Response {
	data, _ := json.Marshal(success)
	return ipc.Response{Success: true, Data: data}
}

func (a *CoreApp) errorResponse(errMsg string) ipc.Response {
	return ipc.Response{Success: false, Error: errMsg}
}

func (a *CoreApp) dataResponse(data any) ipc.Response {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return a.errorResponse("Failed to serialize data: " + err.Error())
	}
	return ipc.Response{Success: true, Data: dataBytes}
}

// onFanDataUpdate handles fan data update callback
func (a *CoreApp) onFanDataUpdate(fanData *types.FanData) {
	a.mutex.Lock()
	cfg := a.configManager.Get()

	// Check work mode changes
	// If "keep config on disconnect" mode is enabled, ignore device state changes to avoid false detection
	if fanData.WorkMode == "挡位工作模式" &&
		cfg.AutoControl &&
		a.lastDeviceMode == "自动模式(实时转速)" &&
		!a.userSetAutoControl &&
		!cfg.IgnoreDeviceOnReconnect {

		a.logInfo("Detected device switched from auto mode to gear mode, automatically disabling smart fan control")
		cfg.AutoControl = false

		a.configManager.Set(cfg)
		a.configManager.Save()

		// Broadcast config update
		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
		}
	} else if fanData.WorkMode == "挡位工作模式" &&
		cfg.AutoControl &&
		a.lastDeviceMode == "自动模式(实时转速)" &&
		!a.userSetAutoControl &&
		cfg.IgnoreDeviceOnReconnect {
		a.logInfo("Device mode change detected, but keep-config-on-disconnect mode is enabled, keeping app config unchanged")
	}

	a.lastDeviceMode = fanData.WorkMode

	if a.userSetAutoControl {
		a.userSetAutoControl = false
	}

	a.mutex.Unlock()

	// Broadcast fan data update
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventFanDataUpdate, fanData)
	}
}

// onDeviceDisconnect handles device disconnect callback
func (a *CoreApp) onDeviceDisconnect() {
	a.mutex.Lock()
	wasConnected := a.isConnected
	a.isConnected = false
	a.mutex.Unlock()

	if wasConnected {
		a.logInfo("Device disconnected, will attempt auto-reconnect during health check")
	}

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}

	// Start auto-reconnect mechanism
	a.safeGo("scheduleReconnect", func() {
		a.scheduleReconnect()
	})
}

// scheduleReconnect schedules device reconnection
func (a *CoreApp) scheduleReconnect() {
	// Delay before reconnect attempts to avoid frequent retries
	retryDelays := []time.Duration{
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
		30 * time.Second,
	}

	for i, delay := range retryDelays {
		// Check if already connected (may have reconnected via other means)
		a.mutex.RLock()
		connected := a.isConnected
		a.mutex.RUnlock()

		if connected {
			a.logInfo("Device reconnected, stopping reconnect attempts")
			return
		}

		a.logInfo("Waiting %v before reconnect attempt #%d...", delay, i+1)
		time.Sleep(delay)

		// Check connection status again
		a.mutex.RLock()
		connected = a.isConnected
		a.mutex.RUnlock()

		if connected {
			a.logInfo("Device reconnected, stopping reconnect attempts")
			return
		}

		a.logInfo("Attempting device reconnect #%d...", i+1)
		if a.ConnectDevice() {
			a.logInfo("Device reconnected successfully")

			// If keep-config-on-disconnect mode is enabled, reapply app config
			cfg := a.configManager.Get()
			if cfg.IgnoreDeviceOnReconnect {
				a.logInfo("Keep-config-on-disconnect mode enabled, reapplying app config")
				a.reapplyConfigAfterReconnect()
			}

			return
		}
		a.logError("Reconnect attempt #%d failed", i+1)
	}

	a.logError("All reconnect attempts failed, waiting for next health check")
}

// ConnectDevice connects to the device
func (a *CoreApp) ConnectDevice() bool {
	success, deviceInfo := a.deviceManager.Connect()
	if success {
		a.mutex.Lock()
		a.isConnected = true
		a.mutex.Unlock()

		if deviceInfo != nil && a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventDeviceConnected, deviceInfo)
		}

		if err := a.applyConfiguredLightStrip(); err != nil {
			a.logError("Failed to apply light strip config: %v", err)
		}
		a.safeGo("startTemperatureMonitoring@ConnectDevice", func() {
			a.startTemperatureMonitoring()
		})
	} else if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceError, "Connection failed")
	}
	return success
}

// DisconnectDevice disconnects from the device
func (a *CoreApp) DisconnectDevice() {
	a.mutex.Lock()
	a.isConnected = false
	a.mutex.Unlock()

	a.deviceManager.Disconnect()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}
}

// reapplyConfigAfterReconnect reapplies app config after device reconnection
func (a *CoreApp) reapplyConfigAfterReconnect() {
	cfg := a.configManager.Get()

	// Reapply smart fan control config
	if cfg.AutoControl {
		a.logInfo("Restarting smart fan control")
	} else if cfg.CustomSpeedEnabled {
		// Reapply custom speed
		a.logInfo("Reapplying custom speed: %d RPM", cfg.CustomSpeedRPM)
		if !a.deviceManager.SetCustomFanSpeed(cfg.CustomSpeedRPM) {
			a.logError("Failed to reapply custom speed")
		}
	}

	// Reapply gear light config
	if cfg.GearLight {
		a.logInfo("Re-enabling gear light")
		if !a.deviceManager.SetGearLight(true) {
			a.logError("Failed to re-enable gear light")
		}
	}

	// Reapply power-on auto-start config
	if cfg.PowerOnStart {
		a.logInfo("Re-enabling power-on auto-start")
		if !a.deviceManager.SetPowerOnStart(true) {
			a.logError("Failed to re-enable power-on auto-start")
		}
	}

	if err := a.applyConfiguredLightStrip(); err != nil {
		a.logError("Failed to reapply light strip config after reconnect: %v", err)
	}
}

// GetDeviceStatus gets device status
func (a *CoreApp) GetDeviceStatus() map[string]any {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	productID := a.deviceManager.GetProductID()
	productIDHex := ""
	if productID != 0 {
		productIDHex = fmt.Sprintf("0x%04X", productID)
	}

	model := ""
	switch productID {
	case device.ProductID1:
		model = "BS2PRO"
	case device.ProductID2:
		model = "BS2"
	}

	return map[string]any{
		"connected":   a.isConnected,
		"monitoring":  a.monitoringTemp,
		"currentData": a.deviceManager.GetCurrentFanData(),
		"temperature": a.currentTemp,
		"productId":   productIDHex,
		"model":       model,
	}
}

// UpdateConfig updates the configuration
func (a *CoreApp) UpdateConfig(cfg types.AppConfig) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	oldCfg := a.configManager.Get()
	if cfg.CurveProfileToggleHotkey == "" {
		cfg.CurveProfileToggleHotkey = oldCfg.CurveProfileToggleHotkey
	}
	if len(cfg.FanCurveProfiles) == 0 && len(oldCfg.FanCurveProfiles) > 0 {
		cfg.FanCurveProfiles = cloneFanCurveProfiles(oldCfg.FanCurveProfiles)
		cfg.ActiveFanCurveProfileID = oldCfg.ActiveFanCurveProfileID
	}
	cfg.ManualGearLevels = cloneManualGearLevels(oldCfg.ManualGearLevels)
	cfg.LightStrip, _ = normalizeLightStripConfig(cfg.LightStrip)
	normalizeCurveProfilesConfig(&cfg)
	if idx := findCurveProfileIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID); idx >= 0 {
		cfg.FanCurveProfiles[idx].Curve = cloneFanCurve(cfg.FanCurve)
	}
	cfg.SmartControl, _ = smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode)
	normalizeHotkeyConfig(&cfg)
	normalizeManualGearMemoryConfig(&cfg)

	cfg.ConfigPath = oldCfg.ConfigPath
	if err := a.configManager.Update(cfg); err != nil {
		return err
	}
	a.syncManualGearLevelMemoryLocked(cfg)
	a.applyHotkeyBindings(cfg)
	return nil
}

// SetFanCurve sets the fan curve
func (a *CoreApp) SetFanCurve(curve []types.FanCurvePoint) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	normalizeCurveProfilesConfig(&cfg)
	cfg.FanCurve = cloneFanCurve(curve)
	idx := findCurveProfileIndex(cfg.FanCurveProfiles, cfg.ActiveFanCurveProfileID)
	if idx >= 0 {
		cfg.FanCurveProfiles[idx].Curve = cloneFanCurve(cfg.FanCurve)
	}
	cfg.SmartControl, _ = smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode)
	return a.configManager.Update(cfg)
}

// SetAutoControl sets smart fan control
func (a *CoreApp) SetAutoControl(enabled bool) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()

	if enabled && cfg.CustomSpeedEnabled {
		return fmt.Errorf("cannot enable smart fan control while custom speed mode is active")
	}

	cfg.AutoControl = enabled

	if enabled {
		a.userSetAutoControl = true
	}

	if !enabled && a.isConnected {
		a.safeGo("applyCurrentGearSetting", func() {
			a.applyCurrentGearSetting()
		})
	}

	a.configManager.Set(cfg)
	err := a.configManager.Save()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return err
}

// applyCurrentGearSetting applies the current gear setting
func (a *CoreApp) applyCurrentGearSetting() {
	fanData := a.deviceManager.GetCurrentFanData()
	if fanData == nil {
		return
	}

	cfg := a.configManager.Get()
	setGear := fanData.SetGear
	if setGear == "" {
		setGear = cfg.ManualGear
	}
	level := a.getRememberedManualLevel(setGear, cfg.ManualLevel)

	a.logInfo("Applying current gear setting: %s %s", setGear, level)
	a.deviceManager.SetManualGear(setGear, level)
}

// SetManualGear sets the manual gear
func (a *CoreApp) SetManualGear(gear, level string) bool {
	cfg := a.configManager.Get()
	cfg.ManualGear = gear
	cfg.ManualLevel = level
	if cfg.ManualGearLevels == nil {
		cfg.ManualGearLevels = map[string]string{}
	}
	cfg.ManualGearLevels[gear] = normalizeManualLevel(level)
	a.configManager.Update(cfg)
	a.rememberManualGearLevel(gear, level)

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return a.deviceManager.SetManualGear(gear, level)
}

// SetCustomSpeed sets the custom fan speed
func (a *CoreApp) SetCustomSpeed(enabled bool, rpm int) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()

	if enabled {
		if cfg.AutoControl {
			cfg.AutoControl = false
		}

		cfg.CustomSpeedEnabled = true
		cfg.CustomSpeedRPM = rpm

		if a.isConnected {
			a.safeGo("setCustomFanSpeed", func() {
				a.deviceManager.SetCustomFanSpeed(rpm)
			})
		}
	} else {
		cfg.CustomSpeedEnabled = false
	}

	a.configManager.Set(cfg)
	err := a.configManager.Save()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return err
}

// SetGearLight sets the gear indicator light
func (a *CoreApp) SetGearLight(enabled bool) bool {
	if !a.deviceManager.SetGearLight(enabled) {
		return false
	}

	cfg := a.configManager.Get()
	cfg.GearLight = enabled
	a.configManager.Update(cfg)

	// Broadcast config update
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}

// SetPowerOnStart sets power-on auto-start
func (a *CoreApp) SetPowerOnStart(enabled bool) bool {
	if !a.deviceManager.SetPowerOnStart(enabled) {
		return false
	}

	cfg := a.configManager.Get()
	cfg.PowerOnStart = enabled
	a.configManager.Update(cfg)

	// Broadcast config update
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}

// SetSmartStartStop sets smart start/stop
func (a *CoreApp) SetSmartStartStop(mode string) bool {
	if !a.deviceManager.SetSmartStartStop(mode) {
		return false
	}

	cfg := a.configManager.Get()
	cfg.SmartStartStop = mode
	a.configManager.Update(cfg)

	// Broadcast config update
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}

// SetBrightness sets the brightness
func (a *CoreApp) SetBrightness(percentage int) bool {
	if !a.deviceManager.SetBrightness(percentage) {
		return false
	}

	cfg := a.configManager.Get()
	cfg.Brightness = percentage
	a.configManager.Update(cfg)

	// Broadcast config update
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}

// SetLightStrip sets the light strip configuration
func (a *CoreApp) SetLightStrip(lightCfg types.LightStripConfig) error {
	lightCfg, _ = normalizeLightStripConfig(lightCfg)

	cfg := a.configManager.Get()
	cfg.LightStrip = lightCfg
	a.configManager.Set(cfg)
	if err := a.configManager.Save(); err != nil {
		return err
	}

	if a.isConnected {
		if err := a.deviceManager.SetLightStrip(lightCfg); err != nil {
			return err
		}
	}

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return nil
}

func (a *CoreApp) applyConfiguredLightStrip() error {
	cfg := a.configManager.Get()
	lightCfg, changed := normalizeLightStripConfig(cfg.LightStrip)

	if changed {
		cfg.LightStrip = lightCfg
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("Failed to save default light strip config: %v", err)
		}
	}

	return a.deviceManager.SetLightStrip(lightCfg)
}

func normalizeLightStripConfig(cfg types.LightStripConfig) (types.LightStripConfig, bool) {
	defaults := types.GetDefaultLightStripConfig()
	changed := false

	if cfg.Mode == "" {
		cfg.Mode = defaults.Mode
		changed = true
	}
	if cfg.Speed == "" {
		cfg.Speed = defaults.Speed
		changed = true
	}
	if cfg.Brightness < 0 || cfg.Brightness > 100 {
		cfg.Brightness = defaults.Brightness
		changed = true
	}
	if len(cfg.Colors) == 0 {
		cfg.Colors = defaults.Colors
		changed = true
	}

	return cfg, changed
}

func normalizeHotkeyConfig(cfg *types.AppConfig) bool {
	if cfg == nil {
		return false
	}

	changed := false
	if cfg.ManualGearToggleHotkey == "" {
		cfg.ManualGearToggleHotkey = types.GetDefaultConfig(false).ManualGearToggleHotkey
		changed = true
	}
	if cfg.AutoControlToggleHotkey == "" {
		cfg.AutoControlToggleHotkey = types.GetDefaultConfig(false).AutoControlToggleHotkey
		changed = true
	}
	if cfg.CurveProfileToggleHotkey == "" {
		cfg.CurveProfileToggleHotkey = types.GetDefaultConfig(false).CurveProfileToggleHotkey
		changed = true
	}

	if _, _, err := hotkeysvc.ParseShortcut(cfg.ManualGearToggleHotkey); err != nil {
		cfg.ManualGearToggleHotkey = types.GetDefaultConfig(false).ManualGearToggleHotkey
		changed = true
	}
	if _, _, err := hotkeysvc.ParseShortcut(cfg.AutoControlToggleHotkey); err != nil {
		cfg.AutoControlToggleHotkey = types.GetDefaultConfig(false).AutoControlToggleHotkey
		changed = true
	}
	if _, _, err := hotkeysvc.ParseShortcut(cfg.CurveProfileToggleHotkey); err != nil {
		cfg.CurveProfileToggleHotkey = types.GetDefaultConfig(false).CurveProfileToggleHotkey
		changed = true
	}

	return changed
}

func normalizeManualGearMemoryConfig(cfg *types.AppConfig) bool {
	if cfg == nil {
		return false
	}

	changed := false
	if cfg.ManualGearLevels == nil {
		cfg.ManualGearLevels = map[string]string{}
		changed = true
	}

	for _, gear := range []string{"Silent", "Standard", "Performance", "Overclock"} {
		if level, ok := cfg.ManualGearLevels[gear]; !ok {
			cfg.ManualGearLevels[gear] = "Mid"
			changed = true
		} else {
			normalized := normalizeManualLevel(level)
			if normalized != level {
				cfg.ManualGearLevels[gear] = normalized
				changed = true
			}
		}
	}

	normalizedCurrent := normalizeManualLevel(cfg.ManualLevel)
	if normalizedCurrent != cfg.ManualLevel {
		cfg.ManualLevel = normalizedCurrent
		changed = true
	}

	if cfg.ManualGear != "" {
		if remembered, ok := cfg.ManualGearLevels[cfg.ManualGear]; !ok || remembered != normalizedCurrent {
			cfg.ManualGearLevels[cfg.ManualGear] = normalizedCurrent
			changed = true
		}
	}

	return changed
}

func (a *CoreApp) applyHotkeyBindings(cfg types.AppConfig) {
	if a.hotkeyManager == nil {
		return
	}
	if err := a.hotkeyManager.UpdateBindings(cfg.ManualGearToggleHotkey, cfg.AutoControlToggleHotkey, cfg.CurveProfileToggleHotkey); err != nil {
		a.logError("Failed to update global hotkeys: %v", err)
	}
}

func (a *CoreApp) handleHotkeyAction(action hotkeysvc.Action, shortcut string) {
	a.safeGo("handleHotkeyAction", func() {
		var message string
		success := true

		switch action {
		case hotkeysvc.ActionToggleManualGear:
			msg, err := a.toggleManualGearByHotkey()
			if err != nil {
				success = false
				message = err.Error()
			} else {
				message = msg
			}
		case hotkeysvc.ActionToggleAutoMode:
			msg, err := a.toggleAutoControlByHotkey()
			if err != nil {
				success = false
				message = err.Error()
			} else {
				message = msg
			}
		case hotkeysvc.ActionToggleCurveProfile:
			msg, err := a.toggleCurveProfileByHotkey()
			if err != nil {
				success = false
				message = err.Error()
			} else {
				message = msg
			}
		default:
			success = false
			message = "Unknown hotkey action"
		}

		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventHotkeyTriggered, map[string]any{
				"action":   string(action),
				"shortcut": shortcut,
				"success":  success,
				"message":  message,
			})
		}

		title := "BS2PRO Hotkey"
		if !success {
			title = "BS2PRO Hotkey Failed"
		}
		if a.notifier != nil {
			a.notifier.Notify(title, message)
		}
	})
}

func (a *CoreApp) toggleCurveProfileByHotkey() (string, error) {
	profile, err := a.CycleFanCurveProfile()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Fan curve switched to: %s", profile.Name), nil
}

func (a *CoreApp) toggleAutoControlByHotkey() (string, error) {
	cfg := a.configManager.Get()
	target := !cfg.AutoControl
	if err := a.SetAutoControl(target); err != nil {
		return "", err
	}
	if target {
		return "Smart fan control enabled", nil
	}
	return "Smart fan control disabled", nil
}

func (a *CoreApp) toggleManualGearByHotkey() (string, error) {
	cfg := a.configManager.Get()

	if cfg.AutoControl {
		if err := a.SetAutoControl(false); err != nil {
			return "", fmt.Errorf("failed to switch to manual mode: %w", err)
		}
	}

	nextGear, nextLevel := a.getNextManualGearWithMemory(cfg.ManualGear, cfg.ManualLevel)
	if ok := a.SetManualGear(nextGear, nextLevel); !ok {
		return "", fmt.Errorf("failed to apply manual gear")
	}

	rpm := getManualGearRPM(nextGear, nextLevel)
	if rpm > 0 {
		return fmt.Sprintf("Manual gear: %s %s (%d RPM)", nextGear, nextLevel, rpm), nil
	}
	return fmt.Sprintf("Manual gear: %s %s", nextGear, nextLevel), nil
}

func (a *CoreApp) getNextManualGearWithMemory(currentGear, currentLevel string) (string, string) {
	sequence := []string{"Silent", "Standard", "Performance", "Overclock"}
	nextIndex := 0

	for i, gear := range sequence {
		if gear == currentGear {
			nextIndex = (i + 1) % len(sequence)
			break
		}
	}

	a.rememberManualGearLevel(currentGear, currentLevel)
	fallbackLevel := normalizeManualLevel(currentLevel)
	level := a.getRememberedManualLevel(sequence[nextIndex], fallbackLevel)

	return sequence[nextIndex], level
}

func normalizeManualLevel(level string) string {
	if level == "Low" || level == "Mid" || level == "High" {
		return level
	}
	return "Mid"
}

func cloneManualGearLevels(source map[string]string) map[string]string {
	cloned := map[string]string{}
	for _, gear := range []string{"Silent", "Standard", "Performance", "Overclock"} {
		if source == nil {
			cloned[gear] = "Mid"
			continue
		}
		cloned[gear] = normalizeManualLevel(source[gear])
	}
	return cloned
}

func (a *CoreApp) syncManualGearLevelMemory(cfg types.AppConfig) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.syncManualGearLevelMemoryLocked(cfg)
}

func (a *CoreApp) syncManualGearLevelMemoryLocked(cfg types.AppConfig) {

	if a.manualGearLevelMemory == nil {
		a.manualGearLevelMemory = map[string]string{}
	}

	defaultLevel := normalizeManualLevel(cfg.ManualLevel)
	for _, gear := range []string{"Silent", "Standard", "Performance", "Overclock"} {
		if fromCfg, ok := cfg.ManualGearLevels[gear]; ok {
			a.manualGearLevelMemory[gear] = normalizeManualLevel(fromCfg)
			continue
		}
		a.manualGearLevelMemory[gear] = defaultLevel
	}

	a.manualGearLevelMemory[cfg.ManualGear] = normalizeManualLevel(cfg.ManualLevel)
}

func (a *CoreApp) rememberManualGearLevel(gear, level string) {
	if gear != "Silent" && gear != "Standard" && gear != "Performance" && gear != "Overclock" {
		return
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.manualGearLevelMemory == nil {
		a.manualGearLevelMemory = map[string]string{}
	}
	a.manualGearLevelMemory[gear] = normalizeManualLevel(level)
}

func (a *CoreApp) getRememberedManualLevel(gear, fallback string) string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	if a.manualGearLevelMemory == nil {
		return normalizeManualLevel(fallback)
	}
	if level, ok := a.manualGearLevelMemory[gear]; ok {
		return normalizeManualLevel(level)
	}
	return normalizeManualLevel(fallback)
}

func getManualGearRPM(gear, level string) int {
	commands, ok := types.GearCommands[gear]
	if !ok {
		return 0
	}

	for _, cmd := range commands {
		if (level == "Low" && containsLevel(cmd.Name, "Low")) ||
			(level == "Mid" && containsLevel(cmd.Name, "Mid")) ||
			(level == "High" && containsLevel(cmd.Name, "High")) {
			return cmd.RPM
		}
	}

	return 0
}

func containsLevel(name, level string) bool {
	return strings.Contains(name, level)
}

// SetWindowsAutoStart sets Windows auto-start
func (a *CoreApp) SetWindowsAutoStart(enable bool) error {
	err := a.autostartManager.SetWindowsAutoStart(enable)
	if err == nil {
		cfg := a.configManager.Get()
		cfg.WindowsAutoStart = enable
		a.configManager.Update(cfg)

		// Broadcast config update
		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
		}
	}
	return err
}

// GetDebugInfo gets debug information
func (a *CoreApp) GetDebugInfo() map[string]any {
	info := map[string]any{
		"debugMode":       a.debugMode,
		"trayReady":       a.trayManager.IsReady(),
		"trayInitialized": a.trayManager.IsInitialized(),
		"isConnected":     a.isConnected,
		"guiLastResponse": time.Unix(atomic.LoadInt64(&a.guiLastResponse), 0).Format("2006-01-02 15:04:05"),
		"monitoringTemp":  a.monitoringTemp,
		"autoStartLaunch": a.isAutoStartLaunch,
		"hasGUIClients":   a.ipcServer != nil && a.ipcServer.HasClients(),
	}
	return info
}

// SetDebugMode sets debug mode
func (a *CoreApp) SetDebugMode(enabled bool) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	cfg.DebugMode = enabled
	cfg.SmartControl, _ = smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, enabled)
	a.debugMode = enabled

	if a.logger != nil {
		a.logger.SetDebugMode(enabled)
		if enabled {
			a.logger.Info("Debug mode enabled, subsequent logs will include debug level")
		} else {
			a.logger.Info("Debug mode disabled, debug level logs will be suppressed")
		}
	}

	a.configManager.Set(cfg)
	if err := a.configManager.Save(); err != nil {
		return err
	}

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return nil
}

func (a *CoreApp) stopTemperatureMonitoring() {
	if !a.monitoringTemp {
		return
	}

	select {
	case a.stopMonitoring <- true:
	default:
	}
}

// startTemperatureMonitoring starts temperature monitoring
func (a *CoreApp) startTemperatureMonitoring() {
	if a.monitoringTemp {
		return
	}

	// Clear any residual stop signals to avoid the new monitoring loop being interrupted immediately.
	select {
	case <-a.stopMonitoring:
	default:
	}

	a.monitoringTemp = true

	// Note: Do not call EnterAutoMode here immediately, because temperature data (bridge program) may not be ready at startup.
	// If we switch to software control mode before temperature is successfully read, the device won't receive speed commands, causing the fan to stop.
	// EnterAutoMode and speed settings will be handled internally by SetFanSpeed after the first successful temperature read.

	cfg := a.configManager.Get()
	updateInterval := time.Duration(cfg.TempUpdateRate) * time.Second

	// Temperature sample buffer
	sampleCount := max(cfg.TempSampleCount, 1)
	tempSamples := make([]int, 0, sampleCount)
	recentAvgTemps := make([]int, 0, 24)
	recentControlTemps := make([]int, 0, 24)
	initialTemp := a.tempReader.Read()
	lastControlTemp := initialTemp.MaxTemp
	lastTargetRPM := -1
	learningDirty := false
	lastLearningSave := time.Now()

	for a.monitoringTemp {
		select {
		case <-a.stopMonitoring:
			a.monitoringTemp = false
			return
		case <-time.After(updateInterval):
			temp := a.tempReader.Read()

			a.mutex.Lock()
			a.currentTemp = temp
			a.mutex.Unlock()

			// Broadcast temperature update
			if a.ipcServer != nil {
				a.ipcServer.BroadcastEvent(ipc.EventTemperatureUpdate, temp)
			}

			cfg := a.configManager.Get()
			smartCfg, smartChanged := smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve, cfg.DebugMode)
			if smartChanged {
				cfg.SmartControl = smartCfg
				a.configManager.Set(cfg)
				if err := a.configManager.Save(); err != nil {
					a.logError("Failed to save smart control config: %v", err)
				}
			}

			if cfg.AutoControl && temp.MaxTemp > 0 {
				// Update sample config
				newSampleCount := max(cfg.TempSampleCount, 1)
				if newSampleCount != sampleCount {
					sampleCount = newSampleCount
					tempSamples = make([]int, 0, sampleCount)
				}

				// Add new sample
				tempSamples = append(tempSamples, temp.MaxTemp)
				if len(tempSamples) > sampleCount {
					tempSamples = tempSamples[len(tempSamples)-sampleCount:]
				}

				// Calculate average temperature
				avgTemp := 0
				for _, t := range tempSamples {
					avgTemp += t
				}
				avgTemp = avgTemp / len(tempSamples)

				maxHistory := max(8, smartCfg.LearnWindow+smartCfg.LearnDelay+4)
				recentAvgTemps = append(recentAvgTemps, avgTemp)
				if len(recentAvgTemps) > maxHistory {
					recentAvgTemps = recentAvgTemps[len(recentAvgTemps)-maxHistory:]
				}

				controlTemp, spikeSuppressed := smartcontrol.FilterTransientSpike(avgTemp, recentAvgTemps, smartCfg.TargetTemp, smartCfg.Hysteresis)
				recentControlTemps = append(recentControlTemps, controlTemp)
				if len(recentControlTemps) > maxHistory {
					recentControlTemps = recentControlTemps[len(recentControlTemps)-maxHistory:]
				}

				curveMinRPM, curveMaxRPM := smartcontrol.GetCurveRPMBounds(cfg.FanCurve)

				baseRPM := temperature.CalculateTargetRPM(controlTemp, cfg.FanCurve)
				targetRPM := baseRPM
				prevTargetRPM := lastTargetRPM

				if cfg.DebugMode && smartCfg.Enabled {
					targetRPM = smartcontrol.CalculateTargetRPM(controlTemp, lastControlTemp, cfg.FanCurve, smartCfg)
				}

				if targetRPM > 0 {
					targetRPM = min(max(targetRPM, curveMinRPM), curveMaxRPM)
				}

				if prevTargetRPM >= 0 {
					targetRPM = smartcontrol.ApplyRampLimit(targetRPM, prevTargetRPM, smartCfg.RampUpLimit, smartCfg.RampDownLimit)
					if targetRPM > 0 {
						targetRPM = min(max(targetRPM, curveMinRPM), curveMaxRPM)
					}
				}

				deltaRPM := targetRPM - prevTargetRPM
				if deltaRPM < 0 {
					deltaRPM = -deltaRPM
				}

				if targetRPM >= 0 && (prevTargetRPM < 0 || deltaRPM >= smartCfg.MinRPMChange || (targetRPM == 0 && prevTargetRPM > 0)) {
					a.deviceManager.SetFanSpeed(targetRPM)
					lastTargetRPM = targetRPM
				}

				if cfg.DebugMode && smartCfg.Enabled && !spikeSuppressed {
					updatedHeatOffsets, updatedCoolOffsets, updatedRateHeat, updatedRateCool, changed := smartcontrol.LearnCurveOffsets(
						controlTemp,
						lastControlTemp,
						targetRPM,
						prevTargetRPM,
						recentControlTemps,
						cfg.FanCurve,
						smartCfg,
					)
					if changed {
						smartCfg.LearnedOffsetsHeat = updatedHeatOffsets
						smartCfg.LearnedOffsetsCool = updatedCoolOffsets
						smartCfg.LearnedRateHeat = updatedRateHeat
						smartCfg.LearnedRateCool = updatedRateCool
						smartCfg.LearnedOffsets = smartcontrol.BlendOffsets(updatedHeatOffsets, updatedCoolOffsets)
						cfg.SmartControl = smartCfg
						a.configManager.Set(cfg)
						learningDirty = true
					}

					if learningDirty && time.Since(lastLearningSave) >= 25*time.Second {
						if err := a.configManager.Save(); err != nil {
							a.logError("Failed to save learned curve: %v", err)
						} else {
							lastLearningSave = time.Now()
							learningDirty = false
							if a.ipcServer != nil {
								a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
							}
						}
					}
				}

				if baseRPM > 0 {
					a.logDebug("Smart control: temp=%d°C avg=%d°C control_temp=%d°C base=%dRPM target=%dRPM", temp.MaxTemp, avgTemp, controlTemp, baseRPM, targetRPM)
				}

				lastControlTemp = controlTemp
			}
		}
	}

	if learningDirty {
		if err := a.configManager.Save(); err != nil {
			a.logError("Failed to save learned curve when exiting monitoring: %v", err)
		}
	}
}

// startHealthMonitoring starts health monitoring
func (a *CoreApp) startHealthMonitoring() {
	a.logInfo("Starting health monitoring system")

	a.healthCheckTicker = time.NewTicker(30 * time.Second)

	a.safeGo("healthMonitoringLoop", func() {
		defer a.healthCheckTicker.Stop()

		for {
			select {
			case <-a.healthCheckTicker.C:
				a.performHealthCheck()
			case <-a.cleanupChan:
				a.logInfo("Health monitoring system stopped")
				return
			}
		}
	})

	if a.logger != nil {
		a.safeGo("cleanOldLogs", func() {
			a.logger.CleanOldLogs()
		})
	}
}

// performHealthCheck performs a health check
func (a *CoreApp) performHealthCheck() {
	defer func() {
		if r := recover(); r != nil {
			a.logError("Panic during health check: %v", r)
		}
	}()

	a.trayManager.CheckHealth()
	a.checkDeviceHealth()

	a.logDebug("Health check completed - tray:%v device_connected:%v",
		a.trayManager.IsInitialized(), a.isConnected)
}

// checkDeviceHealth checks device health status
func (a *CoreApp) checkDeviceHealth() {
	a.mutex.RLock()
	connected := a.isConnected
	a.mutex.RUnlock()

	if !connected {
		a.logInfo("Health check: device not connected, attempting reconnect")
		a.safeGo("healthReconnect", func() {
			if a.ConnectDevice() {
				a.logInfo("Health check: device reconnected successfully")
			} else {
				a.logDebug("Health check: device reconnect failed, waiting for next check")
			}
		})
	} else {
		// Verify actual device connection status
		if !a.deviceManager.IsConnected() {
			a.logError("Health check: detected inconsistent device status, triggering disconnect callback")
			a.onDeviceDisconnect()
		}
	}
}

// cleanup cleans up resources
func (a *CoreApp) cleanup() {
	if a.healthCheckTicker != nil {
		a.healthCheckTicker.Stop()
	}

	select {
	case a.cleanupChan <- true:
	default:
	}

	if a.logger != nil {
		a.logger.Info("Core service exiting, cleaning up resources")
		a.logger.Close()
	}
}

// Logging helper methods
func (a *CoreApp) logInfo(format string, v ...any) {
	if a.logger != nil {
		a.logger.Info(format, v...)
	}
}

func (a *CoreApp) logError(format string, v ...any) {
	if a.logger != nil {
		a.logger.Error(format, v...)
	}
}

func (a *CoreApp) logDebug(format string, v ...any) {
	if a.logger != nil {
		a.logger.Debug(format, v...)
	}
}

func (a *CoreApp) safeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				capturePanic(a, "goroutine:"+name, r)
			}
		}()

		fn()
	}()
}

// launchGUI launches the GUI program
func launchGUI() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	exeDir := filepath.Dir(exePath)
	guiPath := filepath.Join(exeDir, "BS2PRO-Controller.exe")

	if _, err := os.Stat(guiPath); os.IsNotExist(err) {
		guiPath = filepath.Join(exeDir, "..", "BS2PRO-Controller.exe")
		if _, err := os.Stat(guiPath); os.IsNotExist(err) {
			return fmt.Errorf("GUI program not found: %s", guiPath)
		}
	}

	cmd := exec.Command(guiPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: false,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch GUI program: %v", err)
	}

	// Use fmt instead of logging system to avoid circular dependency
	fmt.Printf("GUI program launched, PID: %d\n", cmd.Process.Pid)

	go func() {
		cmd.Wait()
	}()

	return nil
}
