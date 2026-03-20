// Package device provides HID device communication functionality
package device

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
	"github.com/sstallion/go-hid"
)

const (
	// VendorID device vendor ID
	VendorID = 0x37D7
	// ProductID1 product ID 1 BS2PRO
	ProductID1 = 0x1002
	// ProductID2 product ID 2 BS2
	ProductID2 = 0x1001
)

// Manager HID device manager
type Manager struct {
	device         *hid.Device
	isConnected    bool
	productID      uint16 // Currently connected product ID
	mutex          sync.RWMutex
	logger         types.Logger
	currentFanData *types.FanData

	// Callback functions
	onFanDataUpdate func(data *types.FanData)
	onDisconnect    func()
}

// NewManager creates a new device manager
func NewManager(logger types.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// SetCallbacks sets callback functions
func (m *Manager) SetCallbacks(onFanDataUpdate func(data *types.FanData), onDisconnect func()) {
	m.onFanDataUpdate = onFanDataUpdate
	m.onDisconnect = onDisconnect
}

// Init initializes the HID library
func (m *Manager) Init() error {
	return hid.Init()
}

// Exit cleans up the HID library
func (m *Manager) Exit() error {
	return hid.Exit()
}

// Connect connects to the HID device
func (m *Manager) Connect() (bool, map[string]string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.isConnected {
		return true, nil
	}

	productIDs := []uint16{ProductID1, ProductID2}
	var device *hid.Device
	var err error

	var connectedProductID uint16
	for _, productID := range productIDs {
		m.logInfo("Connecting to device - Vendor ID: 0x%04X, Product ID: 0x%04X", VendorID, productID)

		device, err = hid.OpenFirst(VendorID, productID)
		if err == nil {
			m.logInfo("Successfully connected to Product ID: 0x%04X", productID)
			connectedProductID = productID
			break
		} else {
			m.logError("Product ID 0x%04X connection failed: %v", productID, err)
		}
	}

	if err != nil {
		m.logError("All device connection attempts failed")
		return false, nil
	}

	m.device = device
	m.isConnected = true
	m.productID = connectedProductID

	modelName := "BS2PRO"
	if connectedProductID == ProductID2 {
		modelName = "BS2"
	}

	// Get device info
	deviceInfo, err := device.GetDeviceInfo()
	var info map[string]string
	if err == nil {
		m.logInfo("Device connected: %s %s (Model: %s)", deviceInfo.MfrStr, deviceInfo.ProductStr, modelName)
		info = map[string]string{
			"manufacturer": deviceInfo.MfrStr,
			"product":      deviceInfo.ProductStr,
			"serial":       deviceInfo.SerialNbr,
			"model":        modelName,
			"productId":    fmt.Sprintf("0x%04X", connectedProductID),
		}
	} else {
		m.logError("Device connected, but failed to get device info: %v", err)
		info = map[string]string{
			"manufacturer": "Unknown",
			"product":      modelName,
			"serial":       "Unknown",
			"model":        modelName,
			"productId":    fmt.Sprintf("0x%04X", connectedProductID),
		}
	}

	// Start monitoring device data
	go m.monitorDeviceData()

	return true, info
}

// Disconnect disconnects the device
func (m *Manager) Disconnect() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected {
		return
	}

	// Close device
	if m.device != nil {
		m.device.Close()
		m.device = nil
	}

	m.isConnected = false
	m.logInfo("Device disconnected")

	if m.onDisconnect != nil {
		m.onDisconnect()
	}
}

// IsConnected checks if the device is connected
func (m *Manager) IsConnected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isConnected
}

// GetProductID gets the product ID of the currently connected device
func (m *Manager) GetProductID() uint16 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.productID
}

// GetModelName gets the model name of the currently connected device
func (m *Manager) GetModelName() string {
	productID := m.GetProductID()
	if productID == ProductID2 {
		return "BS2"
	}
	if productID == ProductID1 {
		return "BS2PRO"
	}
	return "Unknown"
}

// GetCurrentFanData gets the current fan data
func (m *Manager) GetCurrentFanData() *types.FanData {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.currentFanData
}

