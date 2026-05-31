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
	"github.com/TIANLI0/THRM/internal/appmeta"
	"github.com/TIANLI0/THRM/internal/types"
)

// Manager 桥接程序管理器
type Manager struct {
	cmd       *exec.Cmd
	conn      net.Conn
	pipeName  string
	ownsCmd   bool
	state     string
	lastError string
	mutex     sync.Mutex
	logger    types.Logger
}

const (
	bridgeCommandTimeout  = 3 * time.Second
	BridgeStateNotStarted = "not_started"
	BridgeStateStarting   = "starting"
	BridgeStateRunning    = "running_owned"
	BridgeStateAttached   = "attached"
	BridgeStateDegraded   = "degraded"
	BridgeStateStopping   = "stopping"
	BridgeStateStopped    = "stopped"
	BridgeStateFailed     = "failed"
)

// NewManager 创建新的桥接程序管理器
func NewManager(logger types.Logger) *Manager {
	return &Manager{
		logger: logger,
		state:  BridgeStateNotStarted,
	}
}

func (m *Manager) setState(state string, err error) {
	m.state = state
	if err != nil {
		m.lastError = err.Error()
	} else if state == BridgeStateRunning || state == BridgeStateAttached || state == BridgeStateStopped || state == BridgeStateNotStarted {
		m.lastError = ""
	}
}

// EnsureRunning 确保桥接程序正在运行
func (m *Manager) EnsureRunning() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 已有连接时优先探活；共享桥接场景下 m.cmd 可以为空。
	if m.conn != nil {
		_, err := m.sendCommandUnsafe("Ping", "")
		if err == nil {
			if m.ownsCmd {
				m.setState(BridgeStateRunning, nil)
			} else {
				m.setState(BridgeStateAttached, nil)
			}
			return nil // 连接正常
		}
		m.logger.Warn("桥接程序连接异常，重新启动: %v", err)
		m.setState(BridgeStateDegraded, err)
		m.stopUnsafe()
	}

	// 状态不一致（仅有进程）时进行自愈清理，避免后续阻塞
	if m.cmd != nil {
		m.logger.Warn("检测到桥接程序状态不一致，执行清理后重启")
		m.setState(BridgeStateDegraded, fmt.Errorf("检测到桥接程序状态不一致"))
		m.stopUnsafe()
	}

	return m.start()
}

