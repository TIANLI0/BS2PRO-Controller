'use client';

import type { CSSProperties, ReactNode } from 'react';
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Copy,
  LineChart,
  LayoutGrid,
  Minus,
  Settings2,
  Square,
  TriangleAlert,
  Wifi,
  WifiOff,
  X,
  Fan,
  Thermometer,
  Sparkles,
} from 'lucide-react';
import { Environment, Quit, WindowIsMaximised, WindowMinimise, WindowToggleMaximise } from '../../../wailsjs/runtime/runtime';
import { types } from '../../../wailsjs/go/models';
import clsx from 'clsx';

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

type WailsDragStyle = CSSProperties & { ['--wails-draggable']?: 'drag' | 'no-drag' };

const DRAG_STYLE: WailsDragStyle = { '--wails-draggable': 'drag' };
const NO_DRAG_STYLE: WailsDragStyle = { '--wails-draggable': 'no-drag' };

/* ──────────────────────────────────────────────────────────────
 * TitleBar — slim, fixed at the very top of the window.
 * Outside the scroll viewport, so window controls never scroll.
 * ────────────────────────────────────────────────────────────── */

function TitleBarButton({
  icon,
  label,
  onClick,
  danger = false,
}: {
  icon: ReactNode;
  label: string;
  onClick: () => void;
  danger?: boolean;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      style={NO_DRAG_STYLE}
      onClick={(event) => {
        event.stopPropagation();
        onClick();
      }}
      className={clsx(
        'flex h-8 w-11 cursor-pointer items-center justify-center text-muted-foreground transition-colors',
        danger
          ? 'hover:bg-red-500 hover:text-white'
          : 'hover:bg-foreground/10 hover:text-foreground',
      )}
    >
      {icon}
    </button>
  );
}

