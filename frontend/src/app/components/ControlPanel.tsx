'use client';

import React, { useState, useCallback, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
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
  Flame,
  Clock3,
  BarChart3,
  Sparkles,
  Rocket,
} from 'lucide-react';
import { apiService } from '../services/api';
import { types } from '../../../wailsjs/go/models';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import { DebugInfo } from '../types/app';
import { ToggleSwitch, Button, Select, ScrollArea, Slider } from './ui/index';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import clsx from 'clsx';

interface ControlPanelProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
}

/* ── Helpers ── */

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
  if (!raw) return defaults;

  const normalized = types.LightStripConfig.createFrom({
    mode: raw.mode || defaults.mode,
    speed: raw.speed || defaults.speed,
    brightness: typeof raw.brightness === 'number' ? Math.max(0, Math.min(100, raw.brightness)) : defaults.brightness,
    colors: Array.isArray(raw.colors) && raw.colors.length > 0 ? raw.colors : defaults.colors,
  });

  if ((normalized.colors || []).length < 3) {
    const merged = [...(normalized.colors || [])];
    while (merged.length < 3) merged.push(defaults.colors[merged.length]);
    normalized.colors = merged;
  }
  return normalized;
}

function rgbToHex(color: types.RGBColor): string {
  const h = (v: number) => v.toString(16).padStart(2, '0');
  return `#${h(color.r || 0)}${h(color.g || 0)}${h(color.b || 0)}`;
}

function hexToRgb(hex: string): types.RGBColor {
  const n = Number.parseInt(hex.replace('#', ''), 16);
  return types.RGBColor.createFrom({ r: (n >> 16) & 255, g: (n >> 8) & 255, b: n & 255 });
}

function getRequiredColorCount(mode: string): number {
  switch (mode) {
    case 'static_single': return 1;
    case 'off': case 'smart_temp': case 'flowing': return 0;
    case 'static_multi': return 3;
    default: return 3;
  }
}

/* ── Section wrapper ── */

function Section({
  title,
  icon: Icon,
  children,
  className,
}: {
  title: string;
  icon: React.ComponentType<{ className?: string }>;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section className={clsx('rounded-2xl border border-border bg-card shadow-sm', className)}>
      <div className="flex items-center gap-2.5 border-b border-border/60 px-5 py-4">
        <Icon className="h-4.5 w-4.5 text-muted-foreground" />
        <h3 className="text-base font-semibold text-foreground">{title}</h3>
      </div>
      <div className="divide-y divide-border/60">{children}</div>
    </section>
  );
}

/* ── Setting row ── */

function SettingRow({
  icon,
  title,
  description,
  tip,
  children,
  disabled,
}: {
  icon?: React.ReactNode;
  title: string;
  description?: string;
  tip?: string;
  children: React.ReactNode;
  disabled?: boolean;
}) {
  return (
    <div className={clsx('flex items-center justify-between gap-4 px-5 py-4', disabled && 'opacity-50')}>
      <div className="flex items-center gap-3 min-w-0">
        {icon && (
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            {icon}
          </div>
        )}
        <div className="min-w-0">
          <div className="text-base font-medium text-foreground">{title}</div>
          {description && <div className="text-sm text-muted-foreground line-clamp-2">{description}</div>}
          {tip && <div className="mt-0.5 text-xs text-primary/80">{tip}</div>}
        </div>
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  );
}

/* ── Main ControlPanel ── */

