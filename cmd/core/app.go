package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/TIANLI0/BS2PRO-Controller/internal/autostart"
	"github.com/TIANLI0/BS2PRO-Controller/internal/bridge"
	"github.com/TIANLI0/BS2PRO-Controller/internal/config"
	"github.com/TIANLI0/BS2PRO-Controller/internal/device"
	"github.com/TIANLI0/BS2PRO-Controller/internal/ipc"
	"github.com/TIANLI0/BS2PRO-Controller/internal/logger"
	"github.com/TIANLI0/BS2PRO-Controller/internal/smartcontrol"
	"github.com/TIANLI0/BS2PRO-Controller/internal/temperature"
	"github.com/TIANLI0/BS2PRO-Controller/internal/tray"
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
	"github.com/TIANLI0/BS2PRO-Controller/internal/version"
)

//go:embed icon.ico
var iconData []byte

// CoreApp 核心应用结构
type CoreApp struct {
	ctx context.Context

	// 管理器
	deviceManager    *device.Manager
	bridgeManager    *bridge.Manager
	tempReader       *temperature.Reader
	configManager    *config.Manager
	trayManager      *tray.Manager
	autostartManager *autostart.Manager
	logger           *logger.CustomLogger
	ipcServer        *ipc.Server

	// 状态
	isConnected        bool
	monitoringTemp     bool
	currentTemp        types.TemperatureData
	lastDeviceMode     string
	userSetAutoControl bool
	isAutoStartLaunch  bool
	debugMode          bool

	// 监控相关
	guiLastResponse   int64
	guiMonitorEnabled bool
	healthCheckTicker *time.Ticker
	cleanupChan       chan bool
	quitChan          chan bool

	// 同步
	mutex          sync.RWMutex
	stopMonitoring chan bool
}

// NewCoreApp 创建核心应用实例
func NewCoreApp(debugMode, isAutoStart bool) *CoreApp {
	// 初始化日志系统
	installDir := config.GetInstallDir()
	customLogger, err := logger.NewCustomLogger(debugMode, installDir)
	if err != nil {
		// 如果初始化失败，无法记录，直接退出
		panic(fmt.Sprintf("初始化日志系统失败: %v", err))
	} else {
		customLogger.Info("核心服务启动")
		customLogger.Info("安装目录: %s", installDir)
		customLogger.Info("调试模式: %v", debugMode)
		customLogger.Info("自启动模式: %v", isAutoStart)
		customLogger.CleanOldLogs()
	}

	// 创建管理器
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
	}

	return app
}

// Start 启动核心服务
func (a *CoreApp) Start() error {
	a.logInfo("=== BS2PRO 核心服务启动 ===")
	a.logInfo("版本: %s", version.Get())
	a.logInfo("安装目录: %s", config.GetInstallDir())
	a.logInfo("调试模式: %v", a.debugMode)
	a.logInfo("当前工作目录: %s", config.GetCurrentWorkingDir())

	// 检测是否为自启动
	a.isAutoStartLaunch = autostart.DetectAutoStartLaunch(os.Args)
	a.logInfo("自启动模式: %v", a.isAutoStartLaunch)

	// 加载配置
	a.logInfo("开始加载配置文件")
	cfg := a.configManager.Load(a.isAutoStartLaunch)
	if normalizedLight, changed := normalizeLightStripConfig(cfg.LightStrip); changed {
		cfg.LightStrip = normalizedLight
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("保存灯带默认配置失败: %v", err)
		}
	}
	if normalizedSmart, changed := smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve); changed {
		cfg.SmartControl = normalizedSmart
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("保存智能控温默认配置失败: %v", err)
		}
	}
	a.logInfo("配置加载完成，配置路径: %s", cfg.ConfigPath)

	// 同步调试模式配置
	if cfg.DebugMode {
		a.debugMode = true
		if a.logger != nil {
			a.logger.SetDebugMode(true)
		}
		a.logInfo("从配置文件同步调试模式: 启用")
	}

	// 检查并同步Windows自启动状态
	a.logInfo("检查Windows自启动状态")
	actualAutoStart := a.autostartManager.CheckWindowsAutoStart()
	if actualAutoStart != cfg.WindowsAutoStart {
		cfg.WindowsAutoStart = actualAutoStart
		a.configManager.Set(cfg)
		if err := a.configManager.Save(); err != nil {
			a.logError("同步Windows自启动状态时保存配置失败: %v", err)
		} else {
			a.logInfo("已同步Windows自启动状态: %v", actualAutoStart)
		}
	}

	// 初始化HID
	a.logInfo("初始化HID库")
	if err := a.deviceManager.Init(); err != nil {
		a.logError("初始化HID库失败: %v", err)
		return err
	}
	a.logInfo("HID库初始化成功")

	// 设置设备回调
	a.deviceManager.SetCallbacks(a.onFanDataUpdate, a.onDeviceDisconnect)

	// 启动 IPC 服务器
	a.logInfo("启动 IPC 服务器")
	a.ipcServer = ipc.NewServer(a.handleIPCRequest, a.logger)
	if err := a.ipcServer.Start(); err != nil {
		a.logError("启动 IPC 服务器失败: %v", err)
		return err
	}

	// 初始化系统托盘
	a.logInfo("开始初始化系统托盘")
	a.initSystemTray()

	// 启动健康监控
	if cfg.GuiMonitoring {
		a.logInfo("启动健康监控")
		a.safeGo("startHealthMonitoring", func() {
			a.startHealthMonitoring()
		})
	}

	a.logInfo("=== BS2PRO 核心服务启动完成 ===")

	// 尝试连接设备
	a.safeGo("delayedConnectDevice", func() {
		time.Sleep(1 * time.Second)
		a.ConnectDevice()
	})

	return nil
}

