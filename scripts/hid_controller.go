package main

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/sstallion/go-hid"
)

// Fan data structure
type FanData struct {
	ReportID     uint8  // Report ID
	MagicSync    uint16 // Sync header 0x5AA5
	Command      uint8  // Command code
	Status       uint8  // Status byte
	GearSettings uint8  // Max gear and set gear
	CurrentMode  uint8  // Current mode
	Reserved1    uint8  // Reserved byte
	CurrentRPM   uint16 // Fan real-time RPM
	TargetRPM    uint16 // Fan target RPM
	MaxGear      uint8  // Max gear (parsed from GearSettings)
	SetGear      uint8  // Set gear (parsed from GearSettings)
}

// Parse gear settings
func parseGearSettings(gearByte uint8) (maxGear, setGear string) {
	maxGearCode := (gearByte >> 4) & 0x0F
	setGearCode := gearByte & 0x0F

	// Mode mapping: 2=Standard, 4=Turbo, 6=Overclock
	maxGearMap := map[uint8]string{
		0x2: "Standard",
		0x4: "Turbo",
		0x6: "Overclock",
	}

	// Gear mapping: 8=Silent, A=Standard, C=Turbo, E=Overclock
	setGearMap := map[uint8]string{
		0x8: "Silent",
		0xA: "Standard",
		0xC: "Turbo",
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

// Parse work mode
func parseWorkMode(mode uint8) string {
	switch mode {
	case 0x04:
		return "Gear Operating Mode"
	case 0x05:
		return "Auto Mode (Real-time Speed)"
	default:
		return fmt.Sprintf("Unknown Mode(0x%02X)", mode)
	}
}

// Parse HID data packet
func parseFanData(data []byte, length int) *FanData {
	if length < 11 {
		fmt.Printf("Packet length insufficient, need at least 11 bytes, actual: %d\n", length)
		return nil
	}

	// Check sync header
	magic := binary.BigEndian.Uint16(data[1:3])
	if magic != 0x5AA5 {
		fmt.Printf("Sync header mismatch, expected: 0x5AA5, actual: 0x%04X\n", magic)
		return nil
	}

	fanData := &FanData{
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

	return fanData
}

// Display fan data
func displayFanData(fanData *FanData) {
	fmt.Println("\n=== Fan Data Parsed ===")
	fmt.Printf("Report ID: 0x%02X\n", fanData.ReportID)
	fmt.Printf("Sync Header: 0x%04X\n", fanData.MagicSync)
	fmt.Printf("Command Code: 0x%02X\n", fanData.Command)
	fmt.Printf("Status Byte: 0x%02X\n", fanData.Status)

	maxGear, setGear := parseGearSettings(fanData.GearSettings)
	fmt.Printf("Gear Settings: 0x%02X (Max Gear: %s, Set Gear: %s)\n",
		fanData.GearSettings, maxGear, setGear)

	fmt.Printf("Current Mode: %s (0x%02X)\n", parseWorkMode(fanData.CurrentMode), fanData.CurrentMode)
	fmt.Printf("Reserved Byte: 0x%02X\n", fanData.Reserved1)
	fmt.Printf("Fan Real-time RPM: %d RPM\n", fanData.CurrentRPM)
	fmt.Printf("Fan Target RPM: %d RPM\n", fanData.TargetRPM)
	fmt.Println("==================")
}

func main() {
	fmt.Println("HID Connection Test")

	// Initialize HID library
	err := hid.Init()
	if err != nil {
		fmt.Printf("Failed to initialize HID library: %v\n", err)
		return
	}
	defer func() {
		// Clean up HID library resources
		if err := hid.Exit(); err != nil {
			fmt.Printf("Failed to clean up HID library: %v\n", err)
		}
	}()

	// Target device vendor ID and product ID
	vendorID := uint16(0x37D7)  // Vendor ID: 0x37D7 (corrected from 0x137D7)
	productID := uint16(0x1002) // Product ID: 0x1002

	fmt.Printf("Connecting to device - Vendor ID: 0x%04X, Product ID: 0x%04X\n", vendorID, productID)

	// Open the first matching device directly, no enumeration needed
	device, err := hid.OpenFirst(vendorID, productID)
	if err != nil {
		fmt.Printf("Failed to open device: %v\n", err)
		return
	}
	defer func() {
		if err := device.Close(); err != nil {
			fmt.Printf("Failed to close device: %v\n", err)
		}
	}()

	fmt.Println("Device connected successfully!")

	// Get device information
	deviceInfo, err := device.GetDeviceInfo()
	if err != nil {
		fmt.Printf("Failed to get device info: %v\n", err)
	} else {
		fmt.Printf("Device Details:\n")
		fmt.Printf("  Manufacturer: %s\n", deviceInfo.MfrStr)
		fmt.Printf("  Product: %s\n", deviceInfo.ProductStr)
		fmt.Printf("  Serial Number: %s\n", deviceInfo.SerialNbr)
		fmt.Printf("  Release Number: 0x%04X\n", deviceInfo.ReleaseNbr)
	}

	// Try to read data (non-blocking mode)
	err = device.SetNonblock(true)
	if err != nil {
		fmt.Printf("Failed to set non-blocking mode: %v\n", err)
	}

	// Read example
	buffer := make([]byte, 64)
	fmt.Println("Attempting to read data (5 second timeout)...")

	n, err := device.ReadWithTimeout(buffer, 5*time.Second)
	if err != nil {
		if err == hid.ErrTimeout {
			fmt.Println("Read timed out, device may not be sending data")
		} else {
			fmt.Printf("Failed to read data: %v\n", err)
		}
	} else {
		fmt.Printf("Read %d bytes of data: ", n)
		for i := range n {
			fmt.Printf("%02X ", buffer[i])
		}
		fmt.Println()

		// Parse fan data
		if fanData := parseFanData(buffer, n); fanData != nil {
			displayFanData(fanData)
		}
	}

	// Send data example (if needed)
	// The first byte is usually the report ID; for devices supporting only a single report, it should be 0
	// outputData := []byte{0x00, 0x01, 0x02, 0x03} // Example data
	// n, err = device.Write(outputData)
	// if err != nil {
	//     fmt.Printf("Failed to send data: %v\n", err)
	// } else {
	//     fmt.Printf("Successfully sent %d bytes of data\n", n)
	// }

	fmt.Println("HID device operation completed")
}
