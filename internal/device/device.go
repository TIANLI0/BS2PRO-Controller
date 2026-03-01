// Package device 提供 HID 设备通信功能
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
	// VendorID 设备厂商ID
	VendorID = 0x37D7
	// ProductID1 产品ID 1 BS2PRO
	ProductID1 = 0x1002
	// ProductID2 产品ID 2 BS2
	ProductID2 = 0x1001
)

// Manager HID 设备管理器
type Manager struct {
	device         *hid.Device
	isConnected    bool
	productID      uint16 // 当前连接的产品ID
	mutex          sync.RWMutex
	logger         types.Logger
	currentFanData *types.FanData

	// 回调函数
	onFanDataUpdate func(data *types.FanData)
	onDisconnect    func()
}

// NewManager 创建新的设备管理器
func NewManager(logger types.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// SetCallbacks 设置回调函数
func (m *Manager) SetCallbacks(onFanDataUpdate func(data *types.FanData), onDisconnect func()) {
	m.onFanDataUpdate = onFanDataUpdate
	m.onDisconnect = onDisconnect
}

// Init 初始化 HID 库
func (m *Manager) Init() error {
	return hid.Init()
}

// Exit 清理 HID 库
func (m *Manager) Exit() error {
	return hid.Exit()
}

// Connect 连接 HID 设备
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
		m.logInfo("正在连接设备 - 厂商ID: 0x%04X, 产品ID: 0x%04X", VendorID, productID)

		device, err = hid.OpenFirst(VendorID, productID)
		if err == nil {
			m.logInfo("成功连接到产品ID: 0x%04X", productID)
			connectedProductID = productID
			break
		} else {
			m.logError("产品ID 0x%04X 连接失败: %v", productID, err)
		}
	}

	if err != nil {
		m.logError("所有设备连接尝试都失败")
		return false, nil
	}

	m.device = device
	m.isConnected = true
	m.productID = connectedProductID

	modelName := "BS2PRO"
	if connectedProductID == ProductID2 {
		modelName = "BS2"
	}

	// 获取设备信息
	deviceInfo, err := device.GetDeviceInfo()
	var info map[string]string
	if err == nil {
		m.logInfo("设备连接成功: %s %s (型号: %s)", deviceInfo.MfrStr, deviceInfo.ProductStr, modelName)
		info = map[string]string{
			"manufacturer": deviceInfo.MfrStr,
			"product":      deviceInfo.ProductStr,
			"serial":       deviceInfo.SerialNbr,
			"model":        modelName,
			"productId":    fmt.Sprintf("0x%04X", connectedProductID),
		}
	} else {
		m.logError("设备连接成功,但获取设备信息失败: %v", err)
		info = map[string]string{
			"manufacturer": "Unknown",
			"product":      modelName,
			"serial":       "Unknown",
			"model":        modelName,
			"productId":    fmt.Sprintf("0x%04X", connectedProductID),
		}
	}

	// 开始监控设备数据
	go m.monitorDeviceData()

	return true, info
}

// Disconnect 断开设备连接
func (m *Manager) Disconnect() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected {
		return
	}

	// 关闭设备
	if m.device != nil {
		m.device.Close()
		m.device = nil
	}

	m.isConnected = false
	m.logInfo("设备连接已断开")

	if m.onDisconnect != nil {
		m.onDisconnect()
	}
}

// IsConnected 检查设备是否已连接
func (m *Manager) IsConnected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isConnected
}

// GetCurrentFanData 获取当前风扇数据
func (m *Manager) GetCurrentFanData() *types.FanData {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.currentFanData
}

// monitorDeviceData 监控设备数据
func (m *Manager) monitorDeviceData() {
	m.mutex.RLock()
	if !m.isConnected || m.device == nil {
		m.mutex.RUnlock()
		return
	}
	m.mutex.RUnlock()

	// 设置非阻塞模式
	err := m.device.SetNonblock(true)
	if err != nil {
		m.logError("设置非阻塞模式失败: %v", err)
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
			m.logInfo("设备已断开，停止数据监控")
			break
		}

		n, err := device.ReadWithTimeout(buffer, 1*time.Second)
		if err != nil {
			if err == hid.ErrTimeout {
				consecutiveErrors = 0 // 超时是正常的，重置错误计数
				continue
			}

			consecutiveErrors++
			m.logError("读取设备数据失败 (%d/%d): %v", consecutiveErrors, maxConsecutiveErrors, err)

			if consecutiveErrors >= maxConsecutiveErrors {
				m.logError("连续读取失败次数过多，设备可能已断开")
				break
			}

			// 短暂等待后重试
			time.Sleep(500 * time.Millisecond)
			continue
		}

		consecutiveErrors = 0 // 成功读取，重置错误计数

		if n > 0 {
			// 解析风扇数据
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

		// 短暂休眠，避免高CPU占用
		time.Sleep(100 * time.Millisecond)
	}

	// 设备监控循环退出，触发断开处理
	m.handleDeviceDisconnected()
}

