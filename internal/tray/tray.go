// Package tray provides system tray management functionality
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

// Manager is the system tray manager
type Manager struct {
	logger          types.Logger
	initialized     int32 // atomic: 0=not initialized, 1=initialized
	readyState      int32 // atomic: 0=not ready, 1=ready
	mutex           sync.Mutex
	done            chan struct{} // close this channel to notify all goroutines to exit
	uiQueue         chan func()
	iconData        []byte
	menuItems       *MenuItems
	onShowWindow    func()
	onQuit          func()
	onToggleAuto    func() bool
	onSetCurve      func(profileID string) string
	getCurveOptions func() ([]CurveOption, string)
	getStatus       func() Status
	curveMenuItems  map[string]*systray.MenuItem

	// Monitor tray health status
	lastIconRefresh  int64
	consecutiveFails int32 // consecutive failure count

	// Prevent tray action re-entry causing occasional unresponsiveness
	showWindowInFlight int32
	toggleAutoInFlight int32
	quitInFlight       int32
}

// MenuItems defines tray menu item structure
type MenuItems struct {
	Show           *systray.MenuItem
	DeviceStatus   *systray.MenuItem
	CPUTemperature *systray.MenuItem
	GPUTemperature *systray.MenuItem
	FanSpeed       *systray.MenuItem
	CurveSelect    *systray.MenuItem
	AutoControl    *systray.MenuItem
	Quit           *systray.MenuItem
}

// CurveOption defines a tray curve option
type CurveOption struct {
	ID   string
	Name string
}

// Status holds status information
type Status struct {
	Connected            bool
	CPUTemp              int
	GPUTemp              int
	CurrentRPM           uint16
	AutoControlState     bool
	ActiveCurveProfileID string
	CurveProfiles        []CurveOption
}

// NewManager creates a new tray manager
func NewManager(logger types.Logger, iconData []byte) *Manager {
	return &Manager{
		logger:         logger,
		done:           make(chan struct{}),
		uiQueue:        make(chan func(), 64),
		iconData:       iconData,
		curveMenuItems: make(map[string]*systray.MenuItem),
	}
}

// SetCallbacks sets callback functions
func (m *Manager) SetCallbacks(
	onShowWindow func(),
	onQuit func(),
	onToggleAuto func() bool,
	onSetCurve func(profileID string) string,
	getCurveOptions func() ([]CurveOption, string),
	getStatus func() Status,
) {
	m.onShowWindow = onShowWindow
	m.onQuit = onQuit
	m.onToggleAuto = onToggleAuto
	m.onSetCurve = onSetCurve
	m.getCurveOptions = getCurveOptions
	m.getStatus = getStatus
}

// Init initializes the system tray
func (m *Manager) Init() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if already initialized
	if !atomic.CompareAndSwapInt32(&m.initialized, 0, 1) {
		m.logDebug("Tray already initialized, skipping duplicate initialization")
		return
	}

	m.logInfo("Initializing system tray")

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		defer func() {
			if r := recover(); r != nil {
				m.logError("Panic during tray initialization: %v", r)
				atomic.StoreInt32(&m.initialized, 0)
				atomic.StoreInt32(&m.readyState, 0)
			}
		}()

		systray.Run(m.onTrayReady, m.onTrayExit)
	}()
}

// onTrayReady is the callback when the tray is ready
func (m *Manager) onTrayReady() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("Panic in tray callback: %v", r)
			atomic.StoreInt32(&m.initialized, 0)
			atomic.StoreInt32(&m.readyState, 0)
		}
	}()

	m.logInfo("Tray callback started")

	if err := m.setupIcon(); err != nil {
		m.logError("Failed to set tray icon: %v", err)
		atomic.StoreInt32(&m.readyState, 0)
		atomic.StoreInt32(&m.initialized, 0)
		systray.Quit()
		return
	}

	// Left-click tray icon: show main window; right-click keeps default behavior (open tray menu)
	systray.SetOnTapped(func() {
		m.logDebug("Tray icon left-clicked: showing main window")
		if m.onShowWindow != nil {
			m.runTrayActionAsync("icon-show-window", &m.showWindowInFlight, m.onShowWindow)
		}
	})

	// Create tray menu
	menuItems, err := m.createMenu()
	if err != nil {
		m.logError("Failed to create tray menu: %v", err)
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
	m.logInfo("System tray initialization complete")

	// Handle tray menu events
	go m.handleMenuEvents()

	// Periodically update tray menu status
	go m.updateMenuStatus()

	// Start tray icon health monitor (periodically refresh icon to handle Explorer restarts, etc.)
	go m.startIconHealthMonitor()
}