// monitorDeviceData monitors device data
func (m *Manager) monitorDeviceData() {
	m.mutex.RLock()
	if !m.isConnected || m.device == nil {
		m.mutex.RUnlock()
		return
	}
	m.mutex.RUnlock()

	// Set non-blocking mode
	err := m.device.SetNonblock(true)
	if err != nil {
		m.logError("Failed to set non-blocking mode: %v", err)
	}

	buffer := make([]byte, 64)
	consecutiveErrors := 0
	const maxConsecutiveErrors = 5

	for {
		m.mutex.RLock()
		connected := m.isConnected
		device := m.device
		m.mutex.RUnlock()

		if !connected || device == nil {
			m.logInfo("Device disconnected, stopping data monitoring")
			break
		}

		n, err := device.ReadWithTimeout(buffer, 1*time.Second)
		if err != nil {
			if err == hid.ErrTimeout {
				consecutiveErrors = 0 // Timeout is normal, reset error count
				continue
			}

			consecutiveErrors++
			m.logError("Failed to read device data (%d/%d): %v", consecutiveErrors, maxConsecutiveErrors, err)

			if consecutiveErrors >= maxConsecutiveErrors {
				m.logError("Too many consecutive read failures, device may be disconnected")
				break
			}

			// Brief wait before retry
			time.Sleep(500 * time.Millisecond)
			continue
		}

		consecutiveErrors = 0 // Successful read, reset error count

		if n > 0 {
			// Parse fan data
			fanData := m.parseFanData(buffer, n)
			if fanData != nil {
				m.mutex.Lock()
				m.currentFanData = fanData
				m.mutex.Unlock()

				if m.onFanDataUpdate != nil {
					m.onFanDataUpdate(fanData)
				}
			}
		}

		// Brief sleep to avoid high CPU usage
		time.Sleep(100 * time.Millisecond)
	}

	// Device monitoring loop exited, trigger disconnect handling
	m.handleDeviceDisconnected()
}

// handleDeviceDisconnected handles device disconnection
func (m *Manager) handleDeviceDisconnected() {
	m.mutex.Lock()
	wasConnected := m.isConnected

	if m.device != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					m.logError("Error occurred while closing device: %v", r)
				}
			}()
			m.device.Close()
		}()
		m.device = nil
	}

	m.isConnected = false
	m.mutex.Unlock()

	if wasConnected {
		m.logInfo("Device disconnected")
		if m.onDisconnect != nil {
			m.onDisconnect()
		}
	}
}

// parseFanData parses fan data
func (m *Manager) parseFanData(data []byte, length int) *types.FanData {
	if length < 11 {
		return nil
	}

	// Check sync header
	magic := binary.BigEndian.Uint16(data[1:3])
	if magic != 0x5AA5 {
		return nil
	}

	if data[3] != 0xEF {
		return nil
	}

	fanData := &types.FanData{
		ReportID:     data[0],
		MagicSync:    magic,
		Command:      data[3],
		Status:       data[4],
		GearSettings: data[5],
		CurrentMode:  data[6],
		Reserved1:    data[7],
	}

	// Parse RPM (little-endian)
	if length >= 10 {
		fanData.CurrentRPM = binary.LittleEndian.Uint16(data[8:10])
	}
	if length >= 12 {
		fanData.TargetRPM = binary.LittleEndian.Uint16(data[10:12])
	}

	// Parse gear settings
	maxGear, setGear := m.parseGearSettings(fanData.GearSettings)
	fanData.MaxGear = maxGear
	fanData.SetGear = setGear

	fanData.WorkMode = m.parseWorkMode(fanData.CurrentMode)

	return fanData
}

// parseGearSettings parses gear settings
func (m *Manager) parseGearSettings(gearByte uint8) (maxGear, setGear string) {
	maxGearCode := (gearByte >> 4) & 0x0F
	setGearCode := gearByte & 0x0F

	maxGearMap := map[uint8]string{
		0x2: "Standard",
		0x4: "Power",
		0x6: "Overclock",
	}

	setGearMap := map[uint8]string{
		0x8: "Silent",
		0xA: "Standard",
		0xC: "Power",
		0xE: "Overclock",
	}

	if val, ok := maxGearMap[maxGearCode]; ok {
		maxGear = val
	} else {
		maxGear = fmt.Sprintf("Unknown(0x%X)", maxGearCode)
	}

	if val, ok := setGearMap[setGearCode]; ok {
		setGear = val
	} else {
		setGear = fmt.Sprintf("Unknown(0x%X)", setGearCode)
	}

	return
}

