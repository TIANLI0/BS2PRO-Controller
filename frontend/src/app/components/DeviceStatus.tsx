'use client';

import React, { memo } from 'react';
import { 
  Cpu,
  Zap,
  RotateCw,
  Monitor,
  Wifi,
  Signal,
  Settings,
  CheckCircle2,
} from 'lucide-react';
import { types } from '../../../wailsjs/go/models';
import { apiService } from '../services/api';
import { ToggleSwitch, Card, Badge, Button } from './ui/index';
import clsx from 'clsx';

interface DeviceStatusProps {
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  config: types.AppConfig;
  onConnect: () => void;
  onDisconnect: () => void;
  onConfigChange: (config: types.AppConfig) => void;
}

// 温度状态判断
const getTempStatus = (temp: number) => {
  if (temp > 85) return { color: 'text-red-500', bg: 'bg-red-500', label: '过热' };
  if (temp > 75) return { color: 'text-orange-500', bg: 'bg-orange-500', label: '偏高' };
  if (temp > 60) return { color: 'text-yellow-500', bg: 'bg-yellow-500', label: '正常' };
  return { color: 'text-green-500', bg: 'bg-green-500', label: '良好' };
};

// ============ 独立的实时数据组件 - 避免触发父组件重绘 ============

// CPU 温度显示组件
const CpuTempDisplay = memo(function CpuTempDisplay({ 
  temp 
}: { 
  temp: number | undefined 
}) {
  const status = getTempStatus(temp || 0);
  return (
    <div className="flex flex-col items-center h-full">
      <div className="flex-1 flex flex-col justify-center">
        <div className="flex items-baseline gap-0.5 justify-center">
          <span className={clsx('text-3xl font-bold tabular-nums', status.color)}>
            {temp ?? '--'}
          </span>
          <span className="text-sm text-gray-400">°C</span>
        </div>
        <div className="text-xs text-gray-500 dark:text-gray-400 mt-1 text-center h-4">
          {status.label}
        </div>
      </div>
      <div className="w-full h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden mt-2">
        <div 
          className={clsx('h-full rounded-full transition-all duration-500', status.bg)}
          style={{ width: `${Math.min(100, ((temp || 0) / 100) * 100)}%` }}
        />
      </div>
    </div>
  );
});

// GPU 温度显示组件
const GpuTempDisplay = memo(function GpuTempDisplay({ 
  temp 
}: { 
  temp: number | undefined 
}) {
  const status = getTempStatus(temp || 0);
  return (
    <div className="flex flex-col items-center h-full">
      <div className="flex-1 flex flex-col justify-center">
        <div className="flex items-baseline gap-0.5 justify-center">
          <span className={clsx('text-3xl font-bold tabular-nums', status.color)}>
            {temp ?? '--'}
          </span>
          <span className="text-sm text-gray-400">°C</span>
        </div>
        <div className="text-xs text-gray-500 dark:text-gray-400 mt-1 text-center h-4">
          {status.label}
        </div>
      </div>
      <div className="w-full h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden mt-2">
        <div 
          className={clsx('h-full rounded-full transition-all duration-500', status.bg)}
          style={{ width: `${Math.min(100, ((temp || 0) / 100) * 100)}%` }}
        />
      </div>
    </div>
  );
});

// 风扇转速显示组件
const FanRpmDisplay = memo(function FanRpmDisplay({ 
  currentRpm,
  targetRpm,
  setGear,
}: { 
  currentRpm: number | undefined;
  targetRpm: number | undefined;
  setGear: string | undefined;
}) {
  const rpmPercentage = Math.min(100, ((currentRpm || 0) / 4000) * 100);
  return (
    <div className="flex flex-col items-center h-full">
      <div className="flex-1 flex flex-col justify-center">
        <div className="flex items-baseline gap-0.5 justify-center">
          <span className="text-3xl font-bold tabular-nums text-blue-600 dark:text-blue-400">
            {currentRpm ?? '--'}
          </span>
          <span className="text-sm text-gray-400">RPM</span>
        </div>
        <div className="text-xs text-gray-500 dark:text-gray-400 mt-1 text-center h-4">
          目标 {targetRpm ?? '--'} · {setGear || '--'}
        </div>
      </div>
      <div className="w-full h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden mt-2">
        <div 
          className="h-full bg-blue-500 rounded-full transition-all duration-300"
          style={{ width: `${rpmPercentage}%` }}
        />
      </div>
    </div>
  );
});

