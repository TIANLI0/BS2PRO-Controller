//go:build !windows

package main

// registerSuspendResumeNotifications 在非 Windows 平台不做处理。
func registerSuspendResumeNotifications(_, _ func()) (func(), error) {
	return func() {}, nil
}
