'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { 
  BarChart3,
  Activity,
  Settings,
  TriangleAlert,
  X,
} from 'lucide-react';
import DeviceStatus from './components/DeviceStatus';
import FanCurve from './components/FanCurve';
import ControlPanel from './components/ControlPanel';
import { apiService } from './services/api';
import { types } from '../../wailsjs/go/models';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';

const BRIDGE_WARNING_MESSAGE = 'CPU/GPU 温度读取失败，可能被 Windows Defender 拦截，请将 TempBridge.exe 加入白名单或尝试重新安装后再试。';

export default function Home() {
  // 状态管理
  const [isConnected, setIsConnected] = useState(false);
  const [config, setConfig] = useState<types.AppConfig | null>(null);
  const [fanData, setFanData] = useState<types.FanData | null>(null);
  const [temperature, setTemperature] = useState<types.TemperatureData | null>(null);
  const [bridgeWarning, setBridgeWarning] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'status' | 'curve' | 'control' | 'debug'>('status');

  // 温度数据处理，附带桥接异常的全局提示
  const handleTemperaturePayload = useCallback((data: types.TemperatureData | null) => {
    setTemperature(data);

    if (data && data.bridgeOk === false) {
      const msg = (data.bridgeMessage || '').trim();
      setBridgeWarning(msg || BRIDGE_WARNING_MESSAGE);
    } else {
      setBridgeWarning(null);
    }
  }, []);

  // 初始化应用
  const initializeApp = useCallback(async () => {
    try {
      setIsLoading(true);
      
      // 获取配置
      const appConfig = await apiService.getConfig();
      setConfig(appConfig);
      
      // 获取设备状态
      const deviceStatus = await apiService.getDeviceStatus();
      setIsConnected(deviceStatus.connected || false);
      setFanData(deviceStatus.currentData || null);
      handleTemperaturePayload(deviceStatus.temperature || null);
      
      setError(null);
    } catch (err) {
      console.error('初始化失败:', err);
      setError('应用初始化失败');
    } finally {
      setIsLoading(false);
    }
  }, []);

  // 连接设备
  const handleConnect = useCallback(async () => {
    try {
      const success = await apiService.connectDevice();
      if (success) {
        setIsConnected(true);
        setError(null);
      }
    } catch (err) {
      console.error('连接失败:', err);
      setError('设备连接失败');
    }
  }, []);

  // 断开设备
  const handleDisconnect = useCallback(async () => {
    try {
      await apiService.disconnectDevice();
      setIsConnected(false);
      setFanData(null);
    } catch (err) {
      console.error('断开连接失败:', err);
    }
  }, []);

  // 更新配置
  const handleConfigChange = useCallback(async (newConfig: types.AppConfig) => {
    try {
      await apiService.updateConfig(newConfig);
      setConfig(newConfig);
    } catch (err) {
      console.error('配置更新失败:', err);
      setError('配置保存失败');
    }
  }, []);

  // 设置事件监听器
  useEffect(() => {
    const unsubscribers: (() => void)[] = [];

    // 设备连接事件
    unsubscribers.push(
      apiService.onDeviceConnected((deviceInfo) => {
        console.log('设备已连接:', deviceInfo);
        setIsConnected(true);
        setError(null);
      })
    );

    // 设备断开事件
    unsubscribers.push(
      apiService.onDeviceDisconnected(() => {
        console.log('设备已断开');
        setIsConnected(false);
        setFanData(null);
      })
    );

    // 设备错误事件
    unsubscribers.push(
      apiService.onDeviceError((errorMsg) => {
        console.error('设备错误:', errorMsg);
        setError(errorMsg);
      })
    );

    // 风扇数据更新事件
    unsubscribers.push(
      apiService.onFanDataUpdate((data) => {
        setFanData(data);
      })
    );

    // 温度数据更新事件
    unsubscribers.push(
      apiService.onTemperatureUpdate((data) => {
        handleTemperaturePayload(data);
      })
    );

    // 配置更新事件
    unsubscribers.push(
      apiService.onConfigUpdate((updatedConfig) => {
        setConfig(updatedConfig);
      })
    );

    // 清理函数
    return () => {
      unsubscribers.forEach(unsub => unsub());
    };
  }, [handleTemperaturePayload]);

  // 组件挂载时初始化
  useEffect(() => {
    initializeApp();
  }, [initializeApp]);

  // 加载状态
  if (isLoading) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto mb-4"></div>
          <div className="text-gray-600 dark:text-gray-400">正在加载...</div>
        </div>
      </div>
    );
  }

  // 错误状态
  if (error && !config) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex items-center justify-center">
        <div className="text-center">
          <div className="text-red-600 dark:text-red-400 mb-4">
            <svg className="w-12 h-12 mx-auto mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.729-.833-2.5 0L4.232 15.5c-.77.833.192 2.5 1.732 2.5z" />
            </svg>
            {error}
          </div>
          <button
            onClick={initializeApp}
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors"
          >
            重试
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex flex-col bg-gradient-to-br from-gray-50 to-gray-100 dark:from-gray-900 dark:to-gray-800">
      {/* 头部 */}
      <header className="bg-white/80 dark:bg-gray-800/80 backdrop-blur-md shadow-lg border-b border-gray-200/50 dark:border-gray-700/50 sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between items-center h-16">
            <div className="flex items-center space-x-4">
              <div className="flex-shrink-0">
                {/* Logo区域 */}
                <div className="flex items-center space-x-3">
                  <div className="w-10 h-10 bg-blue-600 rounded-xl flex items-center justify-center shadow-lg">
                    <svg className="w-6 h-6 text-white" viewBox="0 0 24 24" fill="currentColor">
                      <circle cx="12" cy="12" r="2" />
                      <path d="M12 6c-1.5 0-3 .5-4 1.5L6.5 6C8 4.5 10 4 12 4s4 .5 5.5 2L16 7.5C15 6.5 13.5 6 12 6z" />
                      <path d="M18 12c0 1.5-.5 3-1.5 4l1.5 1.5c1.5-1.5 2-3.5 2-5.5s-.5-4-2-5.5L16.5 8c1 1 1.5 2.5 1.5 4z" />
                      <path d="M12 18c1.5 0 3-.5 4-1.5l1.5 1.5c-1.5 1.5-3.5 2-5.5 2s-4-.5-5.5-2L8 16.5c1 1 2.5 1.5 4 1.5z" />
                      <path d="M6 12c0-1.5.5-3 1.5-4L6 6.5C4.5 8 4 10 4 12s.5 4 2 5.5L7.5 16C6.5 15 6 13.5 6 12z" />
                    </svg>
                  </div>
                  <div>
                    <h1 className="text-xl font-bold text-gray-900 dark:text-white">
                      BS2PRO 压风控制器
                    </h1>
                    <p className="text-xs text-gray-500 dark:text-gray-400">飞智空间站替代</p>
                  </div>
                </div>
              </div>
            </div>
            
            <div className="flex items-center space-x-4">
              {/* 快速状态显示 */}
              {isConnected && (
                <div className="hidden xl:flex items-center space-x-6 text-sm bg-gray-50 dark:bg-gray-700/50 rounded-xl px-4 py-2 border border-gray-200 dark:border-gray-600">
                  <div className="flex items-center space-x-2">
                    <div className="w-2 h-2 rounded-full bg-orange-500 animate-pulse"></div>
                    <span className="text-gray-600 dark:text-gray-300">CPU:</span>
                    <span className={`font-bold ${
                      temperature?.maxTemp && temperature.maxTemp > 80 
                        ? 'text-red-600 dark:text-red-400' 
                        : temperature?.maxTemp && temperature.maxTemp > 70 
                        ? 'text-yellow-600 dark:text-yellow-400'
                        : 'text-green-600 dark:text-green-400'
                    }`}>
                      {temperature?.maxTemp || '--'}°C
                    </span>
                  </div>
                  
                  <div className="w-px h-6 bg-gray-300 dark:bg-gray-600"></div>
                  
                  <div className="flex items-center space-x-2">
                    <div className="w-2 h-2 rounded-full bg-blue-500 animate-pulse"></div>
                    <span className="text-gray-600 dark:text-gray-300">转速:</span>
                    <span className="font-bold text-blue-600 dark:text-blue-400">
                      {fanData?.currentRpm || '--'} RPM
                    </span>
                  </div>
                  
                  <div className="w-px h-6 bg-gray-300 dark:bg-gray-600"></div>
                  
                  <div className="flex items-center space-x-2">
                    <div className={`w-2 h-2 rounded-full ${
                      config?.autoControl ? 'bg-green-500' : 'bg-gray-400'
                    }`}></div>
                    <span className="text-gray-600 dark:text-gray-300">自动:</span>
                    <span className={`font-bold ${
                      config?.autoControl 
                        ? 'text-green-600 dark:text-green-400' 
                        : 'text-gray-600 dark:text-gray-400'
                    }`}>
                      {config?.autoControl ? '开启' : '关闭'}
                    </span>
                  </div>
                </div>
              )}
              
              {/* 错误提示 */}
              {error && (
                <div className="text-sm text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 px-3 py-2 rounded-lg border border-red-200 dark:border-red-800 flex items-center space-x-2">
                  <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                    <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
                  </svg>
                  <span>{error}</span>
                </div>
              )}
              
              {/* 连接状态指示 */}
              <div className={`flex items-center space-x-2 px-4 py-2 rounded-full text-sm font-medium transition-all duration-200 ${
                isConnected 
                  ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400 border border-green-200 dark:border-green-800 shadow-lg shadow-green-100 dark:shadow-green-900/20'
                  : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300 border border-gray-200 dark:border-gray-600'
              }`}>
                <div className={`w-2 h-2 rounded-full transition-all duration-200 ${
                  isConnected ? 'bg-green-500 shadow-lg shadow-green-400' : 'bg-gray-400'
                }`}></div>
                <span>{isConnected ? '已连接' : '未连接'}</span>
              </div>
            </div>
          </div>
        </div>
      </header>

      {/* 桥接程序异常的全局提醒 */}
      {bridgeWarning && (
        <div className="sticky top-16 z-40">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-3">
            <div className="relative rounded-2xl border border-amber-200 bg-amber-50 text-amber-900 shadow-md">
              <div className="flex items-start gap-3 px-4 py-3">
                <div className="mt-0.5">
                  <TriangleAlert className="w-5 h-5 text-amber-500" />
                </div>
                <div className="flex-1">
                  <div className="text-sm font-semibold">温度读取受阻</div>
                  <p className="text-sm leading-relaxed text-amber-800">{bridgeWarning}</p>
                </div>
                <button
                  type="button"
                  onClick={() => setBridgeWarning(null)}
                  className="p-1 text-amber-700 hover:text-amber-900 hover:bg-amber-100 rounded-lg transition-colors"
                  aria-label="关闭提示"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* 主内容 */}
      <main className="flex-1 max-w-7xl mx-auto w-full px-4 sm:px-6 lg:px-8 py-4 md:py-8">
        {/* 标签页导航 */}
        <div className="mb-6 md:mb-8">
          <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-lg border border-gray-200 dark:border-gray-700 p-1 md:p-2">
            <nav className="flex space-x-0.5 md:space-x-1" aria-label="Tabs">
              {[
                { id: 'status', name: '设备状态', icon: BarChart3, desc: '实时监控' },
                { id: 'curve', name: '风扇曲线', icon: Activity, desc: '温度控制' },
                { id: 'control', name: '控制面板', icon: Settings, desc: '手动调节' },
              ].map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id as typeof activeTab)}
                  className={`${
                    activeTab === tab.id
                      ? 'bg-gradient-to-r from-blue-500 to-indigo-600 text-white shadow-lg shadow-blue-500/30'
                      : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-gray-700'
                  } group relative rounded-xl px-3 md:px-6 py-2 md:py-3 font-medium text-sm transition-all duration-200 flex-1 flex flex-col items-center space-y-0.5 md:space-y-1`}
                >
                  <div className="flex items-center space-x-1 md:space-x-2">
                    <tab.icon className="w-4 h-4 md:w-5 md:h-5" />
                    <span className="font-semibold text-xs md:text-sm">{tab.name}</span>
                  </div>
                  <span className={`text-xs hidden md:block ${
                    activeTab === tab.id 
                      ? 'text-blue-100' 
                      : 'text-gray-500 dark:text-gray-500 group-hover:text-gray-600 dark:group-hover:text-gray-400'
                  }`}>
                    {tab.desc}
                  </span>
                </button>
              ))}
            </nav>
          </div>
        </div>

        {/* 标签页内容 */}
        <div className="space-y-4 md:space-y-8">
          {activeTab === 'status' && (
            <DeviceStatus
              isConnected={isConnected}
              fanData={fanData}
              temperature={temperature}
              config={config || new types.AppConfig()}
              onConnect={handleConnect}
              onDisconnect={handleDisconnect}
              onConfigChange={handleConfigChange}
            />
          )}

          {activeTab === 'curve' && config && (
            <FanCurve
              config={config}
              onConfigChange={handleConfigChange}
              isConnected={isConnected}
              fanData={fanData}
              temperature={temperature}
            />
          )}

          {activeTab === 'control' && config && (
            <ControlPanel
              config={config}
              onConfigChange={handleConfigChange}
              isConnected={isConnected}
              fanData={fanData}
              temperature={temperature}
            />
          )}


        </div>
      </main>

      {/* 底部信息 */}
      <footer className="mt-auto border-t border-gray-200 dark:border-gray-700">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="text-center text-sm text-gray-500 dark:text-gray-400">
            BS2PRO Controller - By{' '}
            <span 
              className="cursor-pointer hover:text-blue-600 dark:hover:text-blue-400 transition-colors group relative"
              title="点击访问开发者主页"
              onClick={() => BrowserOpenURL('https://www.tianli0.top/')}
            >
              Tianli
              {/* Tooltip */}
              <div className="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 px-3 py-2 bg-gray-900 dark:bg-gray-700 text-white text-xs rounded-lg opacity-0 group-hover:opacity-100 transition-opacity duration-200 pointer-events-none whitespace-nowrap z-50">
                <div className="text-center">
                  <div className="font-medium">Tianli</div>
                  <div className="text-gray-300 dark:text-gray-400">www.tianli0.top</div>
                </div>
                {/* 箭头 */}
                <div className="absolute top-full left-1/2 transform -translate-x-1/2 w-0 h-0 border-l-4 border-r-4 border-t-4 border-transparent border-t-gray-900 dark:border-t-gray-700"></div>
              </div>
            </span>
          </div>
        </div>
      </footer>
    </div>
  );
}
