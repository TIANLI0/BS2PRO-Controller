// Package config provides configuration management functionality
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
)

// Manager configuration manager
type Manager struct {
	config     types.AppConfig
	installDir string
	logger     types.Logger
}

// NewManager creates a new configuration manager
func NewManager(installDir string, logger types.Logger) *Manager {
	return &Manager{
		installDir: installDir,
		logger:     logger,
	}
}

// Load loads the configuration
func (m *Manager) Load(isAutoStart bool) types.AppConfig {
	// Try loading config from default directory first
	defaultConfigDir := m.GetDefaultConfigDir()
	defaultConfigPath := filepath.Join(defaultConfigDir, "config.json")

	installConfigPath := filepath.Join(m.installDir, "config", "config.json")

	m.logInfo("Trying to load config from default directory: %s", defaultConfigPath)

	// Try loading from default directory first
	if m.tryLoadFromPath(defaultConfigPath) {
		m.config.ConfigPath = defaultConfigPath
		m.logInfo("Config loaded from default directory: %s", defaultConfigPath)
		return m.config
	}

	m.logInfo("Failed to load from default directory, trying install directory: %s", installConfigPath)

	// Default directory failed, try install directory
	if m.tryLoadFromPath(installConfigPath) {
		m.config.ConfigPath = installConfigPath
		m.logInfo("Config loaded from install directory: %s", installConfigPath)
		return m.config
	}

	m.logError("All config directories failed, using default config")

	m.config = types.GetDefaultConfig(isAutoStart)
	m.config.ConfigPath = defaultConfigPath
	if err := m.Save(); err != nil {
		m.logError("Failed to save default config: %v", err)
	}

	return m.config
}

// tryLoadFromPath tries to load config from the specified path
func (m *Manager) tryLoadFromPath(configPath string) bool {
	if _, err := os.Stat(configPath); err != nil {
		m.logDebug("Config file does not exist: %s", configPath)
		return false
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		m.logError("Failed to read config file %s: %v", configPath, err)
		return false
	}

	var config types.AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		m.logError("Failed to parse config file %s: %v", configPath, err)
		return false
	}

	m.config = config
	return true
}

// Save saves the configuration
func (m *Manager) Save() error {
	// Try saving to default directory first
	defaultConfigDir := m.GetDefaultConfigDir()
	defaultConfigPath := filepath.Join(defaultConfigDir, "config.json")

	m.logDebug("Trying to save config to default directory: %s", defaultConfigPath)

	// Ensure default config directory exists
	if err := os.MkdirAll(defaultConfigDir, 0755); err != nil {
		m.logError("Failed to create default config directory: %v", err)
	} else {
		data, err := json.MarshalIndent(m.config, "", "  ")
		if err != nil {
			m.logError("Failed to serialize config: %v", err)
		} else {
			if err := os.WriteFile(defaultConfigPath, data, 0644); err != nil {
				m.logError("Failed to save config to default directory: %v", err)
			} else {
				m.config.ConfigPath = defaultConfigPath
				m.logInfo("Config saved to default directory: %s", defaultConfigPath)
				return nil
			}
		}
	}

	installConfigDir := filepath.Join(m.installDir, "config")
	installConfigPath := filepath.Join(installConfigDir, "config.json")

	m.logInfo("Failed to save to default directory, trying install directory: %s", installConfigPath)

	if err := os.MkdirAll(installConfigDir, 0755); err != nil {
		m.logError("Failed to create install config directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		m.logError("Failed to serialize config: %v", err)
		return err
	}

	if err := os.WriteFile(installConfigPath, data, 0644); err != nil {
		m.logError("Failed to save config to install directory: %v", err)
		return err
	}

	m.config.ConfigPath = installConfigPath
	m.logInfo("Config saved to install directory: %s", installConfigPath)
	return nil
}

// GetDefaultConfigDir gets the default config directory
func (m *Manager) GetDefaultConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		m.logError("Failed to get user home directory: %v", err)
		return filepath.Join(m.installDir, "config")
	}
	return filepath.Join(homeDir, ".bs2pro-controller")
}

// Get gets the current config
func (m *Manager) Get() types.AppConfig {
	return m.config
}

// Set sets the config
func (m *Manager) Set(config types.AppConfig) {
	m.config = config
}

// Update updates and saves the config
func (m *Manager) Update(config types.AppConfig) error {
	m.config = config
	return m.Save()
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

// GetConfigDir gets the config directory (backward compatible)
func (m *Manager) GetConfigDir() string {
	return m.GetDefaultConfigDir()
}

// GetInstallDir gets the install directory
func GetInstallDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exePath)
}

// GetCurrentWorkingDir gets the current working directory
func GetCurrentWorkingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return dir
}

// ValidateFanCurve validates whether a fan curve is valid
func ValidateFanCurve(curve []types.FanCurvePoint) error {
	if len(curve) < 2 {
		return fmt.Errorf("fan curve requires at least 2 points")
	}

	for i := 1; i < len(curve); i++ {
		if curve[i].Temperature <= curve[i-1].Temperature {
			return fmt.Errorf("fan curve temperature points must be increasing")
		}
	}

	for i, point := range curve {
		if point.RPM < 0 || point.RPM > 4000 {
			return fmt.Errorf("fan curve point %d RPM out of range (0-4000)", i+1)
		}
	}

	return nil
}
