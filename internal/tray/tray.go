// Package tray 提供系统托盘管理功能
package tray

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/systray"
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
)

// Manager 系统托盘管理器
type Manager struct {
	logger       types.Logger
	initialized  int32 // atomic: 0=未初始化, 1=已初始化
	readyState   int32 // atomic: 0=未就绪, 1=就绪
	mutex        sync.Mutex
	done         chan struct{} // 关闭此通道以通知所有 goroutine 退出
	uiQueue      chan func()
	iconData     []byte
	menuItems    *MenuItems
	onShowWindow func()
	onQuit       func()
	onToggleAuto func() bool
	getStatus    func() Status

	// 监控托盘健康状态
	lastIconRefresh  int64
	consecutiveFails int32 // 连续失败计数

	// 防止托盘动作重入导致偶发无响应
	showWindowInFlight int32
	toggleAutoInFlight int32
	quitInFlight       int32
}

// MenuItems 托盘菜单项结构
type MenuItems struct {
	Show           *systray.MenuItem
	DeviceStatus   *systray.MenuItem
	CPUTemperature *systray.MenuItem
	GPUTemperature *systray.MenuItem
	FanSpeed       *systray.MenuItem
	AutoControl    *systray.MenuItem
	Quit           *systray.MenuItem
}

// Status 状态信息
type Status struct {
	Connected        bool
	CPUTemp          int
	GPUTemp          int
	CurrentRPM       uint16
	AutoControlState bool
}

// NewManager 创建新的托盘管理器
func NewManager(logger types.Logger, iconData []byte) *Manager {
	return &Manager{
		logger:   logger,
		done:     make(chan struct{}),
		uiQueue:  make(chan func(), 64),
		iconData: iconData,
	}
}

// SetCallbacks 设置回调函数
func (m *Manager) SetCallbacks(
	onShowWindow func(),
	onQuit func(),
	onToggleAuto func() bool,
	getStatus func() Status,
) {
	m.onShowWindow = onShowWindow
	m.onQuit = onQuit
	m.onToggleAuto = onToggleAuto
	m.getStatus = getStatus
}

// Init 初始化系统托盘
func (m *Manager) Init() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查是否已经初始化
	if !atomic.CompareAndSwapInt32(&m.initialized, 0, 1) {
		m.logDebug("托盘已经初始化，跳过重复初始化")
		return
	}

	m.logInfo("正在初始化系统托盘")

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		defer func() {
			if r := recover(); r != nil {
				m.logError("托盘初始化过程中发生panic: %v", r)
				atomic.StoreInt32(&m.initialized, 0)
				atomic.StoreInt32(&m.readyState, 0)
			}
		}()

		systray.Run(m.onTrayReady, m.onTrayExit)
	}()
}

// onTrayReady 托盘准备就绪时的回调
func (m *Manager) onTrayReady() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("托盘回调函数中发生panic: %v", r)
			atomic.StoreInt32(&m.initialized, 0)
			atomic.StoreInt32(&m.readyState, 0)
		}
	}()

	m.logInfo("托盘回调函数已启动")

	if err := m.setupIcon(); err != nil {
		m.logError("设置托盘图标失败: %v", err)
		atomic.StoreInt32(&m.readyState, 0)
		atomic.StoreInt32(&m.initialized, 0)
		systray.Quit()
		return
	}

	// 左键单击托盘图标：显示主窗口；右键保持默认行为（打开托盘菜单）
	systray.SetOnTapped(func() {
		m.logDebug("托盘图标左键点击: 显示主窗口")
		if m.onShowWindow != nil {
			m.runTrayActionAsync("icon-show-window", &m.showWindowInFlight, m.onShowWindow)
		}
	})

	// 创建托盘菜单
	menuItems, err := m.createMenu()
	if err != nil {
		m.logError("创建托盘菜单失败: %v", err)
		atomic.StoreInt32(&m.readyState, 0)
		atomic.StoreInt32(&m.initialized, 0)
		systray.Quit()
		return
	}
	m.menuItems = menuItems
	m.startUIWorker()

	atomic.StoreInt32(&m.readyState, 1)
	atomic.StoreInt64(&m.lastIconRefresh, time.Now().Unix())
	atomic.StoreInt32(&m.consecutiveFails, 0)
	m.logInfo("系统托盘初始化完成")

	// 处理托盘菜单事件
	go m.handleMenuEvents()

	// 定期更新托盘菜单状态
	go m.updateMenuStatus()

	// 启动托盘健康监控（定期刷新图标以应对 Explorer 重启等）
	go m.startIconHealthMonitor()
}

