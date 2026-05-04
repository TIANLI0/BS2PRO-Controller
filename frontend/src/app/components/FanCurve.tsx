'use client';

import React, { useState, useEffect, useCallback, memo, useMemo, useRef } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip, ResponsiveContainer } from 'recharts';
import { motion, AnimatePresence } from 'framer-motion';
import {
  RotateCw,
  Check,
  Info,
  Spline,
  TriangleAlert,
  Plus,
  Trash2,
  Clipboard,
  Download,
  Upload,
} from 'lucide-react';
import { Tooltip, TooltipContent, TooltipTrigger, TooltipProvider } from '@/components/ui/tooltip';
import { Input } from '@/components/ui/input';
import { apiService } from '../services/api';
import { types } from '../../../wailsjs/go/models';
import { MANUAL_GEAR_PRESETS, BS1_MANUAL_GEAR_PRESETS } from '../lib/manualGearPresets';
import FanCurveProfileSelect from './FanCurveProfileSelect';
import { toast } from 'sonner';
import { ToggleSwitch, Button, Badge, Slider } from './ui/index';
import clsx from 'clsx';

const LOW_RPM_WARNING_DATE_KEY = 'fanCurveLowRpmWarningDate';
const FAN_CURVE_MIN_TEMP = 30;
const FAN_CURVE_MAX_TEMP = 110;
const FAN_CURVE_TEMP_STEP = 5;
const DEFAULT_CURVE_LENGTH = ((FAN_CURVE_MAX_TEMP - FAN_CURVE_MIN_TEMP) / FAN_CURVE_TEMP_STEP) + 1;
type LearningProfile = 'quiet' | 'balanced' | 'performance';
type CurveProfile = { id: string; name: string; curve: types.FanCurvePoint[] };

function syncCurveRpmAtIndex(
  curve: types.FanCurvePoint[],
  index: number,
  targetRpm: number,
  minRpm: number,
  maxRpm: number,
) {
  const currentPoint = curve[index];
  if (!currentPoint) {
    return { curve, changed: false, hasLowRpmPoint: false };
  }

  const normalizedRpm = Math.max(minRpm, Math.min(maxRpm, Math.round(targetRpm / 50) * 50));
  const nextCurve = [...curve];
  let changed = false;

  if (currentPoint.rpm !== normalizedRpm) {
    nextCurve[index] = { ...currentPoint, rpm: normalizedRpm };
    changed = true;
  }

  for (let left = index - 1; left >= 0; left -= 1) {
    if (nextCurve[left].rpm <= nextCurve[left + 1].rpm) {
      break;
    }

    nextCurve[left] = {
      ...nextCurve[left],
      rpm: nextCurve[left + 1].rpm,
    };
    changed = true;
  }

  for (let right = index + 1; right < nextCurve.length; right += 1) {
    if (nextCurve[right].rpm >= nextCurve[right - 1].rpm) {
      break;
    }

    nextCurve[right] = {
      ...nextCurve[right],
      rpm: nextCurve[right - 1].rpm,
    };
    changed = true;
  }

  return {
    curve: nextCurve,
    changed,
    hasLowRpmPoint: nextCurve.some((point) => point.rpm < 1000),
  };
}

interface FanCurveProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  deviceModel: string | null;
}

/* ── Temperature indicator overlay (memo, doesn't re-render chart) ── */

const TemperatureIndicator = memo(function TemperatureIndicator({
  temperature,
  chartRef,
  temperatureRange,
}: {
  temperature: number | null;
  chartRef: React.RefObject<HTMLDivElement | null>;
  temperatureRange: { min: number; max: number };
}) {
  const [position, setPosition] = useState<{ x: number; top: number; height: number } | null>(null);

  useEffect(() => {
    if (temperature === null || !chartRef.current) { setPosition(null); return; }
    const updatePosition = () => {
      const chartArea = chartRef.current?.querySelector('.recharts-cartesian-grid');
      if (!chartArea) return;
      const rect = chartArea.getBoundingClientRect();
      const containerRect = chartRef.current!.querySelector('.recharts-responsive-container')?.getBoundingClientRect();
      if (!containerRect) return;
      const chartWidth = rect.width;
      const chartLeft = rect.left - containerRect.left;
      const tempPercent = (temperature - temperatureRange.min) / (temperatureRange.max - temperatureRange.min);
      const x = chartLeft + tempPercent * chartWidth;
      setPosition({ x, top: rect.top - containerRect.top, height: rect.height });
    };
    updatePosition();
    window.addEventListener('resize', updatePosition);
    return () => window.removeEventListener('resize', updatePosition);
  }, [temperature, chartRef, temperatureRange]);

  if (!position || temperature === null) return null;

  return (
    <svg className="absolute inset-0 pointer-events-none overflow-visible" style={{ width: '100%', height: '100%' }}>
      <line x1={position.x} y1={position.top} x2={position.x} y2={position.top + position.height} stroke="var(--chart-temperature-indicator)" strokeWidth={2} strokeDasharray="5 5" />
      <rect x={position.x - 45} y={position.top - 22} width={90} height={20} rx={4} fill="var(--chart-temperature-indicator)" />
      <text x={position.x} y={position.top - 8} textAnchor="middle" fill="white" fontSize={11} fontWeight={500}>当前 {temperature}°C</text>
    </svg>
  );
});

/* ── Tooltip label helper ── */

const ConfigTooltipLabel = memo(function ConfigTooltipLabel({ label, description }: { label: string; description: string }) {
  return (
    <span className="inline-flex items-center gap-1">
      <span>{label}</span>
      <Tooltip>
        <TooltipTrigger asChild>
          <button type="button" className="inline-flex cursor-pointer items-center justify-center rounded text-muted-foreground transition-colors hover:text-foreground" aria-label={`${label}说明`}>
            <Info className="h-3.5 w-3.5" />
          </button>
        </TooltipTrigger>
        <TooltipContent className="max-w-[260px] leading-relaxed">{description}</TooltipContent>
      </Tooltip>
    </span>
  );
});

/* ── Draggable chart point ── */

