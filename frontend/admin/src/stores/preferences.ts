import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { Preferences } from '@/types';
import { storageKey } from './storage';

export interface BuiltinThemePreset {
  color: string;
  darkPrimaryColor?: string;
  primaryColor?: string;
  type: string;
}

/** Matches frontend-vben/packages/@core/preferences/src/constants.ts order and values. */
export const BUILT_IN_THEME_PRESETS: BuiltinThemePreset[] = [
  { color: 'hsl(212 100% 45%)', type: 'default' },
  { color: 'hsl(245 82% 67%)', type: 'violet' },
  { color: 'hsl(347 77% 60%)', type: 'pink' },
  { color: 'hsl(42 84% 61%)', type: 'yellow' },
  { color: 'hsl(231 98% 65%)', type: 'sky-blue' },
  { color: 'hsl(161 90% 43%)', type: 'green' },
  {
    color: 'hsl(240 5% 26%)',
    darkPrimaryColor: 'hsl(0 0% 98%)',
    primaryColor: 'hsl(240 5.9% 10%)',
    type: 'zinc',
  },
  { color: 'hsl(181 84% 32%)', type: 'deep-green' },
  { color: 'hsl(211 91% 39%)', type: 'deep-blue' },
  { color: 'hsl(18 89% 40%)', type: 'orange' },
  { color: 'hsl(0 75% 42%)', type: 'rose' },
  {
    color: 'hsl(0 0% 25%)',
    darkPrimaryColor: 'hsl(0 0% 98%)',
    primaryColor: 'hsl(240 5.9% 10%)',
    type: 'neutral',
  },
  {
    color: 'hsl(215 25% 27%)',
    darkPrimaryColor: 'hsl(0 0% 98%)',
    primaryColor: 'hsl(240 5.9% 10%)',
    type: 'slate',
  },
  {
    color: 'hsl(217 19% 27%)',
    darkPrimaryColor: 'hsl(0 0% 98%)',
    primaryColor: 'hsl(240 5.9% 10%)',
    type: 'gray',
  },
  { color: '', type: 'custom' },
];

/** Matches vben DEFAULT_TIME_ZONE_OPTIONS values; labels come from i18n. */
export const TIMEZONE_OPTIONS = [
  { value: 'America/New_York' },
  { value: 'Europe/London' },
  { value: 'Asia/Shanghai' },
  { value: 'Asia/Tokyo' },
  { value: 'Asia/Seoul' },
] as const;

export type TimezoneValue = (typeof TIMEZONE_OPTIONS)[number]['value'];

export const defaultPreferences: Preferences = {
  app: {
    authPageLayout: 'panel-center',
    colorGrayMode: false,
    colorWeakMode: false,
    compact: false,
    contentCompact: 'wide',
    contentCompactWidth: 1200,
    contentPadding: 0,
    defaultHomePath: '/analytics',
    dynamicTitle: true,
    enableCheckUpdates: true,
    enableCopyPreferences: true,
    enablePreferences: true,
    enableRefreshToken: true,
    enableStickyPreferencesNavigationBar: true,
    layout: 'sidebar-nav',
    locale: 'zh-CN',
    loginExpiredMode: 'page',
    name: import.meta.env.VITE_APP_TITLE || 'Dragz Admin',
    preferencesButtonPosition: 'auto',
    timezone: 'Asia/Shanghai',
    watermark: false,
    watermarkContent: '',
  },
  breadcrumb: { enable: true, hideOnlyOne: false, showHome: false, showIcon: true, styleType: 'normal' },
  copyright: { companyName: 'Dragz Admin', companySiteLink: '', date: '', enable: true, icp: '', icpLink: '', settingShow: true },
  footer: { enable: false, fixed: false, height: 32 },
  header: { enable: true, height: 50, hidden: false, menuAlign: 'start', mode: 'fixed' },
  navigation: { accordion: true, split: true, styleType: 'rounded' },
  sidebar: {
    autoActivateChild: false,
    collapsed: false,
    collapsedButton: true,
    collapsedShowTitle: false,
    collapseWidth: 60,
    draggable: true,
    enable: true,
    expandOnHover: true,
    extraCollapse: false,
    extraCollapsedWidth: 60,
    fixedButton: true,
    hidden: false,
    mixedWidth: 80,
    width: 224,
  },
  tabbar: {
    draggable: true,
    enable: true,
    height: 38,
    keepAlive: true,
    maxCount: 0,
    middleClickToClose: false,
    persist: true,
    showIcon: true,
    showMaximize: true,
    showMore: true,
    showRefresh: true,
    styleType: 'chrome',
    visitHistory: true,
    wheelable: true,
  },
  theme: {
    builtinType: 'default',
    colorDestructive: 'hsl(348 100% 61%)',
    colorPrimary: 'hsl(212 100% 45%)',
    colorSuccess: 'hsl(144 57% 58%)',
    colorWarning: 'hsl(42 84% 61%)',
    fontSize: 16,
    mode: 'dark',
    radius: '0.5',
    semiDarkHeader: false,
    semiDarkSidebar: false,
    semiDarkSidebarSub: false,
  },
  transition: { enable: true, loading: true, name: 'fade-slide', progress: true },
  // widget positions/order match apps/web-antd/src/preferences.ts overrides
  widget: {
    fullscreen: false, fullscreenButtonPosition: 'none',
    globalSearch: true, globalSearchButtonPosition: 'header',
    languageToggle: true, languageToggleButtonPosition: 'header',
    lockScreen: true, lockScreenButtonPosition: 'user-dropdown',
    logoutButtonPosition: 'user-dropdown',
    notification: true, notificationButtonPosition: 'header',
    order: ['globalSearch', 'preferences', 'themeToggle', 'languageToggle', 'notification', 'lockScreenBtn', 'logoutBtn'],
    refresh: false, refreshButtonPosition: 'none', sidebarToggle: true,
    themeToggle: true, themeToggleButtonPosition: 'header',
    timezone: false, timezoneButtonPosition: 'none',
  },
  shortcut: { enable: true, escape: false, globalSearch: true, lockScreen: true, logout: true, preferences: true },
  custom: { defaultTableSize: 20, enableFormFullscreen: true, reportTitle: '', tenantMode: 'single' },
};

