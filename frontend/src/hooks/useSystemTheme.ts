import { useEffect, useState, useCallback } from 'react';
import {
  getNextThemeMode,
  getResolvedTheme,
  getStoredThemeMode,
  type ThemeMode,
} from '../../../theme/src/utils/unitCore';

export type { ThemeMode };

export const useSystemTheme = (): 'light' | 'dark' => {
  const [resolvedTheme, setResolvedTheme] = useState<'light' | 'dark'>(() =>
    getResolvedTheme(getStoredThemeMode())
  );

  useEffect(() => {
    const handleUpdate = () => {
      const mode = getStoredThemeMode();
      const resolved = getResolvedTheme(mode);
      setResolvedTheme(resolved);
      document.documentElement.classList.toggle('dark', resolved === 'dark');
    };

    handleUpdate();

    const mediaQuery = window.matchMedia?.('(prefers-color-scheme: dark)');
    const handleMediaChange = () => {
      if (getStoredThemeMode() === 'auto') {
        handleUpdate();
      }
    };

    const handleStorageChange = (e: StorageEvent) => {
      if (e.key === 'themeMode') {
        handleUpdate();
      }
    };

    const handleCustomChange = () => handleUpdate();

    mediaQuery?.addEventListener('change', handleMediaChange);
    window.addEventListener('storage', handleStorageChange);
    window.addEventListener('kml-theme-change', handleCustomChange);

    return () => {
      mediaQuery?.removeEventListener('change', handleMediaChange);
      window.removeEventListener('storage', handleStorageChange);
      window.removeEventListener('kml-theme-change', handleCustomChange);
    };
  }, []);

  return resolvedTheme;
};

export const useThemeMode = () => {
  const [themeMode, setThemeModeState] = useState<ThemeMode>(getStoredThemeMode);
  const resolvedTheme = useSystemTheme();

  useEffect(() => {
    const handleSync = () => {
      setThemeModeState(getStoredThemeMode());
    };
    window.addEventListener('storage', handleSync);
    window.addEventListener('kml-theme-change', handleSync);
    return () => {
      window.removeEventListener('storage', handleSync);
      window.removeEventListener('kml-theme-change', handleSync);
    };
  }, []);

  const setThemeMode = useCallback((mode: ThemeMode) => {
    try {
      localStorage.setItem('themeMode', mode);
      window.dispatchEvent(new Event('kml-theme-change'));
      const resolved = getResolvedTheme(mode);
      document.documentElement.classList.toggle('dark', resolved === 'dark');
      setThemeModeState(mode);
    } catch {
      /* ignore */
    }
  }, []);

  const toggleThemeMode = useCallback(() => {
    setThemeMode(getNextThemeMode(getStoredThemeMode()));
  }, [setThemeMode]);

  return {
    themeMode,
    resolvedTheme,
    setThemeMode,
    toggleThemeMode,
  };
};
