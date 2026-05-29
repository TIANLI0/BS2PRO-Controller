//go:build windows

package tray

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32       = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW = modUser32.NewProc("FindWindowW")
)

// isShellReady 判断 Windows 任务栏外壳是否已就绪。
func isShellReady() bool {
	classNamePtr, err := windows.UTF16PtrFromString("Shell_TrayWnd")
	if err != nil {
		return true
	}
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(classNamePtr)), 0)
	return hwnd != 0
}

// waitForShellReady 在启动系统托盘前等待外壳就绪。
func waitForShellReady(done <-chan struct{}, timeout time.Duration) bool {
	if isShellReady() {
		return true
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return false
		case <-ticker.C:
			if isShellReady() {
				return true
			}
			if time.Now().After(deadline) {
				return true
			}
		}
	}
}
