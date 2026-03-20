// Package temperature provides temperature reading functionality
package temperature

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/TIANLI0/BS2PRO-Controller/internal/bridge"
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
	"github.com/shirou/gopsutil/v4/sensors"
)

// Reader is a temperature reader
type Reader struct {
	bridgeManager *bridge.Manager
	logger        types.Logger
}

// NewReader creates a new temperature reader
func NewReader(bridgeManager *bridge.Manager, logger types.Logger) *Reader {
	return &Reader{
		bridgeManager: bridgeManager,
		logger:        logger,
	}
}

// Read reads the temperature
func (r *Reader) Read() types.TemperatureData {
	temp := types.TemperatureData{
		UpdateTime: time.Now().Unix(),
		BridgeOk:   true,
	}

	// Prefer using the bridge program to read temperature
	bridgeTemp := r.bridgeManager.GetTemperature()
	if bridgeTemp.Success {
		if bridgeTemp.CpuTemp == 0 && bridgeTemp.GpuTemp == 0 {
			temp.BridgeOk = false
			temp.BridgeMsg = "Bridge returned empty temperature (both CPU/GPU are 0). Please restart the software or reinstall the PawnIO driver."
			r.logger.Warn("Bridge returned empty temperature data, using fallback method")

			temp.CPUTemp = r.readCPUTemperature()
			temp.GPUTemp = r.readGPUTemperature()
			temp.MaxTemp = max(temp.CPUTemp, temp.GPUTemp)
			return temp
		}

		temp.CPUTemp = bridgeTemp.CpuTemp
		temp.GPUTemp = bridgeTemp.GpuTemp
		temp.MaxTemp = bridgeTemp.MaxTemp
		temp.BridgeOk = true
		temp.BridgeMsg = ""
		return temp
	}

	// If the bridge program fails, use fallback method
	r.logger.Warn("Bridge failed to read temperature: %s, using fallback method", bridgeTemp.Error)
	temp.BridgeOk = false
	temp.BridgeMsg = bridgeTemp.Error
	if strings.TrimSpace(temp.BridgeMsg) == "" {
		temp.BridgeMsg = "Failed to read CPU/GPU temperature. Please check if PawnIO is installed correctly, or upgrade to the latest version."
	}

	// Read CPU temperature
	temp.CPUTemp = r.readCPUTemperature()

	// Read GPU temperature
	temp.GPUTemp = r.readGPUTemperature()

	// Calculate maximum temperature
	temp.MaxTemp = max(temp.CPUTemp, temp.GPUTemp)

	return temp
}

// readCPUTemperature reads the CPU temperature
func (r *Reader) readCPUTemperature() int {
	sensorTemps, err := sensors.SensorsTemperatures()
	if err == nil {
		for _, sensor := range sensorTemps {
			// Look for ACPI ThermalZone TZ00_0 or similar CPU temperature sensors
			if strings.Contains(strings.ToLower(sensor.SensorKey), "tz00") ||
				strings.Contains(strings.ToLower(sensor.SensorKey), "cpu") ||
				strings.Contains(strings.ToLower(sensor.SensorKey), "core") {
				return int(sensor.Temperature)
			}
		}
	}

	// If sensor method fails, try via WMI (Windows)
	return r.readWindowsCPUTemp()
}

// readGPUTemperature reads the GPU temperature
func (r *Reader) readGPUTemperature() int {
	vendor := r.detectGPUVendor()
	return r.readGPUTempByVendor(vendor)
}

// readWindowsCPUTemp reads Windows CPU temperature via WMI
func (r *Reader) readWindowsCPUTemp() int {
	output, err := execCommandHidden("wmic", "/namespace:\\\\root\\wmi", "PATH", "MSAcpi_ThermalZoneTemperature", "get", "CurrentTemperature", "/value")
	if err != nil {
		r.logger.Debug("Failed to read Windows CPU temperature: %v", err)
		return 0
	}

	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "CurrentTemperature="); ok {
			tempStr := after
			tempStr = strings.TrimSpace(tempStr)
			if tempStr != "" {
				if temp, err := strconv.Atoi(tempStr); err == nil {
					celsius := (temp - 2732) / 10
					if celsius > 0 && celsius < 150 {
						return celsius
					}
				}
			}
		}
	}

	return 0
}

// detectGPUVendor detects the GPU vendor
func (r *Reader) detectGPUVendor() string {
	// Try NVIDIA
	if _, err := execCommandHidden("nvidia-smi", "--version"); err == nil {
		return "nvidia"
	}

	return "unknown"
}

// readGPUTempByVendor reads GPU temperature based on vendor
func (r *Reader) readGPUTempByVendor(vendor string) int {
	switch vendor {
	case "nvidia":
		return r.readNvidiaGPUTemp()
	case "amd":
		return 0
	default:
		return 0
	}
}

// readNvidiaGPUTemp safely reads NVIDIA GPU temperature
func (r *Reader) readNvidiaGPUTemp() int {
	output, err := execCommandHidden("nvidia-smi", "--query-gpu=temperature.gpu", "--format=csv,noheader,nounits")
	if err != nil {
		r.logger.Debug("Failed to read NVIDIA GPU temperature: %v", err)
		return 0
	}

	tempStr := strings.TrimSpace(string(output))
	lines := strings.Split(tempStr, "\n")

	if len(lines) > 0 && lines[0] != "" {
		if temp, err := strconv.Atoi(lines[0]); err == nil {
			return temp
		}
	}

	return 0
}

// execCommandHidden executes a command with a hidden window
func execCommandHidden(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	return cmd.Output()
}

// CalculateTargetRPM calculates target RPM based on temperature
func CalculateTargetRPM(temperature int, fanCurve []types.FanCurvePoint) int {
	if len(fanCurve) < 2 {
		return 0
	}

	if temperature <= fanCurve[0].Temperature {
		return fanCurve[0].RPM
	}

	lastPoint := fanCurve[len(fanCurve)-1]
	if temperature >= lastPoint.Temperature {
		return lastPoint.RPM
	}

	// Calculate RPM using linear interpolation
	for i := 0; i < len(fanCurve)-1; i++ {
		p1 := fanCurve[i]
		p2 := fanCurve[i+1]

		if temperature >= p1.Temperature && temperature <= p2.Temperature {
			// Linear interpolation
			ratio := float64(temperature-p1.Temperature) / float64(p2.Temperature-p1.Temperature)
			rpm := float64(p1.RPM) + ratio*float64(p2.RPM-p1.RPM)
			return int(rpm)
		}
	}

	return 0
}
