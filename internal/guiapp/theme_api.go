package guiapp

import (
	"fmt"
	"os"
	"os/exec"
	goruntime "runtime"

	"github.com/TIANLI0/THRM/internal/theme"
)

// ListThemes 返回所有可用的自定义主题（供前端「界面主题」下拉动态渲染）。
func (a *App) ListThemes() []theme.Meta {
	if a.themeManager == nil {
		return []theme.Meta{}
	}
	return a.themeManager.List()
}

// GetThemeCSS 返回指定自定义主题的 CSS 内容（供前端注入页面）。
func (a *App) GetThemeCSS(id string) (string, error) {
	if a.themeManager == nil {
		return "", fmt.Errorf("主题管理器未初始化")
	}
	return a.themeManager.ReadCSS(id)
}

// OpenThemesFolder 在系统文件管理器中打开主题文件夹，方便用户编辑/新增主题。
func (a *App) OpenThemesFolder() error {
	if a.themeManager == nil {
		return fmt.Errorf("主题管理器未初始化")
	}
	dir := a.themeManager.ResolveDir()
	if dir == "" {
		return fmt.Errorf("无法定位主题文件夹")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		guiLogger.Warnf("创建主题文件夹失败: %v", err)
	}

	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("打开主题文件夹失败: %w", err)
	}
	return nil
}
