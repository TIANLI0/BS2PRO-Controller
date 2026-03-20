package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/TIANLI0/BS2PRO-Controller/internal/ipc"
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
	"github.com/TIANLI0/BS2PRO-Controller/internal/version"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

// App struct - GUI application structure
type App struct {
	ctx       context.Context
	ipcClient *ipc.Client
	mutex     sync.RWMutex

	// Cached state
	isConnected bool
	currentTemp types.TemperatureData
}

// Re-export types for frontend API compatibility
type (
	FanCurvePoint           = types.FanCurvePoint
	FanCurveProfile         = types.FanCurveProfile
	FanCurveProfilesPayload = types.FanCurveProfilesPayload
	FanData                 = types.FanData
	GearCommand             = types.GearCommand
	TemperatureData         = types.TemperatureData
	BridgeTemperatureData   = types.BridgeTemperatureData
	AppConfig               = types.AppConfig
)

var guiLogger *zap.SugaredLogger

func init() {
	logger, _ := zap.NewProduction()
	guiLogger = logger.Sugar()
}

// NewApp creates a GUI application instance
func NewApp() *App {
	return &App{
		ipcClient:   ipc.NewClient(nil),
		currentTemp: types.TemperatureData{BridgeOk: true},
	}
}

// startup called when the application starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	guiLogger.Info("=== BS2PRO GUI starting ===")

	// Connect to core service
	if err := a.ipcClient.Connect(); err != nil {
		guiLogger.Errorf("Failed to connect to core service: %v", err)
		runtime.EventsEmit(ctx, "core-service-error", "Unable to connect to core service")
	} else {
		guiLogger.Info("Connected to core service")

		// Set event handler
		a.ipcClient.SetEventHandler(a.handleCoreEvent)
	}

	guiLogger.Info("=== BS2PRO GUI startup complete ===")
}

// GetAppVersion returns the application version (from version module)
func (a *App) GetAppVersion() string {
	return version.Get()
}

// handleCoreEvent handles events pushed by the core service
func (a *App) handleCoreEvent(event ipc.Event) {
	if a.ctx == nil {
		return
	}

	switch event.Type {
	case ipc.EventFanDataUpdate:
		var fanData types.FanData
		if err := json.Unmarshal(event.Data, &fanData); err == nil {
			runtime.EventsEmit(a.ctx, "fan-data-update", fanData)
		}

	case ipc.EventTemperatureUpdate:
		var temp types.TemperatureData
		if err := json.Unmarshal(event.Data, &temp); err == nil {
			a.mutex.Lock()
			a.currentTemp = temp
			a.mutex.Unlock()
			runtime.EventsEmit(a.ctx, "temperature-update", temp)
		}

	case ipc.EventDeviceConnected:
		var deviceInfo map[string]string
		json.Unmarshal(event.Data, &deviceInfo)
		a.mutex.Lock()
		a.isConnected = true
		a.mutex.Unlock()
		runtime.EventsEmit(a.ctx, "device-connected", deviceInfo)

	case ipc.EventDeviceDisconnected:
		a.mutex.Lock()
		a.isConnected = false
		a.mutex.Unlock()
		runtime.EventsEmit(a.ctx, "device-disconnected", nil)

	case ipc.EventDeviceError:
		var errMsg string
		json.Unmarshal(event.Data, &errMsg)
		runtime.EventsEmit(a.ctx, "device-error", errMsg)

	case ipc.EventConfigUpdate:
		var cfg types.AppConfig
		if err := json.Unmarshal(event.Data, &cfg); err == nil {
			runtime.EventsEmit(a.ctx, "config-update", cfg)
		}

	case ipc.EventHotkeyTriggered:
		var payload map[string]any
		if err := json.Unmarshal(event.Data, &payload); err == nil {
			runtime.EventsEmit(a.ctx, "hotkey-triggered", payload)
		}

	case ipc.EventHealthPing:
		var timestamp int64
		json.Unmarshal(event.Data, &timestamp)
		runtime.EventsEmit(a.ctx, "health-ping", timestamp)

	case ipc.EventHeartbeat:
		var timestamp int64
		json.Unmarshal(event.Data, &timestamp)
		runtime.EventsEmit(a.ctx, "heartbeat", timestamp)

	case "show-window":
		a.ShowWindow()

	case "quit":
		a.QuitApp()
	}
}

