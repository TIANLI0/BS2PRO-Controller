package main

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/theme"
)

// embeddedThemes 内置默认主题（含官方 THRM 参考主题）。
//
// 作用：1) 首次运行时把这些主题播种到安装目录，方便用户直接编辑；
//  2. 当磁盘上的主题文件缺失时作为安全兜底，保证 THRM 始终可选。
//
//go:embed all:themes
var embeddedThemes embed.FS

// newThemeManager 基于当前可执行文件位置与用户目录构造主题管理器。
func newThemeManager() *theme.Manager {
	// 内置主题：把 embed 根从 "themes" 下沉，使路径形如 "thrm/theme.json"。
	var builtin fs.FS
	if sub, err := fs.Sub(embeddedThemes, "themes"); err == nil {
		builtin = sub
	}

	// 安装目录下的 themes（与可执行文件同级）。
	installThemesDir := ""
	if exePath, err := os.Executable(); err == nil {
		installThemesDir = filepath.Join(filepath.Dir(exePath), "themes")
	}

	// 用户目录下的 themes（安装目录不可写时的可写兜底）。
	userThemesDir := ""
	if home, err := os.UserHomeDir(); err == nil {
		userThemesDir = filepath.Join(appmeta.UserConfigDir(home), "themes")
	}

	return theme.NewManager(installThemesDir, userThemesDir, builtin)
}
