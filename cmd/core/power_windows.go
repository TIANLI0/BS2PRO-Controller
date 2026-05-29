//go:build windows

package main

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modPowrprof                                  = windows.NewLazySystemDLL("powrprof.dll")
	procPowerRegisterSuspendResumeNotification   = modPowrprof.NewProc("PowerRegisterSuspendResumeNotification")
	procPowerUnregisterSuspendResumeNotification = modPowrprof.NewProc("PowerUnregisterSuspendResumeNotification")
)

const (
	// 来自 winuser.h 的电源广播事件类型
	pbtAPMSuspend         = 0x0004
	pbtAPMResumeSuspend   = 0x0007
	pbtAPMResumeAutomatic = 0x0012

	// DEVICE_NOTIFY_CALLBACK：以回调而非窗口消息方式接收通知
	deviceNotifyCallback = 0x00000002
)

// deviceNotifySubscribeParameters 对应 _DEVICE_NOTIFY_SUBSCRIBE_PARAMETERS。
type deviceNotifySubscribeParameters struct {
	callback uintptr
	context  uintptr
}

// powerNotifier 持有注册句柄与回调，防止其被 GC 回收。
type powerNotifier struct {
	handle    uintptr
	params    deviceNotifySubscribeParameters
	callback  uintptr
	onSuspend func()
	onResume  func()
	stopOnce  sync.Once
}

// registerSuspendResumeNotifications 注册系统睡眠/唤醒通知。
func registerSuspendResumeNotifications(onSuspend, onResume func()) (func(), error) {
	if err := procPowerRegisterSuspendResumeNotification.Find(); err != nil {
		return nil, fmt.Errorf("当前系统不支持电源通知注册: %w", err)
	}

	pn := &powerNotifier{
		onSuspend: onSuspend,
		onResume:  onResume,
	}

	pn.callback = windows.NewCallback(func(context uintptr, eventType uint32, setting uintptr) uintptr {
		defer func() {
			// 回调运行在系统线程上，任何 panic 都不能逃逸到运行时之外。
			_ = recover()
		}()

		switch eventType {
		case pbtAPMSuspend:
			if pn.onSuspend != nil {
				pn.onSuspend()
			}
		case pbtAPMResumeSuspend, pbtAPMResumeAutomatic:
			if pn.onResume != nil {
				pn.onResume()
			}
		}
		return 0
	})
	pn.params.callback = pn.callback

	ret, _, err := procPowerRegisterSuspendResumeNotification.Call(
		uintptr(deviceNotifyCallback),
		uintptr(unsafe.Pointer(&pn.params)),
		uintptr(unsafe.Pointer(&pn.handle)),
	)
	// 成功返回 ERROR_SUCCESS(0)
	if ret != 0 {
		return nil, fmt.Errorf("注册电源通知失败，错误码=%d: %v", ret, err)
	}

	stop := func() {
		pn.stopOnce.Do(func() {
			if pn.handle != 0 {
				_, _, _ = procPowerUnregisterSuspendResumeNotification.Call(pn.handle)
				pn.handle = 0
			}
		})
	}
	return stop, nil
}