// setupIcon 设置托盘图标
func (m *Manager) setupIcon() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("设置托盘图标时发生panic: %v", r)
		}
	}()

	if len(m.iconData) == 0 {
		return fmt.Errorf("托盘图标数据为空")
	}

	systray.SetIcon(m.iconData)
	systray.SetTitle("BS2PRO 控制器")
	systray.SetTooltip("BS2PRO 风扇控制器 - 运行中")
	return nil
}

// createMenu 创建托盘菜单
func (m *Manager) createMenu() (items *MenuItems, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("创建托盘菜单时发生panic: %v", r)
		}
	}()

	items = &MenuItems{}

	items.Show = systray.AddMenuItem("显示主窗口", "显示控制器主窗口")
	systray.AddSeparator()

	items.DeviceStatus = systray.AddMenuItem("设备状态", "查看设备连接状态")
	items.DeviceStatus.Disable()

	items.CPUTemperature = systray.AddMenuItem("CPU温度", "显示当前CPU温度")
	items.CPUTemperature.Disable()

	items.GPUTemperature = systray.AddMenuItem("GPU温度", "显示当前GPU温度")
	items.GPUTemperature.Disable()

	items.FanSpeed = systray.AddMenuItem("风扇转速", "显示当前风扇转速")
	items.FanSpeed.Disable()

	// 智能变频状态 - 获取当前配置状态
	autoControlEnabled := false
	if m.getStatus != nil {
		autoControlEnabled = m.getStatus().AutoControlState
	}
	items.AutoControl = systray.AddMenuItemCheckbox("智能变频", "启用/禁用智能变频", autoControlEnabled)

	systray.AddSeparator()
	items.Quit = systray.AddMenuItem("退出", "完全退出应用")

	return items, nil
}

// handleMenuEvents 处理托盘菜单事件
func (m *Manager) handleMenuEvents() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("处理托盘菜单事件时发生panic: %v", r)
		}
	}()

	if m.menuItems == nil || m.menuItems.Show == nil || m.menuItems.AutoControl == nil || m.menuItems.Quit == nil {
		m.logError("托盘菜单未正确初始化，无法处理菜单事件")
		return
	}

	for {
		select {
		case <-m.menuItems.Show.ClickedCh:
			m.logDebug("托盘菜单: 显示主窗口")
			if m.onShowWindow != nil {
				m.runTrayActionAsync("menu-show-window", &m.showWindowInFlight, m.onShowWindow)
			}
		case <-m.menuItems.AutoControl.ClickedCh:
			m.logDebug("托盘菜单: 切换智能变频状态")
			if m.onToggleAuto != nil {
				m.runTrayActionAsync("menu-toggle-auto", &m.toggleAutoInFlight, func() {
					newState := m.onToggleAuto()
					m.enqueueUI("menu-toggle-auto-ui", func() {
						if m.menuItems == nil || m.menuItems.AutoControl == nil {
							return
						}
						if newState {
							m.menuItems.AutoControl.Check()
						} else {
							m.menuItems.AutoControl.Uncheck()
						}
					})
				})
			}
		case <-m.menuItems.Quit.ClickedCh:
			m.logInfo("托盘菜单: 用户请求退出应用")
			if m.onQuit != nil {
				m.runTrayActionAsync("menu-quit", &m.quitInFlight, m.onQuit)
			}
			return
		case <-m.done:
			return
		}
	}
}

