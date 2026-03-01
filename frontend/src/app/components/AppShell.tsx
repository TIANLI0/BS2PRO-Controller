'use client';

import type { ReactNode } from 'react';
import { useRef, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  LineChart,
  LayoutGrid,
  Settings2,
  TriangleAlert,
  Wifi,
  WifiOff,
  X,
  Fan,
  Thermometer,
  Sparkles,
} from 'lucide-react';
import { types } from '../../../wailsjs/go/models';

const TAB_ITEMS = [
  { id: 'status', title: '状态', icon: LayoutGrid },
  { id: 'curve', title: '曲线', icon: LineChart },
  { id: 'control', title: '设置', icon: Settings2 },
] as const;

type ActiveTab = (typeof TAB_ITEMS)[number]['id'];

interface AppShellProps {
  activeTab: ActiveTab;
  onTabChange: (tab: ActiveTab) => void;
  isConnected: boolean;
  fanData: types.FanData | null;
  temperature: types.TemperatureData | null;
  autoControl: boolean;
  error: string | null;
  bridgeWarning: string | null;
  onDismissBridgeWarning: () => void;
  statusContent: ReactNode;
  curveContent: ReactNode;
  controlContent: ReactNode;
}

function getTempColor(temp?: number) {
  if (!temp) return 'text-muted-foreground';
  if (temp > 80) return 'text-red-500';
  if (temp > 70) return 'text-amber-500';
  return 'text-emerald-500';
}

function getFanSpinDuration(rpm?: number) {
  if (!rpm || rpm <= 0) return 0;
  if (rpm >= 4200) return 0.48;
  if (rpm >= 3200) return 0.72;
  if (rpm >= 2200) return 1;
  return 1.35;
}

const slideVariants = {
  initial: (dir: number) => ({ x: dir * 50, opacity: 0 }),
  animate: { x: 0, opacity: 1 },
  exit: (dir: number) => ({ x: dir * -50, opacity: 0 }),
};

