'use client';

import React, { useState, useEffect, useCallback, memo, useMemo, useRef } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { 
  RotateCw,
  Check,
  Info,
} from 'lucide-react';
import { apiService } from '../services/api';
import { types } from '../../../wailsjs/go/models';
import { ToggleSwitch, Select, Button, Badge, Card, Slider } from './ui/index';
import clsx from 'clsx';

interface FanCurveProps {
  config: types.AppConfig;
  onConfigChange: (config: types.AppConfig) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
}

// 独立的温度指示线组件 - 不会触发图表重绘
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
    if (temperature === null || !chartRef.current) {
      setPosition(null);
      return;
    }

    const updatePosition = () => {
      const chartArea = chartRef.current?.querySelector('.recharts-cartesian-grid');
      if (!chartArea) return;

      const rect = chartArea.getBoundingClientRect();
      const containerRect = chartRef.current!.querySelector('.recharts-responsive-container')?.getBoundingClientRect();
      if (!containerRect) return;

      const chartWidth = rect.width;
      const chartLeft = rect.left - containerRect.left;
      
      // 计算温度对应的 X 位置
      const tempPercent = (temperature - temperatureRange.min) / (temperatureRange.max - temperatureRange.min);
      const x = chartLeft + tempPercent * chartWidth;
      
      setPosition({
        x,
        top: rect.top - containerRect.top,
        height: rect.height
      });
    };

    updatePosition();
    
    // 监听窗口大小变化
    window.addEventListener('resize', updatePosition);
    return () => window.removeEventListener('resize', updatePosition);
  }, [temperature, chartRef, temperatureRange]);

  if (!position || temperature === null) return null;

  return (
    <svg 
      className="absolute inset-0 pointer-events-none overflow-visible"
      style={{ width: '100%', height: '100%' }}
    >
      {/* 虚线 */}
      <line
        x1={position.x}
        y1={position.top}
        x2={position.x}
        y2={position.top + position.height}
        stroke="#ef4444"
        strokeWidth={2}
        strokeDasharray="5 5"
      />
      {/* 标签背景 */}
      <rect
        x={position.x - 45}
        y={position.top - 22}
        width={90}
        height={20}
        rx={4}
        fill="#ef4444"
      />
      {/* 标签文字 */}
      <text
        x={position.x}
        y={position.top - 8}
        textAnchor="middle"
        fill="white"
        fontSize={11}
        fontWeight={500}
      >
        当前 {temperature}°C
      </text>
    </svg>
  );
});

// 独立的实时状态显示组件 - 不会触发父组件重绘
const RealtimeStatus = memo(function RealtimeStatus({
  temperature,
  fanData,
}: {
  temperature: types.TemperatureData | null;
  fanData: types.FanData | null;
}) {
  return (
    <div className="flex items-center gap-6 text-sm">
      <div className="flex items-center gap-2">
        <span className="text-gray-500 dark:text-gray-400">温度</span>
        <span className={clsx(
          'font-semibold text-lg tabular-nums',
          (temperature?.maxTemp ?? 0) > 80 ? 'text-red-500' :
          (temperature?.maxTemp ?? 0) > 70 ? 'text-yellow-500' : 'text-green-500'
        )}>
          {temperature?.maxTemp ?? '--'}°C
        </span>
      </div>
      <div className="flex items-center gap-2">
        <span className="text-gray-500 dark:text-gray-400">转速</span>
        <span className="font-semibold text-lg text-blue-600 dark:text-blue-400 tabular-nums">
          {fanData?.currentRpm ?? '--'} RPM
        </span>
      </div>
    </div>
  );
});

