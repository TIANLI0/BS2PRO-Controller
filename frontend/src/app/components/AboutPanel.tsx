'use client';

import { useEffect, useMemo, useState } from 'react';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
import { Rocket } from 'lucide-react';
import { BRAND } from '../lib/brand';
import { apiService } from '../services/api';
import { ScrollArea } from './ui/index';

function openUrl(url: string) {
  try {
    BrowserOpenURL(url);
  } catch {
    window.open(url, '_blank', 'noopener,noreferrer');
  }
}

export default function AboutPanel() {
  const [appVersion, setAppVersion] = useState('');
  const [latestReleaseTag, setLatestReleaseTag] = useState('');
  const [latestReleaseUrl, setLatestReleaseUrl] = useState(BRAND.latestReleaseUrl);
  const [latestReleaseBody, setLatestReleaseBody] = useState('');
  const [releaseLoading, setReleaseLoading] = useState(false);
  const [releaseError, setReleaseError] = useState('');

  useEffect(() => {
    let disposed = false;
    apiService.getAppVersion()
      .then((value) => {
        if (!disposed) setAppVersion(value || 'dev');
      })
      .catch(() => {
        if (!disposed) setAppVersion('dev');
      });
    return () => {
      disposed = true;
    };
  }, []);

  useEffect(() => {
    let disposed = false;

    const loadLatestRelease = async () => {
      setReleaseLoading(true);
      setReleaseError('');
      try {
        const response = await fetch('https://api.github.com/repos/TIANLI0/BS2PRO-Controller/releases/latest', {
          headers: { Accept: 'application/vnd.github+json' },
        });
        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }
        const data = await response.json();
        if (disposed) {
          return;
        }
        setLatestReleaseTag(data?.tag_name || '');
        setLatestReleaseUrl(data?.html_url || BRAND.latestReleaseUrl);
        setLatestReleaseBody(data?.body || '');
      } catch {
        if (!disposed) {
          setLatestReleaseTag('');
          setLatestReleaseUrl(BRAND.latestReleaseUrl);
          setLatestReleaseBody('');
          setReleaseError('检查更新失败，请稍后重试。');
        }
      } finally {
        if (!disposed) {
          setReleaseLoading(false);
        }
      }
    };

    void loadLatestRelease();

    return () => {
      disposed = true;
    };
  }, []);

  const hasNewVersion = useMemo(() => {
    const normalize = (value: string) => value.trim().replace(/^v/i, '');
    if (!appVersion || !latestReleaseTag) {
      return false;
    }
    return normalize(appVersion) !== normalize(latestReleaseTag);
  }, [appVersion, latestReleaseTag]);

  return (
    <div className="mx-auto max-w-[760px]">
      <section className="rounded-2xl border border-border bg-card">
        <div className="flex items-center gap-2 border-b border-border/60 px-4 py-3">
          <Rocket className="h-4 w-4 text-muted-foreground" />
          <h3 className="text-sm font-semibold text-foreground">关于与更新</h3>
          <span className="ml-auto text-[11px] text-muted-foreground">{BRAND.name}</span>
        </div>

        <div className="space-y-3 border-b border-border/60 px-4 py-3.5">
          <div className="flex flex-wrap items-center gap-2 rounded-xl border border-border/70 bg-muted/35 px-3 py-3">
            <span className="inline-flex items-center rounded-full border border-border/70 bg-background/70 px-2.5 py-1 text-xs font-medium text-foreground">
              {BRAND.name}
            </span>
            <span className="inline-flex items-center rounded-full border border-border/70 bg-background/70 px-2.5 py-1 text-xs text-muted-foreground">
              {`当前 ${appVersion ? `v${appVersion}` : '--'}`}
            </span>
            <button
              type="button"
              onClick={() => openUrl(latestReleaseUrl || BRAND.latestReleaseUrl)}
              className="inline-flex cursor-pointer items-center gap-1.5 rounded-full border border-primary/40 bg-primary/10 px-2.5 py-1 text-xs font-medium text-primary transition-colors hover:bg-primary/15"
            >
              {`最新 ${releaseLoading ? '检查中…' : latestReleaseTag || '--'}`}
              {hasNewVersion && !releaseLoading && <span className="h-2 w-2 rounded-full bg-destructive" />}
            </button>
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
                <button
                  type="button"
                  onClick={() => openUrl('mailto:wutianli@tianli0.top')}
                  className="cursor-pointer text-foreground transition-colors hover:text-foreground/80"
                >
                  wutianli@tianli0.top
                </button>
              </div>
              <div className="flex items-center justify-between gap-3">
                <span className="text-muted-foreground">反馈群</span>
                <button
                  type="button"
                  onClick={() => openUrl('https://qm.qq.com/q/2lEOycrLjq')}
                  className="inline-flex cursor-pointer items-center rounded-full border border-primary/40 bg-primary/10 px-2.5 py-1 font-medium text-primary transition-colors hover:bg-primary/15"
                >
                  QQ 群入口
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}