const DraggablePoint = memo(function DraggablePoint({
  cx, cy, index, rpm, onDragStart, isActive,
}: {
  cx: number; cy: number; index: number; temperature: number; rpm: number;
  onDragStart: (index: number) => void; isActive: boolean;
}) {
  const handleMouseDown = useCallback((e: React.MouseEvent) => { e.preventDefault(); e.stopPropagation(); onDragStart(index); }, [index, onDragStart]);
  const handleTouchStart = useCallback((e: React.TouchEvent) => { e.preventDefault(); e.stopPropagation(); onDragStart(index); }, [index, onDragStart]);

  return (
    <g>
      <circle cx={cx} cy={cy} r={isActive ? 14 : 10} fill="transparent" stroke="transparent" style={{ cursor: 'ns-resize' }} onMouseDown={handleMouseDown} onTouchStart={handleTouchStart} />
      <circle cx={cx} cy={cy} r={isActive ? 8 : 6} fill={isActive ? 'var(--chart-primary-active)' : 'var(--chart-primary)'} stroke="var(--card)" strokeWidth={2}
        style={{ cursor: 'ns-resize', transition: isActive ? 'none' : 'all 0.2s ease', filter: isActive ? 'drop-shadow(0 4px 8px var(--chart-primary-glow))' : 'drop-shadow(0 2px 4px var(--chart-point-shadow))' }}
        onMouseDown={handleMouseDown} onTouchStart={handleTouchStart}
      />
      {isActive && (
        <g>
          <rect x={cx - 35} y={cy - 35} width={70} height={24} rx={4} fill="var(--chart-primary-active)" opacity={0.95} />
          <text x={cx} y={cy - 19} textAnchor="middle" fill="white" fontSize={12} fontWeight={600}>{rpm} RPM</text>
        </g>
      )}
    </g>
  );
});

/* ═══════════════════════════════════════════════════════════
   ─── Main FanCurve Component ───
   ═══════════════════════════════════════════════════════════ */