// 自定义可拖拽的点组件
const DraggablePoint = memo(function DraggablePoint({
  cx,
  cy,
  index,
  rpm,
  onDragStart,
  isActive,
}: {
  cx: number;
  cy: number;
  index: number;
  temperature: number;
  rpm: number;
  onDragStart: (index: number) => void;
  isActive: boolean;
}) {
  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    onDragStart(index);
  }, [index, onDragStart]);

  const handleTouchStart = useCallback((e: React.TouchEvent) => {
    e.preventDefault();
    e.stopPropagation();
    onDragStart(index);
  }, [index, onDragStart]);

  return (
    <g>
      {/* 外圈 - 交互区域 */}
      <circle
        cx={cx}
        cy={cy}
        r={isActive ? 14 : 10}
        fill="transparent"
        stroke="transparent"
        style={{ cursor: 'ns-resize' }}
        onMouseDown={handleMouseDown}
        onTouchStart={handleTouchStart}
      />
      {/* 主点 */}
      <circle
        cx={cx}
        cy={cy}
        r={isActive ? 8 : 6}
        fill={isActive ? '#1d4ed8' : '#3b82f6'}
        stroke="white"
        strokeWidth={2}
        style={{ 
          cursor: 'ns-resize',
          transition: isActive ? 'none' : 'all 0.2s ease',
          filter: isActive ? 'drop-shadow(0 4px 8px rgba(59, 130, 246, 0.5))' : 'drop-shadow(0 2px 4px rgba(0, 0, 0, 0.1))'
        }}
        onMouseDown={handleMouseDown}
        onTouchStart={handleTouchStart}
      />
      {/* 活动状态时显示数值 */}
      {isActive && (
        <g>
          <rect
            x={cx - 35}
            y={cy - 35}
            width={70}
            height={24}
            rx={4}
            fill="#1e40af"
            opacity={0.95}
          />
          <text
            x={cx}
            y={cy - 19}
            textAnchor="middle"
            fill="white"
            fontSize={12}
            fontWeight={600}
          >
            {rpm} RPM
          </text>
        </g>
      )}
    </g>
  );
});

