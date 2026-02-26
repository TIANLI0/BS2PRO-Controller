// 应用类型定义

// 风扇曲线点
export interface FanCurvePoint {
  temperature: number; // 温度 °C
  rpm: number;         // 转速 RPM
}

// 风扇数据结构
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

// 温度数据
export interface TemperatureData {
  cpuTemp: number;     // CPU温度
  gpuTemp: number;     // GPU温度
  maxTemp: number;     // 最高温度
  updateTime: number;  // 更新时间戳
  bridgeOk?: boolean;  // 桥接程序是否正常
  bridgeMessage?: string; // 桥接程序提示
}

// 应用配置
export interface AppConfig {
  autoControl: boolean;         // 智能变频开关
  fanCurve: FanCurvePoint[];   // 风扇曲线
  gearLight: boolean;          // 挡位灯
  powerOnStart: boolean;       // 通电自启动
  windowsAutoStart: boolean;   // Windows开机自启动
  smartStartStop: string;      // 智能启停
  brightness: number;          // 亮度
  tempUpdateRate: number;      // 温度更新频率(秒)
  configPath: string;          // 配置文件路径
  manualGear: string;          // 手动挡位设置
  manualLevel: string;         // 手动挡位级别(低中高)
  debugMode: boolean;          // 调试模式
  guiMonitoring: boolean;      // GUI监控开关
  customSpeedEnabled: boolean; // 自定义转速开关
  customSpeedRPM: number;      // 自定义转速值(无上下限)
  smartControl: SmartControlConfig; // 学习型智能控温
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
}

// 调试信息
export interface DebugInfo {
  debugMode: boolean;
  trayReady: boolean;
  trayInitialized: boolean;
  isConnected: boolean;
  guiLastResponse: string;
  monitoringTemp: boolean;
  autoStartLaunch: boolean;
}

// 自启动方式
export type AutoStartMethod = 'none' | 'task_scheduler' | 'registry';

// 自启动信息
export interface AutoStartInfo {
  enabled: boolean;
  method: AutoStartMethod;
  isAdmin: boolean;
}

// 挡位命令
export interface GearCommand {
  name: string;    // 挡位名称
  command: number[]; // 命令字节
  rpm: number;     // 对应转速
}

// 设备状态
export interface DeviceStatus {
  connected: boolean;
  monitoring: boolean;
  currentData: FanData | null;
  temperature: TemperatureData;
}

// 设备信息
export interface DeviceInfo {
  manufacturer: string;
  product: string;
  serial: string;
}