const FanCurve = memo(function FanCurve({ config, onConfigChange, isConnected, temperature, deviceModel }: FanCurveProps) {
  const [localCurve, setLocalCurve] = useState<types.FanCurvePoint[]>([]);
  const [curveProfiles, setCurveProfiles] = useState<CurveProfile[]>([]);
  const [activeProfileId, setActiveProfileId] = useState('');
  const [profileNameInput, setProfileNameInput] = useState('');
  const [isProfileNameComposing, setIsProfileNameComposing] = useState(false);
  const [profileOpLoading, setProfileOpLoading] = useState(false);
  const [exportCode, setExportCode] = useState('');
  const [importCode, setImportCode] = useState('');
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);
  const [isInitialized, setIsInitialized] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [tempTrendDelta, setTempTrendDelta] = useState(0);
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [isInteracting, setIsInteracting] = useState(false);
  const [showLowRpmWarning, setShowLowRpmWarning] = useState(false);
  const chartRef = useRef<HTMLDivElement>(null);
  const previousMaxTempRef = useRef<number | null>(null);
  const lowRpmWarnedInDragRef = useRef(false);
  const chartBoundsRef = useRef<{ top: number; bottom: number; left: number; right: number; yMin: number; yMax: number } | null>(null);
  const [rpmRange, setRpmRange] = useState({ min: 0, max: 4000, ticks: [0, 500, 1000, 1500, 2000, 2500, 3000, 3500, 4000] });

  const activeProfile = useMemo(() => curveProfiles.find((p) => p.id === activeProfileId) ?? null, [curveProfiles, activeProfileId]);
  const externalActiveProfileId = ((config as any).activeFanCurveProfileId || '') as string;

  const shouldShowLowRpmWarningToday = useCallback(() => {
    if (typeof window === 'undefined') return false;
    const today = new Date().toISOString().slice(0, 10);
    const lastShownDate = window.localStorage.getItem(LOW_RPM_WARNING_DATE_KEY);
    if (lastShownDate === today) return false;
    window.localStorage.setItem(LOW_RPM_WARNING_DATE_KEY, today);
    return true;
  }, []);

  const temperatureRange = useMemo(() => ({
    min: FAN_CURVE_MIN_TEMP,
    max: FAN_CURVE_MAX_TEMP,
    ticks: Array.from({ length: DEFAULT_CURVE_LENGTH }, (_, i) => FAN_CURVE_MIN_TEMP + i * FAN_CURVE_TEMP_STEP),
  }), []);

  const syncConfigFromBackend = useCallback(async () => {
    try {
      const latest = await apiService.getConfig();
      onConfigChange(types.AppConfig.createFrom(latest));
    } catch {
      /* noop */
    }
  }, [onConfigChange]);

  const loadCurveProfiles = useCallback(async () => {
    try {
      const payload = await apiService.getFanCurveProfiles();
      const profiles = Array.isArray(payload?.profiles) ? payload.profiles : [];
      const activeId = payload?.activeId || profiles[0]?.id || '';
      setCurveProfiles(profiles);
      setActiveProfileId(activeId);
      const current = profiles.find((p) => p.id === activeId) ?? profiles[0];
      if (current) {
        setProfileNameInput(current.name || '');
        setLocalCurve([...(current.curve || [])]);
        setHasUnsavedChanges(false);
      }
    } catch {
      /* noop */
    }
  }, []);

  const curveRpmBounds = useMemo(() => {
    const source = localCurve.length > 0 ? localCurve : (config.fanCurve ?? []);
    if (source.length === 0) {
      return { min: rpmRange.min, max: rpmRange.max };
    }
    let minCurveRPM = source[0].rpm;
    let maxCurveRPM = source[0].rpm;
    for (let i = 1; i < source.length; i++) {
      const rpm = source[i].rpm;
      if (rpm < minCurveRPM) minCurveRPM = rpm;
      if (rpm > maxCurveRPM) maxCurveRPM = rpm;
    }
    return { min: minCurveRPM, max: maxCurveRPM };
  }, [config.fanCurve, localCurve, rpmRange.max, rpmRange.min]);

  /* ── Smart control state ── */

  const smartControl = useMemo(() => {
    const curveLength = config.fanCurve?.length || localCurve.length || DEFAULT_CURVE_LENGTH;
    const defaultOffsets = Array.from({ length: curveLength }, () => 0);
    const defaultRateOffsets = Array.from({ length: 7 }, () => 0);
    const existing = config.smartControl;
    const normalizeOffsets = (source?: number[]) => Array.isArray(source) ? [...source.slice(0, curveLength), ...defaultOffsets].slice(0, curveLength) : defaultOffsets;
    const normalizeRateOffsets = (source?: number[]) => Array.isArray(source) ? [...source.slice(0, 7), ...defaultRateOffsets].slice(0, 7) : defaultRateOffsets;

    if (!existing) {
      return { enabled: true, learning: config.debugMode, filterTransientSpike: true, targetTemp: 68, aggressiveness: 5, hysteresis: 2, minRpmChange: 50, rampUpLimit: 220, rampDownLimit: 160, learnRate: 4, learnWindow: 6, learnDelay: 2, overheatWeight: 8, rpmDeltaWeight: 5, noiseWeight: 4, trendGain: 5, maxLearnOffset: 600, learnedOffsets: defaultOffsets, learnedOffsetsHeat: defaultOffsets, learnedOffsetsCool: defaultOffsets, learnedRateHeat: defaultRateOffsets, learnedRateCool: defaultRateOffsets };
    }

    return {
      ...existing,
      learning: config.debugMode,
      filterTransientSpike: existing.filterTransientSpike ?? true,
      hysteresis: Math.max(1, existing.hysteresis ?? 2),
      learnWindow: existing.learnWindow ?? 6, learnDelay: existing.learnDelay ?? 2,
      overheatWeight: existing.overheatWeight ?? 8, rpmDeltaWeight: existing.rpmDeltaWeight ?? 5,
      noiseWeight: existing.noiseWeight ?? 4, trendGain: existing.trendGain ?? 5,
      learnedOffsets: normalizeOffsets(existing.learnedOffsets),
      learnedOffsetsHeat: normalizeOffsets(existing.learnedOffsetsHeat),
      learnedOffsetsCool: normalizeOffsets(existing.learnedOffsetsCool),
      learnedRateHeat: normalizeRateOffsets(existing.learnedRateHeat),
      learnedRateCool: normalizeRateOffsets(existing.learnedRateCool),
    };
  }, [config.debugMode, config.fanCurve, config.smartControl, localCurve.length]);

  const effectiveTrendDelta = useMemo(() => tempTrendDelta, [tempTrendDelta]);

  const learningProfile = useMemo<LearningProfile>(() => {
    const score = smartControl.aggressiveness * 2 + smartControl.trendGain - smartControl.noiseWeight;
    if (score >= 16) return 'performance';
    if (score <= 8) return 'quiet';
    return 'balanced';
  }, [smartControl.aggressiveness, smartControl.noiseWeight, smartControl.trendGain]);

  useEffect(() => {
    const maxTemp = temperature?.maxTemp;
    if (maxTemp === null || maxTemp === undefined || maxTemp <= 0) return;
    const previous = previousMaxTempRef.current;
    if (previous !== null) setTempTrendDelta(maxTemp - previous);
    previousMaxTempRef.current = maxTemp;
  }, [temperature?.maxTemp]);

  /* ── Learning insight ── */

  const learningInsight = useMemo(() => {
    const offsets = smartControl.learnedOffsets || [];

    const points = (config.fanCurve && config.fanCurve.length > 0 ? config.fanCurve : localCurve)
      .map((point, index) => ({ index, temperature: point.temperature, offset: offsets[index] ?? 0 }));

    if (points.length === 0) {
      return { currentOffset: 0, currentTempLabel: '--', maxAbsOffset: 0, avgAbsOffset: 0, significantPoints: [] as Array<{ temperature: number; offset: number }> };
    }

    const maxTemp = temperature?.maxTemp ?? null;
    let currentPoint = points[0];
    if (maxTemp !== null) {
      currentPoint = points.reduce((best, item) => Math.abs(item.temperature - maxTemp) < Math.abs(best.temperature - maxTemp) ? item : best, points[0]);
    }

    const absOffsets = points.map((p) => Math.abs(p.offset));
    const totalAbsOffset = absOffsets.reduce((s, v) => s + v, 0);

    const significantPoints = points.filter((p) => Math.abs(p.offset) >= 20).sort((a, b) => Math.abs(b.offset) - Math.abs(a.offset)).slice(0, 6).map((p) => ({ temperature: p.temperature, offset: p.offset }));

    return {
      currentOffset: currentPoint.offset,
      currentTempLabel: `${currentPoint.temperature}°C`,
      maxAbsOffset: absOffsets.reduce((m, v) => Math.max(m, v), 0),
      avgAbsOffset: Math.round(totalAbsOffset / points.length),
      significantPoints,
    };
  }, [config.fanCurve, localCurve, smartControl.learnedOffsets, temperature?.maxTemp]);

  /* ── Init ── */

  useEffect(() => {
    if (!isInitialized && config.fanCurve && config.fanCurve.length > 0) {
      setLocalCurve([...config.fanCurve]);
      setIsInitialized(true);
    }
  }, [config.fanCurve, isInitialized]);

  useEffect(() => {
    loadCurveProfiles().catch(() => {});
  }, [loadCurveProfiles]);

  useEffect(() => {
    if (externalActiveProfileId && externalActiveProfileId !== activeProfileId) {
      loadCurveProfiles().catch(() => {});
    }
  }, [activeProfileId, externalActiveProfileId, loadCurveProfiles]);

  /* ── Chart data ── */

  const chartData = useMemo(() => {
    const blendedOffsets = smartControl.learnedOffsets || [];
    const heatOffsets = smartControl.learnedOffsetsHeat || [];
    const coolOffsets = smartControl.learnedOffsetsCool || [];
    const rateHeat = smartControl.learnedRateHeat || [];
    const rateCool = smartControl.learnedRateCool || [];
    const rateBucketIndex = Math.max(0, Math.min(6, effectiveTrendDelta + 3));
    const trendOffsets = effectiveTrendDelta > 0 ? heatOffsets : effectiveTrendDelta < 0 ? coolOffsets : blendedOffsets;
    const trendRateBias = effectiveTrendDelta >= 0 ? (rateHeat[rateBucketIndex] ?? 0) : (rateCool[rateBucketIndex] ?? 0);

    return localCurve.map((point, index) => {
      const baseOffset = trendOffsets[index] ?? blendedOffsets[index] ?? 0;
      const offset = config.debugMode ? baseOffset + trendRateBias : 0;
      return {
        temperature: point.temperature,
        rpm: point.rpm,
        coupledRpm: Math.max(curveRpmBounds.min, Math.min(curveRpmBounds.max, point.rpm + offset)),
        index,
      };
    });
  }, [config.debugMode, curveRpmBounds.max, curveRpmBounds.min, localCurve, smartControl.learnedOffsets, smartControl.learnedOffsetsHeat, smartControl.learnedOffsetsCool, smartControl.learnedRateHeat, smartControl.learnedRateCool, effectiveTrendDelta]);

  const showCoupledCurve = config.autoControl && config.debugMode && smartControl.enabled;

  /* ── Point update + drag ── */

  const updatePoint = useCallback((index: number, newRpm: number) => {
    let didChange = false;

    setLocalCurve((prev) => {
      const nextState = syncCurveRpmAtIndex(prev, index, newRpm, rpmRange.min, rpmRange.max);

      if (nextState.hasLowRpmPoint && !lowRpmWarnedInDragRef.current) {
        lowRpmWarnedInDragRef.current = true;
        if (shouldShowLowRpmWarningToday()) {
          setShowLowRpmWarning(true);
        }
      }

      if (!nextState.changed) {
        return prev;
      }

      didChange = true;
      return nextState.curve;
    });

    if (didChange) {
      setHasUnsavedChanges(true);
    }
  }, [rpmRange, shouldShowLowRpmWarningToday]);

  const handleDragStart = useCallback((index: number) => {
    setDragIndex(index);
    setIsInteracting(true);
    lowRpmWarnedInDragRef.current = false;
    if (chartRef.current) {
      const chartArea = chartRef.current.querySelector('.recharts-cartesian-grid');
      if (chartArea) {
        const rect = chartArea.getBoundingClientRect();
        chartBoundsRef.current = { top: rect.top, bottom: rect.bottom, left: rect.left, right: rect.right, yMin: rpmRange.min, yMax: rpmRange.max };
      }
    }
  }, [rpmRange]);

  const handleDrag = useCallback((clientY: number) => {
    if (dragIndex === null || !chartBoundsRef.current) return;
    const bounds = chartBoundsRef.current;
    const relativeY = Math.max(0, Math.min(1, (bounds.bottom - clientY) / (bounds.bottom - bounds.top)));
    updatePoint(dragIndex, bounds.yMin + relativeY * (bounds.yMax - bounds.yMin));
  }, [dragIndex, updatePoint]);

  const handleDragEnd = useCallback(() => { setDragIndex(null); setTimeout(() => setIsInteracting(false), 100); }, []);

  useEffect(() => {
    if (dragIndex === null) return;
    const mm = (e: MouseEvent) => { e.preventDefault(); handleDrag(e.clientY); };
    const tm = (e: TouchEvent) => { if (e.touches.length > 0) handleDrag(e.touches[0].clientY); };
    const end = () => handleDragEnd();
    document.addEventListener('mousemove', mm);
    document.addEventListener('mouseup', end);
    document.addEventListener('touchmove', tm, { passive: false });
    document.addEventListener('touchend', end);
    return () => { document.removeEventListener('mousemove', mm); document.removeEventListener('mouseup', end); document.removeEventListener('touchmove', tm); document.removeEventListener('touchend', end); };
  }, [dragIndex, handleDrag, handleDragEnd]);

  /* ── Save / Reset ── */

  const persistCurrentCurve = useCallback(async () => {
    if (isSaving) return;
    try {
      setIsSaving(true);
      const profileID = activeProfileId || (((config as any).activeFanCurveProfileId || '') as string);
      const profileName = activeProfile?.name || '当前曲线';
      await apiService.saveFanCurveProfile(profileID, profileName, localCurve, true);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      setHasUnsavedChanges(false);
      return true;
    } catch (e) {
      toast.error(`保存曲线失败: ${e}`);
      return false;
    } finally {
      setIsSaving(false);
    }
  }, [activeProfile?.name, activeProfileId, config, isSaving, loadCurveProfiles, localCurve, syncConfigFromBackend]);

  const saveCurve = useCallback(async () => {
    await persistCurrentCurve();
  }, [persistCurrentCurve]);

  const getSafeProfileName = useCallback((input: string, fallback: string) => {
    const name = (input || '').trim() || fallback;
    const runes = Array.from(name);
    return runes.slice(0, 6).join('');
  }, []);

  const trimProfileNameToLimit = useCallback((value: string) => {
    return Array.from(value).slice(0, 6).join('');
  }, []);

  const handleProfileNameInputChange = useCallback((value: string, composing: boolean) => {
    if (composing || isProfileNameComposing) {
      setProfileNameInput(value);
      return;
    }
    setProfileNameInput(trimProfileNameToLimit(value));
  }, [isProfileNameComposing, trimProfileNameToLimit]);

  const handleProfileNameCompositionStart = useCallback(() => {
    setIsProfileNameComposing(true);
  }, []);

  const handleProfileNameCompositionEnd = useCallback((value: string) => {
    setIsProfileNameComposing(false);
    setProfileNameInput(trimProfileNameToLimit(value));
  }, [trimProfileNameToLimit]);

  const switchProfile = useCallback(async (id: string) => {
    try {
      setProfileOpLoading(true);
      await apiService.setActiveFanCurveProfile(id);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      toast.success('温控曲线已切换');
    } catch (e) {
      toast.error(`切换失败: ${e}`);
    } finally {
      setProfileOpLoading(false);
    }
  }, [loadCurveProfiles, syncConfigFromBackend]);

  const saveCurrentProfileName = useCallback(async () => {
    const fallbackName = activeProfile?.name || '当前曲线';
    const safeName = getSafeProfileName(profileNameInput, fallbackName);
    try {
      setProfileOpLoading(true);
      const profileCurve = activeProfile?.curve || localCurve;
      await apiService.saveFanCurveProfile(activeProfileId, safeName, profileCurve, false);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      toast.success('方案命名已更新');
    } catch (e) {
      toast.error(`更新命名失败: ${e}`);
    } finally {
      setProfileOpLoading(false);
    }
  }, [activeProfile?.curve, activeProfile?.name, activeProfileId, getSafeProfileName, loadCurveProfiles, localCurve, profileNameInput, syncConfigFromBackend]);

  const createNewProfile = useCallback(async () => {
    const rawName = (profileNameInput || '').trim();
    const activeName = (activeProfile?.name || '').trim();
    const shouldUseDefaultNewName = !rawName || rawName === activeName;
    const safeName = shouldUseDefaultNewName ? '新曲线' : getSafeProfileName(rawName, '新曲线');
    try {
      setProfileOpLoading(true);
      await apiService.saveFanCurveProfile('', safeName, localCurve, true);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      setProfileNameInput('');
      toast.success('已另存为新方案');
    } catch (e) {
      toast.error(`另存失败: ${e}`);
    } finally {
      setProfileOpLoading(false);
    }
  }, [activeProfile?.name, getSafeProfileName, loadCurveProfiles, localCurve, profileNameInput, syncConfigFromBackend]);

  const removeActiveProfile = useCallback(async () => {
    if (!activeProfileId) return;
    try {
      setProfileOpLoading(true);
      await apiService.deleteFanCurveProfile(activeProfileId);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      toast.success('已删除曲线方案');
    } catch (e) {
      toast.error(`删除失败: ${e}`);
    } finally {
      setProfileOpLoading(false);
    }
  }, [activeProfileId, loadCurveProfiles, syncConfigFromBackend]);

  const exportProfiles = useCallback(async () => {
    try {
      if (hasUnsavedChanges) {
        const ok = await persistCurrentCurve();
        if (!ok) {
          return;
        }
        await loadCurveProfiles();
      }
      const code = await apiService.exportFanCurveProfiles();
      setExportCode(code);
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(code);
        toast.success('导出字符串已复制');
      } else {
        toast.success('导出字符串已生成');
      }
    } catch (e) {
      toast.error(`导出失败: ${e}`);
    }
  }, [hasUnsavedChanges, loadCurveProfiles, persistCurrentCurve]);

  const importProfiles = useCallback(async () => {
    const code = importCode.trim();
    if (!code) {
      toast.error('请先粘贴导入字符串');
      return;
    }
    try {
      setProfileOpLoading(true);
      await apiService.importFanCurveProfiles(code);
      await loadCurveProfiles();
      await syncConfigFromBackend();
      setImportCode('');
      toast.success('曲线方案导入成功');
    } catch (e) {
      toast.error(`导入失败: ${e}`);
    } finally {
      setProfileOpLoading(false);
    }
  }, [importCode, loadCurveProfiles, syncConfigFromBackend]);

  const resetCurve = useCallback(() => {
    const d: types.FanCurvePoint[] = [
      { temperature: 30, rpm: 1000 }, { temperature: 35, rpm: 1200 }, { temperature: 40, rpm: 1400 }, { temperature: 45, rpm: 1600 },
      { temperature: 50, rpm: 1800 }, { temperature: 55, rpm: 2000 }, { temperature: 60, rpm: Math.min(2300, rpmRange.max) },
      { temperature: 65, rpm: Math.min(2600, rpmRange.max) }, { temperature: 70, rpm: Math.min(2900, rpmRange.max) },
      { temperature: 75, rpm: Math.min(3200, rpmRange.max) }, { temperature: 80, rpm: Math.min(3500, rpmRange.max) },
      { temperature: 85, rpm: Math.min(3800, rpmRange.max) }, { temperature: 90, rpm: rpmRange.max }, { temperature: 95, rpm: rpmRange.max },
      { temperature: 100, rpm: rpmRange.max }, { temperature: 105, rpm: rpmRange.max }, { temperature: 110, rpm: rpmRange.max },
    ];
    setLocalCurve(d);
    setHasUnsavedChanges(true);
  }, [rpmRange.max]);

  /* ── Auto control / smart control handlers ── */

  const handleAutoControlChange = useCallback(async (enabled: boolean) => {
    try { await apiService.setAutoControl(enabled); onConfigChange(types.AppConfig.createFrom({ ...config, autoControl: enabled })); } catch { /* noop */ }
  }, [config, onConfigChange]);

  const updateSmartControl = useCallback(async (patch: Partial<typeof smartControl>) => {
    try {
      const merged = { ...smartControl, ...patch, learning: config.debugMode };
      onConfigChange(types.AppConfig.createFrom({ ...config, smartControl: merged }));
    } catch { /* noop */ }
  }, [config, onConfigChange, smartControl]);

  const applyLearningProfile = useCallback((profile: LearningProfile) => {
    if (profile === 'quiet') {
      updateSmartControl({
        aggressiveness: 3,
        trendGain: 3,
        rampUpLimit: 160,
        rampDownLimit: 120,
        minRpmChange: 70,
        noiseWeight: 7,
        rpmDeltaWeight: 7,
        overheatWeight: 7,
      });
      return;
    }

    if (profile === 'performance') {
      updateSmartControl({
        aggressiveness: 8,
        trendGain: 8,
        rampUpLimit: 320,
        rampDownLimit: 220,
        minRpmChange: 40,
        noiseWeight: 2,
        rpmDeltaWeight: 3,
        overheatWeight: 10,
      });
      return;
    }

    updateSmartControl({
      aggressiveness: 5,
      trendGain: 5,
      rampUpLimit: 220,
      rampDownLimit: 160,
      minRpmChange: 50,
      noiseWeight: 4,
      rpmDeltaWeight: 5,
      overheatWeight: 8,
    });
  }, [updateSmartControl]);

  const resetLearning = useCallback(() => {
    const len = localCurve.length || config.fanCurve.length || 14;
    const z = Array.from({ length: len }, () => 0);
    const r = Array.from({ length: 7 }, () => 0);
    updateSmartControl({ learnedOffsets: z, learnedOffsetsHeat: z, learnedOffsetsCool: z, learnedRateHeat: r, learnedRateCool: r });
  }, [config.fanCurve.length, localCurve.length, updateSmartControl]);

  /* ── Manual gear ── */

  const isBs1 = deviceModel === 'BS1';
  const manualGearPresets = isBs1 ? BS1_MANUAL_GEAR_PRESETS : MANUAL_GEAR_PRESETS;

  const manualPoints = useMemo(() => {
    return manualGearPresets.flatMap((preset, gearIndex) => preset.levels.map((item, levelIndex) => ({
      key: `${preset.gear}-${item.level}`,
      gear: preset.gear,
      level: item.level,
      rpm: item.rpm,
      gearIndex,
      levelIndex,
      colorClass: preset.colorClass,
      borderClass: preset.borderClass,
      bgClass: preset.bgClass,
    })));
  }, [manualGearPresets]);

  const selectedManualPointIndex = useMemo(() => {
    const selected = manualPoints.findIndex((p) => p.gear === (config.manualGear || '标准') && p.level === (config.manualLevel || '中'));
    return selected >= 0 ? selected : 4;
  }, [config.manualGear, config.manualLevel, manualPoints]);

  const rememberedManualGearLevels = useMemo(() => {
    return ((config as any).manualGearLevels ?? {}) as Record<string, string>;
  }, [config]);

  const applyManualGearPreset = useCallback(async (gear: string, level: string) => {
    try {
      await apiService.setManualGear(gear, level);
      onConfigChange(types.AppConfig.createFrom({
        ...config,
        manualGear: gear,
        manualLevel: level,
        manualGearLevels: {
          ...rememberedManualGearLevels,
          [gear]: level,
        },
      }));
    } catch { /* noop */ }
  }, [config, onConfigChange, rememberedManualGearLevels]);

  const handleManualPointSelect = useCallback(async (index: number) => {
    const selected = manualPoints[index];
    if (!selected) return;
    await applyManualGearPreset(selected.gear, selected.level);
  }, [applyManualGearPreset, manualPoints]);

  const handleGearCardSelect = useCallback(async (gear: string) => {
    const rememberedLevel = rememberedManualGearLevels[gear];
    const nextLevel = rememberedLevel === '低' || rememberedLevel === '中' || rememberedLevel === '高'
      ? rememberedLevel
      : (config.manualLevel || '中');
    await applyManualGearPreset(gear, nextLevel);
  }, [applyManualGearPreset, config, rememberedManualGearLevels]);

  /* ── Custom dot renderer ── */

  const CustomDot = useCallback((props: any): React.ReactElement<SVGElement> => {
    const { cx, cy, index, payload } = props;
    if (cx === undefined || cy === undefined) return <g />;
    return <DraggablePoint key={`dot-${index}`} cx={cx} cy={cy} index={index} temperature={payload.temperature} rpm={payload.rpm} onDragStart={handleDragStart} isActive={dragIndex === index} />;
  }, [dragIndex, handleDragStart]);

  /* ═══════════════════ RENDER ═══════════════════ */

  return (
    <TooltipProvider>
      <div className="relative space-y-4 overflow-hidden">
        {/* ── Header ── */}
        <motion.div
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.2 }}
          className="relative px-1 py-1"
        >
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <Spline className="h-4 w-4 text-primary" />
              <h2 className="text-base font-semibold text-foreground">风扇曲线</h2>
              {hasUnsavedChanges && <Badge variant="warning">未保存</Badge>}
              {isInteracting && <Badge variant="info">编辑中</Badge>}
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <FanCurveProfileSelect
                profiles={curveProfiles}
                activeProfileId={activeProfileId}
                onChange={switchProfile}
                loading={profileOpLoading}
              />
              <ToggleSwitch enabled={config.autoControl} onChange={handleAutoControlChange} label="智能变频" size="sm" color="blue" />
            </div>
          </div>
        </motion.div>

        {/* ── Manual gear (when auto off) ── */}
        <AnimatePresence>
          {!config.autoControl && isConnected && (
            <motion.div initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: 'auto' }} exit={{ opacity: 0, height: 0 }} className="overflow-hidden">
              <div className="rounded-2xl border border-border/70 bg-card p-4 space-y-4">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">手动挡位</span>
                  <span className="text-xs text-muted-foreground">{isBs1 ? '4 控制点滑块' : '12 控制点滑块'}</span>
                </div>

                <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                  {manualGearPresets.map((preset) => {
                    const isActiveGear = (config.manualGear || '标准') === preset.gear;
                    const rememberedLevel = isActiveGear
                      ? (config.manualLevel || '中')
                      : rememberedManualGearLevels[preset.gear];
                    const activeLevel = preset.levels.find((l) => l.level === rememberedLevel) ?? preset.levels[0];
                    return (
                      <button
                        key={preset.gear}
                        type="button"
                        onClick={() => isBs1 ? applyManualGearPreset(preset.gear, '中') : handleGearCardSelect(preset.gear)}
                        className={clsx(
                          'cursor-pointer rounded-xl border px-3 py-2.5 text-left transition-colors',
                          isActiveGear ? `${preset.borderClass} ${preset.bgClass}` : 'border-border/70 bg-background/40 hover:bg-muted/35',
                        )}
                      >
                        <div className={clsx('text-lg font-bold', isActiveGear ? preset.colorClass : 'text-foreground')}>{preset.gear}</div>
                        {!isBs1 && <div className={clsx('mt-1 text-base font-semibold', preset.colorClass)}>{activeLevel.rpm}RPM</div>}
                      </button>
                    );
                  })}
                </div>

                <div className="rounded-xl border border-border/70 bg-background/40 p-3">
                  <div className="relative mb-3 px-2">
                    <div className="absolute left-2 right-2 top-1/2 h-1 -translate-y-1/2 rounded-full bg-muted" />
                    <div className="relative flex items-center justify-between">
                      {manualPoints.map((point, index) => {
                        const isActivePoint = selectedManualPointIndex === index;
                        const isPassed = index < selectedManualPointIndex;
                        return (
                          <button
                            key={point.key}
                            type="button"
                            onClick={() => handleManualPointSelect(index)}
                            className="flex h-6 w-6 shrink-0 cursor-pointer items-center justify-center"
                            title={`${point.gear} ${point.level} · ${point.rpm} RPM`}
                          >
                            <span
                              className={clsx(
                                'block h-4 w-4 rounded-full border border-border/80 bg-card transition-transform duration-150',
                                isActivePoint ? `scale-125 ${point.borderClass} ${point.bgClass}` : '',
                                isPassed && !isActivePoint ? point.bgClass : '',
                              )}
                            />
                          </button>
                        );
                      })}
                    </div>
                  </div>

                  <div className="flex items-start justify-between px-2 text-[11px]">
                    {manualPoints.map((point) => (
                      <span key={`${point.key}-label`} className={clsx('w-6 text-center truncate', point.colorClass)}>
                        {point.levelIndex + 1}档
                      </span>
                    ))}
                  </div>
                </div>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {/* ── Chart ── */}
        <div
          ref={chartRef}
          className={clsx('relative rounded-3xl border bg-card p-4 shadow-sm', dragIndex !== null ? 'ring-2 ring-primary/40 border-primary/30' : 'border-border/70')}
        >
          <div className="h-80 md:h-96 relative">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={chartData} margin={{ top: 20, right: 30, left: 20, bottom: 20 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" />
                <XAxis dataKey="temperature" type="number" domain={[temperatureRange.min, temperatureRange.max]} ticks={temperatureRange.ticks} tickLine={false} axisLine={{ stroke: 'var(--chart-axis)' }} tick={{ fill: 'var(--chart-tick)', fontSize: 11 }} allowDataOverflow label={{ value: '温度 (°C)', position: 'insideBottom', offset: -10, fill: 'var(--chart-tick)', fontSize: 12 }} />
                <YAxis type="number" domain={[rpmRange.min, rpmRange.max]} ticks={rpmRange.ticks} tickLine={false} axisLine={{ stroke: 'var(--chart-axis)' }} tick={{ fill: 'var(--chart-tick)', fontSize: 11 }} allowDataOverflow label={{ value: '转速 (RPM)', angle: -90, position: 'insideLeft', fill: 'var(--chart-tick)', fontSize: 12 }} />
                <RechartsTooltip
                  formatter={(value: number, name: string) => name === 'coupledRpm' ? [`${value} RPM`, '学习曲线'] : [`${value} RPM`, '基础曲线']}
                  labelFormatter={(v) => `温度: ${v}°C`}
                  contentStyle={{ backgroundColor: 'var(--chart-tooltip-bg)', border: '1px solid', borderColor: 'var(--chart-tooltip-border)', borderRadius: '8px', boxShadow: 'var(--chart-tooltip-shadow)', padding: '8px 12px', color: 'var(--chart-tooltip-text)' }}
                  labelStyle={{ color: 'var(--chart-tooltip-text)', fontWeight: 600 }}
                  itemStyle={{ color: 'var(--chart-tooltip-text)' }}
                />
                <Line type="monotone" dataKey="rpm" stroke="var(--chart-primary)" strokeWidth={3} dot={CustomDot} activeDot={false} isAnimationActive={false} />
                {showCoupledCurve && <Line type="monotone" dataKey="coupledRpm" stroke="var(--chart-primary)" strokeWidth={2} strokeDasharray="6 4" dot={false} activeDot={false} isAnimationActive={false} />}
              </LineChart>
            </ResponsiveContainer>
            <TemperatureIndicator temperature={temperature?.maxTemp ?? null} chartRef={chartRef} temperatureRange={temperatureRange} />
          </div>
        </div>

        {/* ── Tips + Actions ── */}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap gap-2">
            <span className="rounded-full border border-border/70 bg-background/60 px-3 py-1 text-[11px] text-muted-foreground backdrop-blur-lg">拖拽蓝色圆点调整转速</span>
            {showCoupledCurve && <span className="rounded-full border border-border/70 bg-background/60 px-3 py-1 text-[11px] text-muted-foreground backdrop-blur-lg">实线: 基础 · 虚线: 学习</span>}
          </div>
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={resetCurve} icon={<RotateCw className="h-3.5 w-3.5" />}>重置</Button>
            <Button variant="primary" size="sm" onClick={saveCurve} disabled={!hasUnsavedChanges} loading={isSaving} icon={<Check className="h-3.5 w-3.5" />}>保存</Button>
          </div>
        </div>

        <section className="rounded-2xl border border-border/70 bg-card p-4 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium">曲线方案</span>
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            {curveProfiles.map((profile) => {
              const isActive = profile.id === activeProfileId;
              return (
                <Button
                  key={profile.id}
                  variant={isActive ? 'primary' : 'outline'}
                  size="sm"
                  onClick={() => switchProfile(profile.id)}
                  disabled={profileOpLoading}
                >
                  {profile.name}
                </Button>
              );
            })}
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <div className="min-w-[220px] flex-1">
              <Input
                value={profileNameInput}
                onChange={(e) => handleProfileNameInputChange(e.target.value, Boolean((e.nativeEvent as InputEvent).isComposing))}
                onCompositionStart={handleProfileNameCompositionStart}
                onCompositionEnd={(e) => handleProfileNameCompositionEnd(e.currentTarget.value)}
                placeholder="当前方案命名（最多6字）"
                className="h-10"
              />
            </div>
            <Button variant="secondary" size="sm" onClick={saveCurrentProfileName} loading={profileOpLoading} icon={<Check className="h-3.5 w-3.5" />}>保存命名</Button>
            <Button variant="secondary" size="sm" onClick={createNewProfile} loading={profileOpLoading} icon={<Plus className="h-3.5 w-3.5" />}>另存为新方案</Button>
            <Button variant="danger" size="sm" onClick={removeActiveProfile} loading={profileOpLoading} icon={<Trash2 className="h-3.5 w-3.5" />} disabled={curveProfiles.length <= 1}>删除当前方案</Button>
          </div>
        </section>

        {/* ── Smart learning (when auto on) ── */}
        <AnimatePresence>
          {config.autoControl && isConnected && (
            <motion.div initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: 'auto' }} exit={{ opacity: 0, height: 0 }} className="overflow-hidden">
              <div className="rounded-2xl border border-border/70 bg-card p-4 space-y-4">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <span className="text-sm font-medium">智能学习控温</span>
                  <ToggleSwitch
                    enabled={config.debugMode && smartControl.enabled}
                    onChange={(e) => {
                      if (e && !config.debugMode) {
                        toast.error('学习模式暂不稳定，请先开启调试模式后再启用。');
                        return;
                      }
                      updateSmartControl({ enabled: config.debugMode && e });
                    }}
                    label="启用"
                    size="sm"
                    color="blue"
                  />
                </div>

                <AnimatePresence initial={false}>
                  {smartControl.enabled && config.debugMode && (
                    <motion.div
                      initial={{ opacity: 0, height: 0 }}
                      animate={{ opacity: 1, height: 'auto' }}
                      exit={{ opacity: 0, height: 0 }}
                      className="overflow-hidden space-y-4"
                    >

                      {/* Core sliders */}
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <div className="space-y-1.5">
                          <div className="flex items-center justify-between text-xs text-muted-foreground">
                            <ConfigTooltipLabel label="响应风格" description="静音更稳重，性能更激进。" />
                          </div>
                          <div className="grid grid-cols-3 gap-1 rounded-xl border border-border/70 bg-background/40 p-1">
                            {([
                              { key: 'quiet', label: '静音' },
                              { key: 'balanced', label: '平衡' },
                              { key: 'performance', label: '性能' },
                            ] as const).map((option) => (
                              <button
                                key={option.key}
                                type="button"
                                onClick={() => applyLearningProfile(option.key)}
                                className={clsx(
                                  'cursor-pointer rounded-lg px-2 py-1.5 text-xs font-medium transition-colors',
                                  learningProfile === option.key
                                    ? 'bg-primary text-primary-foreground shadow-sm'
                                    : 'text-muted-foreground hover:bg-muted/40 hover:text-foreground'
                                )}
                              >
                                {option.label}
                              </button>
                            ))}
                          </div>
                        </div>
                        <div className="space-y-1.5">
                          <div className="flex items-center justify-between text-xs text-muted-foreground">
                            <ConfigTooltipLabel label="目标温度" description="智能控制会尽量把温度稳定在这个值附近。" />
                            <span>{smartControl.targetTemp}°C</span>
                          </div>
                          <Slider value={smartControl.targetTemp} onChange={(v) => updateSmartControl({ targetTemp: v })} min={55} max={85} step={1} showValue={false} />
                        </div>
                        <div className="space-y-1.5">
                          <div className="flex items-center justify-between text-xs text-muted-foreground">
                            <ConfigTooltipLabel label="抗尖峰强度" description="越高越能忽略 1-2 秒温度尖峰，但会稍慢响应真实升温。" />
                            <span>{smartControl.hysteresis}</span>
                          </div>
                          <Slider value={smartControl.hysteresis} onChange={(v) => updateSmartControl({ hysteresis: v })} min={1} max={6} step={1} showValue={false} />
                        </div>
                      </div>

                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div className="space-y-1.5">
                          <div className="flex items-center justify-between text-xs text-muted-foreground">
                            <ConfigTooltipLabel label="学习速率" description="学习偏移更新速度，建议保持中等。" />
                            <span>{smartControl.learnRate}</span>
                          </div>
                          <Slider value={smartControl.learnRate} onChange={(v) => updateSmartControl({ learnRate: v })} min={1} max={10} step={1} showValue={false} />
                        </div>
                        <div className="space-y-1.5">
                          <div className="flex items-center justify-between text-xs text-muted-foreground">
                            <ConfigTooltipLabel label="学习范围" description="允许学习偏移的最大幅度，过高会导致偏离用户曲线。" />
                            <span>{smartControl.maxLearnOffset} RPM</span>
                          </div>
                          <Slider value={smartControl.maxLearnOffset} onChange={(v) => updateSmartControl({ maxLearnOffset: v })} min={100} max={1200} step={50} showValue={false} />
                        </div>
                      </div>

                      <div className="flex justify-end">
                        <Button variant="secondary" size="sm" onClick={resetLearning}>重置学习</Button>
                      </div>

                      {/* Learning visualization */}
                      <div className="rounded-xl border border-border/70 bg-muted/30 p-3 space-y-3">
                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <span className="text-xs font-medium text-muted-foreground">学习状态</span>
                          <span className="text-xs text-muted-foreground">温区 {learningInsight.currentTempLabel}</span>
                        </div>

                        <div className="grid grid-cols-3 gap-2">
                          {[
                            ['当前偏移', `${learningInsight.currentOffset > 0 ? '+' : ''}${learningInsight.currentOffset} RPM`, learningInsight.currentOffset > 0 ? 'text-amber-600' : learningInsight.currentOffset < 0 ? 'text-primary' : ''],
                            ['平均强度', `${learningInsight.avgAbsOffset} RPM`, ''],
                            ['最大偏移', `${learningInsight.maxAbsOffset} RPM`, ''],
                          ].map(([label, value, clr]) => (
                            <div key={label} className="rounded-lg bg-card px-3 py-2">
                              <div className="text-[11px] text-muted-foreground">{label}</div>
                              <div className={clsx('text-sm font-semibold', clr)}>{value}</div>
                            </div>
                          ))}
                        </div>

                        {/* Significant points */}
                        {learningInsight.significantPoints.length > 0 ? (
                          <div className="flex flex-wrap gap-1.5">
                            {learningInsight.significantPoints.map((p) => (
                              <span key={p.temperature} className={clsx('rounded-full border px-2 py-0.5 text-[11px]', p.offset > 0 ? 'border-amber-300/60 bg-amber-500/10 text-amber-700 dark:text-amber-300' : 'border-primary/30 bg-primary/10 text-primary')}>
                                {p.temperature}°C {p.offset > 0 ? '+' : ''}{p.offset}
                              </span>
                            ))}
                          </div>
                        ) : (
                          <p className="text-xs text-muted-foreground">暂未形成显著偏移（|偏移| &lt; 20 RPM）。</p>
                        )}
                      </div>
                    </motion.div>
                  )}
                </AnimatePresence>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        <section className="rounded-2xl border border-border/70 bg-card p-4 space-y-3">
          <div className="flex items-center justify-between gap-2">
            <span className="text-sm font-medium">导入 / 导出曲线方案</span>
            <span className="text-xs text-muted-foreground">可复制粘贴短字符串</span>
          </div>

          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <div className="space-y-2 rounded-xl border border-border/70 bg-background/30 p-3">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium text-muted-foreground">导出</span>
                <div className="flex items-center gap-2">
                  <Button variant="secondary" size="sm" onClick={exportProfiles} icon={<Download className="h-3.5 w-3.5" />}>生成</Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={async () => {
                      if (!exportCode) return;
                      if (navigator?.clipboard?.writeText) {
                        await navigator.clipboard.writeText(exportCode);
                        toast.success('已复制导出字符串');
                      }
                    }}
                    icon={<Clipboard className="h-3.5 w-3.5" />}
                    disabled={!exportCode}
                  >
                    复制
                  </Button>
                </div>
              </div>
              <textarea
                value={exportCode}
                readOnly
                rows={3}
                className="w-full rounded-lg border border-border/70 bg-background px-3 py-2 text-xs leading-relaxed"
                placeholder="点击“生成”后显示导出字符串"
              />
            </div>

            <div className="space-y-2 rounded-xl border border-border/70 bg-background/30 p-3">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium text-muted-foreground">导入</span>
                <Button variant="secondary" size="sm" onClick={importProfiles} loading={profileOpLoading} icon={<Upload className="h-3.5 w-3.5" />}>导入</Button>
              </div>
              <textarea
                value={importCode}
                onChange={(e) => setImportCode(e.target.value)}
                rows={3}
                className="w-full rounded-lg border border-border/70 bg-background px-3 py-2 text-xs leading-relaxed"
                placeholder="粘贴 B2C1. 开头的导入字符串"
              />
            </div>
          </div>
        </section>

        <AnimatePresence>
          {showLowRpmWarning && (
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
                className="w-full max-w-md rounded-2xl border border-border bg-card p-6 shadow-xl"
              >
                <div className="mb-4 flex justify-center">
                  <div className="flex h-14 w-14 items-center justify-center rounded-full bg-amber-500/15">
                    <TriangleAlert className="h-8 w-8 text-amber-600" />
                  </div>
                </div>

                <h3 className="mb-3 text-center text-lg font-bold text-foreground">注意</h3>
                <p className="mb-6 rounded-xl border border-amber-300/40 bg-amber-500/10 p-4 text-sm leading-relaxed text-foreground">
                  低于1000转非飞智官方设计最低转速标准，由此引发的任何问题需要由用户自行承担！
                </p>

                <div className="flex justify-end">
                  <Button variant="secondary" size="sm" onClick={() => setShowLowRpmWarning(false)}>
                    我已知悉
                  </Button>
                </div>
              </motion.div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </TooltipProvider>
  );
});

export default FanCurve;
