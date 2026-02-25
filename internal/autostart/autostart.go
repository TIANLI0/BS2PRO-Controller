// Package autostart 提供 Windows 自启动管理功能
package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Manager 自启动管理器
type Manager struct {
	logger types.Logger
}

// NewManager 创建新的自启动管理器
func NewManager(logger types.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// IsRunningAsAdmin 检查是否以管理员权限运行
func (m *Manager) IsRunningAsAdmin() bool {
	var sid *windows.SID

	// 创建管理员组的SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		m.logger.Error("创建管理员SID失败: %v", err)
		return false
	}
	defer windows.FreeSid(sid)

	// 检查当前进程令牌
	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		m.logger.Error("检查管理员权限失败: %v", err)
		return false
	}

	return member
}

// SetWindowsAutoStart 设置Windows开机自启动
func (m *Manager) SetWindowsAutoStart(enable bool) error {
	if enable {
		// 优先使用注册表，失败后回退到任务计划程序
		if err := m.setRegistryAutoStart(); err == nil {
			m.deleteScheduledTask()
			return nil
		} else {
			m.logger.Error("注册表自启动设置失败: %v", err)
		}

		if m.IsRunningAsAdmin() {
			if err := m.createScheduledTask(); err == nil {
				return nil
			}
		}

		return fmt.Errorf("设置自启动失败：注册表方式失败，且任务计划程序方式不可用")
	} else {
		// 删除任务计划程序和注册表项
		m.deleteScheduledTask()
		return m.removeRegistryAutoStart()
	}
}

// createScheduledTask 创建任务计划程序
func (m *Manager) createScheduledTask() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取程序路径失败: %v", err)
	}

	// 获取核心服务路径
	exeDir := filepath.Dir(exePath)
	corePath := filepath.Join(exeDir, "BS2PRO-Core.exe")
	if _, err := os.Stat(corePath); os.IsNotExist(err) {
		corePath = exePath
	}
	taskCommand := fmt.Sprintf("\"%s\" --autostart", corePath)
	cmd := exec.Command("schtasks", "/create",
		"/tn", "BS2PRO-Controller",
		"/tr", taskCommand,
		"/sc", "onlogon",
		"/delay", "0000:15",
		"/rl", "highest",
		"/f")

	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("创建任务计划失败: %v, 输出: %s", err, string(output))
	}

	m.logger.Info("已通过任务计划程序设置开机自启动")
	return nil
}

// deleteScheduledTask 删除任务计划程序
func (m *Manager) deleteScheduledTask() error {
	cmd := exec.Command("schtasks", "/delete", "/tn", "BS2PRO-Controller", "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "不存在") || strings.Contains(string(output), "cannot be found") {
			return nil
		}
		return fmt.Errorf("删除任务计划失败: %v, 输出: %s", err, string(output))
	}

	m.logger.Info("已删除任务计划程序的自启动任务")
	return nil
}

// removeRegistryAutoStart 删除注册表自启动项
func (m *Manager) removeRegistryAutoStart() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %v", err)
	}
	defer key.Close()

	// 删除注册表项
	err = key.DeleteValue("BS2PRO-Controller")
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("删除注册表项失败: %v", err)
	}

	m.logger.Info("已删除注册表自启动项")
	return nil
}

// GetAutoStartMethod 获取当前的自启动方式
func (m *Manager) GetAutoStartMethod() string {
	if m.checkScheduledTask() {
		return "task_scheduler"
	}
	if m.checkRegistryAutoStart() {
		return "registry"
	}
	return "none"
}

// SetAutoStartWithMethod 使用指定方式设置自启动
func (m *Manager) SetAutoStartWithMethod(enable bool, method string) error {
	if !enable {
		m.deleteScheduledTask()
		m.removeRegistryAutoStart()
		return nil
	}

	switch method {
	case "task_scheduler":
		if err := m.removeRegistryAutoStart(); err != nil {
			m.logger.Error("清理注册表自启动失败: %v", err)
		}
		if !m.IsRunningAsAdmin() {
			return fmt.Errorf("使用任务计划程序需要管理员权限，请以管理员身份运行程序进行设置")
		}
		return m.createScheduledTask()

	case "registry":
		if err := m.deleteScheduledTask(); err != nil {
			m.logger.Error("清理任务计划自启动失败: %v", err)
		}
		if err := m.setRegistryAutoStart(); err == nil {
			return nil
		} else {
			m.logger.Error("注册表自启动设置失败: %v", err)
			if m.IsRunningAsAdmin() {
				m.logger.Info("尝试回退到任务计划程序自启动")
				return m.createScheduledTask()
			}
			return fmt.Errorf("注册表自启动失败，且当前非管理员无法回退任务计划程序: %v", err)
		}

	default:
		return fmt.Errorf("不支持的自启动方式: %s", method)
	}
}