// start 启动桥接程序
func (m *Manager) start() error {
	m.setState(BridgeStateStarting, nil)

	if conn, pipeName, err := m.connectToAnyPipe(appmeta.BridgePipeCandidates(), 500*time.Millisecond); err == nil {
		m.conn = conn
		m.pipeName = pipeName
		m.ownsCmd = false
		m.setState(BridgeStateAttached, nil)
		m.logger.Info("复用已存在的桥接程序，管道名称: %s", pipeName)
		return nil
	}

	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		m.setState(BridgeStateFailed, err)
		return fmt.Errorf("获取程序目录失败: %v", err)
	}

	possiblePaths := appmeta.BridgeExecutableCandidates(exeDir)
	bridgePath := appmeta.FirstExistingPath(possiblePaths)

	// 检查桥接程序是否存在
	if bridgePath == "" {
		err := fmt.Errorf("%s 不存在，已尝试以下路径: %v", appmeta.BridgeExecutableName, possiblePaths)
		m.setState(BridgeStateFailed, err)
		return err
	}

	m.logger.Info("找到桥接程序: %s", bridgePath)

	// 启动桥接程序
	cmd := exec.Command(bridgePath, "--pipe")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	// 获取输出管道来读取管道名称
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建stdout管道失败: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建stderr管道失败: %v", err)
	}

	if err := cmd.Start(); err != nil {
		m.setState(BridgeStateFailed, err)
		return fmt.Errorf("启动桥接程序失败: %v", err)
	}

	go func() {
		scannerErr := bufio.NewScanner(stderr)
		for scannerErr.Scan() {
			line := strings.TrimSpace(scannerErr.Text())
			if line != "" {
				m.logger.Error("桥接程序stderr: %s", line)
			}
		}
		if err := scannerErr.Err(); err != nil {
			m.logger.Debug("读取桥接程序stderr失败: %v", err)
		}
	}()

	// 读取管道名称
	scanner := bufio.NewScanner(stdout)
	fmt.Printf("等待桥接程序输出管道名称...\n")
	var pipeName string
	var attachMode bool
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	done := make(chan struct{})
	go func() {
		if scanner.Scan() {
			line := scanner.Text()
			fmt.Printf("桥接程序输出: %s\n", line)
			if after, ok := strings.CutPrefix(line, "PIPE:"); ok {
				parts := strings.SplitN(after, "|", 2)
				pipeName = strings.TrimSpace(parts[0])
				if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "ATTACH") {
					attachMode = true
				}
			} else if after0, ok0 := strings.CutPrefix(line, "ERROR:"); ok0 {
				m.logger.Error("桥接程序启动错误: %s", after0)
			}
		}
		close(done)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				m.logger.Debug("桥接程序stdout: %s", line)
			}
		}

		if err := scanner.Err(); err != nil {
			m.logger.Debug("读取桥接程序stdout失败: %v", err)
		}
	}()

	select {
	case <-done:
		if pipeName == "" {
			cmd.Process.Kill()
			err := fmt.Errorf("未能获取管道名称")
			m.setState(BridgeStateFailed, err)
			return err
		}
	case <-timeout.C:
		cmd.Process.Kill()
		err := fmt.Errorf("等待桥接程序启动超时")
		m.setState(BridgeStateFailed, err)
		return err
	}

	// 连接到命名管道
	conn, err := m.connectToPipe(pipeName, 5*time.Second)
	if err != nil {
		cmd.Process.Kill()
		m.setState(BridgeStateFailed, err)
		return fmt.Errorf("连接管道失败: %v", err)
	}

	m.conn = conn
	m.pipeName = pipeName
	m.ownsCmd = !attachMode
	if attachMode {
		go func() {
			_ = cmd.Wait()
		}()
		m.setState(BridgeStateAttached, nil)
		m.logger.Info("桥接程序已存在，附着到共享实例，管道名称: %s", pipeName)
		return nil
	}

	m.cmd = cmd
	m.setState(BridgeStateRunning, nil)

	m.logger.Info("桥接程序启动成功，管道名称: %s", pipeName)
	return nil
}

