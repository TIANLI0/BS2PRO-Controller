//go:build windows

package fnqpowermode

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// DetectSupport 检测当前主机是否支持 Lenovo Legion Fn+Q 插件。
func DetectSupport() (bool, HostInfo, error) {
	if _, err := exec.LookPath("powershell.exe"); err != nil {
		return false, HostInfo{}, fmt.Errorf("powershell.exe not found: %w", err)
	}

	cmd := newPowerShellCommand("-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", detectHostScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, HostInfo{}, fmt.Errorf("query Lenovo host info failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	var info HostInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return false, HostInfo{}, fmt.Errorf("decode Lenovo host info failed: %w", err)
	}

	info.Manufacturer = strings.TrimSpace(info.Manufacturer)
	info.Model = strings.TrimSpace(info.Model)
	info.Family = strings.TrimSpace(info.Family)
	info.Product = strings.TrimSpace(info.Product)
	info.Version = strings.TrimSpace(info.Version)
	info.Vendor = strings.TrimSpace(info.Vendor)

	return isSupportedHost(info), info, nil
}

func isSupportedHost(info HostInfo) bool {
	manufacturer := strings.ToUpper(info.Manufacturer)
	vendor := strings.ToUpper(info.Vendor)
	if !strings.Contains(manufacturer, "LENOVO") && !strings.Contains(vendor, "LENOVO") {
		return false
	}

	for _, field := range []string{info.Model, info.Family, info.Product, info.Version} {
		if strings.Contains(strings.ToUpper(field), "LEGION") {
			return true
		}
	}

	return false
}

const detectHostScript = `
$ErrorActionPreference = 'Stop'

$system = Get-WmiObject -Class Win32_ComputerSystem -ErrorAction Stop | Select-Object -First 1 Manufacturer, Model, SystemFamily
$product = Get-WmiObject -Class Win32_ComputerSystemProduct -ErrorAction SilentlyContinue | Select-Object -First 1 Name, Version, Vendor

[pscustomobject]@{
    manufacturer = [string]$system.Manufacturer
    model = [string]$system.Model
    family = [string]$system.SystemFamily
    product = [string]$product.Name
    version = [string]$product.Version
    vendor = [string]$product.Vendor
} | ConvertTo-Json -Compress
`