// parseWorkMode parses the work mode
func (m *Manager) parseWorkMode(mode uint8) string {
	switch mode {
	case 0x04, 0x02, 0x06, 0x0A, 0x08, 0x00:
		return "Gear Mode"
	case 0x05, 0x03, 0x07, 0x0B, 0x09, 0x01:
		return "Auto Mode (Real-time RPM)"
	default:
		return fmt.Sprintf("Unknown Mode(0x%02X)", mode)
	}
}

// SetFanSpeed sets the fan speed
func (m *Manager) SetFanSpeed(rpm int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	if rpm < 0 || rpm > 4000 {
		return false
	}

	// First enter real-time RPM mode
	enterModeCmd := []byte{0x02, 0x5A, 0xA5, 0x23, 0x02, 0x25, 0x00}
	// Pad to 23 bytes
	enterModeCmd = append(enterModeCmd, make([]byte, 23-len(enterModeCmd))...)

	_, err := m.device.Write(enterModeCmd)
	if err != nil {
		m.logError("Failed to enter real-time RPM mode: %v", err)
		return false
	}

	time.Sleep(50 * time.Millisecond)

	// Construct RPM setting command
	speedBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(speedBytes, uint16(rpm))

	// Calculate checksum
	checksum := (0x5A + 0xA5 + 0x21 + 0x04 + int(speedBytes[0]) + int(speedBytes[1]) + 1) & 0xFF

	cmd := []byte{0x02, 0x5A, 0xA5, 0x21, 0x04}
	cmd = append(cmd, speedBytes...)
	cmd = append(cmd, byte(checksum))
	// Pad to 23 bytes
	cmd = append(cmd, make([]byte, 23-len(cmd))...)

	_, err = m.device.Write(cmd)
	if err != nil {
		m.logError("Failed to set fan speed: %v", err)
		return false
	}

	m.logDebug("Fan speed set to: %d RPM", rpm)
	return true
}

// SetCustomFanSpeed sets custom fan speed (no limits)
func (m *Manager) SetCustomFanSpeed(rpm int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	m.logWarn("Warning: setting custom speed %d RPM (no upper/lower limits)", rpm)

	enterModeCmd := []byte{0x02, 0x5A, 0xA5, 0x23, 0x02, 0x25, 0x00}
	enterModeCmd = append(enterModeCmd, make([]byte, 23-len(enterModeCmd))...)

	_, err := m.device.Write(enterModeCmd)
	if err != nil {
		m.logError("Failed to enter real-time RPM mode: %v", err)
		return false
	}

	time.Sleep(50 * time.Millisecond)

	speedBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(speedBytes, uint16(rpm))

	// Calculate checksum
	checksum := (0x5A + 0xA5 + 0x21 + 0x04 + int(speedBytes[0]) + int(speedBytes[1]) + 1) & 0xFF

	cmd := []byte{0x02, 0x5A, 0xA5, 0x21, 0x04}
	cmd = append(cmd, speedBytes...)
	cmd = append(cmd, byte(checksum))
	cmd = append(cmd, make([]byte, 23-len(cmd))...)

	_, err = m.device.Write(cmd)
	if err != nil {
		m.logError("Failed to set custom fan speed: %v", err)
		return false
	}

	m.logInfo("Custom fan speed set to: %d RPM", rpm)
	return true
}

// EnterAutoMode enters auto mode
func (m *Manager) EnterAutoMode() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return fmt.Errorf("device not connected")
	}

	// Send command to enter real-time RPM mode
	enterModeCmd := []byte{0x02, 0x5A, 0xA5, 0x23, 0x02, 0x25, 0x00}
	// Pad to 23 bytes
	enterModeCmd = append(enterModeCmd, make([]byte, 23-len(enterModeCmd))...)

	_, err := m.device.Write(enterModeCmd)
	if err != nil {
		return fmt.Errorf("failed to enter auto mode: %v", err)
	}

	m.logInfo("Switched to auto mode, starting smart fan control")
	return nil
}

