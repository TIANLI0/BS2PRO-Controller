'use client';

import { memo, useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import {
  AlertTriangle,
  CircleHelp,
  Cpu,
  Zap,
  RotateCw,
  Wifi,
  Fan,
  Gpu,
  Settings,
  Gauge,
  Power,
  ShieldCheck,
  Sparkles,
} from 'lucide-react';
import { types } from '../../../wailsjs/go/models';
import { apiService } from '../services/api';
import { getReportedMaxRpm } from '../lib/manualGearPresets';
import { ToggleSwitch, Button } from './ui/index';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import clsx from 'clsx';

interface DeviceStatusProps {
  isConnected: boolean;
  deviceProductId: string | null;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  config: types.AppConfig;
  onConnect: () => void;
  onDisconnect: () => void;
  onConfigChange: (config: types.AppConfig) => void;
}

const getTempStatus = (temp: number) => {
  if (temp > 85) return { color: 'text-red-500', bg: 'bg-red-500', label: '过热' };
  if (temp > 75) return { color: 'text-orange-500', bg: 'bg-orange-500', label: '偏高' };
  if (temp > 60) return { color: 'text-yellow-500', bg: 'bg-yellow-500', label: '正常' };
  return { color: 'text-green-500', bg: 'bg-green-500', label: '良好' };
};

const getFanSpinDuration = (rpm?: number) => {
  if (!rpm || rpm <= 0) return 0;
  if (rpm >= 4200) return 0.45;
  if (rpm >= 3200) return 0.7;
  if (rpm >= 2200) return 1;
  return 1.35;
};

/* ── Memo sub-components to avoid parent re-renders ── */

const CpuTempDisplay = memo(function CpuTempDisplay({ temp }: { temp: number | undefined }) {
  const status = getTempStatus(temp || 0);
  return (
    <div className="flex flex-col items-center">
      <div className="flex items-baseline gap-0.5">
        <span className={clsx('text-2xl font-bold tabular-nums', status.color)}>
          {temp ?? '--'}
        </span>
        <span className="text-xs text-muted-foreground">°C</span>
      </div>
      <span className="mt-1 text-[11px] text-muted-foreground">{status.label}</span>
      <div className="mt-2.5 h-1 w-full overflow-hidden rounded-full bg-muted">
        <div
          className={clsx('h-full rounded-full transition-all duration-500', status.bg)}
          style={{ width: `${Math.min(100, ((temp || 0) / 100) * 100)}%` }}
        />
      </div>
    </div>
  );
});

const GpuTempDisplay = memo(function GpuTempDisplay({ temp }: { temp: number | undefined }) {
  const status = getTempStatus(temp || 0);
  return (
    <div className="flex flex-col items-center">
      <div className="flex items-baseline gap-0.5">
        <span className={clsx('text-2xl font-bold tabular-nums', status.color)}>
          {temp ?? '--'}
        </span>
        <span className="text-xs text-muted-foreground">°C</span>
      </div>
      <span className="mt-1 text-[11px] text-muted-foreground">{status.label}</span>
      <div className="mt-2.5 h-1 w-full overflow-hidden rounded-full bg-muted">
        <div
          className={clsx('h-full rounded-full transition-all duration-500', status.bg)}
          style={{ width: `${Math.min(100, ((temp || 0) / 100) * 100)}%` }}
        />
      </div>
    </div>
  );
});

const FanRpmDisplay = memo(function FanRpmDisplay({
  currentRpm,
  targetRpm,
  setGear,
}: {
  currentRpm: number | undefined;
  targetRpm: number | undefined;
  setGear: string | undefined;
}) {
  const pct = Math.min(100, ((currentRpm || 0) / 4000) * 100);

  return (
    <div className="flex flex-col items-center">
      <div className="flex items-baseline gap-0.5">
        <span className="text-2xl font-bold tabular-nums text-primary">{currentRpm ?? '--'}</span>
        <span className="text-xs text-muted-foreground">RPM</span>
      </div>
      <span className="mt-1 text-[11px] text-muted-foreground">
        目标 {targetRpm ?? '--'} · {setGear || '--'}
      </span>
      <div className="mt-2.5 h-1 w-full overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full bg-primary transition-all duration-300" style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
});

/* ── Main component ── */

export default function DeviceStatus({
  isConnected,
  deviceProductId,
  fanData,
  temperature,
  config,
  onConnect,
  onDisconnect,
  onConfigChange,
}: DeviceStatusProps) {
  const [bridgeWarningReady, setBridgeWarningReady] = useState(false);
  const hasBridgeWarning = isConnected && temperature?.bridgeOk === false;

  useEffect(() => {
    if (!hasBridgeWarning) {
      setBridgeWarningReady(false);
      return;
    }
    const timer = window.setTimeout(() => setBridgeWarningReady(true), 2000);
    return () => window.clearTimeout(timer);
  }, [hasBridgeWarning]);

  const handleAutoControlChange = async (enabled: boolean) => {
    try {
      await apiService.setAutoControl(enabled);
      onConfigChange(types.AppConfig.createFrom({ ...config, autoControl: enabled }));
    } catch (err) {
      console.error('设置智能变频失败:', err);
    }
  };

  const normalizedProductId = deviceProductId?.trim().toUpperCase() ?? '';
  const isProModel = normalizedProductId === '0X1002';
  const isBs2Model = normalizedProductId === '0X1001';
  const deviceModel = isProModel ? 'BS2 PRO' : isBs2Model ? 'BS2' : '未知设备';
  const deviceImageSrc = isBs2Model ? '/bs2.png' : '/bs2pro.png';
  const modeTitle = config.autoControl ? '智能控制' : config.customSpeedEnabled ? '固定转速' : '手动策略';
  const modeDesc = config.autoControl
    ? '根据实时温度自动调节转速'
    : config.customSpeedEnabled
      ? `当前固定为 ${config.customSpeedRPM || fanData?.currentRpm || '--'} RPM`
      : '可在设置页调整模式与参数';
  const fanSpinDuration = getFanSpinDuration(fanData?.currentRpm);
  const maxRpmInfo = getReportedMaxRpm(fanData?.gearSettings, fanData?.maxGear);
  const maxGearHighLevelRpm = maxRpmInfo.rpm;
  const maxRpmHint =
    maxGearHighLevelRpm === 4000
      ? '当前已解锁超频上限，最高可达 4000 RPM。'
      : maxGearHighLevelRpm === 3300
        ? '当前最高为强劲档，最高可达 3300 RPM，使用PD 27W充电头以解锁上限。'
        : maxGearHighLevelRpm === 2760
          ? '当前最高为标准档，最高可达 2760 RPM，使用PD 27W充电头以解锁上限。'
          : maxRpmInfo.codeHex
            ? `设备上报了未映射的最高挡位编码：${maxRpmInfo.codeHex}`
            : '等待设备上报最高转速能力。';

  return (
    <div className="space-y-4">
      {/* ── Device header card ── */}
      <div className="relative overflow-hidden rounded-3xl border border-border/70 bg-card/80 p-5 shadow-lg shadow-primary/5 backdrop-blur-2xl">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div
              className="flex h-12 w-16 items-center justify-center overflow-hidden"
            >
              <img
                src={deviceImageSrc}
                alt={`${deviceModel} device`}
                className="h-full w-full object-contain"
                draggable={false}
              />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="text-lg font-semibold text-foreground">{deviceModel}</span>
                <span
                  className={clsx(
                    'rounded-full px-2.5 py-1 text-xs font-medium',
                    isConnected
                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                      : 'bg-red-500/10 text-red-500',
                  )}
                >
                  {isConnected ? '已连接' : '离线'}
                </span>
              </div>
              {isConnected && (
                <div className="mt-0.5 flex items-center gap-1.5 text-sm text-muted-foreground">
                  {config.autoControl ? (
                    <Zap className="h-3 w-3 text-primary" />
                  ) : (
                    <Settings className="h-3 w-3" />
                  )}
                  <span>{modeTitle} · {modeDesc}</span>
                </div>
              )}
              {!isConnected && <p className="mt-0.5 text-sm text-muted-foreground">等待蓝牙连接…</p>}
            </div>
          </div>

          <div className="flex items-center gap-3">
            {isConnected && (
              <ToggleSwitch
                enabled={config.autoControl}
                onChange={handleAutoControlChange}
                label="智能变频"
                size="md"
                color="blue"
              />
            )}
            <Button
              variant={isConnected ? 'secondary' : 'primary'}
              size="md"
              onClick={isConnected ? onDisconnect : onConnect}
            >
              {isConnected ? '断开' : '连接'}
            </Button>
          </div>
        </div>
      </div>

      {/* ── Metric cards ── */}
      {isConnected ? (
        <motion.div
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3, ease: 'easeOut' }}
          className="grid grid-cols-3 items-stretch gap-4"
        >
          {/* CPU */}
          <div className="flex h-full flex-col rounded-2xl border border-border/70 bg-card/85 p-5 backdrop-blur-xl transition-shadow hover:shadow-md hover:shadow-primary/10">
            <div className="mb-3 flex items-center justify-center gap-2 text-sm font-medium text-muted-foreground">
              <Cpu className="h-4 w-4" />
              CPU
            </div>
            <CpuTempDisplay temp={temperature?.cpuTemp} />
          </div>

          {/* GPU */}
          <div className="flex h-full flex-col rounded-2xl border border-border/70 bg-card/85 p-5 backdrop-blur-xl transition-shadow hover:shadow-md hover:shadow-primary/10">
            <div className="mb-3 flex items-center justify-center gap-2 text-sm font-medium text-muted-foreground">
              <Gpu className="h-4 w-4" />
              GPU
            </div>
            <GpuTempDisplay temp={temperature?.gpuTemp} />
          </div>

          {/* Fan */}
          <div className="flex h-full flex-col rounded-2xl border border-border/70 bg-card/85 p-5 backdrop-blur-xl transition-shadow hover:shadow-md hover:shadow-primary/10">
            <div className="mb-3 flex items-center justify-center gap-2 text-sm font-medium text-muted-foreground">
              <motion.div
                animate={fanSpinDuration ? { rotate: 360 } : { rotate: 0 }}
                transition={fanSpinDuration ? { duration: fanSpinDuration, repeat: Infinity, ease: 'linear' } : undefined}
              >
                <Fan className="h-4 w-4" />
              </motion.div>
              风扇
            </div>
            <FanRpmDisplay
              currentRpm={fanData?.currentRpm}
              targetRpm={fanData?.targetRpm}
              setGear={fanData?.setGear}
            />
          </div>
        </motion.div>
      ) : (
        <motion.div
          initial={{ opacity: 0, scale: 0.98 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.3 }}
          className="rounded-2xl border border-dashed border-border bg-card p-14 text-center"
        >
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-muted">
            <Wifi className="h-7 w-7 text-muted-foreground" />
          </div>
          <h3 className="mb-1.5 text-lg font-semibold">设备未连接</h3>
          <p className="mb-5 text-base text-muted-foreground">请将散热器通过蓝牙连接到电脑</p>
          <Button onClick={onConnect} size="md" icon={<RotateCw className="h-4 w-4" />}>
            连接设备
          </Button>
        </motion.div>
      )}

      {/* ── Bridge warning ── */}
      {bridgeWarningReady && (
        <motion.div
          initial={{ opacity: 0, height: 0 }}
          animate={{ opacity: 1, height: 'auto' }}
          className="overflow-hidden"
        >
          <div className="rounded-xl border border-amber-200 bg-amber-50/70 p-3 text-sm dark:border-amber-800/60 dark:bg-amber-900/20">
            <div className="flex items-start gap-2 text-amber-800 dark:text-amber-200">
              <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
              <p>{temperature?.bridgeMessage || '温度桥接程序读取失败，请检查 PawnIO 驱动后重试。'}</p>
            </div>
          </div>
        </motion.div>
      )}

      {/* ── Running details ── */}
      {isConnected && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.15, duration: 0.3 }}
          className="rounded-3xl border border-border/70 bg-card/80 p-5 backdrop-blur-2xl"
        >
          <div className="mb-4 flex items-center gap-2">
            <Gauge className="h-4 w-4 text-muted-foreground" />
            <h3 className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
              控制与保护
            </h3>
          </div>

          <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
            <div className="rounded-xl border border-border/70 bg-background/50 p-3.5 backdrop-blur-lg">
              <div className="mb-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                <Sparkles className="h-3.5 w-3.5" />
                控制模式
              </div>
              <div className={clsx('text-base font-semibold', config.autoControl ? 'text-primary' : 'text-amber-600 dark:text-amber-400')}>
                {modeTitle}
              </div>
            </div>

            <div className="group rounded-xl border border-border/70 bg-background/50 p-3.5 backdrop-blur-lg">
              <div className="mb-1 flex items-center justify-between gap-2 text-xs text-muted-foreground">
                <div className="flex items-center gap-1.5">
                  <Power className="h-3.5 w-3.5" />
                  最高转速
                </div>
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <button
                        type="button"
                        className="inline-flex h-4 w-4 items-center justify-center rounded text-muted-foreground/80 opacity-0 transition-opacity hover:text-foreground group-hover:opacity-100"
                        aria-label="最高转速提示"
                      >
                        <CircleHelp className="h-3.5 w-3.5" />
                      </button>
                    </TooltipTrigger>
                    <TooltipContent>{maxRpmHint}</TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              </div>
              <div className="text-base font-semibold">
                {maxGearHighLevelRpm
                  ? `${maxGearHighLevelRpm} RPM`
                  : maxRpmInfo.codeHex || '--'}
              </div>
            </div>

            <div className="rounded-xl border border-border/70 bg-background/50 p-3.5 backdrop-blur-lg">
              <div className="mb-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                <Fan className="h-3.5 w-3.5" />
                工作模式
              </div>
              <div className="text-base font-semibold">{fanData?.workMode || '--'}</div>
            </div>

            <div className="rounded-xl border border-border/70 bg-background/50 p-3.5 backdrop-blur-lg">
              <div className="mb-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                <ShieldCheck className="h-3.5 w-3.5" />
                温度状态
              </div>
              <div className={clsx('text-base font-semibold tabular-nums', getTempStatus(temperature?.maxTemp || 0).color)}>
                {getTempStatus(temperature?.maxTemp || 0).label}
              </div>
            </div>
          </div>

        </motion.div>
      )}
    </div>
  );
}
