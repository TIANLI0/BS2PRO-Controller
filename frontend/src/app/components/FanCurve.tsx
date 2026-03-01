'use client';

import React, { useState, useEffect, useCallback, memo, useMemo, useRef } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip, ResponsiveContainer } from 'recharts';
import { motion, AnimatePresence } from 'framer-motion';
import {
  RotateCw,
  Check,
  Info,
  Spline,
} from 'lucide-react';
import { Tooltip, TooltipContent, TooltipTrigger, TooltipProvider } from '@/components/ui/tooltip';
import { apiService } from '../services/api';
import { types } from '../../../wailsjs/go/models';
import { ToggleSwitch, Select, Button, Badge, Slider } from './ui/index';
import clsx from 'clsx';

interface FanCurveProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
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
          <button type="button" className="inline-flex items-center justify-center rounded text-muted-foreground transition-colors hover:text-foreground" aria-label={`${label}说明`}>
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

const FanCurve = memo(function FanCurve({ config, onConfigChange, isConnected, fanData, temperature }: FanCurveProps) {
  const [localCurve, setLocalCurve] = useState<types.FanCurvePoint[]>([]);
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);
  const [isInitialized, setIsInitialized] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [tempTrendDelta, setTempTrendDelta] = useState(0);
  const [trendPreviewMode, setTrendPreviewMode] = useState<'auto' | 'heat' | 'cool'>('auto');
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [isInteracting, setIsInteracting] = useState(false);
  const chartRef = useRef<HTMLDivElement>(null);
  const previousMaxTempRef = useRef<number | null>(null);
  const chartBoundsRef = useRef<{ top: number; bottom: number; left: number; right: number; yMin: number; yMax: number } | null>(null);
  const [rpmRange, setRpmRange] = useState({ min: 0, max: 4000, ticks: [0, 500, 1000, 1500, 2000, 2500, 3000, 3500, 4000] });

  const temperatureRange = useMemo(() => ({ min: 30, max: 95, ticks: Array.from({ length: 14 }, (_, i) => 30 + i * 5) }), []);

  /* ── Smart control state ── */

  const smartControl = useMemo(() => {
    const curveLength = config.fanCurve?.length || localCurve.length || 14;
    const defaultOffsets = Array.from({ length: curveLength }, () => 0);
    const defaultRateOffsets = Array.from({ length: 7 }, () => 0);
    const existing = config.smartControl;
    const normalizeOffsets = (source?: number[]) => Array.isArray(source) ? [...source.slice(0, curveLength), ...defaultOffsets].slice(0, curveLength) : defaultOffsets;
    const normalizeRateOffsets = (source?: number[]) => Array.isArray(source) ? [...source.slice(0, 7), ...defaultRateOffsets].slice(0, 7) : defaultRateOffsets;

    if (!existing) {
      return { enabled: true, learning: true, targetTemp: 68, aggressiveness: 5, hysteresis: 2, minRpmChange: 50, rampUpLimit: 220, rampDownLimit: 160, learnRate: 4, learnWindow: 6, learnDelay: 2, overheatWeight: 8, rpmDeltaWeight: 5, noiseWeight: 4, trendGain: 5, maxLearnOffset: 600, learnedOffsets: defaultOffsets, learnedOffsetsHeat: defaultOffsets, learnedOffsetsCool: defaultOffsets, learnedRateHeat: defaultRateOffsets, learnedRateCool: defaultRateOffsets };
    }

    return {
      ...existing,
      learning: true,
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

  const effectiveTrendDelta = useMemo(() => {
    if (trendPreviewMode === 'heat') return 1;
    if (trendPreviewMode === 'cool') return -1;
    return tempTrendDelta;
  }, [trendPreviewMode, tempTrendDelta]);

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
    const heatOffsets = smartControl.learnedOffsetsHeat || [];
    const coolOffsets = smartControl.learnedOffsetsCool || [];
    const rateHeat = smartControl.learnedRateHeat || [];
    const rateCool = smartControl.learnedRateCool || [];
    const rateBucketLabels = ['≤-3', '-2', '-1', '0', '+1', '+2', '≥+3'];
    const rateBucketIndex = Math.max(0, Math.min(6, effectiveTrendDelta + 3));
    const activeRateBias = effectiveTrendDelta >= 0 ? (rateHeat[rateBucketIndex] ?? 0) : (rateCool[rateBucketIndex] ?? 0);

    const points = (config.fanCurve && config.fanCurve.length > 0 ? config.fanCurve : localCurve)
      .map((point, index) => ({ index, temperature: point.temperature, offset: offsets[index] ?? 0, heatOffset: heatOffsets[index] ?? 0, coolOffset: coolOffsets[index] ?? 0 }));

    if (points.length === 0) {
      return { currentOffset: 0, currentHeatOffset: 0, currentCoolOffset: 0, activeRateBias: 0, currentTempLabel: '--', maxAbsOffset: 0, avgAbsOffset: 0, maxAbsHeatOffset: 0, maxAbsCoolOffset: 0, significantPoints: [] as Array<{ temperature: number; offset: number }>, rateBuckets: [] as Array<{ label: string; heat: number; cool: number; isActive: boolean }> };
    }

    const maxTemp = temperature?.maxTemp ?? null;
    let currentPoint = points[0];
    if (maxTemp !== null) {
      currentPoint = points.reduce((best, item) => Math.abs(item.temperature - maxTemp) < Math.abs(best.temperature - maxTemp) ? item : best, points[0]);
    }

    const absOffsets = points.map((p) => Math.abs(p.offset));
    const absHeatOffsets = points.map((p) => Math.abs(p.heatOffset));
    const absCoolOffsets = points.map((p) => Math.abs(p.coolOffset));
    const totalAbsOffset = absOffsets.reduce((s, v) => s + v, 0);

    const significantPoints = points.filter((p) => Math.abs(p.offset) >= 20).sort((a, b) => Math.abs(b.offset) - Math.abs(a.offset)).slice(0, 6).map((p) => ({ temperature: p.temperature, offset: p.offset }));

    const rateBuckets = [0, 1, 2, 3, 4, 5, 6].map((idx) => ({
      label: rateBucketLabels[idx],
      heat: rateHeat[idx] ?? 0,
      cool: rateCool[idx] ?? 0,
      isActive: idx === rateBucketIndex,
    }));

    return {
      currentOffset: currentPoint.offset,
      currentHeatOffset: currentPoint.heatOffset,
      currentCoolOffset: currentPoint.coolOffset,
      activeRateBias,
      currentTempLabel: `${currentPoint.temperature}°C`,
      maxAbsOffset: absOffsets.reduce((m, v) => Math.max(m, v), 0),
      avgAbsOffset: Math.round(totalAbsOffset / points.length),
      maxAbsHeatOffset: absHeatOffsets.reduce((m, v) => Math.max(m, v), 0),
      maxAbsCoolOffset: absCoolOffsets.reduce((m, v) => Math.max(m, v), 0),
      significantPoints,
      rateBuckets,
    };
  }, [config.fanCurve, localCurve, smartControl.learnedOffsets, smartControl.learnedOffsetsHeat, smartControl.learnedOffsetsCool, smartControl.learnedRateHeat, smartControl.learnedRateCool, effectiveTrendDelta, temperature?.maxTemp]);

  /* ── Init ── */

  useEffect(() => {
    if (!isInitialized && config.fanCurve && config.fanCurve.length > 0) {
      setLocalCurve([...config.fanCurve]);
      setIsInitialized(true);
      if (fanData?.maxGear) {
        let maxRpm = 4000;
        switch (fanData.maxGear) { case '标准': maxRpm = 2760; break; case '强劲': maxRpm = 3300; break; case '超频': maxRpm = 4000; break; }
        const step = 500;
        setRpmRange({ min: 0, max: maxRpm, ticks: Array.from({ length: Math.floor(maxRpm / step) + 1 }, (_, i) => i * step) });
      }
    }
  }, [config.fanCurve, fanData?.maxGear, isInitialized]);

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
      const offset = baseOffset + trendRateBias;
      return { temperature: point.temperature, rpm: point.rpm, coupledRpm: Math.max(rpmRange.min, Math.min(rpmRange.max, point.rpm + offset)), index };
    });
  }, [localCurve, smartControl.learnedOffsets, smartControl.learnedOffsetsHeat, smartControl.learnedOffsetsCool, smartControl.learnedRateHeat, smartControl.learnedRateCool, effectiveTrendDelta, rpmRange.max, rpmRange.min]);

  const showCoupledCurve = config.autoControl && smartControl.enabled;

  /* ── Point update + drag ── */

  const updatePoint = useCallback((index: number, newRpm: number) => {
    const clampedRpm = Math.max(rpmRange.min, Math.min(rpmRange.max, Math.round(newRpm / 50) * 50));
    setLocalCurve((prev) => { if (prev[index]?.rpm === clampedRpm) return prev; const c = [...prev]; c[index] = { ...c[index], rpm: clampedRpm }; return c; });
    setHasUnsavedChanges(true);
  }, [rpmRange]);

  const handleDragStart = useCallback((index: number) => {
    setDragIndex(index);
    setIsInteracting(true);
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

  const saveCurve = useCallback(async () => {
    if (isSaving) return;
    try { setIsSaving(true); await apiService.setFanCurve(localCurve); onConfigChange(types.AppConfig.createFrom({ ...config, fanCurve: localCurve })); setHasUnsavedChanges(false); } catch { /* noop */ } finally { setIsSaving(false); }
  }, [localCurve, config, onConfigChange, isSaving]);

  const resetCurve = useCallback(() => {
    const d: types.FanCurvePoint[] = [
      { temperature: 30, rpm: 1000 }, { temperature: 35, rpm: 1200 }, { temperature: 40, rpm: 1400 }, { temperature: 45, rpm: 1600 },
      { temperature: 50, rpm: 1800 }, { temperature: 55, rpm: 2000 }, { temperature: 60, rpm: Math.min(2300, rpmRange.max) },
      { temperature: 65, rpm: Math.min(2600, rpmRange.max) }, { temperature: 70, rpm: Math.min(2900, rpmRange.max) },
      { temperature: 75, rpm: Math.min(3200, rpmRange.max) }, { temperature: 80, rpm: Math.min(3500, rpmRange.max) },
      { temperature: 85, rpm: Math.min(3800, rpmRange.max) }, { temperature: 90, rpm: rpmRange.max }, { temperature: 95, rpm: rpmRange.max },
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
      const merged = { ...smartControl, ...patch, learning: true };
      onConfigChange(types.AppConfig.createFrom({ ...config, smartControl: merged }));
    } catch { /* noop */ }
  }, [config, onConfigChange, smartControl]);

  const resetLearning = useCallback(() => {
    const len = localCurve.length || config.fanCurve.length || 14;
    const z = Array.from({ length: len }, () => 0);
    const r = Array.from({ length: 7 }, () => 0);
    updateSmartControl({ learnedOffsets: z, learnedOffsetsHeat: z, learnedOffsetsCool: z, learnedRateHeat: r, learnedRateCool: r });
  }, [config.fanCurve.length, localCurve.length, updateSmartControl]);

  /* ── Manual gear ── */

  const gearOptions = [
    { value: '静音', label: '静音', description: '低噪音' },
    { value: '标准', label: '标准', description: '平衡' },
    { value: '强劲', label: '强劲', description: '高性能' },
    { value: '超频', label: '超频', description: '极限' },
  ];
  const levelOptions = [{ value: '低', label: '低' }, { value: '中', label: '中' }, { value: '高', label: '高' }];

  const handleGearChange = useCallback(async (gear: string) => {
    try { await apiService.setManualGear(gear, config.manualLevel || '中'); onConfigChange(types.AppConfig.createFrom({ ...config, manualGear: gear })); } catch { /* noop */ }
  }, [config, onConfigChange]);

  const handleLevelChange = useCallback(async (level: string) => {
    try { await apiService.setManualGear(config.manualGear || '标准', level); onConfigChange(types.AppConfig.createFrom({ ...config, manualLevel: level })); } catch { /* noop */ }
  }, [config, onConfigChange]);

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
              <ToggleSwitch enabled={config.autoControl} onChange={handleAutoControlChange} label="智能变频" size="sm" color="blue" />
            </div>
          </div>
        </motion.div>

        {/* ── Manual gear (when auto off) ── */}
        <AnimatePresence>
          {!config.autoControl && isConnected && (
            <motion.div initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: 'auto' }} exit={{ opacity: 0, height: 0 }} className="overflow-hidden">
              <div className="rounded-2xl border border-border/70 bg-card p-4">
                <div className="flex flex-wrap items-center gap-4">
                  <span className="text-sm font-medium">手动挡位</span>
                  <div className="flex items-center gap-3">
                    <Select value={config.manualGear || '标准'} onChange={handleGearChange} options={gearOptions} size="sm" />
                    <Select value={config.manualLevel || '中'} onChange={handleLevelChange} options={levelOptions} size="sm" />
                  </div>
                </div>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {/* ── Smart learning (when auto on) ── */}
        <AnimatePresence>
          {config.autoControl && isConnected && (
            <motion.div initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: 'auto' }} exit={{ opacity: 0, height: 0 }} className="overflow-hidden">
              <div className="rounded-2xl border border-border/70 bg-card p-4 space-y-4">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <span className="text-sm font-medium">智能学习控温</span>
                  <ToggleSwitch enabled={smartControl.enabled} onChange={(e) => updateSmartControl({ enabled: e })} label="启用" size="sm" color="blue" />
                </div>

                {/* Core sliders */}
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  <div className="space-y-1.5">
                    <div className="flex items-center justify-between text-xs text-muted-foreground">
                      <ConfigTooltipLabel label="目标温度" description="智能控制会尽量把温度稳定在这个值附近。" />
                      <span>{smartControl.targetTemp}°C</span>
                    </div>
                    <Slider value={smartControl.targetTemp} onChange={(v) => updateSmartControl({ targetTemp: v })} min={55} max={85} step={1} showValue={false} />
                  </div>
                  <div className="space-y-1.5">
                    <div className="flex items-center justify-between text-xs text-muted-foreground">
                      <ConfigTooltipLabel label="响应强度" description="温度变化时转速调整的激进程度。" />
                      <span>{smartControl.aggressiveness}</span>
                    </div>
                    <Slider value={smartControl.aggressiveness} onChange={(v) => updateSmartControl({ aggressiveness: v })} min={1} max={10} step={1} showValue={false} />
                  </div>
                  <div className="space-y-1.5">
                    <div className="flex items-center justify-between text-xs text-muted-foreground">
                      <ConfigTooltipLabel label="学习速率" description="学习偏移更新速度。" />
                      <span>{smartControl.learnRate}</span>
                    </div>
                    <Slider value={smartControl.learnRate} onChange={(v) => updateSmartControl({ learnRate: v })} min={1} max={10} step={1} showValue={false} />
                  </div>
                </div>

                {/* Debug-mode advanced sliders */}
                {config.debugMode && (
                  <>
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                      <div className="space-y-1.5">
                        <div className="flex items-center justify-between text-xs text-muted-foreground">
                          <ConfigTooltipLabel label="学习曲线预览" description="强制按升温或降温工况预览学习曲线。" />
                        </div>
                        <Select value={trendPreviewMode} onChange={(v) => setTrendPreviewMode(v as any)} options={[{ value: 'auto', label: '自动（实时ΔT）' }, { value: 'heat', label: '强制升温态' }, { value: 'cool', label: '强制降温态' }]} size="sm" />
                      </div>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                      {([
                        ['学习窗口', '连续观察的稳定采样点数量。', smartControl.learnWindow, 3, 24, 'learnWindow'],
                        ['学习延迟', '补偿散热系统热惯性。', smartControl.learnDelay, 1, 8, 'learnDelay'],
                        ['温升趋势增益', '升温阶段前馈增益。', smartControl.trendGain, 1, 12, 'trendGain'],
                      ] as const).map(([label, desc, val, mn, mx, key]) => (
                        <div key={key} className="space-y-1.5">
                          <div className="flex items-center justify-between text-xs text-muted-foreground">
                            <ConfigTooltipLabel label={label} description={desc} />
                            <span>{val}</span>
                          </div>
                          <Slider value={val} onChange={(v) => updateSmartControl({ [key]: v })} min={mn} max={mx} step={1} showValue={false} />
                        </div>
                      ))}
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                      {([
                        ['过热惩罚', '过热时升速增益。', smartControl.overheatWeight, 1, 12, 'overheatWeight'],
                        ['转速变化惩罚', '降低来回拉扯。', smartControl.rpmDeltaWeight, 1, 12, 'rpmDeltaWeight'],
                        ['噪音惩罚', '高转速噪音惩罚。', smartControl.noiseWeight, 0, 12, 'noiseWeight'],
                      ] as const).map(([label, desc, val, mn, mx, key]) => (
                        <div key={key} className="space-y-1.5">
                          <div className="flex items-center justify-between text-xs text-muted-foreground">
                            <ConfigTooltipLabel label={label} description={desc} />
                            <span>{val}</span>
                          </div>
                          <Slider value={val} onChange={(v) => updateSmartControl({ [key]: v })} min={mn} max={mx} step={1} showValue={false} />
                        </div>
                      ))}
                    </div>
                  </>
                )}

                {!config.debugMode && (
                  <p className="text-xs text-muted-foreground">高级学习参数可在「设置 → 调试面板」中微调。</p>
                )}

                <div className="flex justify-end">
                  <Button variant="secondary" size="sm" onClick={resetLearning}>重置学习</Button>
                </div>

                {/* Learning visualization */}
                {smartControl.enabled && (
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

                    <div className="grid grid-cols-3 gap-2">
                      {[
                        ['升/降温偏移', `+${learningInsight.currentHeatOffset} / ${learningInsight.currentCoolOffset > 0 ? '+' : ''}${learningInsight.currentCoolOffset} RPM`],
                        ['升温最大', `${learningInsight.maxAbsHeatOffset} RPM`],
                        ['降温最大', `${learningInsight.maxAbsCoolOffset} RPM`],
                      ].map(([label, value]) => (
                        <div key={label} className="rounded-lg bg-card px-3 py-2">
                          <div className="text-[11px] text-muted-foreground">{label}</div>
                          <div className="text-sm font-semibold">{value}</div>
                        </div>
                      ))}
                    </div>

                    {/* Rate buckets */}
                    <div className="rounded-lg bg-card px-3 py-2 space-y-2">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <ConfigTooltipLabel label="变化率分桶" description="按温度变化率 ΔT 分桶学习偏置。" />
                        <span className="text-[11px] text-muted-foreground">偏置 {learningInsight.activeRateBias > 0 ? '+' : ''}{learningInsight.activeRateBias} RPM（ΔT={effectiveTrendDelta > 0 ? '+' : ''}{effectiveTrendDelta}）</span>
                      </div>
                      <div className="flex flex-wrap gap-1.5">
                        {learningInsight.rateBuckets.map((b) => (
                          <span key={b.label} className={clsx('rounded-full border px-2 py-0.5 text-[11px]', b.isActive ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border text-muted-foreground')}>
                            ΔT{b.label} H{b.heat > 0 ? '+' : ''}{b.heat}/C{b.cool > 0 ? '+' : ''}{b.cool}
                          </span>
                        ))}
                      </div>
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
                )}
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
      </div>
    </TooltipProvider>
  );
});

export default FanCurve;