func (m *Manager) connectToAnyPipe(pipeNames []string, timeout time.Duration) (net.Conn, string, error) {
	var lastErr error
	for _, pipeName := range pipeNames {
		conn, err := m.connectToPipe(pipeName, timeout)
		if err == nil {
			return conn, pipeName, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("未找到可用桥接管道")
	}
	return nil, "", lastErr
}

// connectToPipe 连接到命名管道 (使用go-winio实现)
//
// 重试策略：指数退避 100ms → 200ms → 400ms → 800ms（上限 1000ms）。
func (m *Manager) connectToPipe(pipeName string, timeout time.Duration) (net.Conn, error) {
	pipePath := `\\.\pipe\` + pipeName
	deadline := time.Now().Add(timeout)
	retryCount := 0
	backoff := 100 * time.Millisecond
	const maxBackoff = 1000 * time.Millisecond

	m.logger.Debug("尝试连接到管道: %s", pipePath)

	for time.Now().Before(deadline) {
		conn, err := winio.DialPipe(pipePath, &timeout)
		if err == nil {
			m.logger.Info("成功连接到管道，重试次数: %d", retryCount)
			return conn, nil
		}

		retryCount++
		if retryCount%5 == 0 {
			m.logger.Debug("连接管道重试中... 第%d次尝试，错误: %v", retryCount, err)
		}

		time.Sleep(backoff)
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
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

	conn := m.conn

	if err := conn.SetDeadline(time.Now().Add(bridgeCommandTimeout)); err != nil {
		m.logger.Debug("设置桥接命令超时失败: %v", err)
	}
	defer func() {
		_ = conn.SetDeadline(time.Time{})
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
	_, err = conn.Write(append(cmdBytes, '\n'))
	if err != nil {
		m.setState(BridgeStateDegraded, err)
		m.closeConnUnsafe()
		return nil, fmt.Errorf("发送命令失败: %v", err)
	}

	reader := bufio.NewReader(conn)
	responseBytes, err := reader.ReadBytes('\n')
	if err != nil {
		m.setState(BridgeStateDegraded, err)
		m.closeConnUnsafe()
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var response types.BridgeResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		m.setState(BridgeStateDegraded, err)
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if m.ownsCmd {
		m.setState(BridgeStateRunning, nil)
	} else {
		m.setState(BridgeStateAttached, nil)
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
	m.setState(BridgeStateStopping, nil)
	ownedCmd := m.cmd
	ownsCmd := m.ownsCmd
	m.cmd = nil
	m.ownsCmd = false
	m.pipeName = ""

	if m.conn != nil {
		if ownsCmd {
			// 仅关闭当前实例自己启动的桥接进程，避免误杀共享桥接
			m.sendCommandUnsafe("Exit", "")
		}
		m.closeConnUnsafe()
	}

	if ownsCmd && ownedCmd != nil && ownedCmd.Process != nil {
		// 给程序一些时间来正常退出
		done := make(chan error, 1)
		go func() {
			done <- ownedCmd.Wait()
		}()

		select {
		case <-done:
			// 程序正常退出
		case <-time.After(3 * time.Second):
			// 强制杀死进程
			ownedCmd.Process.Kill()
		}
	}

	m.setState(BridgeStateStopped, nil)
}

// GetTemperature 从桥接程序读取温度
func (m *Manager) GetTemperature(selection types.TemperatureSelection) types.BridgeTemperatureData {
	selection = types.NormalizeTemperatureSelection(selection)
	selectionPayload, err := json.Marshal(selection)
	if err != nil {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   fmt.Sprintf("序列化温度选择配置失败: %v", err),
		}
	}

	if err := m.EnsureRunning(); err != nil {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   fmt.Sprintf("启动桥接程序失败: %v", err),
		}
	}

	// 通过管道发送温度请求
	response, err := m.SendCommand("GetTemperature", string(selectionPayload))
	if err != nil {
		// 尝试重启桥接程序
		m.Stop()
		return types.BridgeTemperatureData{
			Success: false,
			Error:   fmt.Sprintf("桥接程序通信失败: %v", err),
		}
	}

	if !response.Success {
		if response.Data != nil {
			result := *response.Data
			result.Success = false
			if strings.TrimSpace(response.Error) != "" {
				result.Error = response.Error
			}
			return result
		}
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
	m.mutex.Lock()
	state := m.state
	ownsCmd := m.ownsCmd
	pipeName := m.pipeName
	lastError := m.lastError
	m.mutex.Unlock()

	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return map[string]any{
			"exists": false,
			"error":  fmt.Sprintf("获取程序目录失败: %v", err),
			"state":  state,
		}
	}

	possiblePaths := appmeta.BridgeExecutableCandidates(exeDir)
	bridgePath := appmeta.FirstExistingPath(possiblePaths)

	if bridgePath == "" {
		return map[string]any{
			"exists":      false,
			"state":       state,
			"ownsProcess": ownsCmd,
			"pipeName":    pipeName,
			"lastError":   lastError,
			"triedPaths":  possiblePaths,
			"error":       fmt.Sprintf("%s 不存在", appmeta.BridgeExecutableName),
		}
	}

	testResult := m.GetTemperature(types.GetDefaultTemperatureSelection())

	m.mutex.Lock()
	state = m.state
	ownsCmd = m.ownsCmd
	pipeName = m.pipeName
	lastError = m.lastError
	m.mutex.Unlock()

	return map[string]any{
		"exists":      true,
		"path":        bridgePath,
		"working":     testResult.Success,
		"state":       state,
		"ownsProcess": ownsCmd,
		"pipeName":    pipeName,
		"lastError":   lastError,
		"testData":    testResult,
	}
}

// RestartPawnIO 重启 PawnIO 驱动并重新初始化硬件监控
func (m *Manager) RestartPawnIO() (types.BridgeTemperatureData, error) {
	if err := m.EnsureRunning(); err != nil {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   fmt.Sprintf("启动桥接程序失败: %v", err),
		}, err
	}

	m.logger.Info("正在通过桥接程序重启 PawnIO 驱动...")
	response, err := m.SendCommand("RestartPawnIO", "")
	if err != nil {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   fmt.Sprintf("发送 RestartPawnIO 命令失败: %v", err),
		}, err
	}

	if !response.Success {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   response.Error,
		}, fmt.Errorf("RestartPawnIO 失败: %s", response.Error)
	}

	result := types.BridgeTemperatureData{Success: true}
	if response.Data != nil {
		result = *response.Data
	}
	m.logger.Info("PawnIO 驱动重启成功")
	return result, nil
}