// runTrayActionAsync 异步执行托盘动作，避免阻塞托盘消息处理
func (m *Manager) runTrayActionAsync(action string, inFlight *int32, fn func()) {
	if fn == nil {
		return
	}

	if inFlight != nil && !atomic.CompareAndSwapInt32(inFlight, 0, 1) {
		m.logDebug("托盘动作[%s]仍在执行，忽略重复触发", action)
		return
	}

	go func() {
		startedAt := time.Now()
		defer func() {
			if inFlight != nil {
				atomic.StoreInt32(inFlight, 0)
			}
			if r := recover(); r != nil {
				m.logError("托盘动作[%s]发生panic: %v", action, r)
			}

			d := time.Since(startedAt)
			if d > 800*time.Millisecond {
				m.logError("托盘动作[%s]执行耗时较长: %v", action, d)
			} else {
				m.logDebug("托盘动作[%s]执行完成: %v", action, d)
			}
		}()

		fn()
	}()
}

// updateMenuStatus 定期更新托盘菜单状态
func (m *Manager) updateMenuStatus() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("更新托盘菜单状态时发生panic: %v", r)
		}
	}()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 如果托盘不可用，跳过本次更新但不退出，等待恢复
			if atomic.LoadInt32(&m.readyState) == 0 || atomic.LoadInt32(&m.initialized) == 0 {
				continue
			}

			if m.getStatus == nil {
				continue
			}

			status := m.getStatus()
			m.enqueueUI("update-menu-status", func() {
				if m.menuItems == nil {
					return
				}

				if status.Connected {
					m.menuItems.DeviceStatus.SetTitle("设备状态: 已连接")
				} else {
					m.menuItems.DeviceStatus.SetTitle("设备状态: 未连接")
				}

				if status.CPUTemp > 0 {
					m.menuItems.CPUTemperature.SetTitle(fmt.Sprintf("CPU温度: %d°C", status.CPUTemp))
				} else {
					m.menuItems.CPUTemperature.SetTitle("CPU温度: 无数据")
				}

				if status.GPUTemp > 0 {
					m.menuItems.GPUTemperature.SetTitle(fmt.Sprintf("GPU温度: %d°C", status.GPUTemp))
				} else {
					m.menuItems.GPUTemperature.SetTitle("GPU温度: 无数据")
				}

				if status.CurrentRPM > 0 {
					m.menuItems.FanSpeed.SetTitle(fmt.Sprintf("风扇转速: %d RPM", status.CurrentRPM))
				} else {
					m.menuItems.FanSpeed.SetTitle("风扇转速: 无数据")
				}

				if status.AutoControlState {
					m.menuItems.AutoControl.Check()
				} else {
					m.menuItems.AutoControl.Uncheck()
				}

				if status.Connected {
					if status.AutoControlState {
						tooltipText := fmt.Sprintf("BS2PRO 控制器 - 智能变频中\nCPU: %d°C GPU: %d°C", status.CPUTemp, status.GPUTemp)
						if status.CurrentRPM > 0 {
							tooltipText += fmt.Sprintf("\n风扇: %d RPM", status.CurrentRPM)
						}
						systray.SetTooltip(tooltipText)
					} else {
						tooltipText := "BS2PRO 控制器 - 手动模式"
						if status.CurrentRPM > 0 {
							tooltipText += fmt.Sprintf("\n风扇: %d RPM", status.CurrentRPM)
						}
						systray.SetTooltip(tooltipText)
					}
				} else {
					systray.SetTooltip("BS2PRO 控制器 - 设备未连接")
				}
			})
		case <-m.done:
			return
		}
	}
}

// onTrayExit 托盘退出时的回调
func (m *Manager) onTrayExit() {
	m.logDebug("托盘退出回调被触发")
	atomic.StoreInt32(&m.readyState, 0)
	atomic.StoreInt32(&m.initialized, 0)
}

// startIconHealthMonitor 启动托盘图标健康监控
func (m *Manager) startIconHealthMonitor() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("托盘图标健康监控发生panic: %v", r)
		}
	}()

	// 每30秒刷新一次托盘图标，更及时地恢复 Explorer 重启后的图标
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if atomic.LoadInt32(&m.readyState) == 0 || atomic.LoadInt32(&m.initialized) == 0 {
				continue // 不退出，等待恢复
			}
			m.refreshTrayIcon()
		case <-m.done:
			return
		}
	}
}

