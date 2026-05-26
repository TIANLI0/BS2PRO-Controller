// Wails API 服务封装
import { EventsOn } from '../../../wailsjs/runtime/runtime';
import { 
  ConnectDevice, 
  DisconnectDevice, 
  GetDeviceStatus,
  GetConfig,
  UpdateConfig,
  SetFanCurve,
  ResetLearnedOffsets,
  GetFanCurve,
  SetAutoControl,
  GetAppVersion,
  SetManualGear,
  GetAvailableGears,
  SetGearLight,
  SetPowerOnStart,
  SetSmartStartStop,
  SetBrightness,
  SetLightStrip,
  GetTemperature,
  GetTemperatureHistory,
  SetTemperatureHistoryEnabled,
  GetCurrentFanData,
  TestTemperatureReading,
  GetDebugInfo,
  SetDebugMode,
  UpdateGuiResponseTime,
  SetCustomSpeed
  // CheckWindowsAutoStart,
  // SetWindowsAutoStart
} from '../../../wailsjs/go/main/App';

import { types } from '../../../wailsjs/go/models';

import type {
  DeviceInfo,
  DebugInfo,
  LegionFnQSupportPayload,
  LegionPowerModePayload,
} from '../types/app';

class ApiService {
  // 设备连接
  async connectDevice(): Promise<boolean> {
    return await ConnectDevice();
  }

  async disconnectDevice(): Promise<void> {
    return await DisconnectDevice();
  }

  async getDeviceStatus(): Promise<any> {
    return await GetDeviceStatus();
  }

  // 配置管理
  async getConfig(): Promise<types.AppConfig> {
    return await GetConfig();
  }

  async getAppVersion(): Promise<string> {
    return await GetAppVersion();
  }

  async updateConfig(config: types.AppConfig): Promise<void> {
    return await UpdateConfig(config);
  }

  // 风扇曲线
  async setFanCurve(curve: types.FanCurvePoint[]): Promise<void> {
    return await SetFanCurve(curve);
  }

  // 清空学习到的曲线偏移；后端清零所有 LearnedOffsets。
  async resetLearnedOffsets(): Promise<void> {
    return await ResetLearnedOffsets();
  }

  async getFanCurve(): Promise<types.FanCurvePoint[]> {
    return await GetFanCurve();
  }

  async getFanCurveProfiles(): Promise<{ profiles: Array<{ id: string; name: string; curve: types.FanCurvePoint[] }>; activeId: string }> {
    return await (window as any).go?.main?.App?.GetFanCurveProfiles();
  }

  async setActiveFanCurveProfile(profileID: string): Promise<void> {
    return await (window as any).go?.main?.App?.SetActiveFanCurveProfile(profileID);
  }

  async saveFanCurveProfile(profileID: string, name: string, curve: types.FanCurvePoint[], setActive: boolean): Promise<{ id: string; name: string; curve: types.FanCurvePoint[] }> {
    return await (window as any).go?.main?.App?.SaveFanCurveProfile(profileID, name, curve, setActive);
  }

  async deleteFanCurveProfile(profileID: string): Promise<void> {
    return await (window as any).go?.main?.App?.DeleteFanCurveProfile(profileID);
  }

  async exportFanCurveProfiles(): Promise<string> {
    return await (window as any).go?.main?.App?.ExportFanCurveProfiles();
  }

  async importFanCurveProfiles(code: string): Promise<void> {
    return await (window as any).go?.main?.App?.ImportFanCurveProfiles(code);
  }

  // 智能变频
  async setAutoControl(enabled: boolean): Promise<void> {
    return await SetAutoControl(enabled);
  }

  // 自定义转速
  async setCustomSpeed(enabled: boolean, rpm: number): Promise<void> {
    return await SetCustomSpeed(enabled, rpm);
  }

  // 手动挡位控制
  async setManualGear(gear: string, level: string): Promise<boolean> {
    return await SetManualGear(gear, level);
  }

  // 获取可用挡位
  async getAvailableGears(): Promise<any> {
    return await GetAvailableGears();
  }

  // 设备设置
  async setGearLight(enabled: boolean): Promise<boolean> {
    return await SetGearLight(enabled);
  }

  async setPowerOnStart(enabled: boolean): Promise<boolean> {
    return await SetPowerOnStart(enabled);
  }

  async setSmartStartStop(mode: string): Promise<boolean> {
    return await SetSmartStartStop(mode);
  }

  async setBrightness(percentage: number): Promise<boolean> {
    return await SetBrightness(percentage);
  }

