// Package bridge provides temperature bridge program management functionality
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

// Manager bridge program manager
type Manager struct {
	cmd      *exec.Cmd
	conn     net.Conn
	pipeName string
	ownsCmd  bool
	mutex    sync.Mutex
	logger   types.Logger
}

const (
	bridgeCommandTimeout = 3 * time.Second
	bridgePipeName       = "BS2PRO_TempBridge"
)

// NewManager creates a new bridge program manager
func NewManager(logger types.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// EnsureRunning ensures the bridge program is running
func (m *Manager) EnsureRunning() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Probe existing connection first; m.cmd can be nil in shared bridge scenarios.
	if m.conn != nil {
		_, err := m.sendCommandUnsafe("Ping", "")
		if err == nil {
			return nil // Connection is healthy
		}
		m.logger.Warn("Bridge program connection error, restarting: %v", err)
		m.stopUnsafe()
	}

	// Self-healing cleanup when state is inconsistent (process only), to avoid blocking
	if m.cmd != nil {
		m.logger.Warn("Inconsistent bridge program state detected, cleaning up and restarting")
		m.stopUnsafe()
	}

	return m.start()
}

// start starts the bridge program
func (m *Manager) start() error {
	if conn, err := m.connectToPipe(bridgePipeName, 500*time.Millisecond); err == nil {
		m.conn = conn
		m.pipeName = bridgePipeName
		m.ownsCmd = false
		m.logger.Info("Reusing existing bridge program, pipe name: %s", bridgePipeName)
		return nil
	}

	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return fmt.Errorf("failed to get program directory: %v", err)
	}

	possiblePaths := []string{
		filepath.Join(exeDir, "bridge", "TempBridge.exe"),       // Standard location: bridge directory alongside exe
		filepath.Join(exeDir, "..", "bridge", "TempBridge.exe"), // Bridge directory in parent directory
		filepath.Join(exeDir, "TempBridge.exe"),                 // Same directory as exe
	}

	var bridgePath string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			bridgePath = path
			break
		}
	}

	// Check if bridge program exists
	if bridgePath == "" {
		return fmt.Errorf("TempBridge.exe not found, tried the following paths: %v", possiblePaths)
	}

	m.logger.Info("Found bridge program: %s", bridgePath)

	// Start bridge program
	cmd := exec.Command(bridgePath, "--pipe")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	// Get output pipe to read pipe name
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start bridge program: %v", err)
	}

	go func() {
		scannerErr := bufio.NewScanner(stderr)
		for scannerErr.Scan() {
			line := strings.TrimSpace(scannerErr.Text())
			if line != "" {
				m.logger.Error("Bridge program stderr: %s", line)
			}
		}
		if err := scannerErr.Err(); err != nil {
			m.logger.Debug("Failed to read bridge program stderr: %v", err)
		}
	}()

	// Read pipe name
	scanner := bufio.NewScanner(stdout)
	fmt.Printf("Waiting for bridge program to output pipe name...\n")
	var pipeName string
	var attachMode bool
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	done := make(chan struct{})
	go func() {
		if scanner.Scan() {
			line := scanner.Text()
			fmt.Printf("Bridge program output: %s\n", line)
			if after, ok := strings.CutPrefix(line, "PIPE:"); ok {
				parts := strings.SplitN(after, "|", 2)
				pipeName = strings.TrimSpace(parts[0])
				if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "ATTACH") {
					attachMode = true
				}
			} else if after0, ok0 := strings.CutPrefix(line, "ERROR:"); ok0 {
				m.logger.Error("Bridge program startup error: %s", after0)
			}
		}
		close(done)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				m.logger.Debug("Bridge program stdout: %s", line)
			}
		}

		if err := scanner.Err(); err != nil {
			m.logger.Debug("Failed to read bridge program stdout: %v", err)
		}
	}()

	select {
	case <-done:
		if pipeName == "" {
			cmd.Process.Kill()
			return fmt.Errorf("failed to get pipe name")
		}
	case <-timeout.C:
		cmd.Process.Kill()
		return fmt.Errorf("timed out waiting for bridge program to start")
	}

	// Connect to named pipe
	conn, err := m.connectToPipe(pipeName, 5*time.Second)
	if err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to connect to pipe: %v", err)
	}

	m.conn = conn
	m.pipeName = pipeName
	m.ownsCmd = !attachMode
	if attachMode {
		go func() {
			_ = cmd.Wait()
		}()
		m.logger.Info("Bridge program already exists, attaching to shared instance, pipe name: %s", pipeName)
		return nil
	}

	m.cmd = cmd

	m.logger.Info("Bridge program started successfully, pipe name: %s", pipeName)
	return nil
}

