// Wails API Service Wrapper
import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime';
import { 
  ConnectDevice, 
  DisconnectDevice, 
  GetDeviceStatus,
  GetConfig,
  UpdateConfig,
  SetFanCurve,
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
  DeviceInfo 
} from '../types/app';

class ApiService {
  // Device connection
  async connectDevice(): Promise<boolean> {
    return await ConnectDevice();
  }

  async disconnectDevice(): Promise<void> {
    return await DisconnectDevice();
  }

  async getDeviceStatus(): Promise<any> {
    return await GetDeviceStatus();
  }

  // Configuration management
  async getConfig(): Promise<types.AppConfig> {
    return await GetConfig();
  }

  async getAppVersion(): Promise<string> {
    return await GetAppVersion();
  }

  async updateConfig(config: types.AppConfig): Promise<void> {
    return await UpdateConfig(config);
  }

  // Fan curve
  async setFanCurve(curve: types.FanCurvePoint[]): Promise<void> {
    return await SetFanCurve(curve);
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

  // Smart speed control
  async setAutoControl(enabled: boolean): Promise<void> {
    return await SetAutoControl(enabled);
  }

  // Custom speed
  async setCustomSpeed(enabled: boolean, rpm: number): Promise<void> {
    return await SetCustomSpeed(enabled, rpm);
  }

  // Manual gear control
  async setManualGear(gear: string, level: string): Promise<boolean> {
    return await SetManualGear(gear, level);
  }

  // Get available gears
  async getAvailableGears(): Promise<any> {
    return await GetAvailableGears();
  }

  // Device settings
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

  // Windows auto-start related
  async checkWindowsAutoStart(): Promise<boolean> {
    // Temporarily using window object call, will update after Wails generates bindings
    return await (window as any).go?.main?.App?.CheckWindowsAutoStart();
  }

  async setWindowsAutoStart(enabled: boolean): Promise<void> {
    // Temporarily using window object call, will update after Wails generates bindings
    return await (window as any).go?.main?.App?.SetWindowsAutoStart(enabled);
  }

  async getAutoStartMethod(): Promise<string> {
    // Get current auto-start method
    return await (window as any).go?.main?.App?.GetAutoStartMethod();
  }

  async setAutoStartWithMethod(enabled: boolean, method: string): Promise<void> {
    // Set auto-start with specified method
    return await (window as any).go?.main?.App?.SetAutoStartWithMethod(enabled, method);
  }

  async isRunningAsAdmin(): Promise<boolean> {
    // Check if running as administrator
    return await (window as any).go?.main?.App?.IsRunningAsAdmin();
  }

  // Data retrieval
  async getTemperature(): Promise<types.TemperatureData> {
    return await GetTemperature();
  }

  async getCurrentFanData(): Promise<types.FanData | null> {
    return await GetCurrentFanData();
  }

  async testTemperatureReading(): Promise<types.TemperatureData> {
    return await TestTemperatureReading();
  }

  // Bridge program related
  async getBridgeProgramStatus(): Promise<any> {
    return await (window as any).go?.main?.App?.GetBridgeProgramStatus();
  }

  async testBridgeProgram(): Promise<any> {
    return await (window as any).go?.main?.App?.TestBridgeProgram();
  }

  // Event listeners
  onDeviceConnected(callback: (data: DeviceInfo) => void): () => void {
    EventsOn('device-connected', callback);
    return () => EventsOff('device-connected');
  }

  onDeviceDisconnected(callback: () => void): () => void {
    EventsOn('device-disconnected', callback);
    return () => EventsOff('device-disconnected');
  }

  onDeviceError(callback: (error: string) => void): () => void {
    EventsOn('device-error', callback);
    return () => EventsOff('device-error');
  }

  onFanDataUpdate(callback: (data: types.FanData) => void): () => void {
    EventsOn('fan-data-update', callback);
    return () => EventsOff('fan-data-update');
  }

  onTemperatureUpdate(callback: (data: types.TemperatureData) => void): () => void {
    EventsOn('temperature-update', callback);
    return () => EventsOff('temperature-update');
  }

  onConfigUpdate(callback: (config: types.AppConfig) => void): () => void {
    EventsOn('config-update', callback);
    return () => EventsOff('config-update');
  }

  onHotkeyTriggered(callback: (payload: { action: string; shortcut: string; success: boolean; message: string }) => void): () => void {
    EventsOn('hotkey-triggered', callback);
    return () => EventsOff('hotkey-triggered');
  }

  // Debug related methods
  async getDebugInfo(): Promise<any> {
    return await GetDebugInfo();
  }

  async setDebugMode(enabled: boolean): Promise<void> {
    return await SetDebugMode(enabled);
  }

  async updateGuiResponseTime(): Promise<void> {
    return await UpdateGuiResponseTime();
  }

  // Debug event listeners
  onHealthPing(callback: (timestamp: number) => void): () => void {
    EventsOn('health-ping', callback);
    return () => EventsOff('health-ping');
  }

  onHeartbeat(callback: (timestamp: number) => void): () => void {
    EventsOn('heartbeat', callback);
    return () => EventsOff('heartbeat');
  }
}

export const apiService = new ApiService();
