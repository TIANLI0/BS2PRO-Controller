'use client';

import { types } from '../../wailsjs/go/models';
import AppFatalError from './components/AppFatalError';
import AppLoadingSkeleton from './components/AppLoadingSkeleton';
import AppShell from './components/AppShell';
import ControlPanel from './components/ControlPanel';
import DeviceStatus from './components/DeviceStatus';
import FanCurve from './components/FanCurve';
import { useAppBootstrap } from './hooks/useAppBootstrap';
import { useAppStore } from './store/app-store';

export default function Home() {
  useAppBootstrap();

  const isConnected = useAppStore((state) => state.isConnected);
  const config = useAppStore((state) => state.config);
  const fanData = useAppStore((state) => state.fanData);
  const temperature = useAppStore((state) => state.temperature);
  const bridgeWarning = useAppStore((state) => state.bridgeWarning);
  const isLoading = useAppStore((state) => state.isLoading);
  const error = useAppStore((state) => state.error);
  const activeTab = useAppStore((state) => state.activeTab);

  const initializeApp = useAppStore((state) => state.initializeApp);
  const connectDevice = useAppStore((state) => state.connectDevice);
  const disconnectDevice = useAppStore((state) => state.disconnectDevice);
  const updateConfig = useAppStore((state) => state.updateConfig);
  const setActiveTab = useAppStore((state) => state.setActiveTab);
  const clearBridgeWarning = useAppStore((state) => state.clearBridgeWarning);

  if (isLoading) {
    return <AppLoadingSkeleton />;
  }

  if (error && !config) {
    return <AppFatalError message={error} onRetry={initializeApp} />;
  }

  const safeConfig = config || new types.AppConfig();

  return (
    <AppShell
      activeTab={activeTab}
      onTabChange={setActiveTab}
      isConnected={isConnected}
      fanData={fanData}
      temperature={temperature}
      autoControl={safeConfig.autoControl}
      error={error}
      bridgeWarning={bridgeWarning}
      onDismissBridgeWarning={clearBridgeWarning}
      statusContent={
        <DeviceStatus
          isConnected={isConnected}
          fanData={fanData}
          temperature={temperature}
          config={safeConfig}
          onConnect={connectDevice}
          onDisconnect={disconnectDevice}
          onConfigChange={updateConfig}
        />
      }
      curveContent={
        <FanCurve
          config={safeConfig}
          onConfigChange={updateConfig}
          isConnected={isConnected}
          fanData={fanData}
          temperature={temperature}
        />
      }
      controlContent={
        <ControlPanel
          config={safeConfig}
          onConfigChange={updateConfig}
          isConnected={isConnected}
          fanData={fanData}
          temperature={temperature}
        />
      }
    />
  );
}
