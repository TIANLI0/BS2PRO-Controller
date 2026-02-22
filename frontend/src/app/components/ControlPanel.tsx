'use client';

import React, { useState, useCallback, useEffect } from 'react';
import { 
  Play,
  Pause,
  Settings,
  Lightbulb,
  Power,
  Zap,
  Monitor,
  Bug,
  Eye,
  EyeOff,
  TriangleAlert,
  CheckCircle2,
  ChevronDown,
  Info,
  Flame,
  Clock3,
  BarChart3,
  Sparkles,
} from 'lucide-react';
import { apiService } from '../services/api';
import { types } from '../../../wailsjs/go/models';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import { DebugInfo } from '../types/app';
import { ToggleSwitch, RadioGroup, Card, Badge, Button, Select, ScrollArea, Slider } from './ui/index';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import clsx from 'clsx';

interface ControlPanelProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
}

function getDefaultLightStripConfig(): types.LightStripConfig {
  return types.LightStripConfig.createFrom({
    mode: 'smart_temp',
    speed: 'medium',
    brightness: 100,
    colors: [
      { r: 255, g: 0, b: 0 },
      { r: 0, g: 255, b: 0 },
      { r: 0, g: 128, b: 255 },
    ],
  });
}

function normalizeLightStripConfig(config: types.AppConfig): types.LightStripConfig {
  const defaults = getDefaultLightStripConfig();
  const raw = (config as any).lightStrip;

  if (!raw) {
    return defaults;
  }

  const normalized = types.LightStripConfig.createFrom({
    mode: raw.mode || defaults.mode,
    speed: raw.speed || defaults.speed,
    brightness: typeof raw.brightness === 'number' ? Math.max(0, Math.min(100, raw.brightness)) : defaults.brightness,
    colors: Array.isArray(raw.colors) && raw.colors.length > 0 ? raw.colors : defaults.colors,
  });

  if ((normalized.colors || []).length < 3) {
    const merged = [...(normalized.colors || [])];
    while (merged.length < 3) {
      merged.push(defaults.colors[merged.length]);
    }
    normalized.colors = merged;
  }

  return normalized;
}

function rgbToHex(color: types.RGBColor): string {
  const toHex = (value: number) => value.toString(16).padStart(2, '0');
  return `#${toHex(color.r || 0)}${toHex(color.g || 0)}${toHex(color.b || 0)}`;
}

function hexToRgb(hex: string): types.RGBColor {
  const clean = hex.replace('#', '');
  const bigint = Number.parseInt(clean, 16);
  return types.RGBColor.createFrom({
    r: (bigint >> 16) & 255,
    g: (bigint >> 8) & 255,
    b: bigint & 255,
  });
}

function getRequiredColorCount(mode: string): number {
  switch (mode) {
    case 'static_single':
      return 1;
    case 'smart_temp':
    case 'flowing':
      return 0;
    case 'static_multi':
      return 3;
    case 'rotation':
    case 'breathing':
    default:
      return 3;
  }
}

// 设置项组件
interface SettingItemProps {
  icon: React.ReactNode;
  iconBgActive: string;
  iconBgInactive: string;
  title: string;
  description: string;
  enabled: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
  loading?: boolean;
  color?: 'blue' | 'green' | 'purple' | 'orange';
}

function SettingItem({ 
  icon, 
  iconBgActive, 
  iconBgInactive, 
  title, 
  description, 
  enabled, 
  onChange, 
  disabled = false,
  loading = false,
  color = 'blue'
}: SettingItemProps) {
  return (
    <div className={clsx(
      'flex items-center justify-between py-4 px-4 -mx-4 rounded-xl transition-all duration-200',
      'hover:bg-gray-50 dark:hover:bg-gray-700/50',
      disabled && 'opacity-60'
    )}>
      <div className="flex items-center gap-4">
        <div className={clsx(
          'p-2.5 rounded-xl transition-all duration-300',
          enabled ? iconBgActive : iconBgInactive,
          enabled && 'scale-105 shadow-sm'
        )}>
          {icon}
        </div>
        <div>
          <div className="font-medium text-gray-900 dark:text-white">{title}</div>
          <div className="text-sm text-gray-500 dark:text-gray-400">{description}</div>
        </div>
      </div>
      <ToggleSwitch
        enabled={enabled}
        onChange={onChange}
        disabled={disabled}
        loading={loading}
        color={color}
      />
    </div>
  );
}