// setupIcon sets up the tray icon
func (m *Manager) setupIcon() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while setting tray icon: %v", r)
		}
	}()

	if len(m.iconData) == 0 {
		return fmt.Errorf("tray icon data is empty")
	}

	systray.SetIcon(m.iconData)
	systray.SetTitle("BS2PRO Controller")
	systray.SetTooltip("BS2PRO Fan Controller - Running")
	return nil
}

// createMenu creates the tray menu
func (m *Manager) createMenu() (items *MenuItems, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while creating tray menu: %v", r)
		}
	}()

	items = &MenuItems{}

	items.Show = systray.AddMenuItem("Show Main Window", "Show the controller main window")
	systray.AddSeparator()

	items.DeviceStatus = systray.AddMenuItem("Device Status", "View device connection status")
	items.DeviceStatus.Disable()

	items.CPUTemperature = systray.AddMenuItem("CPU Temperature", "Show current CPU temperature")
	items.CPUTemperature.Disable()

	items.GPUTemperature = systray.AddMenuItem("GPU Temperature", "Show current GPU temperature")
	items.GPUTemperature.Disable()

	items.FanSpeed = systray.AddMenuItem("Fan Speed", "Show current fan speed")
	items.FanSpeed.Disable()
	items.CurveSelect = systray.AddMenuItem("Select Fan Curve", "Switch to a specific fan curve")

	if m.getCurveOptions != nil {
		profiles, activeID := m.getCurveOptions()
		m.ensureCurveMenuItems(items.CurveSelect, profiles)
		m.updateCurveMenuSelection(activeID)
	}

	// Smart control state - get current configuration state
	autoControlEnabled := false
	if m.getStatus != nil {
		autoControlEnabled = m.getStatus().AutoControlState
	}
	items.AutoControl = systray.AddMenuItemCheckbox("Smart Control", "Enable/Disable smart control", autoControlEnabled)

	systray.AddSeparator()
	items.Quit = systray.AddMenuItem("Quit", "Completely exit the application")

	return items, nil
}

// handleMenuEvents handles tray menu events
func (m *Manager) handleMenuEvents() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("Panic while handling tray menu events: %v", r)
		}
	}()

	if m.menuItems == nil || m.menuItems.Show == nil || m.menuItems.AutoControl == nil || m.menuItems.Quit == nil {
		m.logError("Tray menu not properly initialized, cannot handle menu events")
		return
	}

	for {
		select {
		case <-m.menuItems.Show.ClickedCh:
			m.logDebug("Tray menu: show main window")
			if m.onShowWindow != nil {
				m.runTrayActionAsync("menu-show-window", &m.showWindowInFlight, m.onShowWindow)
			}
		case <-m.menuItems.AutoControl.ClickedCh:
			m.logDebug("Tray menu: toggle smart control state")
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
			m.logInfo("Tray menu: user requested to quit application")
			if m.onQuit != nil {
				m.runTrayActionAsync("menu-quit", &m.quitInFlight, m.onQuit)
			}
			return
		case <-m.done:
			return
		}
	}
}

// runTrayActionAsync asynchronously executes tray actions to avoid blocking tray message processing
func (m *Manager) runTrayActionAsync(action string, inFlight *int32, fn func()) {
	if fn == nil {
		return
	}

	if inFlight != nil && !atomic.CompareAndSwapInt32(inFlight, 0, 1) {
		m.logDebug("Tray action [%s] still in progress, ignoring duplicate trigger", action)
		return
	}

	go func() {
		startedAt := time.Now()
		defer func() {
			if inFlight != nil {
				atomic.StoreInt32(inFlight, 0)
			}
			if r := recover(); r != nil {
				m.logError("Panic in tray action [%s]: %v", action, r)
			}

			d := time.Since(startedAt)
			if d > 800*time.Millisecond {
				m.logError("Tray action [%s] took too long: %v", action, d)
			} else {
				m.logDebug("Tray action [%s] completed: %v", action, d)
			}
		}()

		fn()
	}()
}