// refreshTrayIcon 刷新托盘图标
func (m *Manager) refreshTrayIcon() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("刷新托盘图标时发生panic: %v", r)
			atomic.AddInt32(&m.consecutiveFails, 1)
		}
	}()

	queued := m.enqueueUI("refresh-tray-icon", func() {
		if len(m.iconData) == 0 {
			atomic.AddInt32(&m.consecutiveFails, 1)
			m.logError("刷新托盘图标失败: 图标数据为空")
			return
		}

		systray.SetIcon(m.iconData)
		systray.SetTooltip("BS2PRO 风扇控制器 - 运行中")

		atomic.StoreInt32(&m.consecutiveFails, 0)
		atomic.StoreInt64(&m.lastIconRefresh, time.Now().Unix())

		m.logDebug("托盘图标已刷新")
	})

	if !queued {
		atomic.AddInt32(&m.consecutiveFails, 1)
	}
}

func (m *Manager) startUIWorker() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.logError("托盘UI队列处理发生panic: %v", r)
			}
		}()

		for {
			select {
			case fn := <-m.uiQueue:
				if fn != nil {
					fn()
				}
			case <-m.done:
				return
			}
		}
	}()
}

func (m *Manager) enqueueUI(action string, fn func()) bool {
	if fn == nil {
		return false
	}

	select {
	case <-m.done:
		return false
	default:
	}

	wrapped := func() {
		defer func() {
			if r := recover(); r != nil {
				m.logError("托盘UI动作[%s]发生panic: %v", action, r)
			}
		}()
		fn()
	}

	select {
	case m.uiQueue <- wrapped:
		return true
	default:
		m.logError("托盘UI队列繁忙，丢弃动作: %s", action)
		return false
	}
}

// IsReady 检查托盘是否就绪
func (m *Manager) IsReady() bool {
	return atomic.LoadInt32(&m.readyState) == 1
}

// IsInitialized 检查托盘是否已初始化
func (m *Manager) IsInitialized() bool {
	return atomic.LoadInt32(&m.initialized) == 1
}

// Quit 退出托盘
func (m *Manager) Quit() {
	atomic.StoreInt32(&m.readyState, 0)

	m.mutex.Lock()
	select {
	case <-m.done:
		// 已经关闭
	default:
		close(m.done)
	}
	m.mutex.Unlock()

	func() {
		defer func() {
			if r := recover(); r != nil {
				m.logDebug("退出托盘时发生错误（可忽略）: %v", r)
			}
		}()
		systray.Quit()
	}()
}

// CheckHealth 检查托盘健康状态
func (m *Manager) CheckHealth() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("检查托盘健康状态时发生panic: %v", r)
		}
	}()

	// 如果托盘未初始化，无需检查
	if atomic.LoadInt32(&m.initialized) == 0 {
		return
	}

	// 检查图标是否长时间未刷新
	lastRefresh := atomic.LoadInt64(&m.lastIconRefresh)
	if lastRefresh > 0 && time.Now().Unix()-lastRefresh > 90 {
		m.logInfo("检测到托盘图标长时间未刷新，尝试刷新")
		m.refreshTrayIcon()
	}

	// 如果连续失败，也强制刷新图标
	if atomic.LoadInt32(&m.consecutiveFails) >= 3 {
		m.logError("检测到托盘连续失败，尝试刷新图标")
		m.refreshTrayIcon()
	}
}

// 日志辅助方法
func (m *Manager) logInfo(format string, v ...any) {
	if m.logger != nil {
		m.logger.Info(format, v...)
	}
}

func (m *Manager) logError(format string, v ...any) {
	if m.logger != nil {
		m.logger.Error(format, v...)
	}
}

func (m *Manager) logDebug(format string, v ...any) {
	if m.logger != nil {
		m.logger.Debug(format, v...)
	}
}
