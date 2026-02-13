// Package bridge 提供温度桥接程序管理功能
package bridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
)

// Manager 桥接程序管理器
type Manager struct {
	cmd      *exec.Cmd
	conn     net.Conn
	pipeName string
	mutex    sync.Mutex
	logger   types.Logger
}

const (
	bridgeCommandTimeout = 3 * time.Second
)

// NewManager 创建新的桥接程序管理器
func NewManager(logger types.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// EnsureRunning 确保桥接程序正在运行
func (m *Manager) EnsureRunning() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查是否已经有连接
	if m.conn != nil && m.cmd != nil {
		_, err := m.sendCommandUnsafe("Ping", "")
		if err == nil {
			return nil // 连接正常
		}
		m.logger.Warn("桥接程序连接异常，重新启动: %v", err)
		m.stopUnsafe()
	}

	// 状态不一致（仅有进程或仅有连接）时进行自愈清理，避免后续阻塞
	if m.conn != nil || m.cmd != nil {
		m.logger.Warn("检测到桥接程序状态不一致，执行清理后重启")
		m.stopUnsafe()
	}

	return m.start()
}

// start 启动桥接程序
func (m *Manager) start() error {
	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return fmt.Errorf("获取程序目录失败: %v", err)
	}

	possiblePaths := []string{
		filepath.Join(exeDir, "bridge", "TempBridge.exe"),       // 标准位置: exe同级的bridge目录
		filepath.Join(exeDir, "..", "bridge", "TempBridge.exe"), // 上级目录的bridge目录
		filepath.Join(exeDir, "TempBridge.exe"),                 // exe同级目录
	}

	var bridgePath string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			bridgePath = path
			break
		}
	}

	// 检查桥接程序是否存在
	if bridgePath == "" {
		return fmt.Errorf("TempBridge.exe 不存在，已尝试以下路径: %v", possiblePaths)
	}

	m.logger.Info("找到桥接程序: %s", bridgePath)

	// 启动桥接程序
	cmd := exec.Command(bridgePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	// 获取输出管道来读取管道名称
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建stdout管道失败: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动桥接程序失败: %v", err)
	}

	// 读取管道名称
	scanner := bufio.NewScanner(stdout)
	fmt.Printf("等待桥接程序输出管道名称...\n")
	var pipeName string
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	done := make(chan bool)
	go func() {
		if scanner.Scan() {
			line := scanner.Text()
			fmt.Printf("桥接程序输出: %s\n", line)
			if after, ok := strings.CutPrefix(line, "PIPE:"); ok {
				pipeName = after
			} else if after0, ok0 := strings.CutPrefix(line, "ERROR:"); ok0 {
				m.logger.Error("桥接程序启动错误: %s", after0)
			}
		}
		done <- true
	}()

	select {
	case <-done:
		if pipeName == "" {
			cmd.Process.Kill()
			return fmt.Errorf("未能获取管道名称")
		}
	case <-timeout.C:
		cmd.Process.Kill()
		return fmt.Errorf("等待桥接程序启动超时")
	}

	// 连接到命名管道
	conn, err := m.connectToPipe(pipeName, 5*time.Second)
	if err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("连接管道失败: %v", err)
	}

	m.cmd = cmd
	m.conn = conn
	m.pipeName = pipeName

	m.logger.Info("桥接程序启动成功，管道名称: %s", pipeName)
	return nil
}