// connectToPipe connects to a named pipe (using go-winio)
func (m *Manager) connectToPipe(pipeName string, timeout time.Duration) (net.Conn, error) {
	pipePath := `\\.\pipe\` + pipeName
	deadline := time.Now().Add(timeout)
	retryCount := 0

	m.logger.Debug("Attempting to connect to pipe: %s", pipePath)

	for time.Now().Before(deadline) {
		// Connect to named pipe using go-winio
		conn, err := winio.DialPipe(pipePath, &timeout)
		if err == nil {
			m.logger.Info("Successfully connected to pipe, retry count: %d", retryCount)
			return conn, nil
		}

		retryCount++
		if retryCount%50 == 0 { // Log every 5 seconds
			m.logger.Debug("Retrying pipe connection... attempt %d, error: %v", retryCount, err)
		}

		time.Sleep(100 * time.Millisecond)
	}

	return nil, fmt.Errorf("pipe connection timed out, total retries: %d, last error may be permissions or pipe not ready", retryCount)
}

// SendCommand sends a command to the bridge program
func (m *Manager) SendCommand(cmdType, data string) (*types.BridgeResponse, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.sendCommandUnsafe(cmdType, data)
}

// sendCommandUnsafe sends a command to the bridge program (lock-free version)
func (m *Manager) sendCommandUnsafe(cmdType, data string) (*types.BridgeResponse, error) {
	if m.conn == nil {
		return nil, fmt.Errorf("bridge program not connected")
	}

	conn := m.conn

	if err := conn.SetDeadline(time.Now().Add(bridgeCommandTimeout)); err != nil {
		m.logger.Debug("Failed to set bridge command timeout: %v", err)
	}
	defer func() {
		_ = conn.SetDeadline(time.Time{})
	}()

	cmd := types.BridgeCommand{
		Type: cmdType,
		Data: data,
	}

	// Serialize command
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize command: %v", err)
	}

	// Send command
	_, err = conn.Write(append(cmdBytes, '\n'))
	if err != nil {
		m.closeConnUnsafe()
		return nil, fmt.Errorf("failed to send command: %v", err)
	}

	reader := bufio.NewReader(conn)
	responseBytes, err := reader.ReadBytes('\n')
	if err != nil {
		m.closeConnUnsafe()
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var response types.BridgeResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return &response, nil
}

// closeConnUnsafe closes and cleans up the current connection (lock-free)
func (m *Manager) closeConnUnsafe() {
	if m.conn != nil {
		_ = m.conn.Close()
		m.conn = nil
	}
}

// Stop stops the bridge program
func (m *Manager) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.stopUnsafe()
}

// stopUnsafe stops the bridge program (lock-free)
func (m *Manager) stopUnsafe() {
	ownedCmd := m.cmd
	ownsCmd := m.ownsCmd
	m.cmd = nil
	m.ownsCmd = false
	m.pipeName = ""

	if m.conn != nil {
		if ownsCmd {
			// Only close bridge processes started by the current instance, to avoid killing shared bridges
			m.sendCommandUnsafe("Exit", "")
		}
		m.closeConnUnsafe()
	}

	if ownsCmd && ownedCmd != nil && ownedCmd.Process != nil {
		// Give the program some time to exit gracefully
		done := make(chan error, 1)
		go func() {
			done <- ownedCmd.Wait()
		}()

		select {
		case <-done:
			// Program exited normally
		case <-time.After(3 * time.Second):
			// Force kill process
			ownedCmd.Process.Kill()
		}
	}
}

// GetTemperature reads temperature from the bridge program
func (m *Manager) GetTemperature() types.BridgeTemperatureData {
	if err := m.EnsureRunning(); err != nil {
		return types.BridgeTemperatureData{
			Success: false,
			Error:   fmt.Sprintf("failed to start bridge program: %v", err),
		}
	}

	// Send temperature request via pipe
	response, err := m.SendCommand("GetTemperature", "")
	if err != nil {
		// Try to restart bridge program
		m.Stop()
		return types.BridgeTemperatureData{
			Success: false,
			Error:   fmt.Sprintf("bridge program communication failed: %v", err),
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
			Error:   "bridge program returned empty data",
		}
	}

	return *response.Data
}

// GetStatus gets the bridge program status
func (m *Manager) GetStatus() map[string]any {
	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return map[string]any{
			"exists": false,
			"error":  fmt.Sprintf("failed to get program directory: %v", err),
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
			"error":      "TempBridge.exe not found",
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
