'use client';

import { useEffect } from 'react';

function applySystemTheme(isDark: boolean) {
  document.documentElement.classList.toggle('dark', isDark);
}

export default function SystemThemeSync() {
  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    applySystemTheme(media.matches);

    const handleChange = (event: MediaQueryListEvent) => {
      applySystemTheme(event.matches);
    };

    media.addEventListener('change', handleChange);
    return () => media.removeEventListener('change', handleChange);
  }, []);

  return null;
}