export default function ControlPanel({ config, onConfigChange, isConnected, fanData, temperature }: ControlPanelProps) {
  const [loadingStates, setLoadingStates] = useState<Record<string, boolean>>({});
  const [debugInfo, setDebugInfo] = useState<DebugInfo | null>(null);
  const [debugInfoLoading, setDebugInfoLoading] = useState(false);
  const [debugPanelOpen, setDebugPanelOpen] = useState(false);
  const [showCustomSpeedWarning, setShowCustomSpeedWarning] = useState(false);
  const [customSpeedInput, setCustomSpeedInput] = useState<number>((config as any).customSpeedRPM || 2000);
  const [appVersion, setAppVersion] = useState('');
  const [latestReleaseTag, setLatestReleaseTag] = useState('');
  const [latestReleaseUrl, setLatestReleaseUrl] = useState('');
  const [latestReleaseBody, setLatestReleaseBody] = useState('');
  const [releaseLoading, setReleaseLoading] = useState(false);
  const [releaseError, setReleaseError] = useState('');
  const [lightStripConfig, setLightStripConfig] = useState<types.LightStripConfig>(() => normalizeLightStripConfig(config));

  const setLoading = (key: string, value: boolean) => setLoadingStates((prev) => ({ ...prev, [key]: value }));

  const handleOpenUrl = useCallback((url: string) => {
    try { BrowserOpenURL(url); } catch { /* noop */ }
  }, []);

  const isLatestVersion = useCallback((currentVersion: string, latestVersion: string) => {
    const parse = (v: string) => (v.match(/\d+/g) || []).map((n) => Number(n));
    const current = parse(currentVersion);
    const latest = parse(latestVersion);
    const length = Math.max(current.length, latest.length);

    for (let i = 0; i < length; i += 1) {
      const currentPart = current[i] ?? 0;
      const latestPart = latest[i] ?? 0;
      if (latestPart > currentPart) return false;
      if (latestPart < currentPart) return true;
    }

    return true;
  }, []);

  const checkLatestRelease = useCallback(async () => {
    setReleaseLoading(true);
    setReleaseError('');
    try {
      const response = await fetch('https://api.github.com/repos/TIANLI0/BS2PRO-Controller/releases/latest', {
        headers: { Accept: 'application/vnd.github+json' },
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const data = await response.json();
      setLatestReleaseTag(data?.tag_name || '');
      setLatestReleaseUrl(data?.html_url || 'https://github.com/TIANLI0/BS2PRO-Controller/releases/latest');
      setLatestReleaseBody(typeof data?.body === 'string' ? data.body.trim() : '');
    } catch {
      setReleaseError('检查更新失败，请稍后重试');
      setLatestReleaseTag('');
      setLatestReleaseUrl('https://github.com/TIANLI0/BS2PRO-Controller/releases/latest');
      setLatestReleaseBody('');
    } finally {
      setReleaseLoading(false);
    }
  }, []);

  const hasNewVersion = !!appVersion && !!latestReleaseTag && !isLatestVersion(appVersion, latestReleaseTag);

  /* ── Handlers (same logic as before) ── */

  const handleAutoControlChange = useCallback(async (enabled: boolean) => {
    setLoading('autoControl', true);
    try {
      await apiService.setAutoControl(enabled);
      onConfigChange(types.AppConfig.createFrom({ ...config, autoControl: enabled }));
    } catch { /* noop */ } finally { setLoading('autoControl', false); }
  }, [config, onConfigChange]);

  const handleCustomSpeedApply = useCallback(async (enabled: boolean, rpm: number) => {
    setLoading('customSpeed', true);
    try {
      await apiService.setCustomSpeed(enabled, rpm);
      onConfigChange(types.AppConfig.createFrom({
        ...config,
        customSpeedEnabled: enabled,
        customSpeedRPM: rpm,
        autoControl: enabled ? false : config.autoControl,
      }));
    } catch { /* noop */ } finally { setLoading('customSpeed', false); }
  }, [config, onConfigChange]);

  const handleCustomSpeedToggle = useCallback((enabled: boolean) => {
    if (enabled) setShowCustomSpeedWarning(true);
    else handleCustomSpeedApply(false, customSpeedInput);
  }, [customSpeedInput, handleCustomSpeedApply]);

  const handleGearLightChange = useCallback(async (enabled: boolean) => {
    if (!isConnected) return;
    setLoading('gearLight', true);
    try {
      const ok = await apiService.setGearLight(enabled);
      if (ok) onConfigChange(types.AppConfig.createFrom({ ...config, gearLight: enabled }));
    } catch { /* noop */ } finally { setLoading('gearLight', false); }
  }, [config, onConfigChange, isConnected]);

  const handlePowerOnStartChange = useCallback(async (enabled: boolean) => {
    if (!isConnected) return;
    setLoading('powerOnStart', true);
    try {
      const ok = await apiService.setPowerOnStart(enabled);
      if (ok) onConfigChange(types.AppConfig.createFrom({ ...config, powerOnStart: enabled }));
    } catch { /* noop */ } finally { setLoading('powerOnStart', false); }
  }, [config, onConfigChange, isConnected]);

  const handleWindowsAutoStartChange = useCallback(async (enabled: boolean) => {
    setLoading('windowsAutoStart', true);
    try {
      const isAdmin = await apiService.isRunningAsAdmin();
      if (enabled) await apiService.setAutoStartWithMethod(true, isAdmin ? 'task_scheduler' : 'registry');
      else await apiService.setAutoStartWithMethod(false, '');
      onConfigChange(types.AppConfig.createFrom({ ...config, windowsAutoStart: enabled }));
    } catch (e) { alert(`设置自启动失败: ${e}`); } finally { setLoading('windowsAutoStart', false); }
  }, [config, onConfigChange]);

  const handleIgnoreDeviceOnReconnectChange = useCallback(async (enabled: boolean) => {
    try {
      const newCfg = types.AppConfig.createFrom({ ...config, ignoreDeviceOnReconnect: enabled });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch { /* noop */ }
  }, [config, onConfigChange]);

  const handleSmartStartStopChange = useCallback(async (mode: string) => {
    if (!isConnected) return;
    try {
      const ok = await apiService.setSmartStartStop(mode);
      if (ok) onConfigChange(types.AppConfig.createFrom({ ...config, smartStartStop: mode }));
    } catch { /* noop */ }
  }, [config, onConfigChange, isConnected]);

  const toggleDebugMode = useCallback(async () => {
    try {
      await apiService.setDebugMode(!config.debugMode);
      onConfigChange(types.AppConfig.createFrom({ ...config, debugMode: !config.debugMode }));
    } catch { /* noop */ }
  }, [config, onConfigChange]);

  const toggleGuiMonitoring = useCallback(async () => {
    try {
      const newCfg = types.AppConfig.createFrom({ ...config, guiMonitoring: !config.guiMonitoring });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch { /* noop */ }
  }, [config, onConfigChange]);

  const fetchDebugInfo = useCallback(async () => {
    setDebugInfoLoading(true);
    try { setDebugInfo(await apiService.getDebugInfo()); } catch { /* noop */ } finally { setDebugInfoLoading(false); }
  }, []);

  const handleSampleCountChange = useCallback(async (count: number) => {
    try {
      const newCfg = types.AppConfig.createFrom({ ...config, tempSampleCount: count });
      await apiService.updateConfig(newCfg);
      onConfigChange(newCfg);
    } catch { /* noop */ }
  }, [config, onConfigChange]);

  useEffect(() => { const i = setInterval(() => { apiService.updateGuiResponseTime().catch(() => {}); }, 10000); return () => clearInterval(i); }, []);
  useEffect(() => { apiService.getAppVersion().then((v) => setAppVersion(v || '')).catch(() => setAppVersion('')); }, []);
  useEffect(() => { checkLatestRelease(); }, [checkLatestRelease]);
  useEffect(() => { setLightStripConfig(normalizeLightStripConfig(config)); }, [config]);

  /* ── Options data ── */

  const smartStartStopOptions = [
    { value: 'off', label: '关闭', description: '禁用智能启停功能' },
    { value: 'immediate', label: '即时', description: '立即响应系统负载变化' },
    { value: 'delayed', label: '延时', description: '延时响应，避免频繁启停' },
  ];

  const sampleCountOptions = [
    { value: 1, label: '1次 (即时)' },
    { value: 2, label: '2次 (2s)' },
    { value: 3, label: '3次 (3s)' },
    { value: 5, label: '5次 (5s)' },
    { value: 10, label: '10次 (10s)' },
  ];

  const lightModeOptions = [
    { value: 'off', label: '关闭灯光', description: '关闭所有RGB灯光' },
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
    { name: '霓虹', colors: [{ r: 255, g: 0, b: 128 }, { r: 0, g: 255, b: 255 }, { r: 128, g: 0, b: 255 }] },
    { name: '森林', colors: [{ r: 86, g: 169, b: 84 }, { r: 161, g: 210, b: 106 }, { r: 44, g: 120, b: 115 }] },
    { name: '冰川', colors: [{ r: 80, g: 170, b: 255 }, { r: 116, g: 214, b: 255 }, { r: 200, g: 240, b: 255 }] },
  ];

  const requiredColorCount = getRequiredColorCount(lightStripConfig.mode);

  const handleLightColorChange = useCallback((index: number, hex: string) => {
    setLightStripConfig((prev) => {
      const colors = [...(prev.colors || [])];
      while (colors.length < 3) colors.push(types.RGBColor.createFrom({ r: 255, g: 255, b: 255 }));
      colors[index] = hexToRgb(hex);
      return types.LightStripConfig.createFrom({ ...prev, colors });
    });
  }, []);

  const handleApplyLightStrip = useCallback(async () => {
    setLoading('lightStrip', true);
    try {
      const normalizedColors = [...(lightStripConfig.colors || [])];
      if (requiredColorCount > 0) while (normalizedColors.length < requiredColorCount) normalizedColors.push(types.RGBColor.createFrom({ r: 255, g: 255, b: 255 }));
      const submitConfig = types.LightStripConfig.createFrom({
        ...lightStripConfig,
        colors: requiredColorCount > 0 ? normalizedColors.slice(0, Math.max(requiredColorCount, 3)) : normalizedColors,
      });
      await apiService.setLightStrip(submitConfig);
      onConfigChange(types.AppConfig.createFrom({ ...config, lightStrip: submitConfig }));
    } catch (e) { alert(`设置灯带失败: ${e}`); } finally { setLoading('lightStrip', false); }
  }, [lightStripConfig, config, onConfigChange, requiredColorCount]);

  return (
    <>
      <div className="space-y-4">
        <section className="rounded-2xl border border-border bg-card p-5 shadow-sm">
          <div className="mb-4 flex items-center gap-2">
            <Settings className="h-4 w-4 text-muted-foreground" />
            <h3 className="text-base font-semibold text-foreground">实时概览</h3>
          </div>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            <div className="rounded-xl border border-border/70 bg-muted/30 p-4 text-center">
              <div className="text-sm text-muted-foreground">当前温度</div>
              <div className={clsx(
                'mt-1 text-2xl font-semibold tabular-nums',
                (temperature?.maxTemp ?? 0) > 80 ? 'text-red-500' : (temperature?.maxTemp ?? 0) > 70 ? 'text-amber-500' : 'text-primary'
              )}>
                {temperature?.maxTemp ?? '--'}°C
              </div>
              <div className="mt-1 text-xs text-muted-foreground">CPU {temperature?.cpuTemp ?? '--'}°C · GPU {temperature?.gpuTemp ?? '--'}°C</div>
            </div>
            <div className="rounded-xl border border-border/70 bg-muted/30 p-4 text-center">
              <div className="text-sm text-muted-foreground">实时转速</div>
              <div className="mt-1 text-2xl font-semibold tabular-nums text-primary">{fanData?.currentRpm ?? '--'} RPM</div>
              <div className="mt-1 text-xs text-muted-foreground">{fanData?.workMode ?? '--'}</div>
            </div>
            <div className="rounded-xl border border-border/70 bg-muted/30 p-4 text-center">
              <div className="text-sm text-muted-foreground">目标转速</div>
              <div className="mt-1 text-2xl font-semibold tabular-nums text-primary">{fanData?.targetRpm ?? '--'} RPM</div>
              <div className="mt-1 text-xs text-muted-foreground">挡位 {fanData?.setGear ?? '--'}</div>
            </div>
          </div>
        </section>

        {/* ═══════════ 1. 灯光效果 ═══════════ */}
        <Section title="灯光效果" icon={Sparkles}>
          <div className="space-y-4 p-5">
            <div className="grid grid-cols-2 gap-3">
              <Select
                value={lightStripConfig.mode}
                onChange={(v: string | number) => setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, mode: v as string }))}
                options={lightModeOptions}
                size="sm"
                label="效果模式"
              />
              <Select
                value={lightStripConfig.speed}
                onChange={(v: string | number) => setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, speed: v as string }))}
                options={lightSpeedOptions}
                size="sm"
                label="动画速度"
                disabled={['off', 'smart_temp', 'static_single', 'static_multi'].includes(lightStripConfig.mode)}
              />
            </div>

            <Slider
              min={0} max={100} step={1}
              value={lightStripConfig.brightness}
              onChange={(v) => setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, brightness: v }))}
              label="亮度"
              valueFormatter={(v) => `${v}%`}
              disabled={lightStripConfig.mode === 'off' || lightStripConfig.mode === 'smart_temp'}
            />

            {lightStripConfig.mode === 'smart_temp' && (
              <div className="rounded-lg border border-amber-300/40 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-300">
                智能温控模式由设备自动控制灯效，不支持手动调节颜色与亮度。
              </div>
            )}

            <AnimatePresence>
              {requiredColorCount > 0 && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  className="space-y-3 overflow-hidden"
                >
                  <div className="flex flex-wrap gap-2">
                    {lightColorPresets.map((preset) => (
                      <button
                        key={preset.name}
                        type="button"
                        onClick={() => setLightStripConfig(types.LightStripConfig.createFrom({ ...lightStripConfig, colors: preset.colors }))}
                        className="rounded-lg border border-border px-3 py-1.5 text-xs text-foreground transition-colors hover:bg-muted"
                      >
                        {preset.name}
                      </button>
                    ))}
                  </div>

                  <div className={clsx('grid gap-3', requiredColorCount === 1 ? 'grid-cols-1' : 'grid-cols-3')}>
                    {Array.from({ length: requiredColorCount }).map((_, i) => (
                      <div key={i}>
                        <label className="mb-1 block text-xs text-muted-foreground">颜色 {i + 1}</label>
                        <input
                          type="color"
                          value={rgbToHex((lightStripConfig.colors || [])[i] || types.RGBColor.createFrom({ r: 255, g: 255, b: 255 }))}
                          onChange={(e) => handleLightColorChange(i, e.target.value)}
                          className="h-9 w-full cursor-pointer rounded-lg border border-border bg-card"
                        />
                      </div>
                    ))}
                  </div>
                </motion.div>
              )}
            </AnimatePresence>

            <div className="flex items-center justify-between pt-1">
              <span className="text-xs text-muted-foreground">
                {isConnected ? '应用后立即生效' : '下次连接时自动生效'}
              </span>
              <Button variant="primary" size="sm" onClick={handleApplyLightStrip} loading={loadingStates.lightStrip}>
                应用
              </Button>
            </div>
          </div>
        </Section>

        {/* ═══════════ 2. 风扇控制 ═══════════ */}
        <Section title="风扇控制" icon={Settings}>
          {/* Auto control */}
          <SettingRow
            icon={config.autoControl ? <Play className="h-4 w-4 text-emerald-500" /> : <Pause className="h-4 w-4" />}
            title="自动温度控制"
            description="根据温度曲线自动调节风扇转速"
            disabled={(config as any).customSpeedEnabled}
          >
            <ToggleSwitch
              enabled={config.autoControl}
              onChange={handleAutoControlChange}
              disabled={(config as any).customSpeedEnabled}
              loading={loadingStates.autoControl}
              size="sm"
              color="green"
            />
          </SettingRow>

          {/* Sample count (conditional) */}
          <AnimatePresence>
            {config.autoControl && (
              <motion.div
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: 'auto' }}
                exit={{ opacity: 0, height: 0 }}
                className="overflow-hidden"
              >
                <SettingRow
                  icon={<BarChart3 className="h-4 w-4" />}
                  title="采样时间"
                  description="降低频繁调整带来的轴噪，不知道默认即可"
                >
                  <div className="w-32">
                    <Select
                      value={(config as any).tempSampleCount || 1}
                      onChange={(v: string | number) => handleSampleCountChange(v as number)}
                      options={sampleCountOptions}
                      size="sm"
                    />
                  </div>
                </SettingRow>
              </motion.div>
            )}
          </AnimatePresence>

          {/* Custom speed */}
          <div className="px-5 py-4">
            <div className="flex items-center justify-between">
              <div className="flex min-w-0 items-center gap-3">
                <div className={clsx(
                  'flex h-9 w-9 shrink-0 items-center justify-center rounded-lg transition-colors',
                  (config as any).customSpeedEnabled ? 'bg-amber-500/15 text-amber-600' : 'bg-muted text-muted-foreground',
                )}>
                  <Flame className="h-4 w-4" />
                </div>
                <div>
                  <div className="text-base font-medium text-foreground">自定义转速</div>
                  <div className="text-sm text-muted-foreground">固定转速，适合特殊场景</div>
                </div>
              </div>
              <ToggleSwitch
                enabled={(config as any).customSpeedEnabled || false}
                onChange={handleCustomSpeedToggle}
                disabled={!isConnected}
                loading={loadingStates.customSpeed}
                size="sm"
                color="orange"
              />
            </div>

            <AnimatePresence>
              {(config as any).customSpeedEnabled && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  className="overflow-hidden"
                >
                  <div className="mt-3 flex items-center gap-3 rounded-xl border border-amber-300/40 bg-amber-50/50 p-3.5 dark:bg-amber-900/10">
                    <input
                      type="number"
                      value={customSpeedInput}
                      onChange={(e) => setCustomSpeedInput(Number(e.target.value))}
                      className="flex-1 rounded-lg border border-border bg-card px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-amber-500/50 focus:border-transparent"
                      min={1000} max={4000} step={50}
                    />
                    <Button variant="primary" size="sm" onClick={() => handleCustomSpeedApply(true, customSpeedInput)} className="bg-amber-600 hover:bg-amber-700 text-white">
                      应用
                    </Button>
                  </div>
                  <p className="mt-2 text-[11px] text-amber-700 dark:text-amber-300">
                    ⚠ 自定义转速会禁用智能温控
                  </p>
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        </Section>

        {/* ═══════════ 3. 设备设置 ═══════════ */}
        <Section title="设备设置" icon={Zap}>
          <SettingRow
            icon={<Lightbulb className={clsx('h-4 w-4', config.gearLight ? 'text-yellow-500' : '')} />}
            title="挡位灯"
            description="控制设备上的挡位指示灯"
            disabled={!isConnected}
          >
            <ToggleSwitch
              enabled={config.gearLight}
              onChange={handleGearLightChange}
              disabled={!isConnected}
              loading={loadingStates.gearLight}
              size="sm"
            />
          </SettingRow>

          <SettingRow
            icon={<Power className={clsx('h-4 w-4', config.powerOnStart ? 'text-primary' : '')} />}
            title="通电自启动"
            description="设备通电后自动运行"
            disabled={!isConnected}
          >
            <ToggleSwitch
              enabled={config.powerOnStart}
              onChange={handlePowerOnStartChange}
              disabled={!isConnected}
              loading={loadingStates.powerOnStart}
              size="sm"
            />
          </SettingRow>

          <SettingRow
            icon={<Zap className="h-4 w-4" />}
            title="智能启停"
            description="系统关闭后何时停止散热"
            disabled={!isConnected}
          >
            <div className="w-40">
              <Select
                value={config.smartStartStop || 'off'}
                onChange={(v: string | number) => handleSmartStartStopChange(v as string)}
                options={smartStartStopOptions.map((item) => ({ value: item.value, label: item.label }))}
                disabled={!isConnected}
                size="sm"
              />
            </div>
          </SettingRow>
        </Section>

        {/* ═══════════ 4. 系统设置 ═══════════ */}
        <Section title="系统设置" icon={Monitor}>
          <SettingRow
            icon={<Monitor className={clsx('h-4 w-4', config.windowsAutoStart ? 'text-emerald-500' : '')} />}
            title="开机自启动"
            description="Windows 启动时自动运行"
            tip="以管理员身份运行可避免每次 UAC 授权"
          >
            <ToggleSwitch
              enabled={config.windowsAutoStart}
              onChange={handleWindowsAutoStartChange}
              loading={loadingStates.windowsAutoStart}
              size="sm"
              color="green"
            />
          </SettingRow>

          <SettingRow
            icon={<Clock3 className={clsx('h-4 w-4', (config as any).ignoreDeviceOnReconnect ? 'text-emerald-500' : '')} />}
            title="断连保持配置"
            description="重连后继续使用 APP 配置"
            tip="推荐开启，防止断连后进入设备默认模式"
          >
            <ToggleSwitch
              enabled={(config as any).ignoreDeviceOnReconnect ?? true}
              onChange={handleIgnoreDeviceOnReconnectChange}
              size="sm"
              color="green"
            />
          </SettingRow>
        </Section>

        {/* ═══════════ Offline tip ═══════════ */}
        {!isConnected && (
          <div className="flex items-center gap-2 rounded-xl border border-border bg-muted/50 px-4 py-3 text-sm text-muted-foreground">
            <TriangleAlert className="h-4 w-4 shrink-0" />
            设备未连接，部分功能不可用
          </div>
        )}

        {/* ═══════════ 5. 关于与更新 ═══════════ */}
        <section className="rounded-2xl border border-border bg-card">
          <div className="flex items-center gap-2 border-b border-border/60 px-4 py-3">
            <Rocket className="h-4 w-4 text-muted-foreground" />
            <h3 className="text-sm font-semibold text-foreground">关于与更新</h3>
            <span className="ml-auto text-[11px] text-muted-foreground">BS2PRO Controller</span>
          </div>

          <div className="space-y-3 border-b border-border/60 px-4 py-3.5">
            <div className="flex flex-wrap items-center gap-2 rounded-xl border border-border/70 bg-muted/35 px-3 py-3">
              <span className="inline-flex items-center rounded-full border border-border/70 bg-background/70 px-2.5 py-1 text-xs font-medium text-foreground">
                BS2PRO Controller
              </span>
              <span className="inline-flex items-center rounded-full border border-border/70 bg-background/70 px-2.5 py-1 text-xs text-muted-foreground">
                当前 {appVersion ? `v${appVersion}` : '--'}
              </span>
              <a
                href="https://github.com/TIANLI0/BS2PRO-Controller/releases/latest"
                onClick={(e) => {
                  e.preventDefault();
                  handleOpenUrl(latestReleaseUrl || 'https://github.com/TIANLI0/BS2PRO-Controller/releases/latest');
                }}
                className="inline-flex items-center gap-1.5 rounded-full border border-primary/40 bg-primary/10 px-2.5 py-1 text-xs font-medium text-primary transition-colors hover:bg-primary/15"
              >
                最新 {releaseLoading ? '检查中…' : latestReleaseTag || '--'}
                {hasNewVersion && !releaseLoading && <span className="h-2 w-2 rounded-full bg-destructive" />}
              </a>
            </div>

            {releaseError && <div className="text-xs text-amber-600 dark:text-amber-300">{releaseError}</div>}

            {hasNewVersion && (
              <div className="rounded-xl border border-border/70 bg-background/50 p-3">
                <div className="mb-2 text-xs font-medium text-muted-foreground">Release 日志</div>
                {latestReleaseBody ? (
                  <ScrollArea className="max-h-40">
                    <p className="whitespace-pre-wrap text-xs leading-relaxed text-foreground/90">{latestReleaseBody}</p>
                  </ScrollArea>
                ) : (
                  <p className="text-xs text-muted-foreground">暂无日志内容，或本次获取失败。</p>
                )}
              </div>
            )}
          </div>

          <div className="px-4 py-3">
            <div className="rounded-xl border border-border/70 bg-muted/35 p-3">
              <div className="mb-2 text-xs text-muted-foreground">开发者</div>
              <div className="flex items-center gap-3">
                <img
                  src="http://q1.qlogo.cn/g?b=qq&nk=507249007&s=640"
                  alt="Tianli 头像"
                  className="h-12 w-12 rounded-full border border-border object-cover"
                  referrerPolicy="no-referrer"
                />
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium text-foreground">Tianli</div>
                  <div className="mt-0.5 text-xs text-muted-foreground">一个不知名开发者</div>
                </div>
              </div>

              <div className="mt-3 space-y-1.5 border-t border-border/60 pt-2.5 text-xs">
                <div className="flex items-center justify-between gap-3">
                  <span className="text-muted-foreground">邮箱</span>
                  <a
                    href="mailto:wutianli@tianli0.top"
                    onClick={(e) => {
                      e.preventDefault();
                      handleOpenUrl('mailto:wutianli@tianli0.top');
                    }}
                    className="text-foreground transition-colors hover:text-foreground/80"
                  >
                    wutianli@tianli0.top
                  </a>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <span className="text-muted-foreground">反馈群</span>
                  <a
                    href="https://qm.qq.com/q/2lEOycrLjq"
                    onClick={(e) => {
                      e.preventDefault();
                      handleOpenUrl('https://qm.qq.com/q/2lEOycrLjq');
                    }}
                    className="inline-flex items-center rounded-full border border-primary/40 bg-primary/10 px-2.5 py-1 font-medium text-primary transition-colors hover:bg-primary/15"
                  >
                    QQ 群入口
                  </a>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* ═══════════ 6. 调试面板 ═══════════ */}
        <Collapsible open={debugPanelOpen} onOpenChange={setDebugPanelOpen}>
          <div className="rounded-2xl border border-border bg-card overflow-hidden">
            <CollapsibleTrigger asChild>
              <button type="button" className="flex w-full items-center justify-between px-4 py-3 transition-colors hover:bg-muted/40">
                <div className="flex items-center gap-2">
                  <Bug className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm font-semibold text-foreground">调试面板</span>
                </div>
                <ChevronDown className={clsx('h-4 w-4 text-muted-foreground transition-transform duration-200', debugPanelOpen && 'rotate-180')} />
              </button>
            </CollapsibleTrigger>

            <CollapsibleContent>
              <div className="space-y-3 border-t border-border/60 p-4">
                <div className="flex items-center justify-between rounded-xl bg-muted/50 px-3 py-2.5">
                  <div className="flex items-center gap-2">
                    <Bug className="h-4 w-4 text-muted-foreground" />
                    <div>
                      <div className="text-sm font-medium">调试模式</div>
                      <div className="text-[11px] text-muted-foreground">启用详细日志</div>
                    </div>
                  </div>
                  <ToggleSwitch enabled={config.debugMode} onChange={toggleDebugMode} size="sm" color="purple" />
                </div>

                <div className="flex items-center justify-between rounded-xl bg-muted/50 px-3 py-2.5">
                  <div className="flex items-center gap-2">
                    {config.guiMonitoring ? <Eye className="h-4 w-4 text-muted-foreground" /> : <EyeOff className="h-4 w-4 text-muted-foreground" />}
                    <div>
                      <div className="text-sm font-medium">GUI 监控</div>
                      <div className="text-[11px] text-muted-foreground">监控 GUI 响应</div>
                    </div>
                  </div>
                  <ToggleSwitch enabled={config.guiMonitoring} onChange={toggleGuiMonitoring} size="sm" color="purple" />
                </div>

                <Button variant="secondary" size="sm" onClick={fetchDebugInfo} loading={debugInfoLoading} className="w-full">
                  刷新调试信息
                </Button>

                {debugInfo && (
                  <ScrollArea className="max-h-48 rounded-xl border border-border bg-background">
                    <pre className="p-3 text-xs text-foreground/90">{JSON.stringify(debugInfo, null, 2)}</pre>
                  </ScrollArea>
                )}
              </div>
            </CollapsibleContent>
          </div>
        </Collapsible>
      </div>

      {/* ═══════════ Custom speed warning dialog ═══════════ */}
      <AnimatePresence>
        {showCustomSpeedWarning && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4"
          >
            <motion.div
              initial={{ scale: 0.95, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0.95, opacity: 0 }}
              transition={{ duration: 0.2 }}
              className="w-full max-w-sm rounded-2xl border border-border bg-card p-6 shadow-xl"
            >
              <div className="mb-4 flex justify-center">
                <div className="flex h-14 w-14 items-center justify-center rounded-full bg-amber-500/15">
                  <TriangleAlert className="h-8 w-8 text-amber-600" />
                </div>
              </div>

              <h3 className="mb-3 text-center text-lg font-bold text-foreground">风险提示</h3>

              <div className="mb-4 rounded-xl border border-amber-300/40 bg-amber-500/10 p-3 text-sm">
                <p className="mb-2 font-medium text-foreground">启用后：</p>
                <ul className="space-y-1 text-xs text-muted-foreground">
                  <li>• 智能温控将被禁用</li>
                  <li>• 风扇以固定转速运行</li>
                  <li>• 可能导致散热不足</li>
                </ul>
              </div>

              <div className="mb-5 rounded-xl bg-muted/60 p-3 text-center">
                <span className="text-xs text-muted-foreground">设置转速</span>
                <div className="text-xl font-bold text-amber-600">{customSpeedInput} RPM</div>
              </div>

              <div className="flex gap-3">
                <Button variant="secondary" onClick={() => setShowCustomSpeedWarning(false)} className="flex-1">
                  取消
                </Button>
                <Button
                  variant="primary"
                  onClick={() => { setShowCustomSpeedWarning(false); handleCustomSpeedApply(true, customSpeedInput); }}
                  className="flex-1 bg-amber-600 text-white hover:bg-amber-700"
                  icon={<CheckCircle2 className="h-4 w-4" />}
                >
                  确认
                </Button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}