// updateMenuStatus periodically updates tray menu status
func (m *Manager) updateMenuStatus() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("Panic while updating tray menu status: %v", r)
		}
	}()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// If tray is unavailable, skip this update but don't exit, wait for recovery
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
					m.menuItems.DeviceStatus.SetTitle("Device Status: Connected")
				} else {
					m.menuItems.DeviceStatus.SetTitle("Device Status: Disconnected")
				}

				if status.CPUTemp > 0 {
					m.menuItems.CPUTemperature.SetTitle(fmt.Sprintf("CPU Temp: %d°C", status.CPUTemp))
				} else {
					m.menuItems.CPUTemperature.SetTitle("CPU Temp: No Data")
				}

				if status.GPUTemp > 0 {
					m.menuItems.GPUTemperature.SetTitle(fmt.Sprintf("GPU Temp: %d°C", status.GPUTemp))
				} else {
					m.menuItems.GPUTemperature.SetTitle("GPU Temp: No Data")
				}

				if status.CurrentRPM > 0 {
					m.menuItems.FanSpeed.SetTitle(fmt.Sprintf("Fan Speed: %d RPM", status.CurrentRPM))
				} else {
					m.menuItems.FanSpeed.SetTitle("Fan Speed: No Data")
				}

				if m.menuItems.CurveSelect != nil {
					m.ensureCurveMenuItems(m.menuItems.CurveSelect, status.CurveProfiles)
					m.updateCurveMenuSelection(status.ActiveCurveProfileID)
				}

				if status.AutoControlState {
					m.menuItems.AutoControl.Check()
				} else {
					m.menuItems.AutoControl.Uncheck()
				}

				if status.Connected {
					if status.AutoControlState {
						tooltipText := fmt.Sprintf("BS2PRO Controller - Smart Control Active\nCPU: %d°C GPU: %d°C", status.CPUTemp, status.GPUTemp)
						if status.CurrentRPM > 0 {
							tooltipText += fmt.Sprintf("\nFan: %d RPM", status.CurrentRPM)
						}
						systray.SetTooltip(tooltipText)
					} else {
						tooltipText := "BS2PRO Controller - Manual Mode"
						if status.CurrentRPM > 0 {
							tooltipText += fmt.Sprintf("\nFan: %d RPM", status.CurrentRPM)
						}
						systray.SetTooltip(tooltipText)
					}
				} else {
					systray.SetTooltip("BS2PRO Controller - Device Disconnected")
				}
			})
		case <-m.done:
			return
		}
	}
}

// onTrayExit is the callback when the tray exits
func (m *Manager) onTrayExit() {
	m.logDebug("Tray exit callback triggered")
	atomic.StoreInt32(&m.readyState, 0)
	atomic.StoreInt32(&m.initialized, 0)
}

// startIconHealthMonitor starts tray icon health monitoring
func (m *Manager) startIconHealthMonitor() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("Panic in tray icon health monitor: %v", r)
		}
	}()

	// Refresh tray icon every 30 seconds to recover icon more promptly after Explorer restarts
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if atomic.LoadInt32(&m.readyState) == 0 || atomic.LoadInt32(&m.initialized) == 0 {
				continue // don't exit, wait for recovery
			}
			m.refreshTrayIcon()
		case <-m.done:
			return
		}
	}
}

// refreshTrayIcon refreshes the tray icon
func (m *Manager) refreshTrayIcon() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("Panic while refreshing tray icon: %v", r)
			atomic.AddInt32(&m.consecutiveFails, 1)
		}
	}()

	queued := m.enqueueUI("refresh-tray-icon", func() {
		if len(m.iconData) == 0 {
			atomic.AddInt32(&m.consecutiveFails, 1)
			m.logError("Failed to refresh tray icon: icon data is empty")
			return
		}

		systray.SetIcon(m.iconData)
		systray.SetTooltip("BS2PRO Fan Controller - Running")

		atomic.StoreInt32(&m.consecutiveFails, 0)
		atomic.StoreInt64(&m.lastIconRefresh, time.Now().Unix())

		m.logDebug("Tray icon refreshed")
	})

	if !queued {
		atomic.AddInt32(&m.consecutiveFails, 1)
	}
}