  async setLightStrip(config: types.LightStripConfig): Promise<void> {
    return await SetLightStrip(config);
  }

  // Windows自启动相关
  async checkWindowsAutoStart(): Promise<boolean> {
    // 临时使用window对象调用，等Wails生成绑定后更新
    return await (window as any).go?.main?.App?.CheckWindowsAutoStart();
  }

  async setWindowsAutoStart(enabled: boolean): Promise<void> {
    // 临时使用window对象调用，等Wails生成绑定后更新
    return await (window as any).go?.main?.App?.SetWindowsAutoStart(enabled);
  }

  async getAutoStartMethod(): Promise<string> {
    // 获取当前自启动方式
    return await (window as any).go?.main?.App?.GetAutoStartMethod();
  }

  async setAutoStartWithMethod(enabled: boolean, method: string): Promise<void> {
    // 使用指定方式设置自启动
    return await (window as any).go?.main?.App?.SetAutoStartWithMethod(enabled, method);
  }

  async isRunningAsAdmin(): Promise<boolean> {
    // 检查是否以管理员权限运行
    return await (window as any).go?.main?.App?.IsRunningAsAdmin();
  }

  // 数据获取
  async getTemperature(): Promise<types.TemperatureData> {
    return await GetTemperature();
  }

  async getTemperatureHistory(): Promise<types.TemperatureHistoryPayload> {
    return await GetTemperatureHistory();
  }

  async setTemperatureHistoryEnabled(enabled: boolean): Promise<void> {
    return await SetTemperatureHistoryEnabled(enabled);
  }

  async getCurrentFanData(): Promise<types.FanData | null> {
    return await GetCurrentFanData();
  }

  async testTemperatureReading(): Promise<types.TemperatureData> {
    return await TestTemperatureReading();
  }

  // 桥接程序相关
  async getBridgeProgramStatus(): Promise<any> {
    return await (window as any).go?.main?.App?.GetBridgeProgramStatus();
  }

  async testBridgeProgram(): Promise<any> {
    return await (window as any).go?.main?.App?.TestBridgeProgram();
  }

  async restartPawnIO(): Promise<any> {
    return await (window as any).go?.main?.App?.RestartPawnIO();
  }

  async reinstallPawnIO(): Promise<any> {
    return await (window as any).go?.main?.App?.ReinstallPawnIO();
  }

  // 事件监听
  onDeviceConnected(callback: (data: DeviceInfo) => void): () => void {
    return EventsOn('device-connected', callback);
  }

  onDeviceDisconnected(callback: () => void): () => void {
    return EventsOn('device-disconnected', callback);
  }

  onDeviceError(callback: (error: string) => void): () => void {
    return EventsOn('device-error', callback);
  }

  onFanDataUpdate(callback: (data: types.FanData) => void): () => void {
    return EventsOn('fan-data-update', callback);
  }

  onTemperatureUpdate(callback: (data: types.TemperatureData) => void): () => void {
    return EventsOn('temperature-update', callback);
  }

  onTemperatureHistoryUpdate(callback: (data: { timestamp: number; cpuTemp: number; gpuTemp: number; fanRpm?: number }) => void): () => void {
    return EventsOn('temperature-history-update', callback);
  }

  onConfigUpdate(callback: (config: types.AppConfig) => void): () => void {
    return EventsOn('config-update', callback);
  }

  onHotkeyTriggered(callback: (payload: { action: string; shortcut: string; success: boolean; message: string }) => void): () => void {
    return EventsOn('hotkey-triggered', callback);
  }

  // 调试相关方法
  onLegionPowerModeUpdate(callback: (payload: LegionPowerModePayload) => void): () => void {
    return EventsOn('legion-power-mode-update', callback);
  }

  onLegionFnQSupportUpdate(callback: (payload: LegionFnQSupportPayload) => void): () => void {
    return EventsOn('legion-fnq-support-update', callback);
  }

  async getDebugInfo(): Promise<DebugInfo> {
    return await GetDebugInfo() as DebugInfo;
  }

  async setDebugMode(enabled: boolean): Promise<void> {
    return await SetDebugMode(enabled);
  }

  async updateGuiResponseTime(): Promise<void> {
    return await UpdateGuiResponseTime();
  }

  // 调试事件监听
  onHealthPing(callback: (timestamp: number) => void): () => void {
    return EventsOn('health-ping', callback);
  }

  onHeartbeat(callback: (timestamp: number) => void): () => void {
    return EventsOn('heartbeat', callback);
  }
}

export const apiService = new ApiService();
