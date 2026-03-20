import { create } from 'zustand';
import { types } from '../../../wailsjs/go/models';
import { configService } from '../services/config-service';
import { deviceService, type DeviceStatusPayload } from '../services/device-service';
import { toast } from 'sonner';

const BRIDGE_WARNING_MESSAGE = 'CPU/GPU temperature reading failed. Please check if PawnIO is installed correctly, or upgrade to the latest version.';

type ActiveTab = 'status' | 'curve' | 'control';

interface AppStore {
  isConnected: boolean;
  deviceProductId: string | null;
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
  deviceProductId: null,
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
        deviceProductId: deviceStatus.productId || null,
        fanData: deviceStatus.currentData || null,
        error: null,
      });

      get().handleTemperaturePayload(deviceStatus.temperature || null);
    } catch (error) {
      console.error('Initialization failed:', error);
      set({ error: 'App initialization failed' });
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
      console.error('Connection failed:', error);
      set({ error: 'Device connection failed' });
    }
  },

  disconnectDevice: async () => {
    try {
      await deviceService.disconnect();
      set({ isConnected: false, deviceProductId: null, fanData: null });
    } catch (error) {
      console.error('Disconnect failed:', error);
    }
  },

  updateConfig: async (config) => {
    try {
      await configService.updateConfig(config);
      set({ config, error: null });
    } catch (error) {
      console.error('Config update failed:', error);
      set({ error: 'Config save failed' });
    }
  },

  startEventListeners: () => {
    const unsubscribers: Array<() => void> = [];

    unsubscribers.push(
      deviceService.onDeviceConnected((deviceInfo) => {
        console.log('Device connected:', deviceInfo);
        const info = deviceInfo as { productId?: string };
        set({
          isConnected: true,
          deviceProductId: info.productId || null,
          error: null,
        });
      })
    );

    unsubscribers.push(
      deviceService.onDeviceDisconnected(() => {
        console.log('Device disconnected');
        set({ isConnected: false, deviceProductId: null, fanData: null });
      })
    );

    unsubscribers.push(
      deviceService.onDeviceError((errorMsg) => {
        console.error('Device error:', errorMsg);
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

    unsubscribers.push(
      deviceService.onHotkeyTriggered((payload) => {
        const message = typeof payload?.message === 'string' ? payload.message : '';
        if (!message) return;
        const ok = payload?.success !== false;
        if (ok) {
          toast.success('Feature Changed', { description: message, duration: 2600 });
        } else {
          toast.error('Operation Failed', { description: message, duration: 3200 });
        }
      })
    );

    return () => {
      unsubscribers.forEach((unsubscribe) => unsubscribe());
    };
  },
}));
