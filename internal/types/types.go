// Package types defines all shared types used in the BS2PRO controller application
package types

// FanCurvePoint represents a point on the fan curve
type FanCurvePoint struct {
	Temperature int `json:"temperature"` // Temperature in °C
	RPM         int `json:"rpm"`         // Speed in RPM
}

// FanCurveProfile represents a fan curve profile
type FanCurveProfile struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Curve []FanCurvePoint `json:"curve"`
}

// FanCurveProfilesPayload is the payload returned for fan curve profiles
type FanCurveProfilesPayload struct {
	Profiles []FanCurveProfile `json:"profiles"`
	ActiveID string            `json:"activeId"`
}

// FanData represents the fan data structure
type FanData struct {
	ReportID     uint8  `json:"reportId"`
	MagicSync    uint16 `json:"magicSync"`
	Command      uint8  `json:"command"`
	Status       uint8  `json:"status"`
	GearSettings uint8  `json:"gearSettings"`
	CurrentMode  uint8  `json:"currentMode"`
	Reserved1    uint8  `json:"reserved1"`
	CurrentRPM   uint16 `json:"currentRpm"`
	TargetRPM    uint16 `json:"targetRpm"`
	MaxGear      string `json:"maxGear"`
	SetGear      string `json:"setGear"`
	WorkMode     string `json:"workMode"`
}

// GearCommand represents a gear command structure
type GearCommand struct {
	Name    string `json:"name"`    // Gear name
	Command []byte `json:"command"` // Command bytes
	RPM     int    `json:"rpm"`     // Corresponding RPM
}

// TemperatureData holds temperature data
type TemperatureData struct {
	CPUTemp    int    `json:"cpuTemp"`       // CPU temperature
	GPUTemp    int    `json:"gpuTemp"`       // GPU temperature
	MaxTemp    int    `json:"maxTemp"`       // Maximum temperature
	UpdateTime int64  `json:"updateTime"`    // Update timestamp
	BridgeOk   bool   `json:"bridgeOk"`      // Whether the bridge is working normally
	BridgeMsg  string `json:"bridgeMessage"` // Bridge fault message
}

// BridgeTemperatureData represents temperature data returned by the bridge program
type BridgeTemperatureData struct {
	CpuTemp    int    `json:"cpuTemp"`
	GpuTemp    int    `json:"gpuTemp"`
	MaxTemp    int    `json:"maxTemp"`
	UpdateTime int64  `json:"updateTime"`
	Success    bool   `json:"success"`
	Error      string `json:"error"`
}

// BridgeCommand represents a bridge program command
type BridgeCommand struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// BridgeResponse represents a bridge program response
type BridgeResponse struct {
	Success bool                   `json:"success"`
	Error   string                 `json:"error"`
	Data    *BridgeTemperatureData `json:"data"`
}

// RGBColor represents an RGB color
type RGBColor struct {
	R byte `json:"r"`
	G byte `json:"g"`
	B byte `json:"b"`
}

// LightStripConfig represents light strip configuration
type LightStripConfig struct {
	Mode       string     `json:"mode"`       // off/smart_temp/static_single/static_multi/rotation/flowing/breathing
	Speed      string     `json:"speed"`      // fast/medium/slow
	Brightness int        `json:"brightness"` // 0-100
	Colors     []RGBColor `json:"colors"`     // Color list
}

// SmartControlConfig represents smart temperature control configuration
type SmartControlConfig struct {
	Enabled            bool  `json:"enabled"`            // Smart coupled control switch
	Learning           bool  `json:"learning"`           // Learning switch
	TargetTemp         int   `json:"targetTemp"`         // Target temperature (°C)
	Aggressiveness     int   `json:"aggressiveness"`     // Response aggressiveness (1-10)
	Hysteresis         int   `json:"hysteresis"`         // Hysteresis temperature difference (°C)
	MinRPMChange       int   `json:"minRpmChange"`       // Minimum effective RPM change (RPM)
	RampUpLimit        int   `json:"rampUpLimit"`        // Maximum ramp-up per update (RPM)
	RampDownLimit      int   `json:"rampDownLimit"`      // Maximum ramp-down per update (RPM)
	LearnRate          int   `json:"learnRate"`          // Learning rate (1-10)
	LearnWindow        int   `json:"learnWindow"`        // Steady-state learning window (sample points)
	LearnDelay         int   `json:"learnDelay"`         // Learning delay steps (for thermal inertia)
	OverheatWeight     int   `json:"overheatWeight"`     // Overheat penalty weight
	RPMDeltaWeight     int   `json:"rpmDeltaWeight"`     // RPM change penalty weight
	NoiseWeight        int   `json:"noiseWeight"`        // High-RPM noise penalty weight
	TrendGain          int   `json:"trendGain"`          // Temperature rise trend feedforward gain
	MaxLearnOffset     int   `json:"maxLearnOffset"`     // Maximum learning offset (RPM)
	LearnedOffsets     []int `json:"learnedOffsets"`     // Learning offset for each curve point (RPM)
	LearnedOffsetsHeat []int `json:"learnedOffsetsHeat"` // Heating condition learning offsets (RPM)
	LearnedOffsetsCool []int `json:"learnedOffsetsCool"` // Cooling condition learning offsets (RPM)
	LearnedRateHeat    []int `json:"learnedRateHeat"`    // Heating rate learning biases (bucketed RPM)
	LearnedRateCool    []int `json:"learnedRateCool"`    // Cooling rate learning biases (bucketed RPM)
}

