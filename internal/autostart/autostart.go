// Package autostart provides Windows auto-start management functionality
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

// Manager auto-start manager
type Manager struct {
	logger types.Logger
}

// NewManager creates a new auto-start manager
func NewManager(logger types.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// IsRunningAsAdmin checks if running with administrator privileges
func (m *Manager) IsRunningAsAdmin() bool {
	var sid *windows.SID

	// Create administrator group SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		m.logger.Error("Failed to create administrator SID: %v", err)
		return false
	}
	defer windows.FreeSid(sid)

	// Check current process token
	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		m.logger.Error("Failed to check administrator privileges: %v", err)
		return false
	}

	return member
}

// SetWindowsAutoStart sets Windows startup auto-launch
func (m *Manager) SetWindowsAutoStart(enable bool) error {
	// Check if running with administrator privileges
	if !m.IsRunningAsAdmin() {
		return fmt.Errorf("administrator privileges required to set auto-start")
	}

	if enable {
		// Use Task Scheduler to set auto-start
		return m.createScheduledTask()
	} else {
		// Delete Task Scheduler task and registry entry
		m.deleteScheduledTask()
		return m.removeRegistryAutoStart()
	}
}

// createScheduledTask creates a scheduled task
func (m *Manager) createScheduledTask() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Get core service path
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
		return fmt.Errorf("failed to create scheduled task: %v, output: %s", err, string(output))
	}

	m.logger.Info("Auto-start set via Task Scheduler")
	return nil
}

// deleteScheduledTask deletes the scheduled task
func (m *Manager) deleteScheduledTask() error {
	cmd := exec.Command("schtasks", "/delete", "/tn", "BS2PRO-Controller", "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "does not exist") || strings.Contains(string(output), "cannot be found") || strings.Contains(string(output), "不存在") {
			return nil
		}
		return fmt.Errorf("failed to delete scheduled task: %v, output: %s", err, string(output))
	}

	m.logger.Info("Deleted auto-start scheduled task")
	return nil
}

// removeRegistryAutoStart removes the registry auto-start entry
func (m *Manager) removeRegistryAutoStart() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry: %v", err)
	}
	defer key.Close()

	// Delete registry entry
	err = key.DeleteValue("BS2PRO-Controller")
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("failed to delete registry entry: %v", err)
	}

	m.logger.Info("Deleted registry auto-start entry")
	return nil
}

// GetAutoStartMethod gets the current auto-start method
func (m *Manager) GetAutoStartMethod() string {
	if m.checkScheduledTask() {
		return "task_scheduler"
	}
	if m.checkRegistryAutoStart() {
		return "registry"
	}
	return "none"
}

// SetAutoStartWithMethod sets auto-start using the specified method
func (m *Manager) SetAutoStartWithMethod(enable bool, method string) error {
	if !enable {
		m.deleteScheduledTask()
		m.removeRegistryAutoStart()
		return nil
	}

	switch method {
	case "task_scheduler":
		if !m.IsRunningAsAdmin() {
			return fmt.Errorf("Task Scheduler requires administrator privileges, please run the program as administrator to configure")
		}
		return m.createScheduledTask()

	case "registry":
		return m.setRegistryAutoStart()

	default:
		return fmt.Errorf("unsupported auto-start method: %s", method)
	}
}

// setRegistryAutoStart sets registry auto-start
func (m *Manager) setRegistryAutoStart() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry: %v", err)
	}
	defer key.Close()

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}
	exeDir := filepath.Dir(exePath)
	corePath := filepath.Join(exeDir, "BS2PRO-Core.exe")

	// If core service does not exist, use the current executable path
	if _, err := os.Stat(corePath); os.IsNotExist(err) {
		corePath = exePath
	}
	exePathWithArgs := fmt.Sprintf("\"%s\" --autostart", corePath)

	err = key.SetStringValue("BS2PRO-Controller", exePathWithArgs)
	if err != nil {
		return fmt.Errorf("failed to set registry value: %v", err)
	}

	m.logger.Info("Auto-start set via registry")
	return nil
}

// CheckWindowsAutoStart checks Windows startup auto-launch status
func (m *Manager) CheckWindowsAutoStart() bool {
	if m.checkScheduledTask() {
		return true
	}

	return m.checkRegistryAutoStart()
}

// checkScheduledTask checks if the auto-start task exists in Task Scheduler
func (m *Manager) checkScheduledTask() bool {
	cmd := exec.Command("schtasks", "/query", "/tn", "BS2PRO-Controller")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	err := cmd.Run()
	return err == nil
}

// checkRegistryAutoStart checks the auto-start entry in the registry
func (m *Manager) checkRegistryAutoStart() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if err != nil {
		m.logger.Debug("Failed to open registry: %v", err)
		return false
	}
	defer key.Close()

	_, _, err = key.GetStringValue("BS2PRO-Controller")
	return err == nil
}

// DetectAutoStartLaunch detects whether the current launch is an auto-start launch
func DetectAutoStartLaunch(args []string) bool {
	for _, arg := range args {
		if arg == "--autostart" || arg == "/autostart" || arg == "-autostart" {
			return true
		}
	}

	if isLaunchedByTaskScheduler() {
		return true
	}

	// Check if the current working directory is a system directory
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

// isLaunchedByTaskScheduler checks if launched by Task Scheduler
func isLaunchedByTaskScheduler() bool {
	// Check parent process on Windows
	cmd := exec.Command("wmic", "process", "where", fmt.Sprintf("ProcessId=%d", os.Getpid()), "get", "ParentProcessId", "/value")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
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

// checkParentProcessName checks the parent process name
func checkParentProcessName(ppid int) bool {
	cmd := exec.Command("wmic", "process", "where", fmt.Sprintf("ProcessId=%d", ppid), "get", "Name", "/value")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "Name="); ok {
			processName := strings.ToLower(strings.TrimSpace(after))
			// Check if it is a Task Scheduler related process
			if processName == "taskeng.exe" || processName == "svchost.exe" || processName == "taskhostw.exe" {
				return true
			}
		}
	}

	return false
}

// parseIntSafe safely parses an integer
func parseIntSafe(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