// handleDeviceDisconnected 处理设备断开
func (m *Manager) handleDeviceDisconnected() {
	m.mutex.Lock()
	wasConnected := m.isConnected

	if m.device != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					m.logError("关闭设备时发生错误: %v", r)
				}
			}()
			m.device.Close()
		}()
		m.device = nil
	}

	m.isConnected = false
	m.mutex.Unlock()

	if wasConnected {
		m.logInfo("设备连接已断开")
		if m.onDisconnect != nil {
			m.onDisconnect()
		}
	}
}

// parseFanData 解析风扇数据
func (m *Manager) parseFanData(data []byte, length int) *types.FanData {
	if length < 11 {
		return nil
	}

	// 检查同步头
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

	// 解析转速 (小端序)
	if length >= 10 {
		fanData.CurrentRPM = binary.LittleEndian.Uint16(data[8:10])
	}
	if length >= 12 {
		fanData.TargetRPM = binary.LittleEndian.Uint16(data[10:12])
	}

	// 解析挡位设置
	maxGear, setGear := m.parseGearSettings(fanData.GearSettings)
	fanData.MaxGear = maxGear
	fanData.SetGear = setGear

	fanData.WorkMode = m.parseWorkMode(fanData.CurrentMode)

	return fanData
}

// parseGearSettings 解析挡位设置
func (m *Manager) parseGearSettings(gearByte uint8) (maxGear, setGear string) {
	maxGearCode := (gearByte >> 4) & 0x0F
	setGearCode := gearByte & 0x0F

	maxGearMap := map[uint8]string{
		0x2: "标准",
		0x4: "强劲",
		0x6: "超频",
	}

	setGearMap := map[uint8]string{
		0x8: "静音",
		0xA: "标准",
		0xC: "强劲",
		0xE: "超频",
	}

	if val, ok := maxGearMap[maxGearCode]; ok {
		maxGear = val
	} else {
		maxGear = fmt.Sprintf("未知(0x%X)", maxGearCode)
	}

	if val, ok := setGearMap[setGearCode]; ok {
		setGear = val
	} else {
		setGear = fmt.Sprintf("未知(0x%X)", setGearCode)
	}

	return
}

// parseWorkMode 解析工作模式
func (m *Manager) parseWorkMode(mode uint8) string {
	switch mode {
	case 0x04, 0x02, 0x06, 0x0A, 0x08, 0x00:
		return "挡位工作模式"
	case 0x05, 0x03, 0x07, 0x0B, 0x09, 0x01:
		return "自动模式(实时转速)"
	default:
		return fmt.Sprintf("未知模式(0x%02X)", mode)
	}
}

// SetFanSpeed 设置风扇转速
func (m *Manager) SetFanSpeed(rpm int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	if rpm < 0 || rpm > 4000 {
		return false
	}

	// 首先进入实时转速模式
	enterModeCmd := []byte{0x02, 0x5A, 0xA5, 0x23, 0x02, 0x25, 0x00}
	// 补齐到23字节
	enterModeCmd = append(enterModeCmd, make([]byte, 23-len(enterModeCmd))...)

	_, err := m.device.Write(enterModeCmd)
	if err != nil {
		m.logError("进入实时转速模式失败: %v", err)
		return false
	}

	time.Sleep(50 * time.Millisecond)

	// 构造转速设置命令
	speedBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(speedBytes, uint16(rpm))

	// 计算校验和
	checksum := (0x5A + 0xA5 + 0x21 + 0x04 + int(speedBytes[0]) + int(speedBytes[1]) + 1) & 0xFF

	cmd := []byte{0x02, 0x5A, 0xA5, 0x21, 0x04}
	cmd = append(cmd, speedBytes...)
	cmd = append(cmd, byte(checksum))
	// 补齐到23字节
	cmd = append(cmd, make([]byte, 23-len(cmd))...)

	_, err = m.device.Write(cmd)
	if err != nil {
		m.logError("设置风扇转速失败: %v", err)
		return false
	}

	m.logDebug("已设置风扇转速: %d RPM", rpm)
	return true
}

