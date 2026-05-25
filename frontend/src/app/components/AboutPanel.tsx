'use client';

import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import { Heart, Mail, MessageCircleMore, RefreshCw, Rocket, Sparkles } from 'lucide-react';
import { BRAND } from '../lib/brand';
import { apiService } from '../services/api';
import { Badge, Button, ScrollArea } from './ui/index';

function openUrl(url: string) {
  try {
    BrowserOpenURL(url);
  } catch {
    window.open(url, '_blank', 'noopener,noreferrer');
  }
}

function isLatestVersion(currentVersion: string, latestVersion: string) {
  const parse = (value: string) => (value.match(/\d+/g) || []).map((item) => Number(item));
  const current = parse(currentVersion);
  const latest = parse(latestVersion);
  const length = Math.max(current.length, latest.length);

  for (let index = 0; index < length; index += 1) {
    const currentPart = current[index] ?? 0;
    const latestPart = latest[index] ?? 0;
    if (latestPart > currentPart) return false;
    if (latestPart < currentPart) return true;
  }

  return true;
}

const SUPPORT_METHODS = [
  {
    label: '支付宝',
    image: '/support/alipay.jpg',
  },
  {
    label: '微信',
    image: '/support/wechat.png',
  },
] as const;

const FAQ_ITEMS = [
  {
    question: '为什么 CPU 温度会显示为 0℃？',
    answer: '当 CPU 温度显示为 0℃ 时，通常表示 PawnIO 通讯未能正常建立。建议重新安装 PawnIO 相关组件，并在安装完成后重启计算机，再重新启动 THRM 进行确认。',
  },
  {
    question: '当前支持哪些设备？',
    answer: '目前支持的设备型号包括飞智 BS1、BS2、BS2PRO、BS3 与 BS3PRO。',
  },
  {
    question: '蓝牙扫描不到设备时应如何处理？',
    answer: '若蓝牙扫描列表中未发现设备，建议长按散热器按键以重置蓝牙广播状态；如仍无法恢复，建议联系官方客服进一步协助处理。',
  },
] as const;

const ABOUT_CARD_CLASS = 'min-w-0 rounded-3xl border border-border/70 bg-card p-5';