// AppConfig represents the application configuration
type AppConfig struct {
	AutoControl              bool               `json:"autoControl"`              // Smart control switch
	ManualGearToggleHotkey   string             `json:"manualGearToggleHotkey"`   // Hotkey to toggle manual gear
	AutoControlToggleHotkey  string             `json:"autoControlToggleHotkey"`  // Hotkey to toggle smart control
	CurveProfileToggleHotkey string             `json:"curveProfileToggleHotkey"` // Hotkey to toggle fan curve profile
	ManualGearLevels         map[string]string  `json:"manualGearLevels"`         // Remembered sub-level for each main gear (low/mid/high)
	FanCurve                 []FanCurvePoint    `json:"fanCurve"`                 // Fan curve
	FanCurveProfiles         []FanCurveProfile  `json:"fanCurveProfiles"`         // Fan curve profile list
	ActiveFanCurveProfileID  string             `json:"activeFanCurveProfileId"`  // Currently active curve profile ID
	GearLight                bool               `json:"gearLight"`                // Gear indicator light
	PowerOnStart             bool               `json:"powerOnStart"`             // Start on power on
	WindowsAutoStart         bool               `json:"windowsAutoStart"`         // Windows auto-start on boot
	SmartStartStop           string             `json:"smartStartStop"`           // Smart start/stop
	Brightness               int                `json:"brightness"`               // Brightness
	TempUpdateRate           int                `json:"tempUpdateRate"`           // Temperature update rate (seconds)
	TempSampleCount          int                `json:"tempSampleCount"`          // Temperature sample count (for averaging)
	ConfigPath               string             `json:"configPath"`               // Configuration file path
	ManualGear               string             `json:"manualGear"`               // Manual gear setting
	ManualLevel              string             `json:"manualLevel"`              // Manual gear level (low/mid/high)
	DebugMode                bool               `json:"debugMode"`                // Debug mode
	GuiMonitoring            bool               `json:"guiMonitoring"`            // GUI monitoring switch
	CustomSpeedEnabled       bool               `json:"customSpeedEnabled"`       // Custom speed switch
	CustomSpeedRPM           int                `json:"customSpeedRPM"`           // Custom speed value (no upper/lower limit)
	IgnoreDeviceOnReconnect  bool               `json:"ignoreDeviceOnReconnect"`  // Ignore device state after disconnect (keep app config)
	SmartControl             SmartControlConfig `json:"smartControl"`             // Learning-based smart temperature control config
	LightStrip               LightStripConfig   `json:"lightStrip"`               // Light strip configuration
}

// GetDefaultLightStripConfig returns the default light strip configuration
func GetDefaultLightStripConfig() LightStripConfig {
	return LightStripConfig{
		Mode:       "smart_temp",
		Speed:      "medium",
		Brightness: 100,
		Colors: []RGBColor{
			{R: 255, G: 0, B: 0},
			{R: 0, G: 255, B: 0},
			{R: 0, G: 128, B: 255},
		},
	}
}

// GetDefaultSmartControlConfig returns the default smart temperature control configuration
func GetDefaultSmartControlConfig(curve []FanCurvePoint) SmartControlConfig {
	offsets := make([]int, len(curve))
	heatOffsets := make([]int, len(curve))
	coolOffsets := make([]int, len(curve))
	heatRate := make([]int, 7)
	coolRate := make([]int, 7)

	return SmartControlConfig{
		Enabled:            true,
		Learning:           true,
		TargetTemp:         68,
		Aggressiveness:     5,
		Hysteresis:         2,
		MinRPMChange:       50,
		RampUpLimit:        220,
		RampDownLimit:      160,
		LearnRate:          4,
		LearnWindow:        6,
		LearnDelay:         2,
		OverheatWeight:     8,
		RPMDeltaWeight:     5,
		NoiseWeight:        4,
		TrendGain:          5,
		MaxLearnOffset:     600,
		LearnedOffsets:     offsets,
		LearnedOffsetsHeat: heatOffsets,
		LearnedOffsetsCool: coolOffsets,
		LearnedRateHeat:    heatRate,
		LearnedRateCool:    coolRate,
	}
}

