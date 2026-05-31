//go:build windows

package coreapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/config"
	"golang.org/x/sys/windows/registry"
)

// ReinstallPawnIO runs the bundled PawnIO installer stored under the install directory.
func (a *CoreApp) ReinstallPawnIO() (map[string]any, error) {
	installDir := config.GetInstallDir()
	installerPath := appmeta.FirstExistingPath(appmeta.PawnIOInstallerCandidates(installDir))
	if installerPath == "" {
		return nil, fmt.Errorf("未找到 PawnIO 安装包，已尝试路径: %v", appmeta.PawnIOInstallerCandidates(installDir))
	}

	result := map[string]any{
		"success": false,
		"path":    installerPath,
	}
	installedVersionBefore := readInstalledPawnIOVersion()
	if installedVersionBefore != "" {
		result["installedVersionBefore"] = installedVersionBefore
	}

	a.logInfo("开始重新安装 PawnIO: %s", installerPath)
	a.bridgeManager.Stop()

	if installedVersionBefore != "" {
		a.logInfo("检测到已安装 PawnIO (版本: %s)，先执行卸载再安装", installedVersionBefore)
		uninstallStep, uninstallErr := a.runPawnIOInstaller(installerPath, "uninstall", "-uninstall", "-silent")
		result["uninstall"] = uninstallStep
		if uninstallErr != nil {
			if isPawnIOInstallerTimeout(uninstallErr) {
				result["error"] = "PawnIO 卸载超时"
				return result, fmt.Errorf("PawnIO 卸载超时，请稍后检查驱动状态或手动运行 %s", installerPath)
			}
			result["uninstallWarning"] = uninstallErr.Error()
			a.logError("PawnIO 卸载返回错误，将继续尝试安装: %v", uninstallErr)
		}
	}

	installStep, installErr := a.runPawnIOInstaller(installerPath, "install", "-install", "-silent")
	result["install"] = installStep
	installedVersionAfter := readInstalledPawnIOVersion()
	if installedVersionAfter != "" {
		result["installedVersionAfter"] = installedVersionAfter
	}

	if installErr != nil {
		if isPawnIOInstallerTimeout(installErr) {
			result["error"] = "PawnIO 安装超时"
			return result, fmt.Errorf("PawnIO 安装超时，请稍后检查驱动状态或手动运行 %s", installerPath)
		}

		if pawnIOInstallerExitCode(installErr) == pawnIOAlreadyExistsExitCode && installedVersionAfter != "" {
			result["alreadyInstalled"] = true
			result["warning"] = "PawnIO 安装器返回 183（已存在），已确认系统中仍有 PawnIO 安装记录。"
			a.logInfo("PawnIO 安装器返回 183，检测到已安装版本 %s，按非致命结果处理", installedVersionAfter)
		} else {
			result["error"] = installErr.Error()
			return result, formatPawnIOInstallerError("PawnIO 安装失败", installErr, installStep)
		}
	} else {
		a.logInfo("PawnIO 安装程序执行完成")
	}

	result["success"] = true
	bridgeResult, bridgeErr := a.bridgeManager.RestartPawnIO()
	if bridgeErr != nil {
		result["bridgeWarning"] = bridgeErr.Error()
		a.logError("PawnIO 安装后重新初始化温度监控失败: %v", bridgeErr)
	} else {
		result["bridge"] = bridgeResult
	}

	return result, nil
}

func (a *CoreApp) runPawnIOInstaller(installerPath, action string, args ...string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(a.ctx, pawnIOInstallerTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, installerPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	outputText := strings.TrimSpace(string(output))
	step := map[string]any{
		"action":   action,
		"args":     args,
		"success":  err == nil,
		"exitCode": pawnIOInstallerExitCode(err),
	}
	if outputText != "" {
		step["output"] = outputText
	}
	if ctx.Err() == context.DeadlineExceeded {
		step["timeout"] = true
		step["success"] = false
		return step, ctx.Err()
	}
	if err != nil {
		step["error"] = err.Error()
	}
	return step, err
}

func readInstalledPawnIOVersion() string {
	for _, access := range []uint32{registry.QUERY_VALUE | registry.WOW64_64KEY, registry.QUERY_VALUE | registry.WOW64_32KEY} {
		key, err := registry.OpenKey(registry.LOCAL_MACHINE, pawnIORegistryPath, access)
		if err != nil {
			continue
		}
		version, _, err := key.GetStringValue("DisplayVersion")
		_ = key.Close()
		if err == nil && strings.TrimSpace(version) != "" {
			return strings.TrimSpace(version)
		}
	}
	return ""
}

func pawnIOInstallerExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func isPawnIOInstallerTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

func formatPawnIOInstallerError(prefix string, err error, step map[string]any) error {
	if output, ok := step["output"].(string); ok && output != "" {
		return fmt.Errorf("%s: %v；输出: %s", prefix, err, output)
	}
	return fmt.Errorf("%s: %v", prefix, err)
}

// launchGUI 启动 GUI 程序
func launchGUI() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	exeDir := filepath.Dir(exePath)
	guiCandidates := append(appmeta.GUIExecutableCandidates(exeDir), appmeta.GUIExecutableCandidates(filepath.Join(exeDir, ".."))...)
	guiPath := appmeta.FirstExistingPath(guiCandidates)
	if guiPath == "" {
		return fmt.Errorf("GUI 程序不存在: %v", guiCandidates)
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
