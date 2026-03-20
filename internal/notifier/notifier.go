package notifier

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
	"github.com/gen2brain/beeep"
)

// Manager sends system notifications using beeep.
type Manager struct {
	logger   types.Logger
	iconPath string
}

func NewManager(logger types.Logger, iconData []byte) *Manager {
	beeep.AppName = "BS2PRO Controller"
	return &Manager{
		logger:   logger,
		iconPath: ensureNotificationIcon(iconData, logger),
	}
}

func (m *Manager) Notify(title, message string) {
	title = strings.TrimSpace(title)
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	toastTitle := "Feature Changed"
	if title != "" {
		toastTitle = title
	}

	if err := beeep.Notify(toastTitle, message, m.iconPath); err != nil {
		if m.logger != nil {
			m.logger.Debug("Failed to send system notification: %v", err)
		}
	}
}

func ensureNotificationIcon(iconData []byte, logger types.Logger) string {
	if len(iconData) == 0 {
		return ""
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		cacheDir = os.TempDir()
	}

	iconDir := filepath.Join(cacheDir, "BS2PRO-Controller")
	if err := os.MkdirAll(iconDir, 0755); err != nil {
		if logger != nil {
			logger.Debug("Failed to create notification icon cache directory: %v", err)
		}
		return ""
	}

	iconPath := filepath.Join(iconDir, "notify-icon.ico")
	if err := os.WriteFile(iconPath, iconData, 0644); err != nil {
		if logger != nil {
			logger.Debug("Failed to write notification icon cache: %v", err)
		}
		return ""
	}

	return iconPath
}
