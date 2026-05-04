'use client';

import { useEffect } from 'react';
import { useAppStore } from '../store/app-store';

type ThemeMode = 'system' | 'light' | 'dark';

function normalizeThemeMode(mode: unknown): ThemeMode {
  if (mode === 'light' || mode === 'dark') return mode;
  return 'system';
}

function applyTheme(mode: ThemeMode) {
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  const isDark = mode === 'dark' || (mode === 'system' && media.matches);
  document.documentElement.classList.toggle('dark', isDark);
}

export default function SystemThemeSync() {
  const themeMode = useAppStore((state) => normalizeThemeMode((state.config as any)?.themeMode));

  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    applyTheme(themeMode);

    const handleChange = (event: MediaQueryListEvent) => {
      if (themeMode !== 'system') {
        return;
      }
      document.documentElement.classList.toggle('dark', event.matches);
    };

    media.addEventListener('change', handleChange);
    return () => media.removeEventListener('change', handleChange);
  }, [themeMode]);

  return null;
}
