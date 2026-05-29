//go:build !windows

package tray

import "time"

// waitForShellReady 在非 Windows 平台无需等待外壳，直接返回。
func waitForShellReady(_ <-chan struct{}, _ time.Duration) bool {
	return true
}