// SetCustomFanSpeed 设置自定义风扇转速（无限制）
func (m *Manager) SetCustomFanSpeed(rpm int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	m.logWarn("警告：设置自定义转速 %d RPM（无上下限限制）", rpm)

	enterModeCmd := []byte{0x02, 0x5A, 0xA5, 0x23, 0x02, 0x25, 0x00}
	enterModeCmd = append(enterModeCmd, make([]byte, 23-len(enterModeCmd))...)

	_, err := m.device.Write(enterModeCmd)
	if err != nil {
		m.logError("进入实时转速模式失败: %v", err)
		return false
	}

	time.Sleep(50 * time.Millisecond)

	speedBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(speedBytes, uint16(rpm))

	// 计算校验和
	checksum := (0x5A + 0xA5 + 0x21 + 0x04 + int(speedBytes[0]) + int(speedBytes[1]) + 1) & 0xFF

	cmd := []byte{0x02, 0x5A, 0xA5, 0x21, 0x04}
	cmd = append(cmd, speedBytes...)
	cmd = append(cmd, byte(checksum))
	cmd = append(cmd, make([]byte, 23-len(cmd))...)

	_, err = m.device.Write(cmd)
	if err != nil {
		m.logError("设置自定义风扇转速失败: %v", err)
		return false
	}

	m.logInfo("已设置自定义风扇转速: %d RPM", rpm)
	return true
}

// EnterAutoMode 进入自动模式
func (m *Manager) EnterAutoMode() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return fmt.Errorf("设备未连接")
	}

	// 发送进入实时转速模式的命令
	enterModeCmd := []byte{0x02, 0x5A, 0xA5, 0x23, 0x02, 0x25, 0x00}
	// 补齐到23字节
	enterModeCmd = append(enterModeCmd, make([]byte, 23-len(enterModeCmd))...)

	_, err := m.device.Write(enterModeCmd)
	if err != nil {
		return fmt.Errorf("进入自动模式失败: %v", err)
	}

	m.logInfo("已切换到自动模式，开始智能变频")
	return nil
}

// SetManualGear 设置手动挡位
func (m *Manager) SetManualGear(gear, level string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isConnected || m.device == nil {
		return false
	}

	commands, exists := types.GearCommands[gear]
	if !exists {
		m.logError("未找到挡位 %s 的命令", gear)
		return false
	}

	var selectedCommand *types.GearCommand
	for i := range commands {
		cmd := &commands[i]
		switch level {
		case "低":
			if strings.Contains(cmd.Name, "低") {
				selectedCommand = cmd
			}
		case "中":
			if strings.Contains(cmd.Name, "中") {
				selectedCommand = cmd
			}
		case "高":
			if strings.Contains(cmd.Name, "高") {
				selectedCommand = cmd
			}
		}
		if selectedCommand != nil {
			break
		}
	}

	if selectedCommand == nil {
		m.logError("未找到挡位 %s %s 的命令", gear, level)
		return false
	}

	// 发送命令，确保第一个字节是ReportID
	cmdWithReportID := append([]byte{0x02}, selectedCommand.Command...)

	_, err := m.device.Write(cmdWithReportID)
	if err != nil {
		m.logError("设置挡位 %s %s 失败: %v", gear, level, err)
		return false
	}

	m.logInfo("设置挡位成功: %s %s (目标转速: %d RPM)", gear, level, selectedCommand.RPM)
	return true
}

// SetGearLight 设置挡位灯
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

	// 补齐到23字节
	cmd = append(cmd, make([]byte, 23-len(cmd))...)

	_, err := m.device.Write(cmd)
	if err != nil {
		m.logError("设置挡位灯失败: %v", err)
		return false
	}

	return true
}

// SetPowerOnStart 设置通电自启动
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

	// 补齐到23字节
	cmd = append(cmd, make([]byte, 23-len(cmd))...)

	_, err := m.device.Write(cmd)
	if err != nil {
		m.logError("设置通电自启动失败: %v", err)
		return false
	}

	return true
}

// SetSmartStartStop 设置智能启停
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

	// 补齐到23字节
	cmd = append(cmd, make([]byte, 23-len(cmd))...)

	_, err := m.device.Write(cmd)
	if err != nil {
		m.logError("设置智能启停失败: %v", err)
		return false
	}

	return true
}

// SetBrightness 设置亮度
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
		// 补齐到23字节
		cmd = append(cmd, make([]byte, 23-len(cmd))...)
	case 100:
		cmd = []byte{0x02, 0x5A, 0xA5, 0x43, 0x02, 0x45}
		// 补齐到23字节
		cmd = append(cmd, make([]byte, 23-len(cmd))...)
	default:
		return false
	}

	_, err := m.device.Write(cmd)
	if err != nil {
		m.logError("设置亮度失败: %v", err)
		return false
	}

	return true
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