function TitleBar({
  isMaximised,
  onMinimise,
  onToggleMaximise,
  onClose,
}: {
  isMaximised: boolean;
  onMinimise: () => void;
  onToggleMaximise: () => void;
  onClose: () => void;
}) {
  return (
    <div
      className="relative z-50 flex h-8 shrink-0 items-center justify-between border-b border-border/60 bg-background/85 backdrop-blur-xl"
      style={DRAG_STYLE}
      onDoubleClick={onToggleMaximise}
    >
      <div className="flex h-full min-w-0 items-center gap-2 pl-3">
        <Fan className="h-3.5 w-3.5 text-primary/80" />
        <span className="truncate text-[12px] font-medium tracking-tight text-foreground/80">
          BS2PRO Controller
        </span>
      </div>

      <div className="flex h-full items-center" style={NO_DRAG_STYLE}>
        <TitleBarButton icon={<Minus className="h-3.5 w-3.5" />} label="最小化" onClick={onMinimise} />
        <TitleBarButton
          icon={isMaximised ? <Copy className="h-3 w-3" /> : <Square className="h-3 w-3" />}
          label={isMaximised ? '还原' : '最大化'}
          onClick={onToggleMaximise}
        />
        <TitleBarButton icon={<X className="h-3.5 w-3.5" />} label="关闭" onClick={onClose} danger />
      </div>
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────
 * OverlayScrollbar — floating thumb, never reserves width.
 * Native scrollbar is hidden via .app-scroll-root--hide-native.
 * ────────────────────────────────────────────────────────────── */

function OverlayScrollbar({ scrollRef }: { scrollRef: React.RefObject<HTMLDivElement | null> }) {
  const trackRef = useRef<HTMLDivElement | null>(null);
  const thumbRef = useRef<HTMLDivElement | null>(null);
  const hideTimerRef = useRef<number | null>(null);
  const draggingRef = useRef<{ startY: number; startScroll: number } | null>(null);
  const [visible, setVisible] = useState(false);
  const [hasOverflow, setHasOverflow] = useState(false);

  const updateThumb = useCallback(() => {
    const el = scrollRef.current;
    const thumb = thumbRef.current;
    const track = trackRef.current;
    if (!el || !thumb || !track) return;

    const { scrollTop, scrollHeight, clientHeight } = el;
    const overflow = scrollHeight - clientHeight;
    if (overflow <= 1) {
      setHasOverflow(false);
      return;
    }
    setHasOverflow(true);

    const trackHeight = track.clientHeight;
    const ratio = clientHeight / scrollHeight;
    const thumbHeight = Math.max(28, trackHeight * ratio);
    const maxThumbTop = trackHeight - thumbHeight;
    const top = (scrollTop / overflow) * maxThumbTop;
    thumb.style.height = `${thumbHeight}px`;
    thumb.style.transform = `translateY(${top}px)`;
  }, [scrollRef]);

  const flashVisible = useCallback(() => {
    setVisible(true);
    if (hideTimerRef.current) {
      window.clearTimeout(hideTimerRef.current);
    }
    hideTimerRef.current = window.setTimeout(() => {
      if (!draggingRef.current) {
        setVisible(false);
      }
    }, 900);
  }, []);

  useLayoutEffect(() => {
    updateThumb();
  });

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;

    const onScroll = () => {
      updateThumb();
      flashVisible();
    };

    el.addEventListener('scroll', onScroll, { passive: true });

    const ro = new ResizeObserver(() => updateThumb());
    ro.observe(el);
    Array.from(el.children).forEach((child) => ro.observe(child));

    const mo = new MutationObserver(() => updateThumb());
    mo.observe(el, { childList: true, subtree: true });

    updateThumb();

    return () => {
      el.removeEventListener('scroll', onScroll);
      ro.disconnect();
      mo.disconnect();
      if (hideTimerRef.current) window.clearTimeout(hideTimerRef.current);
    };
  }, [scrollRef, updateThumb, flashVisible]);

  const handleThumbPointerDown = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      const el = scrollRef.current;
      if (!el) return;
      event.preventDefault();
      (event.target as HTMLElement).setPointerCapture(event.pointerId);
      draggingRef.current = { startY: event.clientY, startScroll: el.scrollTop };
      setVisible(true);
    },
    [scrollRef],
  );

  const handleThumbPointerMove = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      const drag = draggingRef.current;
      const el = scrollRef.current;
      const track = trackRef.current;
      const thumb = thumbRef.current;
      if (!drag || !el || !track || !thumb) return;
      const dy = event.clientY - drag.startY;
      const trackHeight = track.clientHeight;
      const thumbHeight = thumb.clientHeight;
      const maxThumbTop = trackHeight - thumbHeight;
      if (maxThumbTop <= 0) return;
      const overflow = el.scrollHeight - el.clientHeight;
      const scrollDelta = (dy / maxThumbTop) * overflow;
      el.scrollTop = drag.startScroll + scrollDelta;
    },
    [scrollRef],
  );

  const handleThumbPointerUp = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      draggingRef.current = null;
      try {
        (event.target as HTMLElement).releasePointerCapture(event.pointerId);
      } catch {
        /* noop */
      }
      flashVisible();
    },
    [flashVisible],
  );

  if (!hasOverflow) return null;

  return (
    <div
      ref={trackRef}
      className={clsx('app-overlay-scrollbar', visible && 'is-visible')}
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={flashVisible}
    >
      <div
        ref={thumbRef}
        className="app-overlay-scrollbar-thumb"
        onPointerDown={handleThumbPointerDown}
        onPointerMove={handleThumbPointerMove}
        onPointerUp={handleThumbPointerUp}
        onPointerCancel={handleThumbPointerUp}
      />
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────
 * AppShell — layout
 * ────────────────────────────────────────────────────────────── */

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
  const [isWindowsChrome, setIsWindowsChrome] = useState(false);
  const [isMaximised, setIsMaximised] = useState(false);
  const prevTabRef = useRef(activeTab);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const fanSpinDuration = getFanSpinDuration(fanData?.currentRpm);

  const syncWindowState = useCallback(async () => {
    try {
      setIsMaximised(await WindowIsMaximised());
    } catch {
      setIsMaximised(false);
    }
  }, []);

  useEffect(() => {
    let disposed = false;
    let cleanup = () => {};

    const initializeWindowChrome = async () => {
      try {
        const env = await Environment();
        if (disposed) return;
        const isWindows = env.platform === 'windows';
        setIsWindowsChrome(isWindows);
        if (!isWindows) {
          setIsMaximised(false);
          return;
        }
        const handleResize = () => void syncWindowState();
        window.addEventListener('resize', handleResize);
        cleanup = () => window.removeEventListener('resize', handleResize);
        await syncWindowState();
      } catch {
        if (!disposed) {
          setIsWindowsChrome(false);
          setIsMaximised(false);
        }
      }
    };

    void initializeWindowChrome();

    return () => {
      disposed = true;
      cleanup();
    };
  }, [syncWindowState]);

  const scheduleWindowStateSync = useCallback(() => {
    window.setTimeout(() => void syncWindowState(), 80);
  }, [syncWindowState]);

  const handleToggleMaximise = useCallback(() => {
    WindowToggleMaximise();
    scheduleWindowStateSync();
  }, [scheduleWindowStateSync]);

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
    <div className="flex h-dvh w-full flex-col overflow-hidden bg-background text-foreground">
      {/* ── Slim native-style title bar (Windows only) ── */}
      {isWindowsChrome && (
        <TitleBar
          isMaximised={isMaximised}
          onMinimise={() => WindowMinimise()}
          onToggleMaximise={handleToggleMaximise}
          onClose={() => Quit()}
        />
      )}

      {/* ── Scroll viewport — title bar is OUTSIDE this region ── */}
      <div className="relative flex-1 overflow-hidden">
        <div
          ref={scrollRef}
          className="app-scroll-root app-scroll-root--hide-native h-full"
        >
          {/* Sub-header card: tabs + live status pills */}
          <div className="px-5 pt-4">
            <div className="mx-auto max-w-[980px]">
              <div className="overflow-hidden rounded-3xl border border-border/70 bg-card/65 shadow-lg shadow-primary/5 backdrop-blur-2xl">
                <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border/60 px-5 py-3.5">
                  <div className="flex min-w-0 items-center gap-3">
                    <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-primary/15 text-primary ring-1 ring-primary/20">
                      <Fan className="h-5 w-5" />
                    </div>
                    <div className="min-w-0">
                      <h1 className="truncate text-[18px] font-semibold tracking-tight">
                        BS2PRO Controller
                      </h1>
                      <div className="mt-1 flex flex-wrap items-center gap-2 text-xs">
                        <span
                          className={clsx(
                            'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 font-medium leading-none transition-colors',
                            isConnected
                              ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                              : 'bg-muted text-muted-foreground',
                          )}
                        >
                          {isConnected ? <Wifi className="h-3.5 w-3.5" /> : <WifiOff className="h-3.5 w-3.5" />}
                          {isConnected ? '设备已连接' : '设备离线'}
                        </span>
                        <span
                          className={clsx(
                            'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 font-medium',
                            autoControl ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground',
                          )}
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
                          <Thermometer className={clsx('h-4 w-4', getTempColor(temperature?.maxTemp))} />
                          <span className={clsx('font-semibold', getTempColor(temperature?.maxTemp))}>
                            {temperature?.maxTemp ?? '--'}°C
                          </span>
                        </div>
                      </div>
                      <div className="rounded-2xl border border-border/70 bg-background/55 px-3 py-2 backdrop-blur-xl">
                        <div className="flex items-center gap-2">
                          <motion.div
                            animate={fanSpinDuration ? { rotate: 360 } : { rotate: 0 }}
                            transition={
                              fanSpinDuration
                                ? { duration: fanSpinDuration, repeat: Infinity, ease: 'linear' }
                                : undefined
                            }
                          >
                            <Fan className="h-4 w-4 text-primary" />
                          </motion.div>
                          <span className="font-semibold text-primary">
                            {fanData?.currentRpm ?? '--'} RPM
                          </span>
                        </div>
                      </div>
                    </div>
                  )}
                </div>

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
                          className={clsx(
                            'relative flex cursor-pointer items-center justify-center gap-2 rounded-2xl px-3 py-2.5 text-[14px] font-medium transition-colors duration-200',
                            isActive
                              ? 'text-foreground'
                              : 'text-muted-foreground hover:bg-background/55 hover:text-foreground/90',
                          )}
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
          </div>

          {/* Alerts */}
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
                      className="cursor-pointer rounded p-0.5 transition hover:bg-amber-200/60 dark:hover:bg-amber-800/40"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </div>

          {/* Tab content */}
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

        {/* Floating overlay scrollbar — never reserves width */}
        <OverlayScrollbar scrollRef={scrollRef} />
      </div>
    </div>
  );
}