func (m *Manager) startUIWorker() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.logError("Panic in tray UI queue processing: %v", r)
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
				m.logError("Panic in tray UI action [%s]: %v", action, r)
			}
		}()
		fn()
	}

	select {
	case m.uiQueue <- wrapped:
		return true
	default:
		m.logError("Tray UI queue busy, dropping action: %s", action)
		return false
	}
}

// IsReady checks if the tray is ready
func (m *Manager) IsReady() bool {
	return atomic.LoadInt32(&m.readyState) == 1
}

// IsInitialized checks if the tray has been initialized
func (m *Manager) IsInitialized() bool {
	return atomic.LoadInt32(&m.initialized) == 1
}

// Quit exits the tray
func (m *Manager) Quit() {
	atomic.StoreInt32(&m.readyState, 0)

	m.mutex.Lock()
	select {
	case <-m.done:
		// already closed
	default:
		close(m.done)
	}
	m.mutex.Unlock()

	func() {
		defer func() {
			if r := recover(); r != nil {
				m.logDebug("Error while quitting tray (can be ignored): %v", r)
			}
		}()
		systray.Quit()
	}()
}

// CheckHealth checks the tray health status
func (m *Manager) CheckHealth() {
	defer func() {
		if r := recover(); r != nil {
			m.logError("Panic while checking tray health: %v", r)
		}
	}()

	// If tray is not initialized, no need to check
	if atomic.LoadInt32(&m.initialized) == 0 {
		return
	}

	// Check if icon has not been refreshed for a long time
	lastRefresh := atomic.LoadInt64(&m.lastIconRefresh)
	if lastRefresh > 0 && time.Now().Unix()-lastRefresh > 90 {
		m.logInfo("Detected tray icon not refreshed for a long time, attempting refresh")
		m.refreshTrayIcon()
	}

	// If there are consecutive failures, also force refresh the icon
	if atomic.LoadInt32(&m.consecutiveFails) >= 3 {
		m.logError("Detected consecutive tray failures, attempting to refresh icon")
		m.refreshTrayIcon()
	}
}

// Log helper methods
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

func (m *Manager) ensureCurveMenuItems(parent *systray.MenuItem, options []CurveOption) {
	if parent == nil {
		return
	}

	if len(options) == 0 {
		if len(m.curveMenuItems) == 0 {
			emptyItem := parent.AddSubMenuItem("No curves available", "")
			emptyItem.Disable()
			m.curveMenuItems["__empty__"] = emptyItem
		}
		return
	}

	if empty, ok := m.curveMenuItems["__empty__"]; ok && empty != nil {
		empty.Hide()
		delete(m.curveMenuItems, "__empty__")
	}

	activeIDs := map[string]bool{}
	for _, option := range options {
		if option.ID != "" {
			activeIDs[option.ID] = true
		}
	}
	for id, item := range m.curveMenuItems {
		if id == "__empty__" || item == nil {
			continue
		}
		if !activeIDs[id] {
			item.Hide()
			delete(m.curveMenuItems, id)
		}
	}

	for _, option := range options {
		if option.ID == "" {
			continue
		}
		if existing, ok := m.curveMenuItems[option.ID]; ok && existing != nil {
			existing.Show()
			existing.SetTitle(option.Name)
			continue
		}

		item := parent.AddSubMenuItemCheckbox(option.Name, "Switch fan curve", false)
		m.curveMenuItems[option.ID] = item

		profileID := option.ID
		go func(menuItem *systray.MenuItem, pid string) {
			for {
				select {
				case <-menuItem.ClickedCh:
					if m.onSetCurve == nil {
						continue
					}
					m.runTrayActionAsync("menu-set-curve", nil, func() {
						_ = m.onSetCurve(pid)
						m.enqueueUI("menu-set-curve-ui", func() {
							m.updateCurveMenuSelection(pid)
						})
					})
				case <-m.done:
					return
				}
			}
		}(item, profileID)
	}
}

func (m *Manager) updateCurveMenuSelection(activeID string) {
	for id, item := range m.curveMenuItems {
		if item == nil || id == "__empty__" {
			continue
		}
		if id == activeID {
			item.Check()
		} else {
			item.Uncheck()
		}
	}
}
