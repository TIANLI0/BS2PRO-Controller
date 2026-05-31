package guiapp

import (
	"context"
	"sync"

	"github.com/TIANLI0/THRM/internal/ipc"
	"github.com/TIANLI0/THRM/internal/theme"
	"github.com/TIANLI0/THRM/internal/types"
	"go.uber.org/zap"
)

// App struct - GUI 应用程序结构
type App struct {
	ctx       context.Context
	ipcClient *ipc.Client
	mutex     sync.RWMutex

	// 缓存的状态
	isConnected bool
	currentTemp types.TemperatureData

	// 自定义主题管理器（发现/播种/读取安装目录与用户目录下的主题）
	themeManager *theme.Manager
}

// 为了与前端 API 兼容，重新导出类型
type (
	FanCurvePoint             = types.FanCurvePoint
	FanCurveProfile           = types.FanCurveProfile
	FanCurveProfilesPayload   = types.FanCurveProfilesPayload
	FanData                   = types.FanData
	GearCommand               = types.GearCommand
	TemperatureData           = types.TemperatureData
	TemperatureHistoryPoint   = types.TemperatureHistoryPoint
	TemperatureHistoryPayload = types.TemperatureHistoryPayload
	BridgeTemperatureData     = types.BridgeTemperatureData
	AppConfig                 = types.AppConfig
)

var guiLogger *zap.SugaredLogger

func init() {
	logger, _ := zap.NewProduction()
	guiLogger = logger.Sugar()
}
