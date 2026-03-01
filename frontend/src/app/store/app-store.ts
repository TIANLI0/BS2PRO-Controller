import { create } from 'zustand';
import { types } from '../../../wailsjs/go/models';
import { configService } from '../services/config-service';
import { deviceService, type DeviceStatusPayload } from '../services/device-service';

const BRIDGE_WARNING_MESSAGE = 'CPU/GPU 温度读取失败，可能被 Windows Defender 拦截，请将 TempBridge.exe 加入白名单或尝试重新安装后再试。';

type ActiveTab = 'status' | 'curve' | 'control';

interface AppStore {
  isConnected: boolean;
  config: types.AppConfig | null;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  bridgeWarning: string | null;
  isLoading: boolean;
  error: string | null;
  activeTab: ActiveTab;

  setActiveTab: (tab: ActiveTab) => void;
  clearBridgeWarning: () => void;
  handleTemperaturePayload: (data: types.TemperatureData | null) => void;

  initializeApp: () => Promise<void>;
  connectDevice: () => Promise<void>;
  disconnectDevice: () => Promise<void>;
  updateConfig: (config: types.AppConfig) => Promise<void>;

  startEventListeners: () => () => void;
}

export const useAppStore = create<AppStore>((set, get) => ({
  isConnected: false,
  config: null,
  fanData: null,
  temperature: null,
  bridgeWarning: null,
  isLoading: true,
  error: null,
  activeTab: 'status',

  setActiveTab: (tab) => set({ activeTab: tab }),

  clearBridgeWarning: () => set({ bridgeWarning: null }),

  handleTemperaturePayload: (data) => {
    const bridgeMessage = data?.bridgeMessage?.trim() ?? '';
    set({
      temperature: data,
      bridgeWarning: data?.bridgeOk === false ? bridgeMessage || BRIDGE_WARNING_MESSAGE : null,
    });
  },

  initializeApp: async () => {
    try {
      set({ isLoading: true });

      const appConfig = await configService.getConfig();
      const deviceStatus = (await deviceService.getStatus()) as DeviceStatusPayload;

      set({
        config: appConfig,
        isConnected: deviceStatus.connected || false,
        fanData: deviceStatus.currentData || null,
        error: null,
      });

      get().handleTemperaturePayload(deviceStatus.temperature || null);
    } catch (error) {
      console.error('初始化失败:', error);
      set({ error: '应用初始化失败' });
    } finally {
      set({ isLoading: false });
    }
  },

  connectDevice: async () => {
    try {
      const success = await deviceService.connect();
      if (success) {
        set({ isConnected: true, error: null });
      }
    } catch (error) {
      console.error('连接失败:', error);
      set({ error: '设备连接失败' });
    }
  },

  disconnectDevice: async () => {
    try {
      await deviceService.disconnect();
      set({ isConnected: false, fanData: null });
    } catch (error) {
      console.error('断开连接失败:', error);
    }
  },

  updateConfig: async (config) => {
    try {
      await configService.updateConfig(config);
      set({ config, error: null });
    } catch (error) {
      console.error('配置更新失败:', error);
      set({ error: '配置保存失败' });
    }
  },

  startEventListeners: () => {
    const unsubscribers: Array<() => void> = [];

    unsubscribers.push(
      deviceService.onDeviceConnected((deviceInfo) => {
        console.log('设备已连接:', deviceInfo);
        set({ isConnected: true, error: null });
      })
    );

    unsubscribers.push(
      deviceService.onDeviceDisconnected(() => {
        console.log('设备已断开');
        set({ isConnected: false, fanData: null });
      })
    );

    unsubscribers.push(
      deviceService.onDeviceError((errorMsg) => {
        console.error('设备错误:', errorMsg);
        set({ error: errorMsg });
      })
    );

    unsubscribers.push(
      deviceService.onFanDataUpdate((data) => {
        set({ fanData: data });
      })
    );

    unsubscribers.push(
      deviceService.onTemperatureUpdate((data) => {
        get().handleTemperaturePayload(data);
      })
    );

    unsubscribers.push(
      configService.onConfigUpdate((updatedConfig) => {
        set({ config: updatedConfig });
      })
    );

    return () => {
      unsubscribers.forEach((unsubscribe) => unsubscribe());
    };
  },
}));
