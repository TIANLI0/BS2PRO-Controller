package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/sensors"
)

func main() {
	// Get CPU information
	cpus, err := cpu.Info()
	if err != nil {
		fmt.Println("Error getting CPU info:", err)
		return
	}

	// Print CPU information
	for _, cpu := range cpus {
		fmt.Printf("CPU: %s\n", cpu.ModelName)
		fmt.Printf("Core Count: %d\n", cpu.Cores)
		fmt.Printf("MHz: %f\n", cpu.Mhz)
	}

	// Get CPU usage
	cpuPercent, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercent) > 0 {
		fmt.Printf("CPU Usage: %.2f%%\n", cpuPercent[0])
	}

	// Get host information (including temperature info if available)
	hostInfo, err := host.Info()
	if err == nil {
		fmt.Printf("Host: %s\n", hostInfo.Hostname)
		fmt.Printf("OS: %s\n", hostInfo.Platform)
		fmt.Printf("OS Version: %s\n", hostInfo.PlatformVersion)
	}

	// Try to get sensor information (may require admin privileges)
	fmt.Println("\n--- Sensor Information ---")
	sensors, err := sensors.SensorsTemperatures()
	if err != nil {
		fmt.Printf("Error getting sensor data: %v\n", err)
	} else {
		// Print sensor information
		for _, sensor := range sensors {
			fmt.Printf("Sensor: %s\n", sensor.SensorKey)
			fmt.Printf("Temperature: %.2f°C\n", sensor.Temperature)
		}
	}

	// Try to get GPU information
	fmt.Println("\n--- GPU Information ---")
	gpus, err := GetNvidiaGPUInfo()
	if err != nil {
		fmt.Printf("Error getting GPU info: %v\n", err)
	} else {
		// Print GPU information
		for _, gpu := range gpus {
			fmt.Printf("GPU: %s\n", gpu.Name)
			fmt.Printf("Temperature: %d°C\n", gpu.Temperature)
		}
	}

}

// GPUInfo represents information for a single GPU
type GPUInfo struct {
	Name        string `json:"name"`
	Temperature int    `json:"temperature"` // Unit: °C
}

// GetNvidiaGPUInfo uses nvidia-smi to get GPU name and temperature
// Returns a slice of GPUInfo, each element corresponding to a GPU
func GetNvidiaGPUInfo() ([]GPUInfo, error) {
	// Use nvidia-smi to query GPU name and temperature, CSV format output
	cmd := exec.Command("nvidia-smi",
		"--query-gpu=name,temperature.gpu",
		"--format=csv,noheader,nounits")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute nvidia-smi: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var gpus []GPUInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: "GPU Name", temperature
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected nvidia-smi output format: %s", line)
		}

		name := strings.TrimSpace(parts[0])
		tempStr := strings.TrimSpace(parts[1])

		temp, err := strconv.Atoi(tempStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse temperature '%s': %w", tempStr, err)
		}

		gpus = append(gpus, GPUInfo{
			Name:        name,
			Temperature: temp,
		})
	}

	return gpus, nil
}