// setRegistryAutoStart 设置注册表自启动
func (m *Manager) setRegistryAutoStart() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %v", err)
	}
	defer key.Close()

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取程序路径失败: %v", err)
	}
	exeDir := filepath.Dir(exePath)
	corePath := filepath.Join(exeDir, "BS2PRO-Core.exe")

	// 如果核心服务不存在，使用当前程序路径
	if _, err := os.Stat(corePath); os.IsNotExist(err) {
		corePath = exePath
	}
	exePathWithArgs := fmt.Sprintf("\"%s\" --autostart", corePath)

	err = key.SetStringValue("BS2PRO-Controller", exePathWithArgs)
	if err != nil {
		return fmt.Errorf("设置注册表失败: %v", err)
	}

	m.logger.Info("已通过注册表设置开机自启动")
	return nil
}

// CheckWindowsAutoStart 检查Windows开机自启动状态
func (m *Manager) CheckWindowsAutoStart() bool {
	if m.checkScheduledTask() {
		return true
	}

	return m.checkRegistryAutoStart()
}

// checkScheduledTask 检查任务计划程序中的自启动任务
func (m *Manager) checkScheduledTask() bool {
	cmd := exec.Command("schtasks", "/query", "/tn", "BS2PRO-Controller")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	err := cmd.Run()
	return err == nil
}

// checkRegistryAutoStart 检查注册表中的自启动项
func (m *Manager) checkRegistryAutoStart() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if err != nil {
		m.logger.Debug("打开注册表失败: %v", err)
		return false
	}
	defer key.Close()

	_, _, err = key.GetStringValue("BS2PRO-Controller")
	return err == nil
}

// DetectAutoStartLaunch 检测是否为自启动启动
func DetectAutoStartLaunch(args []string) bool {
	for _, arg := range args {
		if arg == "--autostart" || arg == "/autostart" || arg == "-autostart" {
			return true
		}
	}

	if isLaunchedByTaskScheduler() {
		return true
	}

	// 检查当前工作目录是否为系统目录
	wd, err := os.Getwd()
	if err == nil {
		systemDirs := []string{
			"C:\\Windows\\System32",
			"C:\\Windows\\SysWOW64",
			"C:\\Windows",
		}

		for _, sysDir := range systemDirs {
			if strings.EqualFold(wd, sysDir) {
				return true
			}
		}
	}

	return false
}

// isLaunchedByTaskScheduler 检查是否由任务计划程序启动
func isLaunchedByTaskScheduler() bool {
	// 在Windows上检查父进程
	cmd := exec.Command("wmic", "process", "where", fmt.Sprintf("ProcessId=%d", os.Getpid()), "get", "ParentProcessId", "/value")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "ParentProcessId="); ok {
			ppidStr := strings.TrimSpace(after)
			if ppidStr != "" && ppidStr != "0" {
				ppid, err := parseIntSafe(ppidStr)
				if err == nil {
					return checkParentProcessName(ppid)
				}
			}
		}
	}

	return false
}

// checkParentProcessName 检查父进程名称
func checkParentProcessName(ppid int) bool {
	cmd := exec.Command("wmic", "process", "where", fmt.Sprintf("ProcessId=%d", ppid), "get", "Name", "/value")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "Name="); ok {
			processName := strings.ToLower(strings.TrimSpace(after))
			// 检查是否为任务计划程序相关进程
			if processName == "taskeng.exe" || processName == "svchost.exe" || processName == "taskhostw.exe" {
				return true
			}
		}
	}

	return false
}

// parseIntSafe 安全解析整数
func parseIntSafe(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
