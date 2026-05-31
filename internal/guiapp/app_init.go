package guiapp

import (
	"context"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/theme"
	"github.com/TIANLI0/THRM/internal/types"
	"github.com/TIANLI0/THRM/internal/version"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// New 创建 GUI 应用实例
func New(themeManager *theme.Manager) *App {
	if themeManager != nil {
		themeManager.EnsureSeeded()
	}
	return &App{
		ipcClient:    ipc.NewClient(nil),
		currentTemp:  types.TemperatureData{BridgeOk: true},
		themeManager: themeManager,
	}
}

// Startup 应用启动时调用
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	guiLogger.Infof("=== %s GUI 启动 ===", appmeta.AppName)

	if err := a.ipcClient.Connect(); err != nil {
		guiLogger.Errorf("连接核心服务失败: %v", err)
		runtime.EventsEmit(ctx, "core-service-error", "无法连接到核心服务")
	} else {
		guiLogger.Info("已连接到核心服务")
		a.ipcClient.SetEventHandler(a.handleCoreEvent)
	}

	guiLogger.Infof("=== %s GUI 启动完成 ===", appmeta.AppName)
}

// GetAppVersion 返回应用版本号（来自版本模块）
func (a *App) GetAppVersion() string {
	return version.Get()
}

// OnWindowClosing 窗口关闭事件处理
func (a *App) OnWindowClosing(ctx context.Context) bool {
	return false
}