// sendRequest sends a request to the core service
func (a *App) sendRequest(reqType ipc.RequestType, data any) (*ipc.Response, error) {
	if !a.ipcClient.IsConnected() {
		// Try to reconnect
		if err := a.ipcClient.Connect(); err != nil {
			return nil, fmt.Errorf("not connected to core service: %v", err)
		}
	}
	return a.ipcClient.SendRequest(reqType, data)
}

// === Frontend API Methods ===
// All public methods below maintain full compatibility with the original app.go

// ConnectDevice connects to the HID device
func (a *App) ConnectDevice() bool {
	resp, err := a.sendRequest(ipc.ReqConnect, nil)
	if err != nil {
		guiLogger.Errorf("Connect device request failed: %v", err)
		return false
	}
	if !resp.Success {
		guiLogger.Errorf("Failed to connect device: %s", resp.Error)
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// DisconnectDevice disconnects the device
func (a *App) DisconnectDevice() {
	a.sendRequest(ipc.ReqDisconnect, nil)
}

// GetDeviceStatus gets the device connection status
func (a *App) GetDeviceStatus() map[string]any {
	resp, err := a.sendRequest(ipc.ReqGetDeviceStatus, nil)
	if err != nil {
		return map[string]any{"connected": false, "error": err.Error()}
	}
	if !resp.Success {
		return map[string]any{"connected": false, "error": resp.Error}
	}
	var status map[string]any
	json.Unmarshal(resp.Data, &status)
	return status
}

// GetConfig gets the current configuration
func (a *App) GetConfig() AppConfig {
	resp, err := a.sendRequest(ipc.ReqGetConfig, nil)
	if err != nil {
		guiLogger.Errorf("Failed to get config: %v", err)
		return types.GetDefaultConfig(false)
	}
	if !resp.Success {
		guiLogger.Errorf("Failed to get config: %s", resp.Error)
		return types.GetDefaultConfig(false)
	}
	var cfg AppConfig
	json.Unmarshal(resp.Data, &cfg)
	return cfg
}

// UpdateConfig updates the configuration
func (a *App) UpdateConfig(cfg AppConfig) error {
	resp, err := a.sendRequest(ipc.ReqUpdateConfig, cfg)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SetFanCurve sets the fan curve
func (a *App) SetFanCurve(curve []FanCurvePoint) error {
	resp, err := a.sendRequest(ipc.ReqSetFanCurve, curve)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// GetFanCurve gets the fan curve
func (a *App) GetFanCurve() []FanCurvePoint {
	resp, err := a.sendRequest(ipc.ReqGetFanCurve, nil)
	if err != nil {
		return types.GetDefaultFanCurve()
	}
	if !resp.Success {
		return types.GetDefaultFanCurve()
	}
	var curve []FanCurvePoint
	json.Unmarshal(resp.Data, &curve)
	return curve
}

// GetFanCurveProfiles gets the list of curve profiles
func (a *App) GetFanCurveProfiles() FanCurveProfilesPayload {
	resp, err := a.sendRequest(ipc.ReqGetFanCurveProfiles, nil)
	if err != nil || !resp.Success {
		cfg := a.GetConfig()
		return types.FanCurveProfilesPayload{
			Profiles: cfg.FanCurveProfiles,
			ActiveID: cfg.ActiveFanCurveProfileID,
		}
	}
	var payload FanCurveProfilesPayload
	json.Unmarshal(resp.Data, &payload)
	return payload
}

// SetActiveFanCurveProfile sets the currently active curve profile
func (a *App) SetActiveFanCurveProfile(profileID string) error {
	resp, err := a.sendRequest(ipc.ReqSetActiveFanCurveProfile, ipc.SetActiveFanCurveProfileParams{ID: profileID})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SaveFanCurveProfile saves a curve profile
func (a *App) SaveFanCurveProfile(profileID, name string, curve []FanCurvePoint, setActive bool) (FanCurveProfile, error) {
	resp, err := a.sendRequest(ipc.ReqSaveFanCurveProfile, ipc.SaveFanCurveProfileParams{
		ID:        profileID,
		Name:      name,
		Curve:     curve,
		SetActive: setActive,
	})
	if err != nil {
		return FanCurveProfile{}, err
	}
	if !resp.Success {
		return FanCurveProfile{}, fmt.Errorf("%s", resp.Error)
	}
	var profile FanCurveProfile
	json.Unmarshal(resp.Data, &profile)
	return profile, nil
}

// DeleteFanCurveProfile deletes a curve profile
func (a *App) DeleteFanCurveProfile(profileID string) error {
	resp, err := a.sendRequest(ipc.ReqDeleteFanCurveProfile, ipc.DeleteFanCurveProfileParams{ID: profileID})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// ExportFanCurveProfiles exports curve profiles
func (a *App) ExportFanCurveProfiles() (string, error) {
	resp, err := a.sendRequest(ipc.ReqExportFanCurveProfiles, nil)
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("%s", resp.Error)
	}
	var code string
	json.Unmarshal(resp.Data, &code)
	return code, nil
}

// ImportFanCurveProfiles imports curve profiles
func (a *App) ImportFanCurveProfiles(code string) error {
	resp, err := a.sendRequest(ipc.ReqImportFanCurveProfiles, ipc.ImportFanCurveProfilesParams{Code: code})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SetAutoControl sets smart fan control
func (a *App) SetAutoControl(enabled bool) error {
	resp, err := a.sendRequest(ipc.ReqSetAutoControl, ipc.SetAutoControlParams{Enabled: enabled})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SetManualGear sets the manual gear
func (a *App) SetManualGear(gear, level string) bool {
	resp, err := a.sendRequest(ipc.ReqSetManualGear, ipc.SetManualGearParams{Gear: gear, Level: level})
	if err != nil {
		return false
	}
	if !resp.Success {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// GetAvailableGears gets available gears
func (a *App) GetAvailableGears() map[string][]GearCommand {
	resp, err := a.sendRequest(ipc.ReqGetAvailableGears, nil)
	if err != nil {
		return types.GearCommands
	}
	if !resp.Success {
		return types.GearCommands
	}
	var gears map[string][]GearCommand
	json.Unmarshal(resp.Data, &gears)
	return gears
}

// ManualSetFanSpeed deprecated method
func (a *App) ManualSetFanSpeed(rpm int) bool {
	guiLogger.Warn("ManualSetFanSpeed is deprecated, use SetManualGear instead")
	return false
}

// SetCustomSpeed sets custom fan speed
func (a *App) SetCustomSpeed(enabled bool, rpm int) error {
	resp, err := a.sendRequest(ipc.ReqSetCustomSpeed, ipc.SetCustomSpeedParams{Enabled: enabled, RPM: rpm})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// SetGearLight sets the gear indicator light
func (a *App) SetGearLight(enabled bool) bool {
	resp, err := a.sendRequest(ipc.ReqSetGearLight, ipc.SetBoolParams{Enabled: enabled})
	if err != nil {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// SetPowerOnStart sets power-on auto-start
func (a *App) SetPowerOnStart(enabled bool) bool {
	resp, err := a.sendRequest(ipc.ReqSetPowerOnStart, ipc.SetBoolParams{Enabled: enabled})
	if err != nil {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// SetSmartStartStop sets smart start/stop
func (a *App) SetSmartStartStop(mode string) bool {
	resp, err := a.sendRequest(ipc.ReqSetSmartStartStop, ipc.SetStringParams{Value: mode})
	if err != nil {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// SetBrightness sets the brightness
func (a *App) SetBrightness(percentage int) bool {
	resp, err := a.sendRequest(ipc.ReqSetBrightness, ipc.SetIntParams{Value: percentage})
	if err != nil {
		return false
	}
	var success bool
	json.Unmarshal(resp.Data, &success)
	return success
}

// SetLightStrip sets the light strip
func (a *App) SetLightStrip(cfg types.LightStripConfig) error {
	resp, err := a.sendRequest(ipc.ReqSetLightStrip, ipc.SetLightStripParams{Config: cfg})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// GetTemperature gets the current temperature
func (a *App) GetTemperature() TemperatureData {
	resp, err := a.sendRequest(ipc.ReqGetTemperature, nil)
	if err != nil {
		a.mutex.RLock()
		defer a.mutex.RUnlock()
		return a.currentTemp
	}
	var temp TemperatureData
	json.Unmarshal(resp.Data, &temp)
	return temp
}

// GetCurrentFanData gets the current fan data
func (a *App) GetCurrentFanData() *FanData {
	resp, err := a.sendRequest(ipc.ReqGetCurrentFanData, nil)
	if err != nil {
		return nil
	}
	var fanData FanData
	if err := json.Unmarshal(resp.Data, &fanData); err != nil {
		return nil
	}
	return &fanData
}

// TestTemperatureReading tests temperature reading
func (a *App) TestTemperatureReading() TemperatureData {
	resp, err := a.sendRequest(ipc.ReqTestTemperatureReading, nil)
	if err != nil {
		return TemperatureData{}
	}
	var temp TemperatureData
	json.Unmarshal(resp.Data, &temp)
	return temp
}

// TestBridgeProgram tests the bridge program
func (a *App) TestBridgeProgram() BridgeTemperatureData {
	resp, err := a.sendRequest(ipc.ReqTestBridgeProgram, nil)
	if err != nil {
		return BridgeTemperatureData{Success: false, Error: err.Error()}
	}
	var data BridgeTemperatureData
	json.Unmarshal(resp.Data, &data)
	return data
}

// GetBridgeProgramStatus gets the bridge program status
func (a *App) GetBridgeProgramStatus() map[string]any {
	resp, err := a.sendRequest(ipc.ReqGetBridgeProgramStatus, nil)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var status map[string]any
	json.Unmarshal(resp.Data, &status)
	return status
}

// SetWindowsAutoStart sets Windows startup auto-launch
func (a *App) SetWindowsAutoStart(enable bool) error {
	resp, err := a.sendRequest(ipc.ReqSetWindowsAutoStart, ipc.SetBoolParams{Enabled: enable})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// IsRunningAsAdmin checks if running with administrator privileges
func (a *App) IsRunningAsAdmin() bool {
	resp, err := a.sendRequest(ipc.ReqIsRunningAsAdmin, nil)
	if err != nil {
		return false
	}
	var isAdmin bool
	json.Unmarshal(resp.Data, &isAdmin)
	return isAdmin
}

// GetAutoStartMethod gets the current auto-start method
func (a *App) GetAutoStartMethod() string {
	resp, err := a.sendRequest(ipc.ReqGetAutoStartMethod, nil)
	if err != nil {
		return "none"
	}
	var method string
	json.Unmarshal(resp.Data, &method)
	return method
}

// SetAutoStartWithMethod sets auto-start using the specified method
func (a *App) SetAutoStartWithMethod(enable bool, method string) error {
	resp, err := a.sendRequest(ipc.ReqSetAutoStartWithMethod, ipc.SetAutoStartWithMethodParams{Enable: enable, Method: method})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// CheckWindowsAutoStart checks Windows startup auto-launch status
func (a *App) CheckWindowsAutoStart() bool {
	resp, err := a.sendRequest(ipc.ReqCheckWindowsAutoStart, nil)
	if err != nil {
		return false
	}
	var enabled bool
	json.Unmarshal(resp.Data, &enabled)
	return enabled
}

// IsAutoStartLaunch returns whether the current launch is an auto-start launch
func (a *App) IsAutoStartLaunch() bool {
	resp, err := a.sendRequest(ipc.ReqIsAutoStartLaunch, nil)
	if err != nil {
		return false
	}
	var isAutoStart bool
	json.Unmarshal(resp.Data, &isAutoStart)
	return isAutoStart
}

// ShowWindow shows the main window
func (a *App) ShowWindow() {
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
		runtime.WindowSetAlwaysOnTop(a.ctx, false)
	}
}

// HideWindow hides the main window to tray
func (a *App) HideWindow() {
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}
}

// QuitApp fully quits the application
func (a *App) QuitApp() {
	guiLogger.Info("GUI requesting quit")

	// Close IPC connection
	if a.ipcClient != nil {
		a.ipcClient.Close()
	}

	// Quit GUI
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// QuitAll fully quits the application (including core service)
func (a *App) QuitAll() {
	guiLogger.Info("GUI requesting full quit (including core service)")

	// Notify core service to quit
	a.sendRequest(ipc.ReqQuitApp, nil)

	// Close IPC connection
	if a.ipcClient != nil {
		a.ipcClient.Close()
	}

	// Quit GUI
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// OnWindowClosing handles the window close event
func (a *App) OnWindowClosing(ctx context.Context) bool {
	// Return false to allow normal window close and quit GUI
	// Core service will continue running in the background
	return false
}

// InitSystemTray initializes the system tray (kept for API compatibility, actually handled by core service)
func (a *App) InitSystemTray() {
	// Tray is managed by core service, GUI does not need to handle it
}

// UpdateGuiResponseTime updates the GUI response time (called by frontend)
func (a *App) UpdateGuiResponseTime() {
	a.sendRequest(ipc.ReqUpdateGuiResponseTime, nil)
}

// GetDebugInfo gets debug information
func (a *App) GetDebugInfo() map[string]any {
	resp, err := a.sendRequest(ipc.ReqGetDebugInfo, nil)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var info map[string]any
	json.Unmarshal(resp.Data, &info)
	return info
}

// SetDebugMode sets the debug mode
func (a *App) SetDebugMode(enabled bool) error {
	resp, err := a.sendRequest(ipc.ReqSetDebugMode, ipc.SetBoolParams{Enabled: enabled})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}
