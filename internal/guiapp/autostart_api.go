package guiapp

import (
	"encoding/json"
	"fmt"

	"github.com/TIANLI0/THRM/internal/ipc"
)

// SetWindowsAutoStart 设置Windows开机自启动
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

// IsRunningAsAdmin 检查是否以管理员权限运行
func (a *App) IsRunningAsAdmin() bool {
	resp, err := a.sendRequest(ipc.ReqIsRunningAsAdmin, nil)
	if err != nil {
		return false
	}
	var isAdmin bool
	json.Unmarshal(resp.Data, &isAdmin)
	return isAdmin
}

// GetAutoStartMethod 获取当前的自启动方式
func (a *App) GetAutoStartMethod() string {
	resp, err := a.sendRequest(ipc.ReqGetAutoStartMethod, nil)
	if err != nil {
		return "none"
	}
	var method string
	json.Unmarshal(resp.Data, &method)
	return method
}

// SetAutoStartWithMethod 使用指定方式设置自启动
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

// CheckWindowsAutoStart 检查Windows开机自启动状态
func (a *App) CheckWindowsAutoStart() bool {
	resp, err := a.sendRequest(ipc.ReqCheckWindowsAutoStart, nil)
	if err != nil {
		return false
	}
	var enabled bool
	json.Unmarshal(resp.Data, &enabled)
	return enabled
}

// IsAutoStartLaunch 返回当前是否为自启动启动
func (a *App) IsAutoStartLaunch() bool {
	resp, err := a.sendRequest(ipc.ReqIsAutoStartLaunch, nil)
	if err != nil {
		return false
	}
	var isAutoStart bool
	json.Unmarshal(resp.Data, &isAutoStart)
	return isAutoStart
}
