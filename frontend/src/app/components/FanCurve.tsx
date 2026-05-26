'use client';

import React, { useState, useEffect, useCallback, memo, useMemo, useRef } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip, ResponsiveContainer } from 'recharts';
import { motion, AnimatePresence } from 'framer-motion';
import {
  RotateCw,
  Check,
  History,
  Info,
  Spline,
  TriangleAlert,
  Plus,
  Trash2,
  Clipboard,
  Download,
  Sparkles,
  Upload,
} from 'lucide-react';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Input } from '@/components/ui/input';
import { apiService } from '../services/api';
import { useTemperatureHistory } from '../hooks/useTemperatureHistory';
import { type HistorySeriesKey } from '../lib/temperature-history';
import type { CurveFocusTarget } from '../store/app-store';
import { types } from '../../../wailsjs/go/models';
import { MANUAL_GEAR_PRESETS, BS1_MANUAL_GEAR_PRESETS } from '../lib/manualGearPresets';
import FanCurveProfileSelect from './FanCurveProfileSelect';
import { toast } from 'sonner';
import { ToggleSwitch, Button, Badge, Select, Slider, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from './ui/index';
import clsx from 'clsx';

const LOW_RPM_WARNING_DATE_KEY = 'fanCurveLowRpmWarningDate';
const FAN_CURVE_MIN_TEMP = 30;
const FAN_CURVE_MAX_TEMP = 110;
const FAN_CURVE_TEMP_STEP = 5;
const DEFAULT_CURVE_LENGTH = ((FAN_CURVE_MAX_TEMP - FAN_CURVE_MIN_TEMP) / FAN_CURVE_TEMP_STEP) + 1;
type CurveProfile = { id: string; name: string; curve: types.FanCurvePoint[] };

const LEARNING_BIAS_OPTIONS = [
  { value: 'balanced', label: '均衡', description: '允许学习曲线在基础曲线上下微调。' },
  { value: 'cooling', label: '散热优先', description: '只允许学习增加转速，避免待机温度被学高。' },
  { value: 'quiet', label: '静音优先', description: '只允许学习降低转速，避免自动把风扇拉高。' },
];

function normalizeLearningBias(value: unknown): string {
  return LEARNING_BIAS_OPTIONS.some((option) => option.value === value) ? String(value) : 'balanced';
}

function constrainOffsetByLearningBias(offset: number, learningBias: string) {
  if (learningBias === 'cooling' && offset < 0) return 0;
  if (learningBias === 'quiet' && offset > 0) return 0;
  return offset;
}

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
  focusTarget: CurveFocusTarget | null;
  onFocusHandled: () => void;
}

function formatHistoryTime(timestamp: number) {
  return new Date(timestamp).toLocaleTimeString('zh-CN', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
  });
}