// SetManualGear sets the manual gear
func (m *Manager) SetManualGear(gear, level string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	commands, exists := types.GearCommands[gear]
	if !exists {
		m.logError("Command not found for gear %s", gear)
		return false
	}

	var selectedCommand *types.GearCommand
	for i := range commands {
		cmd := &commands[i]
		switch level {
		case "低", "low":
			if strings.Contains(cmd.Name, "低") || strings.Contains(cmd.Name, "low") {
				selectedCommand = cmd
			}
		case "中", "medium":
			if strings.Contains(cmd.Name, "中") || strings.Contains(cmd.Name, "medium") {
				selectedCommand = cmd
			}
		case "高", "high":
			if strings.Contains(cmd.Name, "高") || strings.Contains(cmd.Name, "high") {
				selectedCommand = cmd
			}
		}
		if selectedCommand != nil {
			break
		}
	}

	if selectedCommand == nil {
		m.logError("Command not found for gear %s level %s", gear, level)
		return false
	}

	// Send command, ensure first byte is ReportID
	cmdWithReportID := append([]byte{0x02}, selectedCommand.Command...)

	_, err := m.device.Write(cmdWithReportID)
	if err != nil {
		m.logError("Failed to set gear %s %s: %v", gear, level, err)
		return false
	}

	m.logInfo("Gear set successfully: %s %s (target RPM: %d)", gear, level, selectedCommand.RPM)
	return true
}

// SetGearLight sets the gear indicator light
func (m *Manager) SetGearLight(enabled bool) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	var cmd []byte
	if enabled {
		cmd = []byte{0x02, 0x5A, 0xA5, 0x48, 0x03, 0x01, 0x4C}
	} else {
		cmd = []byte{0x02, 0x5A, 0xA5, 0x48, 0x03, 0x00, 0x4B}
	}

	// Pad to 23 bytes
	cmd = append(cmd, make([]byte, 23-len(cmd))...)

	_, err := m.device.Write(cmd)
	if err != nil {
		m.logError("Failed to set gear light: %v", err)
		return false
	}

	return true
}

// SetPowerOnStart sets power-on auto-start
func (m *Manager) SetPowerOnStart(enabled bool) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	var cmd []byte
	if enabled {
		cmd = []byte{0x02, 0x5A, 0xA5, 0x0C, 0x03, 0x02, 0x11}
	} else {
		cmd = []byte{0x02, 0x5A, 0xA5, 0x0C, 0x03, 0x01, 0x10}
	}

	// Pad to 23 bytes
	cmd = append(cmd, make([]byte, 23-len(cmd))...)

	_, err := m.device.Write(cmd)
	if err != nil {
		m.logError("Failed to set power-on auto-start: %v", err)
		return false
	}

	return true
}

// SetSmartStartStop sets smart start/stop
func (m *Manager) SetSmartStartStop(mode string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	var cmd []byte
	switch mode {
	case "off":
		cmd = []byte{0x02, 0x5A, 0xA5, 0x0D, 0x03, 0x00, 0x10}
	case "immediate":
		cmd = []byte{0x02, 0x5A, 0xA5, 0x0D, 0x03, 0x01, 0x11}
	case "delayed":
		cmd = []byte{0x02, 0x5A, 0xA5, 0x0D, 0x03, 0x02, 0x12}
	default:
		return false
	}

	// Pad to 23 bytes
	cmd = append(cmd, make([]byte, 23-len(cmd))...)

	_, err := m.device.Write(cmd)
	if err != nil {
		m.logError("Failed to set smart start/stop: %v", err)
		return false
	}

	return true
}

// SetBrightness sets the brightness
func (m *Manager) SetBrightness(percentage int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	if percentage < 0 || percentage > 100 {
		return false
	}

	var cmd []byte
	switch percentage {
	case 0:
		cmd = []byte{0x02, 0x5A, 0xA5, 0x47, 0x0D, 0x1C, 0x00, 0xFF}
		// Pad to 23 bytes
		cmd = append(cmd, make([]byte, 23-len(cmd))...)
	case 100:
		cmd = []byte{0x02, 0x5A, 0xA5, 0x43, 0x02, 0x45}
		// Pad to 23 bytes
		cmd = append(cmd, make([]byte, 23-len(cmd))...)
	default:
		return false
	}

	_, err := m.device.Write(cmd)
	if err != nil {
		m.logError("Failed to set brightness: %v", err)
		return false
	}

	return true
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

func (m *Manager) logWarn(format string, v ...any) {
	if m.logger != nil {
		m.logger.Warn(format, v...)
	}
}

func (m *Manager) logDebug(format string, v ...any) {
	if m.logger != nil {
		m.logger.Debug(format, v...)
	}
}