// Stop 停止核心服务
func (a *CoreApp) Stop() {
	a.logInfo("核心服务正在停止...")

	// 清理资源
	a.cleanup()

	// 停止所有监控
	a.DisconnectDevice()

	// 停止桥接程序
	a.bridgeManager.Stop()

	// 停止 IPC 服务器
	if a.ipcServer != nil {
		a.ipcServer.Stop()
	}

	// 停止托盘
	a.trayManager.Quit()

	a.logInfo("核心服务已停止")
}

// initSystemTray 初始化系统托盘
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
		func() tray.Status {
			a.mutex.RLock()
			defer a.mutex.RUnlock()
			cfg := a.configManager.Get()
			fanData := a.deviceManager.GetCurrentFanData()
			var currentRPM uint16
			if fanData != nil {
				currentRPM = fanData.CurrentRPM
			}
			return tray.Status{
				Connected:        a.isConnected,
				CPUTemp:          a.currentTemp.CPUTemp,
				GPUTemp:          a.currentTemp.GPUTemp,
				CurrentRPM:       currentRPM,
				AutoControlState: cfg.AutoControl,
			}
		},
	)
	a.trayManager.Init()
}

// onShowWindowRequest 显示窗口请求回调
func (a *CoreApp) onShowWindowRequest() {
	a.logInfo("收到显示窗口请求")

	// 通知所有已连接的 GUI 客户端显示窗口
	if a.ipcServer != nil && a.ipcServer.HasClients() {
		a.ipcServer.BroadcastEvent("show-window", nil)
	} else {
		// 没有 GUI 连接，启动 GUI
		a.logInfo("没有 GUI 连接，尝试启动 GUI")
		if err := launchGUI(); err != nil {
			a.logError("启动 GUI 失败: %v", err)
		}
	}
}

// onQuitRequest 退出请求回调
func (a *CoreApp) onQuitRequest() {
	a.logInfo("收到退出请求")

	// 通知所有 GUI 客户端退出
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent("quit", nil)
	}

	// 发送退出信号
	select {
	case a.quitChan <- true:
	default:
	}
}