export default function AppShell({
  activeTab,
  onTabChange,
  isConnected,
  fanData,
  temperature,
  autoControl,
  error,
  bridgeWarning,
  onDismissBridgeWarning,
  statusContent,
  curveContent,
  controlContent,
}: AppShellProps) {
  const [direction, setDirection] = useState(0);
  const prevTabRef = useRef(activeTab);
  const fanSpinDuration = getFanSpinDuration(fanData?.currentRpm);

  const handleTabChange = (tab: ActiveTab) => {
    if (tab === activeTab) return;
    const curIdx = TAB_ITEMS.findIndex((t) => t.id === activeTab);
    const newIdx = TAB_ITEMS.findIndex((t) => t.id === tab);
    setDirection(newIdx > curIdx ? 1 : -1);
    prevTabRef.current = activeTab;
    onTabChange(tab);
  };

  const contentMap: Record<ActiveTab, ReactNode> = {
    status: statusContent,
    curve: curveContent,
    control: controlContent,
  };

  return (
    <div className="relative min-h-screen overflow-x-hidden bg-background text-foreground">
      {/* ── Sticky Header ── */}
      <header className="sticky top-0 z-40 px-5 pb-2 pt-4">
        <div className="mx-auto max-w-[980px]">
          <div className="relative overflow-hidden rounded-3xl border border-border/70 bg-card/65 shadow-lg shadow-primary/5 backdrop-blur-2xl">
            {/* Title row */}
            <div className="relative flex items-center justify-between border-b border-border/60 px-5 py-4">
              <div className="flex min-w-0 items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-primary/15 text-primary ring-1 ring-primary/20">
                  <Fan className="h-5 w-5" />
                </div>
                <div className="min-w-0">
                  <h1 className="truncate text-[19px] font-semibold tracking-tight">BS2PRO Controller</h1>
                  <div className="mt-1 flex flex-wrap items-center gap-2 text-xs">
                    <span
                      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 font-medium leading-none transition-colors ${
                        isConnected
                          ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                          : 'bg-muted text-muted-foreground'
                      }`}
                    >
                      {isConnected ? <Wifi className="h-3.5 w-3.5" /> : <WifiOff className="h-3.5 w-3.5" />}
                      {isConnected ? '设备已连接' : '设备离线'}
                    </span>
                    <span
                      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 font-medium ${
                        autoControl
                          ? 'bg-primary/10 text-primary'
                          : 'bg-muted text-muted-foreground'
                      }`}
                    >
                      <Sparkles className="h-3.5 w-3.5" />
                      {autoControl ? '智能控制开启' : '手动模式'}
                    </span>
                  </div>
                </div>
              </div>

              {isConnected && (
                <div className="flex items-center gap-2.5 text-[15px] tabular-nums">
                  <div className="rounded-2xl border border-border/70 bg-background/55 px-3 py-2 backdrop-blur-xl">
                    <div className="flex items-center gap-2">
                      <Thermometer className={`h-4 w-4 ${getTempColor(temperature?.maxTemp)}`} />
                      <span className={`font-semibold ${getTempColor(temperature?.maxTemp)}`}>
                        {temperature?.maxTemp ?? '--'}°C
                      </span>
                    </div>
                  </div>
                  <div className="rounded-2xl border border-border/70 bg-background/55 px-3 py-2 backdrop-blur-xl">
                    <div className="flex items-center gap-2">
                      <motion.div
                        animate={fanSpinDuration ? { rotate: 360 } : { rotate: 0 }}
                        transition={fanSpinDuration ? { duration: fanSpinDuration, repeat: Infinity, ease: 'linear' } : undefined}
                      >
                        <Fan className="h-4 w-4 text-primary" />
                      </motion.div>
                      <span className="font-semibold text-primary">{fanData?.currentRpm ?? '--'} RPM</span>
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* Tab bar */}
            <div className="px-3 py-2.5">
              <nav className="grid grid-cols-3 gap-2" role="tablist">
                {TAB_ITEMS.map((tab) => {
                  const Icon = tab.icon;
                  const isActive = activeTab === tab.id;
                  return (
                    <button
                      key={tab.id}
                      role="tab"
                      onClick={() => handleTabChange(tab.id)}
                      className={`relative flex items-center justify-center gap-2 rounded-2xl px-3 py-2.5 text-[14px] font-medium transition-colors duration-200 ${
                        isActive
                          ? 'text-foreground'
                          : 'text-muted-foreground hover:bg-background/55 hover:text-foreground/90'
                      }`}
                    >
                      {isActive && (
                        <motion.div
                          layoutId="tab-glass-indicator"
                          className="absolute inset-0 rounded-2xl border border-border/80 bg-background/70 shadow-sm backdrop-blur-xl"
                          transition={{ type: 'spring', stiffness: 420, damping: 34 }}
                        />
                      )}
                      <Icon className="relative z-10 h-4 w-4" />
                      <span className="relative z-10">{tab.title}</span>
                    </button>
                  );
                })}
              </nav>
            </div>
          </div>
        </div>
      </header>

      {/* ── Alerts ── */}
      <div className="mx-auto max-w-[980px] px-5">
        <AnimatePresence>
          {error && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              className="overflow-hidden"
            >
              <div className="mt-3 rounded-xl border border-destructive/30 bg-destructive/5 px-4 py-2.5 text-sm text-destructive">
                {error}
              </div>
            </motion.div>
          )}

          {bridgeWarning && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              className="overflow-hidden"
            >
              <div className="mt-3 flex items-start gap-3 rounded-xl border border-amber-300/50 bg-amber-50/80 px-4 py-2.5 text-amber-800 dark:border-amber-700/40 dark:bg-amber-900/15 dark:text-amber-200">
                <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" />
                <p className="flex-1 text-sm leading-relaxed">{bridgeWarning}</p>
                <button
                  type="button"
                  aria-label="关闭告警"
                  onClick={onDismissBridgeWarning}
                  className="rounded p-0.5 transition hover:bg-amber-200/60 dark:hover:bg-amber-800/40"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>

      {/* ── Tab Content ── */}
      <main className="mx-auto max-w-[980px] px-5 py-6">
        <AnimatePresence mode="wait" custom={direction} initial={false}>
          <motion.div
            key={activeTab}
            custom={direction}
            variants={slideVariants}
            initial="initial"
            animate="animate"
            exit="exit"
            transition={{ duration: 0.22, ease: [0.25, 0.1, 0.25, 1] }}
          >
            {contentMap[activeTab]}
          </motion.div>
        </AnimatePresence>
      </main>
    </div>
  );
}
