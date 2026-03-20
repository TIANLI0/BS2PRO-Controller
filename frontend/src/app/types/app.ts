// App type definitions

// Fan curve point
export interface FanCurvePoint {
  temperature: number; // Temperature °C
  rpm: number;         // Speed RPM
}

export interface FanCurveProfile {
  id: string;
  name: string;
  curve: FanCurvePoint[];
}

// Fan data structure
export interface FanData {
  reportId: number;
  magicSync: number;
  command: number;
  status: number;
  gearSettings: number;
  currentMode: number;
  reserved1: number;
  currentRpm: number;
  targetRpm: number;
  maxGear: string;
  setGear: string;
  workMode: string;
}

// Temperature data
export interface TemperatureData {
  cpuTemp: number;     // CPU temperature
  gpuTemp: number;     // GPU temperature
  maxTemp: number;     // Max temperature
  updateTime: number;  // Update timestamp
  bridgeOk?: boolean;  // Bridge program status
  bridgeMessage?: string; // Bridge program message
}

// App configuration
export interface AppConfig {
  autoControl: boolean;         // Smart speed control switch
  curveProfileToggleHotkey?: string; // Curve profile toggle hotkey
  fanCurve: FanCurvePoint[];   // Fan curve
  fanCurveProfiles?: FanCurveProfile[];
  activeFanCurveProfileId?: string;
  gearLight: boolean;          // Gear indicator light
  powerOnStart: boolean;       // Power-on auto start
  windowsAutoStart: boolean;   // Windows boot auto start
  smartStartStop: string;      // Smart start/stop
  brightness: number;          // Brightness
  tempUpdateRate: number;      // Temperature update rate (seconds)
  configPath: string;          // Config file path
  manualGear: string;          // Manual gear setting
  manualLevel: string;         // Manual gear level (low/mid/high)
  debugMode: boolean;          // Debug mode
  guiMonitoring: boolean;      // GUI monitoring switch
  customSpeedEnabled: boolean; // Custom speed switch
  customSpeedRPM: number;      // Custom speed value (no limits)
  smartControl: SmartControlConfig; // Learning smart temperature control
}

export interface SmartControlConfig {
  enabled: boolean;
  learning: boolean;
  targetTemp: number;
  aggressiveness: number;
  hysteresis: number;
  minRpmChange: number;
  rampUpLimit: number;
  rampDownLimit: number;
  learnRate: number;
  learnWindow: number;
  learnDelay: number;
  overheatWeight: number;
  rpmDeltaWeight: number;
  noiseWeight: number;
  trendGain: number;
  maxLearnOffset: number;
  learnedOffsets: number[];
  learnedOffsetsHeat: number[];
  learnedOffsetsCool: number[];
  learnedRateHeat: number[];
  learnedRateCool: number[];
}

// Debug info
export interface DebugInfo {
  debugMode: boolean;
  trayReady: boolean;
  trayInitialized: boolean;
  isConnected: boolean;
  guiLastResponse: string;
  monitoringTemp: boolean;
  autoStartLaunch: boolean;
}

// Auto-start method
export type AutoStartMethod = 'none' | 'task_scheduler' | 'registry';

// Auto-start info
export interface AutoStartInfo {
  enabled: boolean;
  method: AutoStartMethod;
  isAdmin: boolean;
}

// Gear command
export interface GearCommand {
  name: string;    // Gear name
  command: number[]; // Command bytes
  rpm: number;     // Corresponding speed
}

// Device status
export interface DeviceStatus {
  connected: boolean;
  monitoring: boolean;
  currentData: FanData | null;
  temperature: TemperatureData;
  productId?: string;
  model?: string;
}

// Device info
export interface DeviceInfo {
  manufacturer: string;
  product: string;
  serial: string;
  model?: string;
  productId?: string;
}