// handleIPCRequest 处理 IPC 请求
func (a *CoreApp) handleIPCRequest(req ipc.Request) ipc.Response {
	switch req.Type {
	// 设备相关
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

	// 配置相关
	case ipc.ReqGetConfig:
		cfg := a.configManager.Get()
		return a.dataResponse(cfg)

	case ipc.ReqUpdateConfig:
		var cfg types.AppConfig
		if err := json.Unmarshal(req.Data, &cfg); err != nil {
			return a.errorResponse("解析配置失败: " + err.Error())
		}
		if err := a.UpdateConfig(cfg); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqSetFanCurve:
		var curve []types.FanCurvePoint
		if err := json.Unmarshal(req.Data, &curve); err != nil {
			return a.errorResponse("解析风扇曲线失败: " + err.Error())
		}
		if err := a.SetFanCurve(curve); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqGetFanCurve:
		curve := a.configManager.Get().FanCurve
		return a.dataResponse(curve)

	// 控制相关
	case ipc.ReqSetAutoControl:
		var params ipc.SetAutoControlParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetAutoControl(params.Enabled); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqSetManualGear:
		var params ipc.SetManualGearParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetManualGear(params.Gear, params.Level)
		return a.successResponse(success)

	case ipc.ReqGetAvailableGears:
		gears := types.GearCommands
		return a.dataResponse(gears)

	case ipc.ReqSetCustomSpeed:
		var params ipc.SetCustomSpeedParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetCustomSpeed(params.Enabled, params.RPM); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqSetGearLight:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetGearLight(params.Enabled)
		return a.successResponse(success)

	case ipc.ReqSetPowerOnStart:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetPowerOnStart(params.Enabled)
		return a.successResponse(success)

	case ipc.ReqSetSmartStartStop:
		var params ipc.SetStringParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetSmartStartStop(params.Value)
		return a.successResponse(success)

	case ipc.ReqSetBrightness:
		var params ipc.SetIntParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		success := a.SetBrightness(params.Value)
		return a.successResponse(success)

	case ipc.ReqSetLightStrip:
		var params ipc.SetLightStripParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetLightStrip(params.Config); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	// 温度相关
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

	// 自启动相关
	case ipc.ReqSetWindowsAutoStart:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
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
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.autostartManager.SetAutoStartWithMethod(params.Enable, params.Method); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	// 窗口相关
	case ipc.ReqShowWindow:
		a.onShowWindowRequest()
		return a.successResponse(true)

	case ipc.ReqHideWindow:
		// GUI 自己处理隐藏
		return a.successResponse(true)

	case ipc.ReqQuitApp:
		a.safeGo("onQuitRequest", func() {
			a.onQuitRequest()
		})
		return a.successResponse(true)

	// 调试相关
	case ipc.ReqGetDebugInfo:
		info := a.GetDebugInfo()
		return a.dataResponse(info)

	case ipc.ReqSetDebugMode:
		var params ipc.SetBoolParams
		if err := json.Unmarshal(req.Data, &params); err != nil {
			return a.errorResponse("解析参数失败: " + err.Error())
		}
		if err := a.SetDebugMode(params.Enabled); err != nil {
			return a.errorResponse(err.Error())
		}
		return a.successResponse(true)

	case ipc.ReqUpdateGuiResponseTime:
		atomic.StoreInt64(&a.guiLastResponse, time.Now().Unix())
		return a.successResponse(true)

	// 系统相关
	case ipc.ReqPing:
		return a.dataResponse("pong")

	case ipc.ReqIsAutoStartLaunch:
		return a.dataResponse(a.isAutoStartLaunch)

	default:
		return a.errorResponse(fmt.Sprintf("未知的请求类型: %s", req.Type))
	}
}

// 响应辅助方法
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
		return a.errorResponse("序列化数据失败: " + err.Error())
	}
	return ipc.Response{Success: true, Data: dataBytes}
}

// onFanDataUpdate 风扇数据更新回调
func (a *CoreApp) onFanDataUpdate(fanData *types.FanData) {
	a.mutex.Lock()
	cfg := a.configManager.Get()

	// 检查工作模式变化
	// 如果开启了"断连保持配置模式"，则忽略设备状态变化，避免误判
	if fanData.WorkMode == "挡位工作模式" &&
		cfg.AutoControl &&
		a.lastDeviceMode == "自动模式(实时转速)" &&
		!a.userSetAutoControl &&
		!cfg.IgnoreDeviceOnReconnect {

		a.logInfo("检测到设备从自动模式切换到挡位工作模式，自动关闭智能变频")
		cfg.AutoControl = false

		if a.monitoringTemp {
			select {
			case a.stopMonitoring <- true:
			default:
			}
		}

		a.configManager.Set(cfg)
		a.configManager.Save()

		// 广播配置更新
		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
		}
	} else if fanData.WorkMode == "挡位工作模式" &&
		cfg.AutoControl &&
		a.lastDeviceMode == "自动模式(实时转速)" &&
		!a.userSetAutoControl &&
		cfg.IgnoreDeviceOnReconnect {
		a.logInfo("检测到设备模式变化，但已开启断连保持配置模式，保持APP配置不变")
	}

	a.lastDeviceMode = fanData.WorkMode

	if a.userSetAutoControl {
		a.userSetAutoControl = false
	}

	a.mutex.Unlock()

	// 广播风扇数据更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventFanDataUpdate, fanData)
	}
}