function formatHistoryDateTime(timestamp: number) {
  return new Date(timestamp).toLocaleTimeString('zh-CN', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

function formatHistoryDuration(startTimestamp: number, endTimestamp: number) {
  const durationMs = Math.max(0, endTimestamp - startTimestamp);
  if (durationMs < 60_000) {
    return '< 1 分钟';
  }
  const totalMinutes = Math.round(durationMs / 60_000);
  if (totalMinutes < 60) {
    return `${totalMinutes} 分钟`;
  }
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return minutes > 0 ? `${hours} 小时 ${minutes} 分钟` : `${hours} 小时`;
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

const FanCurve = memo(function FanCurve({ config, onConfigChange, isConnected, temperature, deviceModel, focusTarget, onFocusHandled }: FanCurveProps) {
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
  const [learningConfigLoading, setLearningConfigLoading] = useState(false);
  const [learningResetLoading, setLearningResetLoading] = useState(false);
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [isInteracting, setIsInteracting] = useState(false);
  const [showLowRpmWarning, setShowLowRpmWarning] = useState(false);
  const [historySeriesVisibility, setHistorySeriesVisibility] = useState<Record<HistorySeriesKey, boolean>>({
    cpu: true,
    gpu: true,
    fan: true,
  });
  const chartRef = useRef<HTMLDivElement>(null);
  const curveEditorRef = useRef<HTMLDivElement>(null);
  const historyDetailsRef = useRef<HTMLElement>(null);
  const lowRpmWarnedInDragRef = useRef(false);
  const chartBoundsRef = useRef<{ top: number; bottom: number; left: number; right: number; yMin: number; yMax: number } | null>(null);
  const dragFrameRef = useRef<number | null>(null);
  const pendingDragYRef = useRef<number | null>(null);
  const [rpmRange, setRpmRange] = useState({ min: 0, max: 4000, ticks: [0, 500, 1000, 1500, 2000, 2500, 3000, 3500, 4000] });
  const {
    points: temperatureHistory,
    enabled: temperatureHistoryEnabled,
    saving: temperatureHistorySaving,
    setEnabled: setTemperatureHistoryEnabled,
  } = useTemperatureHistory();

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
      return { enabled: true, learning: true, learningBias: 'balanced', filterTransientSpike: true, targetTemp: 68, aggressiveness: 5, hysteresis: 2, minRpmChange: 50, rampUpLimit: 220, rampDownLimit: 160, learnRate: 4, learnWindow: 6, learnDelay: 2, overheatWeight: 8, rpmDeltaWeight: 5, noiseWeight: 4, trendGain: 5, maxLearnOffset: 600, learnedOffsets: defaultOffsets, learnedOffsetsHeat: defaultOffsets, learnedOffsetsCool: defaultOffsets, learnedRateHeat: defaultRateOffsets, learnedRateCool: defaultRateOffsets };
    }

    return {
      ...existing,
      learning: existing.learning ?? true,
      learningBias: normalizeLearningBias((existing as any).learningBias),
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
  }, [config.fanCurve, config.smartControl, localCurve.length]);

  const currentLearningBias = normalizeLearningBias((smartControl as any).learningBias);
  const currentLearningBiasOption = LEARNING_BIAS_OPTIONS.find((option) => option.value === currentLearningBias) ?? LEARNING_BIAS_OPTIONS[0];

  useEffect(() => {
    if (!focusTarget) {
      return;
    }

    const target = focusTarget === 'history-details' ? historyDetailsRef.current : curveEditorRef.current;
    if (!target) {
      return;
    }

    const frame = window.requestAnimationFrame(() => {
      target.scrollIntoView({ block: 'start' });
      onFocusHandled();
    });

    return () => {
      window.cancelAnimationFrame(frame);
    };
  }, [focusTarget, onFocusHandled]);

  const learnedOffsetSummary = useMemo(() => {
    const sourceCurve = localCurve.length > 0 ? localCurve : (config.fanCurve || []);
    return (smartControl.learnedOffsets || [])
      .map((value, index) => ({ value: constrainOffsetByLearningBias(typeof value === 'number' ? value : 0, currentLearningBias), index }))
      .filter((item) => item.value !== 0 && item.index < sourceCurve.length)
      .sort((left, right) => Math.abs(right.value) - Math.abs(left.value))
      .slice(0, 4)
      .map((item) => ({
        ...item,
        temperature: sourceCurve[item.index]?.temperature,
      }));
  }, [config.fanCurve, currentLearningBias, localCurve, smartControl.learnedOffsets]);

  const detailHistoryPoints = useMemo(() => temperatureHistory.slice(-720), [temperatureHistory]);

  const historyTempDomain = useMemo<[number, number]>(() => {
    const values = detailHistoryPoints.flatMap((point) => [point.cpuTemp, point.gpuTemp]).filter((value) => value > 0);
    if (values.length === 0) {
      return [30, 90];
    }
    const min = Math.max(0, Math.floor((Math.min(...values) - 4) / 5) * 5);
    const max = Math.min(110, Math.ceil((Math.max(...values) + 4) / 5) * 5);
    return [min, Math.max(min + 10, max)];
  }, [detailHistoryPoints]);

  const historyFanMax = useMemo(() => {
    return Math.max(4000, ...detailHistoryPoints.map((point) => point.fanRpm).filter((value) => value > 0), 0);
  }, [detailHistoryPoints]);

  const historySummary = useMemo(() => {
    const latest = temperatureHistory[temperatureHistory.length - 1] ?? null;
    const first = temperatureHistory[0] ?? null;
    const cpuValues = temperatureHistory.map((point) => point.cpuTemp).filter((value) => value > 0);
    const gpuValues = temperatureHistory.map((point) => point.gpuTemp).filter((value) => value > 0);
    const fanValues = temperatureHistory.map((point) => point.fanRpm).filter((value) => value > 0);
    const average = (values: number[]) => values.length > 0 ? Math.round(values.reduce((sum, value) => sum + value, 0) / values.length) : 0;

    return {
      sampleCount: temperatureHistory.length,
      latest,
      latestLabel: latest ? formatHistoryDateTime(latest.timestamp) : '--',
      durationLabel: first && latest ? formatHistoryDuration(first.timestamp, latest.timestamp) : '--',
      cpuPeak: cpuValues.length > 0 ? Math.max(...cpuValues) : 0,
      cpuAverage: average(cpuValues),
      gpuPeak: gpuValues.length > 0 ? Math.max(...gpuValues) : 0,
      gpuAverage: average(gpuValues),
      fanPeak: fanValues.length > 0 ? Math.max(...fanValues) : 0,
      fanAverage: average(fanValues),
    };
  }, [temperatureHistory]);
  const historyChartData = useMemo(() => detailHistoryPoints.map((point) => ({ ...point })), [detailHistoryPoints]);

  const historySeriesMeta = useMemo(() => ([
    { key: 'cpu' as const, label: 'CPU', color: '#2f6df6' },
    { key: 'gpu' as const, label: 'GPU', color: '#f97316' },
    { key: 'fan' as const, label: '风扇 RPM', color: '#10b981' },
  ]), []);

  const toggleHistorySeries = useCallback((series: HistorySeriesKey) => {
    setHistorySeriesVisibility((prev) => ({
      ...prev,
      [series]: !prev[series],
    }));
  }, []);

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
    const offsets = smartControl.learnedOffsets || [];
    return localCurve.map((point, index) => {
      const offset = constrainOffsetByLearningBias(offsets[index] ?? 0, currentLearningBias);
      return {
        temperature: point.temperature,
        rpm: point.rpm,
        coupledRpm: Math.max(curveRpmBounds.min, Math.min(curveRpmBounds.max, point.rpm + offset)),
        index,
      };
    });
  }, [curveRpmBounds.max, curveRpmBounds.min, currentLearningBias, localCurve, smartControl.learnedOffsets]);

  const hasLearnedOffsets = learnedOffsetSummary.length > 0;
  const showCoupledCurve = config.autoControl && !!smartControl.learning && hasLearnedOffsets;

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

  const scheduleDrag = useCallback((clientY: number) => {
    pendingDragYRef.current = clientY;
    if (dragFrameRef.current !== null) {
      return;
    }

    dragFrameRef.current = window.requestAnimationFrame(() => {
      dragFrameRef.current = null;
      const nextClientY = pendingDragYRef.current;
      pendingDragYRef.current = null;
      if (nextClientY !== null) {
        handleDrag(nextClientY);
      }
    });
  }, [handleDrag]);

  const handleDragEnd = useCallback(() => {
    if (dragFrameRef.current !== null) {
      window.cancelAnimationFrame(dragFrameRef.current);
      dragFrameRef.current = null;
    }
    pendingDragYRef.current = null;
    setDragIndex(null);
    setTimeout(() => setIsInteracting(false), 100);
  }, []);

  useEffect(() => {
    if (dragIndex === null) return;
    const mm = (e: MouseEvent) => { e.preventDefault(); scheduleDrag(e.clientY); };
    const tm = (e: TouchEvent) => { if (e.touches.length > 0) scheduleDrag(e.touches[0].clientY); };
    const end = () => handleDragEnd();
    document.addEventListener('mousemove', mm);
    document.addEventListener('mouseup', end);
    document.addEventListener('touchmove', tm, { passive: false });
    document.addEventListener('touchend', end);
    return () => {
      document.removeEventListener('mousemove', mm);
      document.removeEventListener('mouseup', end);
      document.removeEventListener('touchmove', tm);
      document.removeEventListener('touchend', end);
      if (dragFrameRef.current !== null) {
        window.cancelAnimationFrame(dragFrameRef.current);
        dragFrameRef.current = null;
      }
      pendingDragYRef.current = null;
    };
  }, [dragIndex, handleDragEnd, scheduleDrag]);

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

  const updateSmartControlConfig = useCallback(async (patch: Partial<types.SmartControlConfig> & { learningBias?: string }) => {
    setLearningConfigLoading(true);
    try {
      const nextSmartControl = types.SmartControlConfig.createFrom({ ...smartControl, ...patch });
      const nextConfig = types.AppConfig.createFrom({ ...config, smartControl: nextSmartControl });
      await apiService.updateConfig(nextConfig);
      onConfigChange(nextConfig);
    } catch (err) {
      toast.error('保存学习设置失败', { description: err instanceof Error ? err.message : String(err) });
    } finally {
      setLearningConfigLoading(false);
    }
  }, [config, onConfigChange, smartControl]);

  const handleLearningToggle = useCallback((enabled: boolean) => {
    void updateSmartControlConfig({ learning: enabled });
  }, [updateSmartControlConfig]);

  const handleLearningBiasChange = useCallback((value: string) => {
    void updateSmartControlConfig({ learningBias: normalizeLearningBias(value) });
  }, [updateSmartControlConfig]);

  const handleResetLearnedOffsets = useCallback(async () => {
    setLearningResetLoading(true);
    try {
      await apiService.resetLearnedOffsets();
      await syncConfigFromBackend();
      toast.success('已重置学习偏移', { description: '风扇曲线将完全按用户设定执行', duration: 2400 });
    } catch (err) {
      toast.error('重置失败', { description: err instanceof Error ? err.message : String(err) });
    } finally {
      setLearningResetLoading(false);
    }
  }, [syncConfigFromBackend]);

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
    <div className="relative space-y-4 px-1 pb-2">
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
        <div ref={curveEditorRef}>
          <div
            ref={chartRef}
            className={clsx('relative rounded-3xl border bg-card p-4 shadow-sm', dragIndex !== null ? 'ring-2 ring-primary/40 border-primary/30' : 'border-border/70')}
          >
            <div className="h-80 md:h-96 relative">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={chartData} margin={{ top: 20, right: 30, left: 20, bottom: 20 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" />
                  <XAxis dataKey="temperature" type="number" domain={[temperatureRange.min, temperatureRange.max]} ticks={temperatureRange.ticks} tickLine={false} axisLine={{ stroke: 'var(--chart-axis)' }} tick={{ fill: 'var(--chart-tick)', fontSize: 11 }} label={{ value: '温度 (°C)', position: 'insideBottom', offset: -10, fill: 'var(--chart-tick)', fontSize: 12 }} />
                  <YAxis type="number" domain={[rpmRange.min, rpmRange.max]} ticks={rpmRange.ticks} tickLine={false} axisLine={{ stroke: 'var(--chart-axis)' }} tick={{ fill: 'var(--chart-tick)', fontSize: 11 }} label={{ value: '转速 (RPM)', angle: -90, position: 'insideLeft', fill: 'var(--chart-tick)', fontSize: 12 }} />
                  <RechartsTooltip
                    formatter={(value, name) => {
                      const numericValue = Number(value ?? 0);
                      return name === 'coupledRpm' ? [`${numericValue} RPM`, '学习曲线'] : [`${numericValue} RPM`, '基础曲线'];
                    }}
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
        </div>

        <section className="rounded-2xl border border-border/70 bg-card p-4 shadow-sm">
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <Sparkles className="h-4 w-4 text-amber-500" />
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="text-sm font-medium text-foreground">自适应学习</div>
                  {!smartControl.learning && <Badge variant="info">已暂停</Badge>}
                </div>
                <div className="text-xs leading-relaxed text-muted-foreground">长时间运行后，自动微调每个温度点的目标转速，使实际温度更贴近曲线设定值。学到的偏移随时可一键重置。</div>
              </div>
            </div>
            <ToggleSwitch
              enabled={!!smartControl.learning}
              onChange={handleLearningToggle}
              loading={learningConfigLoading}
              size="sm"
              color="purple"
              srLabel="切换自适应学习"
            />
          </div>

          <div className="mt-3 flex flex-col gap-3 rounded-xl border border-border/70 bg-background/45 p-3">
            <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
              <div className="min-w-0">
                <div className="text-xs font-medium text-muted-foreground">学习倾向</div>
                <div className="mt-1 text-xs leading-relaxed text-muted-foreground">{currentLearningBiasOption.description}</div>
              </div>
              <Select
                value={currentLearningBias}
                onChange={handleLearningBiasChange}
                options={LEARNING_BIAS_OPTIONS}
                disabled={learningConfigLoading}
                size="sm"
                className="w-full md:w-44"
              />
            </div>

            <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div className="min-w-0">
                <div className="text-xs font-medium text-muted-foreground">学习偏移</div>
                <div className="mt-1 text-xs leading-relaxed text-muted-foreground">当前学习曲线相对基础曲线的主要 RPM 修正点。</div>
              </div>
              <Button
                variant="secondary"
                size="sm"
                onClick={handleResetLearnedOffsets}
                loading={learningResetLoading}
                disabled={!hasLearnedOffsets}
                icon={<Sparkles className="h-3.5 w-3.5" />}
              >
                重置学习
              </Button>
            </div>

            {hasLearnedOffsets ? (
              <div className="grid grid-cols-2 gap-2 text-xs text-muted-foreground md:grid-cols-4">
                {learnedOffsetSummary.map((item) => (
                  <div key={item.index} className="rounded-lg border border-border/70 bg-card/70 px-3 py-2 tabular-nums">
                    <span>{item.temperature}°C </span>
                    <span className={clsx('font-semibold', item.value > 0 ? 'text-orange-500' : 'text-blue-500')}>
                      {item.value > 0 ? '+' : ''}{item.value} RPM
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-lg border border-dashed border-border/70 bg-card/55 px-3 py-2 text-xs text-muted-foreground">暂无学习偏移</div>
            )}
          </div>
        </section>

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

        <section ref={historyDetailsRef} className="rounded-2xl border border-border/70 bg-card p-4 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2">
              <History className="h-4 w-4 text-primary" />
              <div>
                <div className="text-sm font-medium text-foreground">温度与风扇历史详情</div>
                <div className="text-xs text-muted-foreground">曲线页保留最近 1 小时历史概览，便于对照当前曲线与温度走势。</div>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <ToggleSwitch
                enabled={temperatureHistoryEnabled}
                onChange={setTemperatureHistoryEnabled}
                loading={temperatureHistorySaving}
                label={temperatureHistorySaving ? '保存中' : '后台记录'}
                size="sm"
                color="blue"
              />
            </div>
          </div>

          {temperatureHistory.length === 0 ? (
            <div className="rounded-xl border border-dashed border-border/70 bg-background/35 px-4 py-8 text-center text-sm text-muted-foreground">
              {temperatureHistoryEnabled ? '后台记录已开启，等待更多温度与风扇样本。' : '后台记录当前已关闭，可在本页开启。'}
            </div>
          ) : (
            <>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                {[
                  ['CPU 峰值', historySummary.cpuPeak ? `${historySummary.cpuPeak}°C` : '--', historySummary.cpuAverage ? `平均 ${historySummary.cpuAverage}°C` : '暂无 CPU 温度'],
                  ['GPU 峰值', historySummary.gpuPeak ? `${historySummary.gpuPeak}°C` : '--', historySummary.gpuAverage ? `平均 ${historySummary.gpuAverage}°C` : '暂无 GPU 温度'],
                  ['风扇峰值', historySummary.fanPeak ? `${historySummary.fanPeak} RPM` : '--', historySummary.fanAverage ? `平均 ${historySummary.fanAverage} RPM` : '暂无风扇数据'],
                ].map(([label, value, hint]) => (
                  <div key={label} className="rounded-xl border border-border/70 bg-background/35 p-3">
                    <div className="text-[11px] text-muted-foreground">{label}</div>
                    <div className="mt-1 text-sm font-semibold text-foreground">{value}</div>
                    <div className="mt-1 text-[11px] text-muted-foreground">{hint}</div>
                  </div>
                ))}
              </div>

              <div className="rounded-xl border border-border/70 bg-background/35 p-3 space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="text-xs font-medium text-muted-foreground">最近趋势</div>
                  <div className="flex flex-wrap items-center gap-2">
                    {historySeriesMeta.map((series) => (
                      <button
                        key={series.key}
                        type="button"
                        onClick={() => toggleHistorySeries(series.key)}
                        className={clsx(
                          'inline-flex cursor-pointer items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] transition-colors',
                          historySeriesVisibility[series.key]
                            ? 'border-border bg-card text-foreground'
                            : 'border-border/60 bg-transparent text-muted-foreground/65',
                        )}
                      >
                        <span className="h-2 w-2 rounded-full" style={{ backgroundColor: series.color }} />
                        {series.label}
                      </button>
                    ))}
                  </div>
                </div>

                {historyChartData.length < 2 ? (
                  <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">已记录 1 条样本，等待更多数据后展示趋势图。</div>
                ) : (
                  <div className="h-72">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={historyChartData} margin={{ top: 12, right: 16, left: 4, bottom: 8 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="var(--chart-grid)" />
                        <XAxis
                          dataKey="timestamp"
                          type="number"
                          domain={['dataMin', 'dataMax']}
                          tickFormatter={(value) => formatHistoryTime(Number(value))}
                          tickLine={false}
                          minTickGap={24}
                          axisLine={{ stroke: 'var(--chart-axis)' }}
                          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
                        />
                        <YAxis
                          yAxisId="temp"
                          type="number"
                          domain={historyTempDomain}
                          tickLine={false}
                          axisLine={{ stroke: 'var(--chart-axis)' }}
                          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
                          width={40}
                        />
                        <YAxis
                          yAxisId="fan"
                          orientation="right"
                          type="number"
                          domain={[0, historyFanMax]}
                          tickLine={false}
                          axisLine={{ stroke: 'var(--chart-axis)' }}
                          tick={{ fill: 'var(--chart-tick)', fontSize: 11 }}
                          width={52}
                        />
                        <RechartsTooltip
                          labelFormatter={(value) => formatHistoryDateTime(Number(value))}
                          formatter={(value, name) => {
                            const numericValue = Number(value ?? 0);
                            if (name === 'fanRpm') {
                              return [`${numericValue} RPM`, '风扇 RPM'];
                            }
                            return [`${numericValue} °C`, name === 'cpuTemp' ? 'CPU' : 'GPU'];
                          }}
                          contentStyle={{ backgroundColor: 'var(--chart-tooltip-bg)', border: '1px solid', borderColor: 'var(--chart-tooltip-border)', borderRadius: '8px', boxShadow: 'var(--chart-tooltip-shadow)', padding: '8px 12px', color: 'var(--chart-tooltip-text)' }}
                          labelStyle={{ color: 'var(--chart-tooltip-text)', fontWeight: 600 }}
                          itemStyle={{ color: 'var(--chart-tooltip-text)' }}
                        />
                        {historySeriesVisibility.cpu && <Line yAxisId="temp" type="monotone" dataKey="cpuTemp" stroke="#2f6df6" strokeWidth={2.3} dot={false} activeDot={false} isAnimationActive={false} connectNulls />}
                        {historySeriesVisibility.gpu && <Line yAxisId="temp" type="monotone" dataKey="gpuTemp" stroke="#f97316" strokeWidth={2.3} dot={false} activeDot={false} isAnimationActive={false} connectNulls />}
                        {historySeriesVisibility.fan && <Line yAxisId="fan" type="monotone" dataKey="fanRpm" stroke="#10b981" strokeWidth={2} strokeDasharray="5 4" dot={false} activeDot={false} isAnimationActive={false} connectNulls />}
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </div>
            </>
          )}
        </section>

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

        <Dialog open={showLowRpmWarning} onOpenChange={setShowLowRpmWarning}>
          <DialogContent
            hideClose
            overlayClassName="bg-black/40 backdrop-blur-sm"
            className="max-w-md rounded-2xl border border-border p-0 shadow-xl"
            onPointerDownOutside={(event) => event.preventDefault()}
            onEscapeKeyDown={(event) => event.preventDefault()}
          >
            <div className="p-6">
              <DialogHeader className="items-center text-center">
                <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-amber-500/15">
                  <TriangleAlert className="h-8 w-8 text-amber-600" />
                </div>
                <DialogTitle className="text-lg font-bold text-foreground">注意</DialogTitle>
                <DialogDescription asChild>
                  <div className="mt-1 rounded-xl border border-amber-300/40 bg-amber-500/10 p-4 text-left text-sm leading-relaxed text-foreground">
                    低于1000转非飞智官方设计最低转速标准，由此引发的任何问题需要由用户自行承担！
                  </div>
                </DialogDescription>
              </DialogHeader>

              <DialogFooter className="mt-6">
                <Button variant="secondary" size="sm" onClick={() => setShowLowRpmWarning(false)}>
                  我已知悉
                </Button>
              </DialogFooter>
            </div>
          </DialogContent>
        </Dialog>
      </div>
  );
});

export default FanCurve;