// Logger is the logger interface
type Logger interface {
	Info(format string, v ...any)
	Error(format string, v ...any)
	Warn(format string, v ...any)
	Debug(format string, v ...any)
	Close()
	CleanOldLogs()
	SetDebugMode(enabled bool)
	GetLogDir() string
}

// GearCommands contains preset gear commands
var GearCommands = map[string][]GearCommand{
	"Silent": {
		{"Gear1-Low", []byte{0x5a, 0xa5, 0x26, 0x05, 0x00, 0x14, 0x05, 0x44, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 1300},
		{"Gear1-Mid", []byte{0x5a, 0xa5, 0x26, 0x05, 0x00, 0xa4, 0x06, 0xd5, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 1700},
		{"Gear1-High", []byte{0x5a, 0xa5, 0x26, 0x05, 0x00, 0x6c, 0x07, 0x9e, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 1900},
	},
	"Standard": {
		{"Gear2-Low", []byte{0x5a, 0xa5, 0x26, 0x05, 0x01, 0x34, 0x08, 0x68, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 2100},
		{"Gear2-Mid", []byte{0x5a, 0xa5, 0x26, 0x05, 0x01, 0x60, 0x09, 0x95, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 2310},
		{"Gear2-High", []byte{0x5a, 0xa5, 0x26, 0x05, 0x01, 0x8c, 0x0a, 0xc2, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 2760},
	},
	"Power": {
		{"Gear3-Low", []byte{0x5a, 0xa5, 0x26, 0x05, 0x02, 0xf0, 0x0a, 0x27, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 2800},
		{"Gear3-Mid", []byte{0x5a, 0xa5, 0x26, 0x05, 0x02, 0xb8, 0x0b, 0xf0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 3000},
		{"Gear3-High", []byte{0x5a, 0xa5, 0x26, 0x05, 0x02, 0xe4, 0x0c, 0x1d, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 3300},
	},
	"Overclock": {
		{"Gear4-Low", []byte{0x5a, 0xa5, 0x26, 0x05, 0x03, 0xac, 0x0d, 0xe7, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 3500},
		{"Gear4-Mid", []byte{0x5a, 0xa5, 0x26, 0x05, 0x03, 0x74, 0x0e, 0xb0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 3700},
		{"Gear4-High", []byte{0x5a, 0xa5, 0x26, 0x05, 0x03, 0xa0, 0x0f, 0xdd, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 4000},
	},
}

// GetDefaultFanCurve returns the default fan curve
func GetDefaultFanCurve() []FanCurvePoint {
	return []FanCurvePoint{
		{Temperature: 30, RPM: 1000},
		{Temperature: 35, RPM: 1200},
		{Temperature: 40, RPM: 1400},
		{Temperature: 45, RPM: 1600},
		{Temperature: 50, RPM: 1800},
		{Temperature: 55, RPM: 2000},
		{Temperature: 60, RPM: 2300},
		{Temperature: 65, RPM: 2600},
		{Temperature: 70, RPM: 2900},
		{Temperature: 75, RPM: 3200},
		{Temperature: 80, RPM: 3500},
		{Temperature: 85, RPM: 3800},
		{Temperature: 90, RPM: 4000},
		{Temperature: 95, RPM: 4000},
	}
}

// GetDefaultConfig returns the default configuration
func GetDefaultConfig(isAutoStart bool) AppConfig {
	defaultCurve := GetDefaultFanCurve()

	return AppConfig{
		AutoControl:              false,
		ManualGearToggleHotkey:   "Ctrl+Alt+Shift+M",
		AutoControlToggleHotkey:  "Ctrl+Alt+Shift+A",
		CurveProfileToggleHotkey: "Ctrl+Alt+Shift+C",
		ManualGearLevels: map[string]string{
			"Silent":    "Mid",
			"Standard":  "Mid",
			"Power":     "Mid",
			"Overclock": "Mid",
		},
		FanCurve: defaultCurve,
		FanCurveProfiles: []FanCurveProfile{
			{ID: "default", Name: "Default", Curve: defaultCurve},
		},
		ActiveFanCurveProfileID: "default",
		GearLight:               true,
		PowerOnStart:            false,
		WindowsAutoStart:        false,
		SmartStartStop:          "off",
		Brightness:              100,
		TempUpdateRate:          2,
		TempSampleCount:         1,
		ConfigPath:              "",
		ManualGear:              "Standard",
		ManualLevel:             "Mid",
		DebugMode:               false,
		GuiMonitoring:           true,
		CustomSpeedEnabled:      false,
		CustomSpeedRPM:          2000,
		IgnoreDeviceOnReconnect: true, // Enabled by default to prevent misjudging user manual switching after disconnect
		SmartControl:            GetDefaultSmartControlConfig(defaultCurve),
		LightStrip:              GetDefaultLightStripConfig(),
	}
}