export function mergePreferences(base: Preferences, next: Partial<Preferences>): Preferences {
  const merged = { ...base } as Preferences;
  for (const key of Object.keys(next) as (keyof Preferences)[]) {
    const value = next[key];
    if (value && typeof value === 'object') {
      (merged as unknown as Record<string, unknown>)[key] = { ...base[key], ...value };
    }
  }
  return merged;
}

export function migrateStoredPreferences(value: unknown): Preferences {
  const stored = value && typeof value === 'object'
    ? ((value as { preferences?: Partial<Preferences> }).preferences ?? value as Partial<Preferences>)
    : {};
  const legacy = stored as Partial<Preferences> & { shortcutKeys?: Partial<Preferences['shortcut']> };
  const migrated: Partial<Preferences> = { ...legacy };
  if (!migrated.shortcut && legacy.shortcutKeys) migrated.shortcut = legacy.shortcutKeys as Preferences['shortcut'];
  delete (migrated as { shortcutKeys?: unknown }).shortcutKeys;
  if ((migrated.app as { layout?: string } | undefined)?.layout === 'full-content') {
    migrated.app = { ...migrated.app, layout: 'sidebar-nav' } as Preferences['app'];
  }
  const headerMode = (migrated.header as { mode?: string } | undefined)?.mode;
  if (migrated.header && headerMode !== 'static' && headerMode !== 'fixed' && headerMode != null) {
    migrated.header = { ...migrated.header, mode: 'fixed' };
  }
  return mergePreferences(defaultPreferences, migrated);
}

interface PreferencesStore {
  preferences: Preferences;
  reset: () => void;
  update: (patch: Partial<Preferences>) => void;
}

export const usePreferencesStore = create<PreferencesStore>()(
  persist(
    (set) => ({
      preferences: defaultPreferences,
      reset: () => set({ preferences: defaultPreferences }),
      update: (patch) => set((state) => ({ preferences: mergePreferences(state.preferences, patch) })),
    }),
    {
      name: storageKey('preferences'),
      version: 2,
      migrate: (persisted) => ({ preferences: migrateStoredPreferences(persisted) }),
      // Deep-merge persisted groups over defaults so new fields keep default values.
      merge: (persisted, current) => {
        return {
          ...current,
          preferences: migrateStoredPreferences(persisted),
        };
      },
    },
  ),
);

export function effectiveDark(preferences: Preferences) {
  return (
    preferences.theme.mode === 'dark' ||
    (preferences.theme.mode === 'auto' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  );
}

export function findBuiltinTheme(type: string) {
  return BUILT_IN_THEME_PRESETS.find((item) => item.type === type);
}

export function resolveThemePrimaryColor(preferences: Preferences) {
  const { theme } = preferences;
  if (theme.builtinType === 'custom') {
    return theme.colorPrimary;
  }

  const preset = findBuiltinTheme(theme.builtinType);
  if (!preset) {
    return theme.colorPrimary;
  }

  if (preset.primaryColor || preset.darkPrimaryColor) {
    const dark = effectiveDark(preferences);
    const color = dark ? preset.darkPrimaryColor || preset.primaryColor : preset.primaryColor;
    return color || preset.color || theme.colorPrimary;
  }

  return preset.color || theme.colorPrimary;
}

export function cssColorToHex(color: string) {
  if (!color) return '#0066e6';
  if (color.startsWith('#')) {
    return color.length >= 7 ? color.slice(0, 7) : color;
  }

  const match = color.match(/hsl\(\s*([\d.]+)\s+([\d.]+)%\s+([\d.]+)%\s*\)/i);
  if (!match) return '#0066e6';

  const h = Number(match[1]) / 360;
  const s = Number(match[2]) / 100;
  const l = Number(match[3]) / 100;
  const hue2rgb = (p: number, q: number, t: number) => {
    let value = t;
    if (value < 0) value += 1;
    if (value > 1) value -= 1;
    if (value < 1 / 6) return p + (q - p) * 6 * value;
    if (value < 1 / 2) return q;
    if (value < 2 / 3) return p + (q - p) * (2 / 3 - value) * 6;
    return p;
  };

  let r: number;
  let g: number;
  let b: number;
  if (s === 0) {
    r = g = b = l;
  } else {
    const q = l < 0.5 ? l * (1 + s) : l + s - l * s;
    const p = 2 * l - q;
    r = hue2rgb(p, q, h + 1 / 3);
    g = hue2rgb(p, q, h);
    b = hue2rgb(p, q, h - 1 / 3);
  }

  return `#${[r, g, b]
    .map((value) => Math.round(value * 255).toString(16).padStart(2, '0'))
    .join('')}`;
}