// onDeviceDisconnect 设备断开回调
func (a *CoreApp) onDeviceDisconnect() {
	a.mutex.Lock()
	wasConnected := a.isConnected
	a.isConnected = false
	a.mutex.Unlock()

	if wasConnected {
		a.logInfo("设备连接已断开，将在健康检查时尝试自动重连")
	}

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}

	// 启动自动重连机制
	a.safeGo("scheduleReconnect", func() {
		a.scheduleReconnect()
	})
}

// scheduleReconnect 安排设备重连
func (a *CoreApp) scheduleReconnect() {
	// 延迟一段时间后尝试重连，避免频繁重试
	retryDelays := []time.Duration{
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
		30 * time.Second,
	}

	for i, delay := range retryDelays {
		// 检查是否已经连接（可能其他途径已重连）
		a.mutex.RLock()
		connected := a.isConnected
		a.mutex.RUnlock()

		if connected {
			a.logInfo("设备已重新连接，停止重连尝试")
			return
		}

		a.logInfo("等待 %v 后尝试第 %d 次重连...", delay, i+1)
		time.Sleep(delay)

		// 再次检查连接状态
		a.mutex.RLock()
		connected = a.isConnected
		a.mutex.RUnlock()

		if connected {
			a.logInfo("设备已重新连接，停止重连尝试")
			return
		}

		a.logInfo("尝试第 %d 次重连设备...", i+1)
		if a.ConnectDevice() {
			a.logInfo("设备重连成功")

			// 如果开启了断连保持配置模式，重新应用APP配置
			cfg := a.configManager.Get()
			if cfg.IgnoreDeviceOnReconnect {
				a.logInfo("断连保持配置模式已开启，重新应用APP配置")
				a.reapplyConfigAfterReconnect()
			}

			return
		}
		a.logError("第 %d 次重连失败", i+1)
	}

	a.logError("所有重连尝试均失败，等待下次健康检查")
}

// ConnectDevice 连接设备
func (a *CoreApp) ConnectDevice() bool {
	success, deviceInfo := a.deviceManager.Connect()
	if success {
		a.mutex.Lock()
		a.isConnected = true
		a.mutex.Unlock()

		if deviceInfo != nil && a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventDeviceConnected, deviceInfo)
		}

		cfg := a.configManager.Get()
		if err := a.applyConfiguredLightStrip(); err != nil {
			a.logError("应用灯带配置失败: %v", err)
		}
		if cfg.AutoControl {
			a.safeGo("startTemperatureMonitoring@ConnectDevice", func() {
				a.startTemperatureMonitoring()
			})
		}
	} else if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceError, "连接失败")
	}
	return success
}

// DisconnectDevice 断开设备连接
func (a *CoreApp) DisconnectDevice() {
	a.mutex.Lock()
	if a.monitoringTemp {
		select {
		case a.stopMonitoring <- true:
		default:
		}
		a.monitoringTemp = false
	}
	a.isConnected = false
	a.mutex.Unlock()

	a.deviceManager.Disconnect()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventDeviceDisconnected, nil)
	}
}

// reapplyConfigAfterReconnect 重连后重新应用APP配置
func (a *CoreApp) reapplyConfigAfterReconnect() {
	cfg := a.configManager.Get()

	// 重新应用智能变频配置
	if cfg.AutoControl {
		a.logInfo("重新启动智能变频")
		a.safeGo("startTemperatureMonitoring@Reconnect", func() {
			a.startTemperatureMonitoring()
		})
	} else if cfg.CustomSpeedEnabled {
		// 重新应用自定义转速
		a.logInfo("重新应用自定义转速: %d RPM", cfg.CustomSpeedRPM)
		if !a.deviceManager.SetCustomFanSpeed(cfg.CustomSpeedRPM) {
			a.logError("重新应用自定义转速失败")
		}
	}

	// 重新应用挡位灯配置
	if cfg.GearLight {
		a.logInfo("重新开启挡位灯")
		if !a.deviceManager.SetGearLight(true) {
			a.logError("重新开启挡位灯失败")
		}
	}

	// 重新应用通电自启动配置
	if cfg.PowerOnStart {
		a.logInfo("重新开启通电自启动")
		if !a.deviceManager.SetPowerOnStart(true) {
			a.logError("重新开启通电自启动失败")
		}
	}

	if err := a.applyConfiguredLightStrip(); err != nil {
		a.logError("重连后重新应用灯带配置失败: %v", err)
	}
}

