package guiapp

import (
	"encoding/json"
	"fmt"

	"github.com/TIANLI0/THRM/internal/ipc"
)

// TestBridgeProgram 测试桥接程序
func (a *App) TestBridgeProgram() BridgeTemperatureData {
	resp, err := a.sendRequest(ipc.ReqTestBridgeProgram, nil)
	if err != nil {
		return BridgeTemperatureData{Success: false, Error: err.Error()}
	}
	var data BridgeTemperatureData
	json.Unmarshal(resp.Data, &data)
	return data
}

// GetBridgeProgramStatus 获取桥接程序状态
func (a *App) GetBridgeProgramStatus() map[string]any {
	resp, err := a.sendRequest(ipc.ReqGetBridgeProgramStatus, nil)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var status map[string]any
	json.Unmarshal(resp.Data, &status)
	return status
}

// RestartPawnIO 重启 PawnIO 驱动并重新初始化温度读取
func (a *App) RestartPawnIO() BridgeTemperatureData {
	resp, err := a.sendRequest(ipc.ReqRestartPawnIO, nil)
	if err != nil {
		return BridgeTemperatureData{Success: false, Error: err.Error()}
	}
	var data BridgeTemperatureData
	json.Unmarshal(resp.Data, &data)
	return data
}

// ReinstallPawnIO 重新运行安装目录中的 PawnIO 安装包
func (a *App) ReinstallPawnIO() (map[string]any, error) {
	resp, err := a.sendRequest(ipc.ReqReinstallPawnIO, nil)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