export default function AboutPanel() {
  const [appVersion, setAppVersion] = useState('');
  const [latestReleaseTag, setLatestReleaseTag] = useState('');
  const [latestReleaseUrl, setLatestReleaseUrl] = useState(BRAND.latestReleaseUrl);
  const [latestReleaseBody, setLatestReleaseBody] = useState('');
  const [releaseLoading, setReleaseLoading] = useState(false);
  const [releaseError, setReleaseError] = useState('');
  const [isSponsorHovered, setIsSponsorHovered] = useState(false);
  const [isSponsorPinned, setIsSponsorPinned] = useState(false);
  const [sponsorPopupStyle, setSponsorPopupStyle] = useState<{ top: number; left: number; placement: 'top' | 'bottom' } | null>(null);
  const sponsorRef = useRef<HTMLDivElement | null>(null);
  const sponsorPopupRef = useRef<HTMLDivElement | null>(null);
  const sponsorHoverTimerRef = useRef<number | null>(null);

  const checkLatestRelease = useCallback(async () => {
    setReleaseLoading(true);
    setReleaseError('');
    try {
      const response = await fetch(BRAND.latestReleaseApiUrl, {
        headers: { Accept: 'application/vnd.github+json' },
      });
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }
      const data = await response.json();
      setLatestReleaseTag(data?.tag_name || '');
      setLatestReleaseUrl(data?.html_url || BRAND.latestReleaseUrl);
      setLatestReleaseBody(typeof data?.body === 'string' ? data.body.trim() : '');
    } catch {
      setLatestReleaseTag('');
      setLatestReleaseUrl(BRAND.latestReleaseUrl);
      setLatestReleaseBody('');
      setReleaseError('检查更新失败，请稍后重试');
    } finally {
      setReleaseLoading(false);
    }
  }, []);

  useEffect(() => {
    let disposed = false;
    apiService.getAppVersion()
      .then((value) => {
        if (!disposed) setAppVersion(value || '');
      })
      .catch(() => {
        if (!disposed) setAppVersion('');
      });
    return () => {
      disposed = true;
    };
  }, []);

  useEffect(() => {
    void checkLatestRelease();
  }, [checkLatestRelease]);

  const clearSponsorHoverTimer = useCallback(() => {
    if (sponsorHoverTimerRef.current !== null) {
      window.clearTimeout(sponsorHoverTimerRef.current);
      sponsorHoverTimerRef.current = null;
    }
  }, []);

  const handleSponsorHoverEnter = useCallback(() => {
    clearSponsorHoverTimer();
    setIsSponsorHovered(true);
  }, [clearSponsorHoverTimer]);

  const handleSponsorHoverLeave = useCallback(() => {
    clearSponsorHoverTimer();
    sponsorHoverTimerRef.current = window.setTimeout(() => {
      setIsSponsorHovered(false);
      sponsorHoverTimerRef.current = null;
    }, 120);
  }, [clearSponsorHoverTimer]);

  useEffect(() => {
    if (!isSponsorPinned) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) {
        return;
      }
      if (!sponsorRef.current?.contains(target)) {
        setIsSponsorPinned(false);
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsSponsorPinned(false);
      }
    };

    window.addEventListener('pointerdown', handlePointerDown);
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('pointerdown', handlePointerDown);
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [isSponsorPinned]);

  useEffect(() => {
    return () => {
      clearSponsorHoverTimer();
    };
  }, [clearSponsorHoverTimer]);

  const hasNewVersion = useMemo(() => {
    return !!appVersion && !!latestReleaseTag && !isLatestVersion(appVersion, latestReleaseTag);
  }, [appVersion, latestReleaseTag]);

  const isSponsorOpen = isSponsorHovered || isSponsorPinned;

  const updateSponsorPopupPosition = useCallback(() => {
    const trigger = sponsorRef.current;
    const popup = sponsorPopupRef.current;
    if (!trigger || !popup) {
      return;
    }

    const gap = 12;
    const viewportPadding = 16;
    const triggerRect = trigger.getBoundingClientRect();
    const popupRect = popup.getBoundingClientRect();
    const width = popupRect.width || 544;
    const height = popupRect.height || 0;

    const horizontalAnchor = triggerRect.left + (triggerRect.width / 2) - (width * 0.38);
    let left = horizontalAnchor;
    left = Math.max(viewportPadding, Math.min(left, window.innerWidth - width - viewportPadding));

    let top = triggerRect.bottom + gap;
    let placement: 'top' | 'bottom' = 'bottom';

    if (top + height > window.innerHeight - viewportPadding && triggerRect.top - gap - height >= viewportPadding) {
      top = triggerRect.top - height - gap;
      placement = 'top';
    }

    setSponsorPopupStyle({ top, left, placement });
  }, []);

  useLayoutEffect(() => {
    if (!isSponsorOpen) {
      setSponsorPopupStyle(null);
      return;
    }

    const handlePositionChange = () => updateSponsorPopupPosition();
    handlePositionChange();

    window.addEventListener('resize', handlePositionChange);
    window.addEventListener('scroll', handlePositionChange, true);

    let resizeObserver: ResizeObserver | null = null;
    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => handlePositionChange());
      if (sponsorRef.current) {
        resizeObserver.observe(sponsorRef.current);
      }
      if (sponsorPopupRef.current) {
        resizeObserver.observe(sponsorPopupRef.current);
      }
    }

    return () => {
      window.removeEventListener('resize', handlePositionChange);
      window.removeEventListener('scroll', handlePositionChange, true);
      resizeObserver?.disconnect();
    };
  }, [isSponsorOpen, updateSponsorPopupPosition]);

  return (
    <div className="mx-auto max-w-[860px] space-y-4">
      <section className="rounded-[28px] border border-border bg-card">
        <div className="flex items-center gap-2 border-b border-border/60 px-5 py-4">
          <Rocket className="h-4 w-4 text-muted-foreground" />
          <h3 className="text-sm font-semibold text-foreground">关于 THRM</h3>
        </div>

        <div className="grid gap-4 p-5 lg:grid-cols-[minmax(0,1fr)_300px]">
          <div className={`${ABOUT_CARD_CLASS} flex h-full flex-col`}>
            <div className="flex flex-1 flex-col justify-between gap-5">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
                <img src="/brand/appicon.png" alt={`${BRAND.name} 标志`} className="h-20 w-20 shrink-0 object-contain" draggable={false} />

                <div className="min-w-0 flex-1">
                  <div>
                    <img src="/brand/wordmark-light.png" alt={`${BRAND.name} 字标`} className="h-auto w-[220px] object-contain dark:hidden" draggable={false} />
                    <img src="/brand/wordmark-dark.png" alt={`${BRAND.name} 字标`} className="hidden h-auto w-[220px] object-contain dark:block" draggable={false} />
                  </div>

                  <p className="mt-4 max-w-[36rem] text-sm leading-relaxed text-muted-foreground">
                    {`${BRAND.name} 是一款${BRAND.description}。`}
                  </p>
                </div>
              </div>

              <div className="rounded-2xl border border-border/70 bg-background/70 p-4">
                <div className="flex flex-wrap gap-2">
                  <span className="inline-flex items-center rounded-full border border-border/70 bg-background px-3 py-1 text-xs text-muted-foreground">
                    {`当前 ${appVersion ? `v${appVersion}` : '--'}`}
                  </span>
                  <span className="inline-flex items-center rounded-full border border-border/70 bg-background px-3 py-1 text-xs text-muted-foreground">
                    {`最新 ${releaseLoading ? '检查中…' : latestReleaseTag || '--'}`}
                  </span>
                  {hasNewVersion && <Badge variant="warning">可更新</Badge>}
                </div>

                <div className="mt-4 flex flex-wrap gap-2">
                  <Button
                    variant="primary"
                    size="sm"
                    loading={releaseLoading}
                    onClick={() => {
                      void checkLatestRelease();
                    }}
                    icon={<RefreshCw className="h-3.5 w-3.5" />}
                  >
                    {releaseLoading ? '检查中' : '检查更新'}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => openUrl(latestReleaseUrl || BRAND.latestReleaseUrl)}
                    icon={<Rocket className="h-3.5 w-3.5" />}
                  >
                    打开发布页
                  </Button>
                  <div
                    ref={sponsorRef}
                    className="relative"
                    onPointerEnter={handleSponsorHoverEnter}
                    onPointerLeave={handleSponsorHoverLeave}
                  >
                    <Button
                      variant={isSponsorPinned ? 'secondary' : 'outline'}
                      size="sm"
                      icon={<Heart className="h-3.5 w-3.5" />}
                      aria-expanded={isSponsorOpen}
                      aria-pressed={isSponsorPinned}
                      onClick={() => {
                        clearSponsorHoverTimer();
                        setIsSponsorHovered(true);
                        setIsSponsorPinned((value) => !value);
                      }}
                    >
                      赞助
                    </Button>
                  </div>
                </div>

                {releaseError && <div className="mt-3 text-xs text-amber-600 dark:text-amber-300">{releaseError}</div>}
              </div>
            </div>

            {hasNewVersion && (
              <div className="mt-5 border-t border-border/60 pt-5">
                <div className="flex items-center gap-2 text-sm font-medium text-foreground">
                  <Sparkles className="h-4 w-4 text-primary" />
                  <span>{`发现新版本 ${latestReleaseTag}`}</span>
                </div>

                <div className="mt-3 rounded-2xl border border-border/70 bg-background/70 p-3">
                  {latestReleaseBody ? (
                    <ScrollArea className="max-h-52">
                      <p className="whitespace-pre-wrap text-xs leading-relaxed text-foreground/90">{latestReleaseBody}</p>
                    </ScrollArea>
                  ) : (
                    <p className="text-xs text-muted-foreground">暂无日志内容，或本次获取失败。</p>
                  )}
                </div>
              </div>
            )}
          </div>

          <div className={ABOUT_CARD_CLASS}>
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <Rocket className="h-4 w-4 text-primary" />
              <span>开发者与联系</span>
            </div>

            <div className="mt-4 flex items-center gap-3 rounded-2xl border border-border/70 bg-background/70 p-3">
              <img
                src="https://q1.qlogo.cn/g?b=qq&nk=507249007&s=640"
                alt="Tianli 头像"
                className="h-14 w-14 rounded-2xl border border-border object-cover"
                referrerPolicy="no-referrer"
              />
              <div className="min-w-0 flex-1">
                <div className="text-base font-semibold text-foreground">Tianli</div>
                <div className="mt-1 text-sm text-muted-foreground">一个不知名开发者</div>
              </div>
            </div>

            <div className="mt-4 space-y-2">
              <button
                type="button"
                onClick={() => openUrl('mailto:wutianli@tianli0.top')}
                className="flex w-full cursor-pointer items-center justify-between rounded-2xl border border-border/70 bg-background/70 px-3 py-2.5 text-left transition-colors hover:border-primary/30 hover:bg-primary/5"
              >
                <span className="flex items-center gap-2 text-sm text-foreground">
                  <Mail className="h-4 w-4 text-muted-foreground" />
                  邮箱
                </span>
                <span className="text-xs text-muted-foreground">wutianli@tianli0.top</span>
              </button>

              <button
                type="button"
                onClick={() => openUrl('https://qm.qq.com/q/2lEOycrLjq')}
                className="flex w-full cursor-pointer items-center justify-between rounded-2xl border border-border/70 bg-background/70 px-3 py-2.5 text-left transition-colors hover:border-primary/30 hover:bg-primary/5"
              >
                <span className="flex items-center gap-2 text-sm text-foreground">
                  <MessageCircleMore className="h-4 w-4 text-muted-foreground" />
                  反馈群
                </span>
                <span className="text-xs text-muted-foreground">QQ 群入口</span>
              </button>

              <button
                type="button"
                onClick={() => openUrl(BRAND.repositoryUrl)}
                className="flex w-full cursor-pointer items-center justify-between rounded-2xl border border-border/70 bg-background/70 px-3 py-2.5 text-left transition-colors hover:border-primary/30 hover:bg-primary/5"
              >
                <span className="flex items-center gap-2 text-sm text-foreground">
                  <Rocket className="h-4 w-4 text-muted-foreground" />
                  开源仓库
                </span>
                <span className="text-xs text-muted-foreground">GitHub</span>
              </button>
            </div>
          </div>

          <div className={`${ABOUT_CARD_CLASS} lg:col-span-2`}>
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <Sparkles className="h-4 w-4 text-primary" />
              <span>常见问题解答</span>
            </div>

            <div className="mt-4 divide-y divide-border/60 rounded-2xl border border-border/70 bg-background/70">
              {FAQ_ITEMS.map((item) => (
                <div key={item.question} className="px-4 py-3">
                  <div className="text-sm font-medium text-foreground">{item.question}</div>
                  <p className="mt-1.5 text-xs leading-relaxed text-muted-foreground">{item.answer}</p>
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      {isSponsorOpen && typeof document !== 'undefined' && createPortal(
        <div
          ref={sponsorPopupRef}
          onPointerEnter={handleSponsorHoverEnter}
          onPointerLeave={handleSponsorHoverLeave}
          className="fixed z-[80] w-[34rem] max-w-[calc(100vw-2rem)] rounded-3xl border border-border/80 bg-popover/98 p-4 backdrop-blur-xl animate-in fade-in-0 zoom-in-95"
          style={sponsorPopupStyle ? { top: sponsorPopupStyle.top, left: sponsorPopupStyle.left } : { top: 0, left: 0, visibility: 'hidden' }}
        >
          <div className="mb-3 flex items-center justify-between px-1">
            <div className="text-xs font-medium uppercase tracking-[0.12em] text-muted-foreground">赞助支持</div>
            {isSponsorPinned && <Badge variant="info">已固定</Badge>}
          </div>

          <div className="grid grid-cols-2 gap-4">
            {SUPPORT_METHODS.map((item) => (
              <div key={item.label} className="rounded-2xl border border-border/70 bg-background/80 p-3">
                <img
                  src={item.image}
                  alt={`${item.label} 二维码`}
                  className="aspect-square w-full rounded-xl object-contain"
                  draggable={false}
                />
                <div className="mt-3 text-center text-sm font-medium text-foreground">{item.label}</div>
              </div>
            ))}
          </div>
        </div>,
        document.body,
      )}
    </div>
  );
}