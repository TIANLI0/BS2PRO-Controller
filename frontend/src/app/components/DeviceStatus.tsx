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
  if (temp > 85) return { color: 'text-red-500', bg: 'bg-red-500', label: 'Overheat' };
  if (temp > 75) return { color: 'text-orange-500', bg: 'bg-orange-500', label: 'High' };
  if (temp > 60) return { color: 'text-yellow-500', bg: 'bg-yellow-500', label: 'Normal' };
  return { color: 'text-green-500', bg: 'bg-green-500', label: 'Good' };
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
        Target {targetRpm ?? '--'} · {setGear || '--'}
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
  const [activeCurveProfileName, setActiveCurveProfileName] = useState('');
  const hasBridgeWarning = isConnected && temperature?.bridgeOk === false;

  useEffect(() => {
    if (!hasBridgeWarning) {
      setBridgeWarningReady(false);
      return;
    }
    const timer = window.setTimeout(() => setBridgeWarningReady(true), 2000);
    return () => window.clearTimeout(timer);
  }, [hasBridgeWarning]);

  useEffect(() => {
    let cancelled = false;

    const loadActiveCurveProfile = async () => {
      try {
        const payload = await apiService.getFanCurveProfiles();
        const profiles = Array.isArray(payload?.profiles) ? payload.profiles : [];
        const preferredActiveId = ((config as any).activeFanCurveProfileId || payload?.activeId || profiles[0]?.id || '') as string;
        const activeProfile = profiles.find((p) => p.id === preferredActiveId) ?? profiles[0];
        if (!cancelled) {
          setActiveCurveProfileName(activeProfile?.name || '');
        }
      } catch {
        if (!cancelled) {
          setActiveCurveProfileName('');
        }
      }
    };

    loadActiveCurveProfile();
    return () => {
      cancelled = true;
    };
  }, [isConnected, (config as any).activeFanCurveProfileId]);

  const handleAutoControlChange = async (enabled: boolean) => {
    try {
      await apiService.setAutoControl(enabled);
      onConfigChange(types.AppConfig.createFrom({ ...config, autoControl: enabled }));
    } catch (err) {
      console.error('Failed to set smart speed control:', err);
    }
  };

  const normalizedProductId = deviceProductId?.trim().toUpperCase() ?? '';
  const isProModel = normalizedProductId === '0X1002';
  const isBs2Model = normalizedProductId === '0X1001';
  const deviceModel = isProModel ? 'BS2 PRO' : isBs2Model ? 'BS2' : 'Unknown Device';
  const deviceImageSrc = isBs2Model ? '/bs2.png' : '/bs2pro.png';
  const modeTitle = config.autoControl ? 'Smart Control' : config.customSpeedEnabled ? 'Fixed Speed' : 'Manual Strategy';
  const modeDesc = config.autoControl
    ? 'Auto-adjusts fan speed based on real-time temperature'
    : config.customSpeedEnabled
      ? `Currently fixed at ${config.customSpeedRPM || fanData?.currentRpm || '--'} RPM`
      : 'Adjust mode and parameters in Settings';
  const modeDisplayTitle = activeCurveProfileName ? `${modeTitle}（${activeCurveProfileName}）` : modeTitle;
  const fanSpinDuration = getFanSpinDuration(fanData?.currentRpm);
  const maxRpmInfo = getReportedMaxRpm(fanData?.gearSettings, fanData?.maxGear);
  const maxGearHighLevelRpm = maxRpmInfo.rpm;
  const maxRpmHint =
    maxGearHighLevelRpm === 4000
      ? 'Overclock limit unlocked, max speed up to 4000 RPM.'
      : maxGearHighLevelRpm === 3300
        ? 'Currently at Power gear, max 3300 RPM. Use a PD 27W charger to unlock the limit.'
        : maxGearHighLevelRpm === 2760
          ? 'Currently at Standard gear, max 2760 RPM. Use a PD 27W charger to unlock the limit.'
          : maxRpmInfo.codeHex
            ? `Device reported an unmapped max gear code: ${maxRpmInfo.codeHex}`
            : 'Waiting for device to report max speed capability.';

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
                  {isConnected ? 'Connected' : 'Offline'}
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
              {!isConnected && <p className="mt-0.5 text-sm text-muted-foreground">Waiting for Bluetooth connection...</p>}
            </div>
          </div>

          <div className="flex items-center gap-3">
            {isConnected && (
              <ToggleSwitch
                enabled={config.autoControl}
                onChange={handleAutoControlChange}
                label="Smart Speed"
                size="md"
                color="blue"
              />
            )}
            <Button
              variant={isConnected ? 'secondary' : 'primary'}
              size="md"
              onClick={isConnected ? onDisconnect : onConnect}
            >
              {isConnected ? 'Disconnect' : 'Connect'}
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
              Fan
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
          <h3 className="mb-1.5 text-lg font-semibold">Device Not Connected</h3>
          <p className="mb-5 text-base text-muted-foreground">Please connect the cooler to your computer via Bluetooth</p>
          <Button onClick={onConnect} size="md" icon={<RotateCw className="h-4 w-4" />}>
            Connect Device
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
              <p>{temperature?.bridgeMessage || 'Temperature bridge program failed to read. Please check the PawnIO driver and try again.'}</p>
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
              Control & Protection
            </h3>
          </div>

          <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
            <div className="rounded-xl border border-border/70 bg-background/50 p-3.5 backdrop-blur-lg">
              <div className="mb-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                <Sparkles className="h-3.5 w-3.5" />
                Control Mode
              </div>
              <div className={clsx('text-base font-semibold', config.autoControl ? 'text-primary' : 'text-amber-600 dark:text-amber-400')}>
                {modeDisplayTitle}
              </div>
            </div>

            <div className="group rounded-xl border border-border/70 bg-background/50 p-3.5 backdrop-blur-lg">
              <div className="mb-1 flex items-center justify-between gap-2 text-xs text-muted-foreground">
                <div className="flex items-center gap-1.5">
                  <Power className="h-3.5 w-3.5" />
                  Max Speed
                </div>
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <button
                        type="button"
                        className="inline-flex h-4 w-4 items-center justify-center rounded text-muted-foreground/80 opacity-0 transition-opacity hover:text-foreground group-hover:opacity-100"
                        aria-label="Max speed hint"
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
                Work Mode
              </div>
              <div className="text-base font-semibold">{fanData?.workMode || '--'}</div>
            </div>

            <div className="rounded-xl border border-border/70 bg-background/50 p-3.5 backdrop-blur-lg">
              <div className="mb-1 flex items-center gap-1.5 text-xs text-muted-foreground">
                <ShieldCheck className="h-3.5 w-3.5" />
                Temp Status
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
