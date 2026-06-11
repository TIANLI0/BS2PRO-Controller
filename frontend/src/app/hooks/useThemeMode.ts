import { useCallback, useEffect, useState } from 'react';

type ThemeMode = 'light' | 'dark';

const STORAGE_KEY = 'bs2pro-theme-mode';

function applyTheme(mode: ThemeMode) {
  document.documentElement.classList.toggle('dark', mode === 'dark');
  // 同步 Windows DWM 沉浸式深色模式属性，使 Mica 材质的明暗与应用内主题保持一致
  void import('../../../wailsjs/runtime/runtime')
    .then(({ WindowSetDarkTheme, WindowSetLightTheme }) => {
      if (mode === 'dark') {
        WindowSetDarkTheme();
      } else {
        WindowSetLightTheme();
      }
    })
    .catch(() => {});
}

export function useThemeMode() {
  const [theme, setTheme] = useState<ThemeMode>('dark');

  useEffect(() => {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    if (saved === 'light' || saved === 'dark') {
      setTheme(saved);
      applyTheme(saved);
      return;
    }

    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    const initialTheme: ThemeMode = prefersDark ? 'dark' : 'light';
    setTheme(initialTheme);
    applyTheme(initialTheme);
  }, []);

  const toggleTheme = useCallback(() => {
    setTheme((prevTheme) => {
      const nextTheme: ThemeMode = prevTheme === 'dark' ? 'light' : 'dark';
      window.localStorage.setItem(STORAGE_KEY, nextTheme);
      applyTheme(nextTheme);
      return nextTheme;
    });
  }, []);

  return { theme, toggleTheme };
}