export default function ControlPanel({ config, onConfigChange, isConnected, fanData, temperature }: ControlPanelProps) {
  // 更新状态
  const [loadingStates, setLoadingStates] = useState<Record<string, boolean>>({});
  
  // 调试面板状态
  const [debugInfo, setDebugInfo] = useState<DebugInfo | null>(null);
  const [debugInfoLoading, setDebugInfoLoading] = useState(false);
  const [debugPanelOpen, setDebugPanelOpen] = useState(false);
  
  // 自定义转速相关状态
  const [showCustomSpeedWarning, setShowCustomSpeedWarning] = useState(false);
  const [customSpeedInput, setCustomSpeedInput] = useState<number>((config as any).customSpeedRPM || 2000);

  // 应用版本号
  const [appVersion, setAppVersion] = useState('');
  
  // iframe 状态
  const [iframeLoaded, setIframeLoaded] = useState(false);

  // 灯带配置状态
  const [lightStripConfig, setLightStripConfig] = useState<types.LightStripConfig>(() => normalizeLightStripConfig(config));

  // 辅助函数
  const setLoading = (key: string, value: boolean) => {
    setLoadingStates(prev => ({ ...prev, [key]: value }));
  };

  const handleOpenUrl = useCallback((url: string) => {
    try {
      BrowserOpenURL(url);
    } catch (error) {
      console.error('打开链接失败:', error);
    }
  }, []);

  // 智能变频控制
  const handleAutoControlChange = useCallback(async (enabled: boolean) => {
    setLoading('autoControl', true);
    try {
      await apiService.setAutoControl(enabled);
      onConfigChange(types.AppConfig.createFrom({ ...config, autoControl: enabled }));
    } catch (error) {
      console.error('设置智能变频失败:', error);
    } finally {
      setLoading('autoControl', false);
    }
  }, [config, onConfigChange]);

  // 自定义转速控制
  const handleCustomSpeedApply = useCallback(async (enabled: boolean, rpm: number) => {
    setLoading('customSpeed', true);
    try {
      await apiService.setCustomSpeed(enabled, rpm);
      onConfigChange(types.AppConfig.createFrom({ 
        ...config, 
        customSpeedEnabled: enabled,
        customSpeedRPM: rpm,
        autoControl: enabled ? false : config.autoControl
      }));
    } catch (error) {
      console.error('设置自定义转速失败:', error);
    } finally {
      setLoading('customSpeed', false);
    }
  }, [config, onConfigChange]);

  const handleCustomSpeedToggle = useCallback((enabled: boolean) => {
    if (enabled) {
      setShowCustomSpeedWarning(true);
    } else {
      handleCustomSpeedApply(false, customSpeedInput);
    }
  }, [customSpeedInput, handleCustomSpeedApply]);

  // 挡位灯控制
  const handleGearLightChange = useCallback(async (enabled: boolean) => {
    if (!isConnected) return;
    setLoading('gearLight', true);
    try {
      const success = await apiService.setGearLight(enabled);
      if (success) {
        onConfigChange(types.AppConfig.createFrom({ ...config, gearLight: enabled }));
      }
    } catch (error) {
      console.error('设置挡位灯失败:', error);
    } finally {
      setLoading('gearLight', false);
    }
  }, [config, onConfigChange, isConnected]);

  // 通电自启动控制
  const handlePowerOnStartChange = useCallback(async (enabled: boolean) => {
    if (!isConnected) return;
    setLoading('powerOnStart', true);
    try {
      const success = await apiService.setPowerOnStart(enabled);
      if (success) {
        onConfigChange(types.AppConfig.createFrom({ ...config, powerOnStart: enabled }));
      }
    } catch (error) {
      console.error('设置通电自启动失败:', error);
    } finally {
      setLoading('powerOnStart', false);
    }
  }, [config, onConfigChange, isConnected]);

  // Windows 开机自启动
  const handleWindowsAutoStartChange = useCallback(async (enabled: boolean) => {
    setLoading('windowsAutoStart', true);
    try {
      const isAdmin = await apiService.isRunningAsAdmin();
      if (enabled) {
        await apiService.setAutoStartWithMethod(true, isAdmin ? 'task_scheduler' : 'registry');
      } else {
        await apiService.setAutoStartWithMethod(false, '');
      }
      onConfigChange(types.AppConfig.createFrom({ ...config, windowsAutoStart: enabled }));
    } catch (error) {
      console.error('设置开机自启动失败:', error);
      alert(`设置自启动失败: ${error}`);
    } finally {
      setLoading('windowsAutoStart', false);
    }
  }, [config, onConfigChange]);

  // 断连保持配置模式
  const handleIgnoreDeviceOnReconnectChange = useCallback(async (enabled: boolean) => {
    try {
      const newConfig = types.AppConfig.createFrom({ ...config, ignoreDeviceOnReconnect: enabled });
      await apiService.updateConfig(newConfig);
      onConfigChange(newConfig);
    } catch (error) {
      console.error('设置断连保持配置模式失败:', error);
    }
  }, [config, onConfigChange]);

  // 智能启停控制
  const handleSmartStartStopChange = useCallback(async (mode: string) => {
    if (!isConnected) return;
    try {
      const success = await apiService.setSmartStartStop(mode);
      if (success) {
        onConfigChange(types.AppConfig.createFrom({ ...config, smartStartStop: mode }));
      }
    } catch (error) {
      console.error('设置智能启停失败:', error);
    }
  }, [config, onConfigChange, isConnected]);

  // 调试模式
  const toggleDebugMode = useCallback(async () => {
    try {
      await apiService.setDebugMode(!config.debugMode);
      onConfigChange(types.AppConfig.createFrom({ ...config, debugMode: !config.debugMode }));
    } catch (error) {
      console.error('设置调试模式失败:', error);
    }
  }, [config, onConfigChange]);

  // GUI 监控
  const toggleGuiMonitoring = useCallback(async () => {
    try {
      const newConfig = types.AppConfig.createFrom({ ...config, guiMonitoring: !config.guiMonitoring });
      await apiService.updateConfig(newConfig);
      onConfigChange(newConfig);
    } catch (error) {
      console.error('设置GUI监控失败:', error);
    }
  }, [config, onConfigChange]);

  // 获取调试信息
  const fetchDebugInfo = useCallback(async () => {
    try {
      setDebugInfoLoading(true);
      const info = await apiService.getDebugInfo();
      setDebugInfo(info);
    } catch (error) {
      console.error('获取调试信息失败:', error);
    } finally {
      setDebugInfoLoading(false);
    }
  }, []);

  // 定期更新 GUI 响应时间
  useEffect(() => {
    const interval = setInterval(() => {
      apiService.updateGuiResponseTime().catch(() => {});
    }, 10000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    apiService.getAppVersion()
      .then((version) => setAppVersion(version || ''))
      .catch(() => setAppVersion(''));
  }, []);

  useEffect(() => {
    setLightStripConfig(normalizeLightStripConfig(config));
  }, [config]);

  // 智能启停选项
  const smartStartStopOptions = [
    { value: 'off', label: '关闭', description: '禁用智能启停功能' },
    { value: 'immediate', label: '即时', description: '立即响应系统负载变化' },
    { value: 'delayed', label: '延时', description: '延时响应，避免频繁启停' },
  ];

  // 采样率选项 (决定多少次采样取平均值)
  const sampleCountOptions = [
    { value: 1, label: '1次 (即时响应)' },
    { value: 2, label: '2次 (2秒平均)' },
    { value: 3, label: '3次 (3秒平均)' },
    { value: 5, label: '5次 (5秒平均)' },
    { value: 10, label: '10次 (10秒平均)' },
  ];

  // 采样率变更
  const handleSampleCountChange = useCallback(async (count: number) => {
    try {
      const newConfig = types.AppConfig.createFrom({ ...config, tempSampleCount: count });
      await apiService.updateConfig(newConfig);
      onConfigChange(newConfig);
    } catch (error) {
      console.error('设置温度采样次数失败:', error);
    }
  }, [config, onConfigChange]);

  const lightModeOptions = [
    { value: 'smart_temp', label: '智能温控', description: '根据温度自动切换灯效' },
    { value: 'static_single', label: '单色常亮', description: '固定单色显示' },
    { value: 'static_multi', label: '多色常亮', description: '三色静态分区' },
    { value: 'rotation', label: '多色旋转', description: '颜色循环旋转' },
    { value: 'flowing', label: '流光', description: '预设流光效果' },
    { value: 'breathing', label: '呼吸', description: '多色呼吸变化' },
  ];

  const lightSpeedOptions = [
    { value: 'fast', label: '快速' },
    { value: 'medium', label: '中速' },
    { value: 'slow', label: '慢速' },
  ];

  const lightColorPresets = [
    {
      name: '霓虹',
      colors: [
        { r: 255, g: 0, b: 128 },
        { r: 0, g: 255, b: 255 },
        { r: 128, g: 0, b: 255 },
      ],
    },
    {
      name: '森林',
      colors: [
        { r: 86, g: 169, b: 84 },
        { r: 161, g: 210, b: 106 },
        { r: 44, g: 120, b: 115 },
      ],
    },
    {
      name: '冰川',
      colors: [
        { r: 80, g: 170, b: 255 },
        { r: 116, g: 214, b: 255 },
        { r: 200, g: 240, b: 255 },
      ],
    },
  ];

  const requiredColorCount = getRequiredColorCount(lightStripConfig.mode);

  const handleLightColorChange = useCallback((index: number, hex: string) => {
    setLightStripConfig((prev) => {
      const colors = [...(prev.colors || [])];
      while (colors.length < 3) {
        colors.push(types.RGBColor.createFrom({ r: 255, g: 255, b: 255 }));
      }
      colors[index] = hexToRgb(hex);
      return types.LightStripConfig.createFrom({ ...prev, colors });
    });
  }, []);

  const handleApplyLightStrip = useCallback(async () => {
    setLoading('lightStrip', true);
    try {
      const normalizedColors = [...(lightStripConfig.colors || [])];
      if (requiredColorCount > 0) {
        while (normalizedColors.length < requiredColorCount) {
          normalizedColors.push(types.RGBColor.createFrom({ r: 255, g: 255, b: 255 }));
        }
      }

      const submitConfig = types.LightStripConfig.createFrom({
        ...lightStripConfig,
        colors: requiredColorCount > 0 ? normalizedColors.slice(0, Math.max(requiredColorCount, 3)) : normalizedColors,
      });

      await apiService.setLightStrip(submitConfig);
      onConfigChange(types.AppConfig.createFrom({ ...config, lightStrip: submitConfig }));
    } catch (error) {
      console.error('设置灯带失败:', error);
      alert(`设置灯带失败: ${error}`);
    } finally {
      setLoading('lightStrip', false);
    }
  }, [lightStripConfig, config, onConfigChange, requiredColorCount]);

  return (
    <>
      <Card className="p-6">
        {/* 标题 */}
        <div className="flex items-center gap-3 mb-6">
          <div className="p-2 rounded-xl bg-gradient-to-br from-blue-500 to-indigo-600">
            <Settings className="w-6 h-6 text-white" />
          </div>
          <h2 className="text-xl font-bold text-gray-900 dark:text-white">控制面板</h2>
        </div>

        {/* 实时状态卡片 */}
        <div className="mb-6 p-5 rounded-2xl bg-gradient-to-r from-gray-50 via-blue-50 to-indigo-50 dark:from-gray-800 dark:via-blue-900/20 dark:to-indigo-900/20 border border-gray-200 dark:border-gray-700">
          <h3 className="text-sm font-semibold text-gray-600 dark:text-gray-400 mb-4">实时状态</h3>
          <div className="grid grid-cols-3 gap-6">
            <div className="text-center">
              <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">当前温度</div>
              <div className={clsx(
                'text-2xl font-bold',
                (temperature?.maxTemp ?? 0) > 80 ? 'text-red-500' :
                (temperature?.maxTemp ?? 0) > 70 ? 'text-yellow-500' : 'text-green-500'
              )}>
                {temperature?.maxTemp ?? '--'}°C
              </div>
              <div className="text-xs text-gray-400 dark:text-gray-500 mt-1">
                CPU {temperature?.cpuTemp ?? '--'}°C | GPU {temperature?.gpuTemp ?? '--'}°C
              </div>
            </div>
            
            <div className="text-center">
              <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">实时转速</div>
              <div className="text-2xl font-bold text-blue-600 dark:text-blue-400">
                {fanData?.currentRpm ?? '--'} <span className="text-sm font-normal">RPM</span>
              </div>
              <div className="text-xs text-gray-400 dark:text-gray-500 mt-1">
                {fanData?.workMode ?? '--'}
              </div>
            </div>
            
            <div className="text-center">
              <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">目标转速</div>
              <div className="text-2xl font-bold text-emerald-600 dark:text-emerald-400">
                {fanData?.targetRpm ?? '--'} <span className="text-sm font-normal">RPM</span>
              </div>
              <div className="text-xs text-gray-400 dark:text-gray-500 mt-1">
                挡位: {fanData?.setGear ?? '--'}
              </div>
            </div>
          </div>
        </div>

        {/* 设置项列表 */}
        <div className="divide-y divide-gray-100 dark:divide-gray-700/50">
          {/* 灯带设置（优先） */}
          <div className="py-4 px-4 -mx-4 rounded-xl bg-gradient-to-r from-pink-50/70 via-purple-50/70 to-indigo-50/70 dark:from-pink-900/10 dark:via-purple-900/10 dark:to-indigo-900/10 border border-pink-200/70 dark:border-pink-800/40 transition-all duration-200">
            <div className="flex items-center gap-4 mb-4">
              <div className="p-2.5 rounded-xl bg-pink-100 dark:bg-pink-900/30">
                <Sparkles className="w-5 h-5 text-pink-600 dark:text-pink-400" />
              </div>
              <div className="flex-1">
                <div className="font-semibold text-gray-900 dark:text-white">灯带效果</div>
                <div className="text-sm text-gray-500 dark:text-gray-400">个性化你的散热器灯光！</div>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mb-3">
              <Select
                value={lightStripConfig.mode}
                onChange={(value: string | number) => setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, mode: value as string }))}
                options={lightModeOptions}
                size="sm"
                label="效果模式"
              />
              <Select
                value={lightStripConfig.speed}
                onChange={(value: string | number) => setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, speed: value as string }))}
                options={lightSpeedOptions}
                size="sm"
                label="动画速度"
                disabled={lightStripConfig.mode === 'smart_temp' || lightStripConfig.mode === 'static_single' || lightStripConfig.mode === 'static_multi'}
              />
            </div>

            <div className="mb-3">
              <Slider
                min={0}
                max={100}
                step={1}
                value={lightStripConfig.brightness}
                onChange={(nextValue) =>
                  setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, brightness: nextValue }))
                }
                label="亮度"
                valueFormatter={(v) => `${v}%`}
              />
            </div>

            {requiredColorCount > 0 && (
              <>
                <div className="flex flex-wrap gap-2 mb-3">
                  {lightColorPresets.map((preset) => (
                    <button
                      key={preset.name}
                      type="button"
                      onClick={() => setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, colors: preset.colors }))}
                      className="px-3 py-1.5 text-xs rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                    >
                      {preset.name}
                    </button>
                  ))}
                </div>

                <div className={clsx('grid gap-3 mb-3', requiredColorCount === 1 ? 'grid-cols-1' : 'grid-cols-3')}>
                  {Array.from({ length: requiredColorCount }).map((_, index) => (
                    <div key={index}>
                      <label className="block text-xs text-gray-500 dark:text-gray-400 mb-1">颜色 {index + 1}</label>
                      <input
                        type="color"
                        value={rgbToHex((lightStripConfig.colors || [])[index] || types.RGBColor.createFrom({ r: 255, g: 255, b: 255 }))}
                        onChange={(e) => handleLightColorChange(index, e.target.value)}
                        className="w-full h-10 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 cursor-pointer"
                      />
                    </div>
                  ))}
                </div>
              </>
            )}

            <div className="flex items-center justify-between">
              <div className="text-xs text-gray-500 dark:text-gray-400">
                {isConnected ? '已连接设备，应用后立即生效' : '设备未连接，配置会在下次连接时自动生效'}
              </div>
              <Button
                variant="primary"
                onClick={handleApplyLightStrip}
                loading={loadingStates.lightStrip}
              >
                应用灯带设置
              </Button>
            </div>
          </div>

          {/* 智能变频 */}
          <SettingItem
            icon={config.autoControl ? 
              <Play className="w-5 h-5 text-green-600 dark:text-green-400" /> : 
              <Pause className="w-5 h-5 text-gray-500 dark:text-gray-400" />
            }
            iconBgActive="bg-green-100 dark:bg-green-900/30"
            iconBgInactive="bg-gray-100 dark:bg-gray-700"
            title="自动温度控制"
            description="根据温度曲线自动调节风扇转速"
            enabled={config.autoControl}
            onChange={handleAutoControlChange}
            disabled={(config as any).customSpeedEnabled}
            loading={loadingStates.autoControl}
            color="green"
          />

          {/* 温度采样平均 - 仅在开启自动温控时显示 */}
          {config.autoControl && (
            <div className="py-4 px-4 -mx-4 rounded-xl hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-all duration-200">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                  <div className="p-2.5 rounded-xl bg-cyan-100 dark:bg-cyan-900/30">
                    <BarChart3 className="w-5 h-5 text-cyan-600 dark:text-cyan-400" />
                  </div>
                  <div>
                    <div className="font-medium text-gray-900 dark:text-white">采样时间</div>
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      控制采样频率，降低频繁调整带来的轴噪
                    </div>
                  </div>
                </div>
                <Select
                  value={(config as any).tempSampleCount || 1}
                  onChange={(val: string | number) => handleSampleCountChange(val as number)}
                  options={sampleCountOptions}
                  size="sm"
                />
              </div>
            </div>
          )}

          {/* 自定义转速控制 */}
          <div className="py-4">
            <div className={clsx(
              'p-4 rounded-xl border-2 transition-all duration-300',
              (config as any).customSpeedEnabled 
                ? 'border-orange-300 dark:border-orange-600 bg-orange-50/50 dark:bg-orange-900/10' 
                : 'border-gray-200 dark:border-gray-700 bg-gray-50/50 dark:bg-gray-800/50'
            )}>
              <div className="flex items-center justify-between mb-4">
                <div className="flex items-center gap-4">
                  <div className={clsx(
                    'p-2.5 rounded-xl transition-all duration-300',
                    (config as any).customSpeedEnabled 
                      ? 'bg-orange-100 dark:bg-orange-900/30 scale-105' 
                      : 'bg-gray-100 dark:bg-gray-700'
                  )}>
                    <Flame className={clsx(
                      'w-5 h-5 transition-colors duration-300',
                      (config as any).customSpeedEnabled 
                        ? 'text-orange-600 dark:text-orange-400' 
                        : 'text-gray-500 dark:text-gray-400'
                    )} />
                  </div>
                  <div>
                    <div className="font-medium text-gray-900 dark:text-white">自定义转速</div>
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      固定风扇转速，适合特殊场景使用
                    </div>
                  </div>
                </div>
                <ToggleSwitch
                  enabled={(config as any).customSpeedEnabled || false}
                  onChange={handleCustomSpeedToggle}
                  disabled={!isConnected}
                  loading={loadingStates.customSpeed}
                  color="orange"
                />
              </div>
              
              {(config as any).customSpeedEnabled && (
                <div className="pt-4 border-t border-orange-200 dark:border-orange-800">
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    目标转速 (RPM)
                  </label>
                  <div className="flex items-center gap-3">
                    <input
                      type="number"
                      value={customSpeedInput}
                      onChange={(e) => setCustomSpeedInput(Number(e.target.value))}
                      className="flex-1 px-4 py-2.5 rounded-xl border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:ring-2 focus:ring-orange-500 focus:border-transparent transition-all duration-200"
                      min={1000}
                      max={4000}
                      step={50}
                    />
                    <Button
                      variant="primary"
                      onClick={() => handleCustomSpeedApply(true, customSpeedInput)}
                      className="!bg-orange-600 hover:!bg-orange-700"
                    >
                      应用
                    </Button>
                  </div>
                  <p className="text-xs text-orange-600 dark:text-orange-400 mt-2">
                    ⚠️ 自定义转速会禁用智能温控，请谨慎使用
                  </p>
                </div>
              )}
            </div>
          </div>

          {/* 挡位灯 */}
          <SettingItem
            icon={<Lightbulb className={clsx(
              'w-5 h-5 transition-colors duration-300',
              config.gearLight ? 'text-yellow-500' : 'text-gray-500 dark:text-gray-400'
            )} />}
            iconBgActive="bg-yellow-100 dark:bg-yellow-900/30"
            iconBgInactive="bg-gray-100 dark:bg-gray-700"
            title="挡位灯"
            description="控制设备上的挡位指示灯"
            enabled={config.gearLight}
            onChange={handleGearLightChange}
            disabled={!isConnected}
            loading={loadingStates.gearLight}
            color="blue"
          />

          {/* 通电自启动 */}
          <SettingItem
            icon={<Power className={clsx(
              'w-5 h-5 transition-colors duration-300',
              config.powerOnStart ? 'text-blue-600 dark:text-blue-400' : 'text-gray-500 dark:text-gray-400'
            )} />}
            iconBgActive="bg-blue-100 dark:bg-blue-900/30"
            iconBgInactive="bg-gray-100 dark:bg-gray-700"
            title="通电自启动"
            description="设备通电后自动开始运行"
            enabled={config.powerOnStart}
            onChange={handlePowerOnStartChange}
            disabled={!isConnected}
            loading={loadingStates.powerOnStart}
            color="blue"
          />

          {/* Windows 开机自启动 */}
          <div className="py-4 px-4 -mx-4 rounded-xl hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-all duration-200">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-4">
                <div className={clsx(
                  'p-2.5 rounded-xl transition-all duration-300',
                  config.windowsAutoStart 
                    ? 'bg-green-100 dark:bg-green-900/30 scale-105' 
                    : 'bg-gray-100 dark:bg-gray-700'
                )}>
                  <Monitor className={clsx(
                    'w-5 h-5 transition-colors duration-300',
                    config.windowsAutoStart 
                      ? 'text-green-600 dark:text-green-400' 
                      : 'text-gray-500 dark:text-gray-400'
                  )} />
                </div>
                <div>
                  <div className="font-medium text-gray-900 dark:text-white">开机自启动</div>
                  <div className="text-sm text-gray-500 dark:text-gray-400">
                    Windows 启动时自动启动本程序
                  </div>
                  <div className="text-xs text-blue-600 dark:text-blue-400 mt-0.5">
                    💡 以管理员身份运行可避免每次UAC授权
                  </div>
                </div>
              </div>
              <ToggleSwitch
                enabled={config.windowsAutoStart}
                onChange={handleWindowsAutoStartChange}
                loading={loadingStates.windowsAutoStart}
                color="green"
              />
            </div>
          </div>

          {/* 断连保持配置模式 */}
          <div className="py-4 px-4 -mx-4 rounded-xl hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-all duration-200">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-4">
                <div className={clsx(
                  'p-2.5 rounded-xl transition-all duration-300',
                  (config as any).ignoreDeviceOnReconnect 
                    ? 'bg-emerald-100 dark:bg-emerald-900/30 scale-105' 
                    : 'bg-gray-100 dark:bg-gray-700'
                )}>
                  <Clock3 className={clsx(
                    'w-5 h-5 transition-colors duration-300',
                    (config as any).ignoreDeviceOnReconnect 
                      ? 'text-emerald-600 dark:text-emerald-400' 
                      : 'text-gray-500 dark:text-gray-400'
                  )} />
                </div>
                <div>
                  <div className="font-medium text-gray-900 dark:text-white">断连保持配置</div>
                  <div className="text-sm text-gray-500 dark:text-gray-400">
                    设备断开重连后继续使用APP配置，而不是设备默认状态
                  </div>
                  <div className="text-xs text-emerald-600 dark:text-emerald-400 mt-0.5">
                    推荐开启，防止设备异常断连导致进入手动模式
                  </div>
                </div>
              </div>
              <ToggleSwitch
                enabled={(config as any).ignoreDeviceOnReconnect ?? true}
                onChange={handleIgnoreDeviceOnReconnectChange}
                color="green"
              />
            </div>
          </div>

          {/* 智能启停 */}
          <div className="py-4">
            <div className="flex items-center gap-4 mb-4">
              <div className="p-2.5 rounded-xl bg-purple-100 dark:bg-purple-900/30">
                <Zap className="w-5 h-5 text-purple-600 dark:text-purple-400" />
              </div>
              <div>
                <div className="font-medium text-gray-900 dark:text-white">智能启停</div>
                <div className="text-sm text-gray-500 dark:text-gray-400">
                  控制在系统关闭后何时停止散热器
                </div>
              </div>
            </div>
            <div className="ml-14">
              <RadioGroup
                value={config.smartStartStop || 'off'}
                onChange={handleSmartStartStopChange}
                options={smartStartStopOptions}
                disabled={!isConnected}
                orientation="horizontal"
              />
            </div>
          </div>

        </div>

        {/* 离线提示 */}
        {!isConnected && (
          <div className="mt-6 p-4 rounded-xl bg-gray-100 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-600">
            <div className="flex items-center gap-3 text-gray-600 dark:text-gray-400">
              <TriangleAlert className="w-5 h-5" />
              <span className="text-sm">设备未连接，部分功能不可用</span>
            </div>
          </div>
        )}

        {/* 版本和关于 */}
        <div className="mt-8 pt-6 border-t border-gray-200 dark:border-gray-700">
          <div className="text-center mb-4">
            <Badge variant="info" size="md">{appVersion ? `v${appVersion}` : 'v--'}</Badge>
          </div>

          {/* 关于页面 iframe */}
          <div className="rounded-2xl border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800">
            <div className="px-4 py-3 bg-gray-50 dark:bg-gray-700/50 border-b border-gray-200 dark:border-gray-600">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Info className="w-5 h-5 text-blue-600 dark:text-blue-400" />
                  <span className="font-medium text-gray-900 dark:text-white">关于 & 更新</span>
                </div>
                <button
                  onClick={() => handleOpenUrl('https://blog.tianli0.top/pages/bs2pro')}
                  className="text-xs text-blue-600 dark:text-blue-400 hover:underline"
                >
                  在浏览器中打开
                </button>
              </div>
            </div>
            <div className="relative h-80">
              <iframe
                src="https://blog.tianli0.top/pages/bs2pro"
                className="w-full h-full border-0"
                title="BS2PRO 关于页面"
                sandbox="allow-scripts allow-same-origin allow-popups allow-forms"
                loading="lazy"
                onLoad={() => setIframeLoaded(true)}
              />
              {!iframeLoaded && (
                <div className="absolute inset-0 flex items-center justify-center bg-gray-50 dark:bg-gray-800">
                  <div className="animate-spin w-8 h-8 border-4 border-blue-600 border-t-transparent rounded-full" />
                </div>
              )}
            </div>
          </div>

          {/* 开发者信息 */}
          <div className="mt-6 p-4 rounded-2xl bg-gradient-to-r from-blue-50 to-purple-50 dark:from-blue-900/20 dark:to-purple-900/20 border border-gray-200 dark:border-gray-700">
            <div className="flex items-center justify-center gap-4">
              <img 
                src="https://q1.qlogo.cn/g?b=qq&nk=507249007&s=640" 
                alt="开发者头像" 
                className="w-12 h-12 rounded-full border-2 border-white shadow-lg"
              />
              <div>
                <div className="font-semibold text-gray-900 dark:text-white">TIANLI</div>
                <button 
                  onClick={() => handleOpenUrl('mailto:wutianli@tianli0.top')}
                  className="text-sm text-blue-600 dark:text-blue-400 hover:underline"
                >
                  wutianli@tianli0.top
                </button>
              </div>
            </div>
          </div>

          {/* 调试面板 */}
          <Collapsible open={debugPanelOpen} onOpenChange={setDebugPanelOpen} className="mt-6">
            <div className="rounded-2xl border border-gray-200 dark:border-gray-700 overflow-hidden">
              <CollapsibleTrigger asChild>
                <button type="button" className="w-full px-4 py-3 flex items-center justify-between bg-gray-50 dark:bg-gray-700/50 hover:bg-gray-100 dark:hover:bg-gray-600/50 transition-colors">
                  <div className="flex items-center gap-3">
                    <Bug className="w-5 h-5 text-indigo-600 dark:text-indigo-400" />
                    <span className="font-medium text-gray-900 dark:text-white">调试面板</span>
                  </div>
                  <ChevronDown className={clsx(
                    'w-5 h-5 text-gray-500 transition-transform duration-200',
                    debugPanelOpen && 'rotate-180'
                  )} />
                </button>
              </CollapsibleTrigger>

              <CollapsibleContent>
                <div className="p-4 space-y-4">
                    {/* 调试模式 */}
                    <div className="flex items-center justify-between p-3 rounded-xl bg-gray-50 dark:bg-gray-700/50">
                      <div className="flex items-center gap-3">
                        <Bug className="w-5 h-5 text-gray-600 dark:text-gray-400" />
                        <div>
                          <div className="font-medium text-gray-900 dark:text-white">调试模式</div>
                          <div className="text-xs text-gray-500 dark:text-gray-400">启用详细日志输出</div>
                        </div>
                      </div>
                      <ToggleSwitch
                        enabled={config.debugMode}
                        onChange={toggleDebugMode}
                        color="purple"
                      />
                    </div>

                    {/* GUI 监控 */}
                    <div className="flex items-center justify-between p-3 rounded-xl bg-gray-50 dark:bg-gray-700/50">
                      <div className="flex items-center gap-3">
                        {config.guiMonitoring ? (
                          <Eye className="w-5 h-5 text-gray-600 dark:text-gray-400" />
                        ) : (
                          <EyeOff className="w-5 h-5 text-gray-600 dark:text-gray-400" />
                        )}
                        <div>
                          <div className="font-medium text-gray-900 dark:text-white">GUI 监控</div>
                          <div className="text-xs text-gray-500 dark:text-gray-400">监控 GUI 响应状态</div>
                        </div>
                      </div>
                      <ToggleSwitch
                        enabled={config.guiMonitoring}
                        onChange={toggleGuiMonitoring}
                        color="purple"
                      />
                    </div>

                    {/* 刷新调试信息 */}
                    <Button
                      variant="secondary"
                      onClick={fetchDebugInfo}
                      loading={debugInfoLoading}
                      className="w-full"
                    >
                      刷新调试信息
                    </Button>

                    {/* 调试信息显示 */}
                    {debugInfo && (
                      <ScrollArea className="max-h-60 rounded-xl bg-gray-900">
                        <pre className="p-3 text-xs text-green-400">
                          {JSON.stringify(debugInfo, null, 2)}
                        </pre>
                      </ScrollArea>
                    )}
                </div>
              </CollapsibleContent>
            </div>
          </Collapsible>
        </div>
      </Card>

      {/* 自定义转速警告对话框 */}
      {showCustomSpeedWarning && (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 p-4">
          <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-2xl max-w-md w-full p-6">
            <div className="flex justify-center mb-4">
              <div className="w-16 h-16 bg-orange-100 dark:bg-orange-900/30 rounded-full flex items-center justify-center">
                <TriangleAlert className="w-10 h-10 text-orange-600 dark:text-orange-400" />
              </div>
            </div>

            <h3 className="text-xl font-bold text-gray-900 dark:text-white text-center mb-3">
              ⚠️ 风险提示
            </h3>

            <div className="bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-800 rounded-xl p-4 mb-4">
              <p className="text-sm text-gray-700 dark:text-gray-300 mb-2 font-medium">
                启用自定义转速模式后：
              </p>
              <ul className="space-y-1 text-sm text-gray-600 dark:text-gray-400">
                <li>• 智能温控将被禁用</li>
                <li>• 风扇将以固定转速运行</li>
                <li>• 可能导致散热不足</li>
                <li>• 请确保了解相关风险</li>
              </ul>
            </div>

            <div className="bg-gray-50 dark:bg-gray-900/50 rounded-xl p-3 mb-4">
              <p className="text-sm text-gray-600 dark:text-gray-400 mb-1">当前设置转速：</p>
              <p className="text-2xl font-bold text-orange-600 dark:text-orange-400 text-center">
                {customSpeedInput} RPM
              </p>
            </div>

            <div className="flex gap-3">
              <Button
                variant="secondary"
                onClick={() => setShowCustomSpeedWarning(false)}
                className="flex-1"
              >
                取消
              </Button>
              <Button
                variant="primary"
                onClick={() => {
                  setShowCustomSpeedWarning(false);
                  handleCustomSpeedApply(true, customSpeedInput);
                }}
                className="flex-1 !bg-orange-600 hover:!bg-orange-700"
                icon={<CheckCircle2 className="w-5 h-5" />}
              >
                我已了解风险
              </Button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