const FanCurve = memo(function FanCurve({ config, onConfigChange, isConnected, fanData, temperature }: FanCurveProps) {
  // 本地编辑状态 - 完全独立于外部配置
  const [localCurve, setLocalCurve] = useState<types.FanCurvePoint[]>([]);
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);
  const [isInitialized, setIsInitialized] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isDarkMode, setIsDarkMode] = useState(false);
  
  // 拖拽状态
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [isInteracting, setIsInteracting] = useState(false);
  
  // Chart ref for coordinate calculations
  const chartRef = useRef<HTMLDivElement>(null);
  const chartBoundsRef = useRef<{ top: number; bottom: number; left: number; right: number; yMin: number; yMax: number } | null>(null);

  // 静态 RPM 范围 - 仅在首次初始化时计算
  const [rpmRange, setRpmRange] = useState({ min: 1000, max: 4000, ticks: [1000, 1500, 2000, 2500, 3000, 3500, 4000] });
  
  // 温度范围
  const temperatureRange = useMemo(() => ({
    min: 30,
    max: 95,
    ticks: Array.from({ length: 14 }, (_, i) => 30 + i * 5)
  }), []);

  const smartControl = useMemo(() => {
    const curveLength = config.fanCurve?.length || localCurve.length || 14;
    const defaultOffsets = Array.from({ length: curveLength }, () => 0);
    const existing = config.smartControl;

    if (!existing) {
      return {
        enabled: true,
        learning: true,
        targetTemp: 68,
        aggressiveness: 5,
        hysteresis: 2,
        minRpmChange: 50,
        rampUpLimit: 220,
        rampDownLimit: 160,
        learnRate: 4,
        maxLearnOffset: 600,
        learnedOffsets: defaultOffsets,
      };
    }

    return {
      ...existing,
      learnedOffsets: Array.isArray(existing.learnedOffsets)
        ? [...existing.learnedOffsets.slice(0, curveLength), ...defaultOffsets].slice(0, curveLength)
        : defaultOffsets,
    };
  }, [config.fanCurve, config.smartControl, localCurve.length]);

  const learningInsight = useMemo(() => {
    const offsets = smartControl.learnedOffsets || [];
    const points = (config.fanCurve && config.fanCurve.length > 0 ? config.fanCurve : localCurve)
      .map((point, index) => ({
        index,
        temperature: point.temperature,
        offset: offsets[index] ?? 0,
      }));

    if (points.length === 0) {
      return {
        currentOffset: 0,
        currentTempLabel: '--',
        maxAbsOffset: 0,
        avgAbsOffset: 0,
        significantPoints: [] as Array<{ temperature: number; offset: number }>,
      };
    }

    const maxTemp = temperature?.maxTemp ?? null;
    let currentPoint = points[0];
    if (maxTemp !== null) {
      currentPoint = points.reduce((best, item) => {
        const bestDistance = Math.abs(best.temperature - maxTemp);
        const itemDistance = Math.abs(item.temperature - maxTemp);
        return itemDistance < bestDistance ? item : best;
      }, points[0]);
    }

    const absOffsets = points.map((item) => Math.abs(item.offset));
    const totalAbsOffset = absOffsets.reduce((sum, value) => sum + value, 0);
    const avgAbsOffset = Math.round(totalAbsOffset / points.length);
    const maxAbsOffset = absOffsets.reduce((maxValue, value) => Math.max(maxValue, value), 0);

    const significantPoints = points
      .filter((item) => Math.abs(item.offset) >= 20)
      .sort((a, b) => Math.abs(b.offset) - Math.abs(a.offset))
      .slice(0, 6)
      .map((item) => ({ temperature: item.temperature, offset: item.offset }));

    return {
      currentOffset: currentPoint.offset,
      currentTempLabel: `${currentPoint.temperature}°C`,
      maxAbsOffset,
      avgAbsOffset,
      significantPoints,
    };
  }, [config.fanCurve, localCurve, smartControl.learnedOffsets, temperature?.maxTemp]);

  useEffect(() => {
    const root = document.documentElement;

    const updateTheme = () => {
      setIsDarkMode(root.classList.contains('dark'));
    };

    updateTheme();

    const observer = new MutationObserver(updateTheme);
    observer.observe(root, { attributes: true, attributeFilter: ['class'] });

    return () => observer.disconnect();
  }, []);

  // 仅在组件首次加载时初始化
  useEffect(() => {
    if (!isInitialized && config.fanCurve && config.fanCurve.length > 0) {
      setLocalCurve([...config.fanCurve]);
      setIsInitialized(true);
      
      // 初始化 RPM 范围
      if (fanData?.maxGear) {
        let maxRpm = 4000;
        switch (fanData.maxGear) {
          case '标准': maxRpm = 2760; break;
          case '强劲': maxRpm = 3300; break;
          case '超频': maxRpm = 4000; break;
        }
        const step = 500;
        const ticks = Array.from({ length: Math.floor((maxRpm - 1000) / step) + 1 }, (_, i) => 1000 + i * step);
        setRpmRange({ min: 1000, max: maxRpm, ticks });
      }
    }
  }, [config.fanCurve, fanData?.maxGear, isInitialized]);

  // 图表数据 - 使用本地状态（含耦合后曲线）
  const chartData = useMemo(() => {
    const offsets = smartControl.learnedOffsets || [];
    return localCurve.map((point, index) => {
      const offset = offsets[index] ?? 0;
      return {
        temperature: point.temperature,
        rpm: point.rpm,
        coupledRpm: Math.max(rpmRange.min, Math.min(rpmRange.max, point.rpm + offset)),
        index,
      };
    });
  }, [localCurve, smartControl.learnedOffsets, rpmRange.max, rpmRange.min]);

  const showCoupledCurve = config.autoControl && smartControl.enabled;

  // 更新单个点
  const updatePoint = useCallback((index: number, newRpm: number) => {
    const clampedRpm = Math.max(rpmRange.min, Math.min(rpmRange.max, Math.round(newRpm / 50) * 50));
    
    setLocalCurve(prev => {
      if (prev[index]?.rpm === clampedRpm) return prev;
      const newCurve = [...prev];
      newCurve[index] = { ...newCurve[index], rpm: clampedRpm };
      return newCurve;
    });
    setHasUnsavedChanges(true);
  }, [rpmRange]);

  // 拖拽处理
  const handleDragStart = useCallback((index: number) => {
    setDragIndex(index);
    setIsInteracting(true);
    
    // 计算图表边界
    if (chartRef.current) {
      const chartArea = chartRef.current.querySelector('.recharts-cartesian-grid');
      if (chartArea) {
        const rect = chartArea.getBoundingClientRect();
        chartBoundsRef.current = {
          top: rect.top,
          bottom: rect.bottom,
          left: rect.left,
          right: rect.right,
          yMin: rpmRange.min,
          yMax: rpmRange.max
        };
      }
    }
  }, [rpmRange]);

  const handleDrag = useCallback((clientY: number) => {
    if (dragIndex === null || !chartBoundsRef.current) return;
    
    const bounds = chartBoundsRef.current;
    const chartHeight = bounds.bottom - bounds.top;
    const relativeY = Math.max(0, Math.min(1, (bounds.bottom - clientY) / chartHeight));
    const newRpm = bounds.yMin + relativeY * (bounds.yMax - bounds.yMin);
    
    updatePoint(dragIndex, newRpm);
  }, [dragIndex, updatePoint]);

  const handleDragEnd = useCallback(() => {
    setDragIndex(null);
    setTimeout(() => setIsInteracting(false), 100);
  }, []);

  // 全局拖拽事件监听
  useEffect(() => {
    if (dragIndex === null) return;

    const handleMouseMove = (e: MouseEvent) => {
      e.preventDefault();
      handleDrag(e.clientY);
    };

    const handleTouchMove = (e: TouchEvent) => {
      if (e.touches.length > 0) {
        handleDrag(e.touches[0].clientY);
      }
    };

    const handleEnd = () => handleDragEnd();

    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleEnd);
    document.addEventListener('touchmove', handleTouchMove, { passive: false });
    document.addEventListener('touchend', handleEnd);

    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleEnd);
      document.removeEventListener('touchmove', handleTouchMove);
      document.removeEventListener('touchend', handleEnd);
    };
  }, [dragIndex, handleDrag, handleDragEnd]);

  // 保存曲线
  const saveCurve = useCallback(async () => {
    if (isSaving) return;
    
    try {
      setIsSaving(true);
      await apiService.setFanCurve(localCurve);
      const newConfig = types.AppConfig.createFrom({ ...config, fanCurve: localCurve });
      onConfigChange(newConfig);
      setHasUnsavedChanges(false);
    } catch (error) {
      console.error('保存风扇曲线失败:', error);
    } finally {
      setIsSaving(false);
    }
  }, [localCurve, config, onConfigChange, isSaving]);

  // 重置曲线
  const resetCurve = useCallback(() => {
    const defaultCurve: types.FanCurvePoint[] = [
      { temperature: 30, rpm: 1000 },
      { temperature: 35, rpm: 1200 },
      { temperature: 40, rpm: 1400 },
      { temperature: 45, rpm: 1600 },
      { temperature: 50, rpm: 1800 },
      { temperature: 55, rpm: 2000 },
      { temperature: 60, rpm: Math.min(2300, rpmRange.max) },
      { temperature: 65, rpm: Math.min(2600, rpmRange.max) },
      { temperature: 70, rpm: Math.min(2900, rpmRange.max) },
      { temperature: 75, rpm: Math.min(3200, rpmRange.max) },
      { temperature: 80, rpm: Math.min(3500, rpmRange.max) },
      { temperature: 85, rpm: Math.min(3800, rpmRange.max) },
      { temperature: 90, rpm: rpmRange.max },
      { temperature: 95, rpm: rpmRange.max },
    ];
    
    setLocalCurve(defaultCurve);
    setHasUnsavedChanges(true);
  }, [rpmRange.max]);

  // 智能变频切换
  const handleAutoControlChange = useCallback(async (enabled: boolean) => {
    try {
      await apiService.setAutoControl(enabled);
      const newConfig = types.AppConfig.createFrom({ ...config, autoControl: enabled });
      onConfigChange(newConfig);
    } catch (error) {
      console.error('设置智能变频失败:', error);
    }
  }, [config, onConfigChange]);

  // 智能学习控温配置
  const updateSmartControl = useCallback(async (patch: Partial<typeof smartControl>) => {
    try {
      const mergedSmartControl = {
        ...smartControl,
        ...patch,
      };

      const newConfig = types.AppConfig.createFrom({
        ...config,
        smartControl: mergedSmartControl,
      });

      onConfigChange(newConfig);
    } catch (error) {
      console.error('更新智能控温配置失败:', error);
    }
  }, [config, onConfigChange, smartControl]);

  const resetLearning = useCallback(() => {
    const resetOffsets = Array.from({ length: localCurve.length || config.fanCurve.length || 14 }, () => 0);
    updateSmartControl({ learnedOffsets: resetOffsets });
  }, [config.fanCurve.length, localCurve.length, updateSmartControl]);

  // 手动挡位选项
  const gearOptions = [
    { value: '静音', label: '静音', description: '低噪音模式' },
    { value: '标准', label: '标准', description: '平衡模式' },
    { value: '强劲', label: '强劲', description: '高性能模式' },
    { value: '超频', label: '超频', description: '极限模式' },
  ];

  const levelOptions = [
    { value: '低', label: '低' },
    { value: '中', label: '中' },
    { value: '高', label: '高' },
  ];

  // 手动挡位切换
  const handleGearChange = useCallback(async (gear: string) => {
    try {
      await apiService.setManualGear(gear, config.manualLevel || '中');
      const newConfig = types.AppConfig.createFrom({ ...config, manualGear: gear });
      onConfigChange(newConfig);
    } catch (error) {
      console.error('设置手动挡位失败:', error);
    }
  }, [config, onConfigChange]);

  const handleLevelChange = useCallback(async (level: string) => {
    try {
      await apiService.setManualGear(config.manualGear || '标准', level);
      const newConfig = types.AppConfig.createFrom({ ...config, manualLevel: level });
      onConfigChange(newConfig);
    } catch (error) {
      console.error('设置挡位级别失败:', error);
    }
  }, [config, onConfigChange]);

  // 自定义点渲染
  const CustomDot = useCallback((props: any): React.ReactElement<SVGElement> => {
    const { cx, cy, index, payload } = props;
    // 如果坐标无效，返回一个空的 g 元素而不是 null
    if (cx === undefined || cy === undefined) {
      return <g />;
    }
    
    return (
      <DraggablePoint
        key={`dot-${index}`}
        cx={cx}
        cy={cy}
        index={index}
        temperature={payload.temperature}
        rpm={payload.rpm}
        onDragStart={handleDragStart}
        isActive={dragIndex === index}
      />
    );
  }, [dragIndex, handleDragStart]);

  return (
    <Card className="p-6">
      {/* 头部 */}
      <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4 mb-6">
        <div className="flex items-center gap-3">
          <h2 className="text-xl font-bold text-gray-900 dark:text-white">
            风扇曲线设置
          </h2>
          {hasUnsavedChanges && (
            <Badge variant="warning">未保存</Badge>
          )}
          {isInteracting && (
            <Badge variant="info">编辑中</Badge>
          )}
        </div>

        {/* 实时状态 - 使用独立组件避免触发父组件重绘 */}
        <div className="flex flex-wrap items-center gap-4">
          <RealtimeStatus temperature={temperature} fanData={fanData} />

          {/* 智能变频开关 */}
          <div className="flex items-center gap-3 pl-4 border-l border-gray-200 dark:border-gray-700">
            <ToggleSwitch
              enabled={config.autoControl}
              onChange={handleAutoControlChange}
              label="智能变频"
              color="blue"
            />
          </div>
        </div>

        {/* 操作按钮 */}
        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            size="sm"
            onClick={resetCurve}
            icon={<RotateCw className="w-4 h-4" />}
          >
            重置
          </Button>
          <Button
            variant="primary"
            size="sm"
            onClick={saveCurve}
            disabled={!hasUnsavedChanges}
            loading={isSaving}
            icon={<Check className="w-4 h-4" />}
          >
            保存
          </Button>
        </div>
      </div>

      {/* 手动挡位控制（仅在关闭智能变频时显示） */}
      {!config.autoControl && isConnected && (
        <div className="mb-6 p-4 bg-gray-50 dark:bg-gray-700/50 rounded-xl border border-gray-200 dark:border-gray-600">
          <div className="flex flex-wrap items-center gap-4">
            <span className="text-sm font-medium text-gray-700 dark:text-gray-300">手动挡位</span>
            <div className="flex items-center gap-3">
              <Select
                value={config.manualGear || '标准'}
                onChange={handleGearChange}
                options={gearOptions}
                size="sm"
              />
              <Select
                value={config.manualLevel || '中'}
                onChange={handleLevelChange}
                options={levelOptions}
                size="sm"
              />
            </div>
          </div>
        </div>
      )}

      {/* 智能控温学习（仅在开启智能变频时显示） */}
      {config.autoControl && isConnected && (
        <div className="mb-6 p-4 bg-gray-50 dark:bg-gray-700/50 rounded-xl border border-gray-200 dark:border-gray-600 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <span className="text-sm font-medium text-gray-700 dark:text-gray-300">智能学习控温</span>
            <div className="flex items-center gap-4">
              <ToggleSwitch
                enabled={smartControl.enabled}
                onChange={(enabled) => updateSmartControl({ enabled })}
                label="智能耦合"
                color="blue"
              />
              <ToggleSwitch
                enabled={smartControl.learning}
                onChange={(learning) => updateSmartControl({ learning })}
                label="学习"
                color="green"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="space-y-2">
              <div className="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
                <span>目标温度</span>
                <span>{smartControl.targetTemp}°C</span>
              </div>
              <Slider
                value={smartControl.targetTemp}
                onChange={(targetTemp) => updateSmartControl({ targetTemp })}
                min={55}
                max={85}
                step={1}
                showValue={false}
              />
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
                <span>响应强度</span>
                <span>{smartControl.aggressiveness}</span>
              </div>
              <Slider
                value={smartControl.aggressiveness}
                onChange={(aggressiveness) => updateSmartControl({ aggressiveness })}
                min={1}
                max={10}
                step={1}
                showValue={false}
              />
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
                <span>学习速率</span>
                <span>{smartControl.learnRate}</span>
              </div>
              <Slider
                value={smartControl.learnRate}
                onChange={(learnRate) => updateSmartControl({ learnRate })}
                min={1}
                max={10}
                step={1}
                showValue={false}
              />
            </div>
          </div>

          <div className="flex items-center justify-end">
            <Button
              variant="secondary"
              size="sm"
              onClick={resetLearning}
            >
              重置学习结果
            </Button>
          </div>

          <div className="rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 p-3 space-y-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <span className="text-xs font-medium text-gray-500 dark:text-gray-400">学习状态可视化</span>
              <span className="text-xs text-gray-500 dark:text-gray-400">
                当前温区 {learningInsight.currentTempLabel}
              </span>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <div className="rounded-md bg-gray-50 dark:bg-gray-700/50 px-3 py-2">
                <div className="text-[11px] text-gray-500 dark:text-gray-400">当前学习偏移</div>
                <div className={clsx(
                  'text-sm font-semibold',
                  learningInsight.currentOffset > 0
                    ? 'text-amber-600 dark:text-amber-400'
                    : learningInsight.currentOffset < 0
                    ? 'text-cyan-600 dark:text-cyan-400'
                    : 'text-gray-700 dark:text-gray-200'
                )}>
                  {learningInsight.currentOffset > 0 ? '+' : ''}{learningInsight.currentOffset} RPM
                </div>
              </div>

              <div className="rounded-md bg-gray-50 dark:bg-gray-700/50 px-3 py-2">
                <div className="text-[11px] text-gray-500 dark:text-gray-400">平均学习强度</div>
                <div className="text-sm font-semibold text-gray-700 dark:text-gray-200">
                  {learningInsight.avgAbsOffset} RPM
                </div>
              </div>

              <div className="rounded-md bg-gray-50 dark:bg-gray-700/50 px-3 py-2">
                <div className="text-[11px] text-gray-500 dark:text-gray-400">最大学习偏移</div>
                <div className="text-sm font-semibold text-gray-700 dark:text-gray-200">
                  {learningInsight.maxAbsOffset} RPM
                </div>
              </div>
            </div>

            {learningInsight.significantPoints.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {learningInsight.significantPoints.map((item) => (
                  <span
                    key={`learn-offset-${item.temperature}`}
                    className={clsx(
                      'text-[11px] px-2.5 py-1 rounded-full border',
                      item.offset > 0
                        ? 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300'
                        : 'border-cyan-200 bg-cyan-50 text-cyan-700 dark:border-cyan-800 dark:bg-cyan-900/20 dark:text-cyan-300'
                    )}
                  >
                    {item.temperature}°C {item.offset > 0 ? '+' : ''}{item.offset} RPM
                  </span>
                ))}
              </div>
            ) : (
              <div className="text-xs text-gray-500 dark:text-gray-400">暂未形成显著学习偏移（|偏移| &lt; 20 RPM）。</div>
            )}
          </div>
        </div>
      )}

      {/* 图表区域 */}
      <div 
        ref={chartRef}
        className={clsx(
          'relative rounded-xl border bg-white dark:bg-gray-800 p-4 mb-6',
          'border-gray-200 dark:border-gray-700',
          dragIndex !== null && 'ring-2 ring-blue-500 ring-opacity-50'
        )}
      >
        <div className="h-80 md:h-96 relative">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart 
              data={chartData} 
              margin={{ top: 20, right: 30, left: 20, bottom: 20 }}
            >
              <CartesianGrid 
                strokeDasharray="3 3" 
                stroke="#e5e7eb"
                className="dark:stroke-gray-600"
              />
              
              <XAxis 
                dataKey="temperature" 
                type="number"
                domain={[temperatureRange.min, temperatureRange.max]}
                ticks={temperatureRange.ticks}
                tickLine={false}
                axisLine={{ stroke: '#d1d5db' }}
                tick={{ fill: '#6b7280', fontSize: 11 }}
                allowDataOverflow={true}
                label={{ 
                  value: '温度 (°C)', 
                  position: 'insideBottom', 
                  offset: -10,
                  fill: '#6b7280',
                  fontSize: 12
                }}
              />
              <YAxis 
                type="number"
                domain={[rpmRange.min, rpmRange.max]}
                ticks={rpmRange.ticks}
                tickLine={false}
                axisLine={{ stroke: '#d1d5db' }}
                tick={{ fill: '#6b7280', fontSize: 11 }}
                allowDataOverflow={true}
                label={{ 
                  value: '转速 (RPM)', 
                  angle: -90, 
                  position: 'insideLeft',
                  fill: '#6b7280',
                  fontSize: 12
                }}
              />
              <Tooltip 
                formatter={(value: number, name: string) => {
                  if (name === 'coupledRpm') {
                    return [`${value} RPM`, '耦合后曲线'];
                  }
                  return [`${value} RPM`, '基础曲线'];
                }}
                labelFormatter={(value) => `温度: ${value}°C`}
                contentStyle={{
                  backgroundColor: isDarkMode ? 'rgba(17, 24, 39, 0.95)' : 'rgba(255, 255, 255, 0.95)',
                  border: '1px solid',
                  borderColor: isDarkMode ? '#374151' : 'transparent',
                  borderRadius: '8px',
                  boxShadow: isDarkMode
                    ? '0 10px 25px -5px rgba(0, 0, 0, 0.6)'
                    : '0 10px 25px -5px rgba(0, 0, 0, 0.1)',
                  padding: '8px 12px',
                  color: isDarkMode ? '#e5e7eb' : '#111827'
                }}
                labelStyle={{
                  color: isDarkMode ? '#e5e7eb' : '#111827',
                  fontWeight: 600
                }}
                itemStyle={{
                  color: isDarkMode ? '#e5e7eb' : '#111827'
                }}
              />
              <Line 
                type="monotone" 
                dataKey="rpm" 
                stroke="#3b82f6" 
                strokeWidth={3}
                dot={CustomDot}
                activeDot={false}
                isAnimationActive={false}
              />
              {showCoupledCurve && (
                <Line
                  type="monotone"
                  dataKey="coupledRpm"
                  stroke="#3b82f6"
                  strokeWidth={2}
                  strokeDasharray="6 4"
                  dot={false}
                  activeDot={false}
                  isAnimationActive={false}
                />
              )}
            </LineChart>
          </ResponsiveContainer>
          
          {/* 独立的温度指示线覆盖层 - 不触发图表重绘 */}
          <TemperatureIndicator 
            temperature={temperature?.maxTemp ?? null}
            chartRef={chartRef}
            temperatureRange={temperatureRange}
          />
        </div>
      </div>

      {/* 拖拽提示 - 移到图表外部 */}
      <div className="text-center mb-4">
        <div className="flex flex-wrap items-center justify-center gap-2">
          <span className="text-xs text-gray-400 dark:text-gray-500 px-3 py-1.5 rounded-full bg-gray-100 dark:bg-gray-700/50">
            💡 拖拽图表上的蓝色圆点可直接调整转速
          </span>
          {showCoupledCurve && (
            <span className="text-xs text-gray-500 dark:text-gray-400 px-3 py-1.5 rounded-full bg-gray-100 dark:bg-gray-700/50">
              实线：基础曲线 · 虚线：耦合后曲线
            </span>
          )}
        </div>
      </div>

      {/* 控制点网格 */}
      <div className="mb-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">控制点调节</h3>
          <span className="text-xs text-gray-500 dark:text-gray-400">
            转速范围: {rpmRange.min} - {rpmRange.max} RPM
          </span>
        </div>
        
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-7 gap-3">
          {localCurve.map((point, index) => (
            <div 
              key={`control-${point.temperature}`}
              className={clsx(
                'p-3 rounded-lg border transition-all duration-200',
                'bg-gray-50 dark:bg-gray-700/50',
                dragIndex === index 
                  ? 'border-blue-500 ring-2 ring-blue-500/20' 
                  : 'border-gray-200 dark:border-gray-600 hover:border-blue-300 dark:hover:border-blue-500'
              )}
            >
              <div className="text-center mb-2">
                <span className="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {point.temperature}°C
                </span>
              </div>
              
              <input
                type="number"
                value={point.rpm}
                onChange={(e) => updatePoint(index, Number(e.target.value))}
                onFocus={() => setIsInteracting(true)}
                onBlur={() => setTimeout(() => setIsInteracting(false), 100)}
                min={rpmRange.min}
                max={rpmRange.max}
                step={50}
                className={clsx(
                  'w-full px-2 py-1.5 text-center text-sm font-medium rounded-md',
                  'bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-600',
                  'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent',
                  'transition-all duration-200'
                )}
              />
              
              <div className="mt-2">
                <Slider
                  value={point.rpm}
                  onChange={(nextValue) => updatePoint(index, nextValue)}
                  onChangeStart={() => setIsInteracting(true)}
                  onChangeEnd={() => setTimeout(() => setIsInteracting(false), 100)}
                  min={rpmRange.min}
                  max={rpmRange.max}
                  step={50}
                  showValue={false}
                />
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* 说明卡片 */}
      <div className="p-4 rounded-xl bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 border border-blue-200 dark:border-blue-800">
        <div className="flex gap-3">
          <Info className="w-5 h-5 text-blue-600 dark:text-blue-400 flex-shrink-0 mt-0.5" />
          <div className="text-sm text-blue-800 dark:text-blue-200 space-y-2">
            <p className="font-medium">使用说明</p>
            <ul className="space-y-1 text-blue-700 dark:text-blue-300">
              <li>• <strong>拖拽图表点：</strong>直接在图表上拖拽蓝色圆点调整转速</li>
              <li>• <strong>数值输入：</strong>在下方控制点卡片中直接输入精确值</li>
              <li>• <strong>滑块调节：</strong>使用滑块快速微调</li>
              <li>• <strong>保存设置：</strong>修改后点击保存按钮应用更改</li>
            </ul>
            <p className="text-xs text-blue-600 dark:text-blue-400 pt-2 border-t border-blue-200 dark:border-blue-700">
              挡位限制：静音 ≤2000 | 标准 ≤2760 | 强劲 ≤3300 | 超频 ≤4000 RPM
            </p>
          </div>
        </div>
      </div>
    </Card>
  );
});

export default FanCurve;