// GetDeviceStatus 获取设备状态
func (a *CoreApp) GetDeviceStatus() map[string]any {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	return map[string]any{
		"connected":   a.isConnected,
		"monitoring":  a.monitoringTemp,
		"currentData": a.deviceManager.GetCurrentFanData(),
		"temperature": a.currentTemp,
	}
}

// UpdateConfig 更新配置
func (a *CoreApp) UpdateConfig(cfg types.AppConfig) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	oldCfg := a.configManager.Get()
	cfg.LightStrip, _ = normalizeLightStripConfig(cfg.LightStrip)
	cfg.SmartControl, _ = smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve)

	if cfg.AutoControl && !a.monitoringTemp && a.isConnected {
		a.safeGo("startTemperatureMonitoring@UpdateConfig", func() {
			a.startTemperatureMonitoring()
		})
	} else if !cfg.AutoControl && a.monitoringTemp {
		select {
		case a.stopMonitoring <- true:
		default:
		}
	}

	cfg.ConfigPath = oldCfg.ConfigPath
	return a.configManager.Update(cfg)
}

// SetFanCurve 设置风扇曲线
func (a *CoreApp) SetFanCurve(curve []types.FanCurvePoint) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	cfg.FanCurve = curve
	cfg.SmartControl, _ = smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve)
	return a.configManager.Update(cfg)
}

// SetAutoControl 设置智能变频
func (a *CoreApp) SetAutoControl(enabled bool) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()

	if enabled && cfg.CustomSpeedEnabled {
		return fmt.Errorf("自定义转速模式下无法开启智能变频")
	}

	cfg.AutoControl = enabled

	if enabled {
		a.userSetAutoControl = true
	}

	if enabled && !a.monitoringTemp && a.isConnected {
		a.safeGo("startTemperatureMonitoring@SetAutoControl", func() {
			a.startTemperatureMonitoring()
		})
	} else if !enabled && a.monitoringTemp {
		select {
		case a.stopMonitoring <- true:
		default:
		}

		if a.isConnected {
			a.safeGo("applyCurrentGearSetting", func() {
				a.applyCurrentGearSetting()
			})
		}
	}

	a.configManager.Set(cfg)
	err := a.configManager.Save()

	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}

	return err
}

// applyCurrentGearSetting 应用当前挡位设置
func (a *CoreApp) applyCurrentGearSetting() {
	fanData := a.deviceManager.GetCurrentFanData()
	if fanData == nil {
		return
	}

	cfg := a.configManager.Get()
	setGear := fanData.SetGear
	level := cfg.ManualLevel

	a.logInfo("应用当前挡位设置: %s %s", setGear, level)
	a.deviceManager.SetManualGear(setGear, level)
}

// SetManualGear 设置手动挡位
func (a *CoreApp) SetManualGear(gear, level string) bool {
	cfg := a.configManager.Get()
	cfg.ManualGear = gear
	cfg.ManualLevel = level
	a.configManager.Update(cfg)

	return a.deviceManager.SetManualGear(gear, level)
}