// 最高温度显示组件
const MaxTempDisplay = memo(function MaxTempDisplay({ 
  temp 
}: { 
  temp: number | undefined 
}) {
  const status = getTempStatus(temp || 0);
  return (
    <span className={clsx('text-sm font-semibold tabular-nums', status.color)}>
      {temp ?? '--'}°C
    </span>
  );
});

// ============ 主组件 ============

export default function DeviceStatus({ 
  isConnected, 
  fanData, 
  temperature, 
  config,
  onConnect, 
  onDisconnect,
  onConfigChange
}: DeviceStatusProps) {
  
  const handleAutoControlChange = async (enabled: boolean) => {
    try {
      await apiService.setAutoControl(enabled);
      const newConfig = types.AppConfig.createFrom({ ...config, autoControl: enabled });
      onConfigChange(newConfig);
    } catch (error) {
      console.error('设置智能变频失败:', error);
    }
  };

  // 设备型号判断
  const deviceModel = fanData?.maxGear === '超频' ? 'BS2 PRO' : 'BS2';

  return (
    <div className="space-y-4">
      {/* 顶部状态栏 */}
      <Card className={clsx(
        'p-5 relative overflow-hidden xl:mx-auto xl:max-w-5xl 2xl:max-w-6xl',
        isConnected && 'bg-gradient-to-r from-white via-white to-blue-50/50 dark:from-gray-800 dark:via-gray-800 dark:to-blue-900/20'
      )}>
        {/* 背景装饰 */}
        {isConnected && (
          <div className="absolute top-0 right-0 w-32 h-32 bg-gradient-to-br from-blue-500/10 to-purple-500/10 rounded-full blur-2xl -translate-y-1/2 translate-x-1/2" />
        )}
        
        <div className="flex items-center justify-between relative">
          {/* 左侧：设备信息 */}
          <div className="flex items-center gap-4">
            {/* 设备图标 */}
            <div className="relative">
              <div className={clsx(
                'w-14 h-14 rounded-2xl flex items-center justify-center transition-all duration-300',
                isConnected 
                  ? 'bg-gradient-to-br from-blue-500 to-indigo-600 shadow-lg shadow-blue-500/30' 
                  : 'bg-gray-100 dark:bg-gray-700/50'
              )}>
                <Monitor className={clsx(
                  'w-7 h-7 transition-colors',
                  isConnected ? 'text-white' : 'text-gray-500 dark:text-gray-400'
                )} />
              </div>
              {/* 连接状态指示点 */}
              <div className={clsx(
                'absolute -bottom-1 -right-1 w-5 h-5 rounded-full flex items-center justify-center ring-2 ring-white dark:ring-gray-800 transition-all duration-300',
                isConnected 
                  ? 'bg-green-500 shadow-lg shadow-green-500/50' 
                  : 'bg-gray-400 dark:bg-gray-500'
              )}>
                {isConnected ? (
                  <CheckCircle2 className="w-5 h-5 text-white" />
                ) : (
                  <span className="w-2 h-2 rounded-full bg-white" />
                )}
              </div>
            </div>

            {/* 设备名称和状态 */}
            <div>
              <div className="flex items-center gap-3">
                <h2 className="text-xl font-bold text-gray-900 dark:text-white">
                  {deviceModel}
                </h2>
                <Badge variant={isConnected ? 'success' : 'error'} size="sm">
                  {isConnected ? '已连接' : '离线'}
                </Badge>
              </div>
              <div className="flex items-center gap-2 mt-1">
                {isConnected ? (
                  <>
                    <span className={clsx(
                      'inline-flex items-center gap-1.5 text-sm',
                      config.autoControl ? 'text-blue-600 dark:text-blue-400' : 'text-orange-600 dark:text-orange-400'
                    )}>
                      {config.autoControl ? (
                        <Zap className="w-4 h-4" />
                      ) : (
                        <Settings className="w-4 h-4" />
                      )}
                      <span className="font-medium">
                        {config.autoControl ? '智能变频运行中' : '手动控制模式'}
                      </span>
                    </span>
                  </>
                ) : (
                  <span className="text-sm text-gray-500 dark:text-gray-400">
                    等待设备连接...
                  </span>
                )}
              </div>
            </div>
          </div>

          {/* 右侧：操作区 */}
          <div className="flex items-center gap-4">
            {isConnected && (
              <div className="flex items-center gap-3 px-4 py-2 bg-gray-50 dark:bg-gray-700/50 rounded-xl border border-gray-200 dark:border-gray-600">
                <ToggleSwitch
                  enabled={config.autoControl}
                  onChange={handleAutoControlChange}
                  label="智能变频"
                  color="blue"
                />
              </div>
            )}
            <Button
              variant={isConnected ? 'secondary' : 'primary'}
              size="sm"
              onClick={isConnected ? onDisconnect : onConnect}
              icon={isConnected ? undefined : <RotateCw className="w-4 h-4" />}
            >
              {isConnected ? '断开连接' : '连接设备'}
            </Button>
          </div>
        </div>
      </Card>

      {/* 核心数据仪表盘 */}
      {isConnected ? (
        <div className="grid grid-cols-3 gap-4 xl:mx-auto xl:max-w-4xl 2xl:max-w-5xl">
          {/* CPU 温度卡片 */}
          <Card className="p-5" hover>
            <div className="flex items-center gap-2 mb-4">
              <div className="p-2 rounded-lg bg-orange-50 dark:bg-orange-900/20">
                <Cpu className="w-5 h-5 text-orange-500" />
              </div>
              <span className="text-sm font-medium text-gray-600 dark:text-gray-400">CPU 温度</span>
            </div>
            
            <div className="h-20">
              <CpuTempDisplay temp={temperature?.cpuTemp} />
            </div>
          </Card>

          {/* GPU 温度卡片 */}
          <Card className="p-5" hover>
            <div className="flex items-center gap-2 mb-4">
              <div className="p-2 rounded-lg bg-purple-50 dark:bg-purple-900/20">
                <svg className="w-5 h-5 text-purple-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                  <rect x="4" y="4" width="16" height="16" rx="2" />
                  <rect x="8" y="8" width="8" height="8" rx="1" />
                  <path d="M4 9h1M4 15h1M19 9h1M19 15h1M9 4v1M15 4v1M9 19v1M15 19v1" strokeLinecap="round" />
                </svg>
              </div>
              <span className="text-sm font-medium text-gray-600 dark:text-gray-400">GPU 温度</span>
            </div>
            
            <div className="h-20">
              <GpuTempDisplay temp={temperature?.gpuTemp} />
            </div>
          </Card>

          {/* 风扇转速卡片 */}
          <Card className="p-5" hover>
            <div className="flex items-center gap-2 mb-4">
              <div className="p-2 rounded-lg bg-blue-50 dark:bg-blue-900/20">
                <Signal className="w-5 h-5 text-blue-500" />
              </div>
              <span className="text-sm font-medium text-gray-600 dark:text-gray-400">风扇转速</span>
            </div>
            
            <div className="h-20">
              <FanRpmDisplay 
                currentRpm={fanData?.currentRpm}
                targetRpm={fanData?.targetRpm}
                setGear={fanData?.setGear}
              />
            </div>
          </Card>
        </div>
      ) : (
        /* 未连接提示 */
        <Card className="p-8">
          <div className="text-center">
            <div className="w-16 h-16 mx-auto mb-4 rounded-xl bg-gray-100 dark:bg-gray-700 flex items-center justify-center">
              <Wifi className="w-8 h-8 text-gray-400" />
            </div>
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
              设备未连接
            </h3>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-6 max-w-xs mx-auto">
              请将 BS2/BS2PRO 散热器通过 蓝牙 连接到电脑
            </p>
            <Button onClick={onConnect} icon={<RotateCw className="w-4 h-4" />}>
              连接设备
            </Button>
          </div>
        </Card>
      )}

      {/* 运行状态详情 */}
      {isConnected && (
        <Card className="p-4 xl:mx-auto xl:max-w-4xl 2xl:max-w-5xl">
          <div className="flex items-center gap-2 mb-4">
            <Zap className="w-4 h-4 text-gray-500" />
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white">运行详情</h3>
          </div>
          
          <div className="grid grid-cols-4 gap-6">
            <div className="text-center">
              <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">控制模式</div>
              <div className={clsx(
                'text-sm font-semibold',
                config.autoControl ? 'text-blue-600 dark:text-blue-400' : 'text-orange-600 dark:text-orange-400'
              )}>
                {config.autoControl ? '智能变频' : '手动控制'}
              </div>
            </div>
            
            <div className="text-center">
              <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">最高功率</div>
              <div className="text-sm font-semibold text-gray-900 dark:text-white">
                {fanData?.maxGear || '--'}
              </div>
            </div>
            
            <div className="text-center">
              <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">工作模式</div>
              <div className="text-sm font-semibold text-gray-900 dark:text-white">
                {fanData?.workMode || '--'}
              </div>
            </div>
            
            <div className="text-center">
              <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">最高温度</div>
              <MaxTempDisplay temp={temperature?.maxTemp} />
            </div>
          </div>
        </Card>
      )}
    </div>
  );
}