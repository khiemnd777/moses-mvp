import { create } from 'zustand';
import { readLocal, writeLocal } from '@/core/utils';

export type DisplayMode = 'dark' | 'light' | 'system';
export type ResolvedDisplayMode = Exclude<DisplayMode, 'system'>;

const STORAGE_KEY = 'legal-display-mode';
const MEDIA_QUERY = '(prefers-color-scheme: dark)';

const isDisplayMode = (value: unknown): value is DisplayMode =>
  value === 'dark' || value === 'light' || value === 'system';

const getSystemDisplayMode = (): ResolvedDisplayMode => {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return 'light';
  }
  return window.matchMedia(MEDIA_QUERY).matches ? 'dark' : 'light';
};

const resolveDisplayMode = (displayMode: DisplayMode): ResolvedDisplayMode =>
  displayMode === 'system' ? getSystemDisplayMode() : displayMode;

const readStoredDisplayMode = (): DisplayMode => {
  if (typeof window === 'undefined') {
    return 'system';
  }
  const value = readLocal<unknown>(STORAGE_KEY, 'system');
  return isDisplayMode(value) ? value : 'system';
};

const applyResolvedDisplayMode = (mode: ResolvedDisplayMode) => {
  if (typeof document === 'undefined') {
    return;
  }
  document.documentElement.dataset.theme = mode;
};

type DisplayModeState = {
  displayMode: DisplayMode;
  resolvedDisplayMode: ResolvedDisplayMode;
  setDisplayMode: (displayMode: DisplayMode) => void;
  syncSystemDisplayMode: () => void;
};

const storedDisplayMode = readStoredDisplayMode();

export const useDisplayModeStore = create<DisplayModeState>((set, get) => ({
  displayMode: storedDisplayMode,
  resolvedDisplayMode: resolveDisplayMode(storedDisplayMode),
  setDisplayMode: (displayMode) => {
    const resolvedDisplayMode = resolveDisplayMode(displayMode);
    writeLocal(STORAGE_KEY, displayMode);
    applyResolvedDisplayMode(resolvedDisplayMode);
    set({ displayMode, resolvedDisplayMode });
  },
  syncSystemDisplayMode: () => {
    const resolvedDisplayMode = resolveDisplayMode(get().displayMode);
    applyResolvedDisplayMode(resolvedDisplayMode);
    set({ resolvedDisplayMode });
  }
}));

let isInitialized = false;

export const initializeDisplayMode = () => {
  if (isInitialized || typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    useDisplayModeStore.getState().syncSystemDisplayMode();
    return;
  }

  isInitialized = true;
  const mediaQuery = window.matchMedia(MEDIA_QUERY);
  const sync = () => useDisplayModeStore.getState().syncSystemDisplayMode();

  sync();

  if (typeof mediaQuery.addEventListener === 'function') {
    mediaQuery.addEventListener('change', sync);
    return;
  }

  mediaQuery.addListener(sync);
};