// SetCustomSpeed 设置自定义转速
func (a *CoreApp) SetCustomSpeed(enabled bool, rpm int) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()

	if enabled {
		if cfg.AutoControl {
			cfg.AutoControl = false
			if a.monitoringTemp {
				select {
				case a.stopMonitoring <- true:
				default:
				}
			}
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

// SetGearLight 设置挡位灯
func (a *CoreApp) SetGearLight(enabled bool) bool {
	if !a.deviceManager.SetGearLight(enabled) {
		return false
	}

	cfg := a.configManager.Get()
	cfg.GearLight = enabled
	a.configManager.Update(cfg)

	// 广播配置更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}

// SetPowerOnStart 设置通电自启动
func (a *CoreApp) SetPowerOnStart(enabled bool) bool {
	if !a.deviceManager.SetPowerOnStart(enabled) {
		return false
	}

	cfg := a.configManager.Get()
	cfg.PowerOnStart = enabled
	a.configManager.Update(cfg)

	// 广播配置更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}

// SetSmartStartStop 设置智能启停
func (a *CoreApp) SetSmartStartStop(mode string) bool {
	if !a.deviceManager.SetSmartStartStop(mode) {
		return false
	}

	cfg := a.configManager.Get()
	cfg.SmartStartStop = mode
	a.configManager.Update(cfg)

	// 广播配置更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}

// SetBrightness 设置亮度
func (a *CoreApp) SetBrightness(percentage int) bool {
	if !a.deviceManager.SetBrightness(percentage) {
		return false
	}

	cfg := a.configManager.Get()
	cfg.Brightness = percentage
	a.configManager.Update(cfg)

	// 广播配置更新
	if a.ipcServer != nil {
		a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
	}
	return true
}

// SetLightStrip 设置灯带
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
			a.logError("保存灯带默认配置失败: %v", err)
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

// SetWindowsAutoStart 设置Windows自启动
func (a *CoreApp) SetWindowsAutoStart(enable bool) error {
	err := a.autostartManager.SetWindowsAutoStart(enable)
	if err == nil {
		cfg := a.configManager.Get()
		cfg.WindowsAutoStart = enable
		a.configManager.Update(cfg)

		// 广播配置更新
		if a.ipcServer != nil {
			a.ipcServer.BroadcastEvent(ipc.EventConfigUpdate, cfg)
		}
	}
	return err
}

// GetDebugInfo 获取调试信息
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

// SetDebugMode 设置调试模式
func (a *CoreApp) SetDebugMode(enabled bool) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cfg := a.configManager.Get()
	cfg.DebugMode = enabled
	a.debugMode = enabled

	if a.logger != nil {
		a.logger.SetDebugMode(enabled)
		if enabled {
			a.logger.Info("调试模式已开启，后续日志将包含调试级别")
		} else {
			a.logger.Info("调试模式已关闭，调试级别日志将被忽略")
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

// startTemperatureMonitoring 开始温度监控
func (a *CoreApp) startTemperatureMonitoring() {
	if a.monitoringTemp {
		return
	}

	a.monitoringTemp = true

	if a.isConnected {
		if err := a.deviceManager.EnterAutoMode(); err != nil {
			a.logError("进入自动模式失败: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	cfg := a.configManager.Get()
	updateInterval := time.Duration(cfg.TempUpdateRate) * time.Second

	// 温度采样缓冲区
	sampleCount := max(cfg.TempSampleCount, 1)
	tempSamples := make([]int, 0, sampleCount)
	recentAvgTemps := make([]int, 0, 24)
	lastAvgTemp := 0
	lastTargetRPM := 0
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

			// 广播温度更新
			if a.ipcServer != nil {
				a.ipcServer.BroadcastEvent(ipc.EventTemperatureUpdate, temp)
			}

			cfg := a.configManager.Get()
			smartCfg, smartChanged := smartcontrol.NormalizeConfig(cfg.SmartControl, cfg.FanCurve)
			if smartChanged {
				cfg.SmartControl = smartCfg
				a.configManager.Set(cfg)
				if err := a.configManager.Save(); err != nil {
					a.logError("保存智能控温配置失败: %v", err)
				}
			}

			if cfg.AutoControl && temp.MaxTemp > 0 {
				// 更新采样配置
				newSampleCount := max(cfg.TempSampleCount, 1)
				if newSampleCount != sampleCount {
					sampleCount = newSampleCount
					tempSamples = make([]int, 0, sampleCount)
				}

				// 添加新采样
				tempSamples = append(tempSamples, temp.MaxTemp)
				if len(tempSamples) > sampleCount {
					tempSamples = tempSamples[len(tempSamples)-sampleCount:]
				}

				// 计算平均温度
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

				baseRPM := temperature.CalculateTargetRPM(avgTemp, cfg.FanCurve)
				targetRPM := baseRPM

				if smartCfg.Enabled {
					targetRPM = smartcontrol.CalculateTargetRPM(avgTemp, lastAvgTemp, cfg.FanCurve, smartCfg)
				}

				if lastTargetRPM > 0 {
					targetRPM = smartcontrol.ApplyRampLimit(targetRPM, lastTargetRPM, smartCfg.RampUpLimit, smartCfg.RampDownLimit)
				}

				deltaRPM := targetRPM - lastTargetRPM
				if deltaRPM < 0 {
					deltaRPM = -deltaRPM
				}

				if targetRPM > 0 && (lastTargetRPM == 0 || deltaRPM >= smartCfg.MinRPMChange) {
					a.deviceManager.SetFanSpeed(targetRPM)
					lastTargetRPM = targetRPM
				}

				if smartCfg.Enabled && smartCfg.Learning {
					updatedHeatOffsets, updatedCoolOffsets, changed := smartcontrol.LearnCurveOffsets(
						avgTemp,
						lastAvgTemp,
						targetRPM,
						lastTargetRPM,
						recentAvgTemps,
						cfg.FanCurve,
						smartCfg,
					)
					if changed {
						smartCfg.LearnedOffsetsHeat = updatedHeatOffsets
						smartCfg.LearnedOffsetsCool = updatedCoolOffsets
						smartCfg.LearnedOffsets = smartcontrol.BlendOffsets(updatedHeatOffsets, updatedCoolOffsets)
						cfg.SmartControl = smartCfg
						a.configManager.Set(cfg)
						learningDirty = true
					}

					if learningDirty && time.Since(lastLearningSave) >= 25*time.Second {
						if err := a.configManager.Save(); err != nil {
							a.logError("保存学习曲线失败: %v", err)
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
					a.logDebug("智能控温: 温度=%d°C 平均=%d°C 基础=%dRPM 目标=%dRPM", temp.MaxTemp, avgTemp, baseRPM, targetRPM)
				}

				lastAvgTemp = avgTemp
			}
		}
	}

	if learningDirty {
		if err := a.configManager.Save(); err != nil {
			a.logError("退出监控时保存学习曲线失败: %v", err)
		}
	}
}

// startHealthMonitoring 启动健康监控
func (a *CoreApp) startHealthMonitoring() {
	a.logInfo("启动健康监控系统")

	a.healthCheckTicker = time.NewTicker(30 * time.Second)

	a.safeGo("healthMonitoringLoop", func() {
		defer a.healthCheckTicker.Stop()

		for {
			select {
			case <-a.healthCheckTicker.C:
				a.performHealthCheck()
			case <-a.cleanupChan:
				a.logInfo("健康监控系统已停止")
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

// performHealthCheck 执行健康检查
func (a *CoreApp) performHealthCheck() {
	defer func() {
		if r := recover(); r != nil {
			a.logError("健康检查中发生panic: %v", r)
		}
	}()

	a.trayManager.CheckHealth()
	a.checkDeviceHealth()

	a.logDebug("健康检查完成 - 托盘:%v 设备连接:%v",
		a.trayManager.IsInitialized(), a.isConnected)
}

// checkDeviceHealth 检查设备健康状态
func (a *CoreApp) checkDeviceHealth() {
	a.mutex.RLock()
	connected := a.isConnected
	a.mutex.RUnlock()

	if !connected {
		a.logInfo("健康检查: 设备未连接，尝试重新连接")
		a.safeGo("healthReconnect", func() {
			if a.ConnectDevice() {
				a.logInfo("健康检查: 设备重连成功")
			} else {
				a.logDebug("健康检查: 设备重连失败，等待下次检查")
			}
		})
	} else {
		// 验证设备实际连接状态
		if !a.deviceManager.IsConnected() {
			a.logError("健康检查: 检测到设备状态不一致，触发断开回调")
			a.onDeviceDisconnect()
		}
	}
}

// cleanup 清理资源
func (a *CoreApp) cleanup() {
	if a.healthCheckTicker != nil {
		a.healthCheckTicker.Stop()
	}

	select {
	case a.cleanupChan <- true:
	default:
	}

	if a.logger != nil {
		a.logger.Info("核心服务正在退出，清理资源")
		a.logger.Close()
	}
}

// 日志辅助方法
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

// launchGUI 启动 GUI 程序
func launchGUI() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	exeDir := filepath.Dir(exePath)
	guiPath := filepath.Join(exeDir, "BS2PRO-Controller.exe")

	if _, err := os.Stat(guiPath); os.IsNotExist(err) {
		guiPath = filepath.Join(exeDir, "..", "BS2PRO-Controller.exe")
		if _, err := os.Stat(guiPath); os.IsNotExist(err) {
			return fmt.Errorf("GUI 程序不存在: %s", guiPath)
		}
	}

	cmd := exec.Command(guiPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: false,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 GUI 程序失败: %v", err)
	}

	// 使用 fmt 而非日志系统，避免循环依赖
	fmt.Printf("GUI 程序已启动，PID: %d\n", cmd.Process.Pid)

	go func() {
		cmd.Wait()
	}()

	return nil
}