// connectToPipe 连接到命名管道 (使用go-winio实现)
func (m *Manager) connectToPipe(pipeName string, timeout time.Duration) (net.Conn, error) {
	pipePath := `\\.\pipe\` + pipeName
	deadline := time.Now().Add(timeout)
	retryCount := 0

	m.logger.Debug("尝试连接到管道: %s", pipePath)

	for time.Now().Before(deadline) {
		// 使用go-winio连接命名管道
		conn, err := winio.DialPipe(pipePath, &timeout)
		if err == nil {
			m.logger.Info("成功连接到管道，重试次数: %d", retryCount)
			return conn, nil
		}

		retryCount++
		if retryCount%50 == 0 { // 每5秒输出一次日志
			m.logger.Debug("连接管道重试中... 第%d次尝试，错误: %v", retryCount, err)
		}

		time.Sleep(100 * time.Millisecond)
	}

	return nil, fmt.Errorf("连接管道超时，总计重试%d次，最后错误可能是权限或管道未就绪", retryCount)
}

// SendCommand 发送命令到桥接程序
func (m *Manager) SendCommand(cmdType, data string) (*types.BridgeResponse, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.sendCommandUnsafe(cmdType, data)
}

// sendCommandUnsafe 发送命令到桥接程序（不加锁版本）
func (m *Manager) sendCommandUnsafe(cmdType, data string) (*types.BridgeResponse, error) {
	if m.conn == nil {
		return nil, fmt.Errorf("桥接程序未连接")
	}

	if err := m.conn.SetDeadline(time.Now().Add(bridgeCommandTimeout)); err != nil {
		m.logger.Debug("设置桥接命令超时失败: %v", err)
	}
	defer func() {
		_ = m.conn.SetDeadline(time.Time{})
	}()

	cmd := types.BridgeCommand{
		Type: cmdType,
		Data: data,
	}

	// 序列化命令
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("序列化命令失败: %v", err)
	}

	// 发送命令
	_, err = m.conn.Write(append(cmdBytes, '\n'))
	if err != nil {
		m.closeConnUnsafe()
		return nil, fmt.Errorf("发送命令失败: %v", err)
	}

	reader := bufio.NewReader(m.conn)
	responseBytes, err := reader.ReadBytes('\n')
	if err != nil {
		m.closeConnUnsafe()
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var response types.BridgeResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return &response, nil
}

// closeConnUnsafe 关闭并清理当前连接（不加锁）
func (m *Manager) closeConnUnsafe() {
	if m.conn != nil {
		_ = m.conn.Close()
		m.conn = nil
	}
}

// Stop 停止桥接程序
func (m *Manager) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.stopUnsafe()
}

// stopUnsafe 停止桥接程序（不加锁）
func (m *Manager) stopUnsafe() {
	if m.conn != nil {
		// 发送退出命令
		m.sendCommandUnsafe("Exit", "")
		m.closeConnUnsafe()
	}

	if m.cmd != nil && m.cmd.Process != nil {
		// 给程序一些时间来正常退出
		done := make(chan error, 1)
		go func() {
			done <- m.cmd.Wait()
		}()

		select {
		case <-done:
			// 程序正常退出
		case <-time.After(3 * time.Second):
			// 强制杀死进程
			m.cmd.Process.Kill()
		}

		m.cmd = nil
	}

	m.pipeName = ""
}

// GetTemperature 从桥接程序读取温度
func (m *Manager) GetTemperature() types.BridgeTemperatureData {
	if err := m.EnsureRunning(); err != nil {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   fmt.Sprintf("启动桥接程序失败: %v", err),
		}
	}

	// 通过管道发送温度请求
	response, err := m.SendCommand("GetTemperature", "")
	if err != nil {
		// 尝试重启桥接程序
		m.Stop()
		return types.BridgeTemperatureData{
			Success: false,
			Error:   fmt.Sprintf("桥接程序通信失败: %v", err),
		}
	}

	if !response.Success {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   response.Error,
		}
	}

	if response.Data == nil {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   "桥接程序返回空数据",
		}
	}

	return *response.Data
}

// GetStatus 获取桥接程序状态
func (m *Manager) GetStatus() map[string]any {
	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return map[string]any{
			"exists": false,
			"error":  fmt.Sprintf("获取程序目录失败: %v", err),
		}
	}

	possiblePaths := []string{
		filepath.Join(exeDir, "bridge", "TempBridge.exe"),
		filepath.Join(exeDir, "..", "bridge", "TempBridge.exe"),
		filepath.Join(exeDir, "TempBridge.exe"),
	}

	var bridgePath string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			bridgePath = path
			break
		}
	}

	if bridgePath == "" {
		return map[string]any{
			"exists":     false,
			"triedPaths": possiblePaths,
			"error":      "TempBridge.exe 不存在",
		}
	}

	testResult := m.GetTemperature()

	return map[string]any{
		"exists":   true,
		"path":     bridgePath,
		"working":  testResult.Success,
		"testData": testResult,
	}
}
