import { beforeEach, describe, expect, it } from 'vitest';
import {
  BUILT_IN_THEME_PRESETS,
  defaultPreferences,
  effectiveDark,
  resolveThemePrimaryColor,
  migrateStoredPreferences,
  usePreferencesStore,
} from './preferences';

describe('preferences store', () => {
  beforeEach(() => {
    localStorage.clear();
    usePreferencesStore.getState().reset();
  });

  it('has 15 built-in theme presets in vben order and accepts all layouts', () => {
    expect(BUILT_IN_THEME_PRESETS).toHaveLength(15);
    expect(BUILT_IN_THEME_PRESETS.map((item) => item.type)).toEqual([
      'default',
      'violet',
      'pink',
      'yellow',
      'sky-blue',
      'green',
      'zinc',
      'deep-green',
      'deep-blue',
      'orange',
      'rose',
      'neutral',
      'slate',
      'gray',
      'custom',
    ]);

    for (const layout of [
      'sidebar-nav',
      'sidebar-mixed-nav',
      'header-nav',
      'header-mixed-nav',
      'header-sidebar-nav',
      'mixed-nav',
    ] as const) {
      usePreferencesStore.getState().update({ app: { ...defaultPreferences.app, layout } });
      expect(usePreferencesStore.getState().preferences.app.layout).toBe(layout);
    }
  });

  it('uses vben default semantic colors and zero content padding', () => {
    expect(defaultPreferences.theme.colorPrimary).toBe('hsl(212 100% 45%)');
    expect(defaultPreferences.theme.colorSuccess).toBe('hsl(144 57% 58%)');
    expect(defaultPreferences.theme.colorWarning).toBe('hsl(42 84% 61%)');
    expect(defaultPreferences.theme.colorDestructive).toBe('hsl(348 100% 61%)');
    expect(defaultPreferences.app.contentPadding).toBe(0);
  });

  it('defaults sidebar edge drag on and keeps width bounds fields', () => {
    expect(defaultPreferences.sidebar.draggable).toBe(true);
    expect(defaultPreferences.sidebar.width).toBe(224);
    expect(defaultPreferences.sidebar.collapseWidth).toBe(60);
  });

  it('matches web-antd widget positions and order', () => {
    expect(defaultPreferences.widget.order).toEqual([
      'globalSearch',
      'preferences',
      'themeToggle',
      'languageToggle',
      'notification',
      'lockScreenBtn',
      'logoutBtn',
    ]);
    expect(defaultPreferences.widget.fullscreenButtonPosition).toBe('none');
    expect(defaultPreferences.widget.refreshButtonPosition).toBe('none');
    expect(defaultPreferences.widget.timezoneButtonPosition).toBe('none');
    expect(defaultPreferences.widget.lockScreenButtonPosition).toBe('user-dropdown');
    expect(defaultPreferences.widget.logoutButtonPosition).toBe('user-dropdown');
    expect(defaultPreferences.app.preferencesButtonPosition).toBe('auto');
  });

  it('merges a preference group without losing siblings and resets', () => {
    usePreferencesStore.getState().update({ theme: { ...defaultPreferences.theme, colorPrimary: '#ff0000' } });
    expect(usePreferencesStore.getState().preferences.theme.mode).toBe(defaultPreferences.theme.mode);
    expect(usePreferencesStore.getState().preferences.theme.colorPrimary).toBe('#ff0000');
    usePreferencesStore.getState().reset();
    expect(usePreferencesStore.getState().preferences).toEqual(defaultPreferences);
  });

  it('migrates old partial caches and fills every new Golden Master field', () => {
    const migrated = migrateStoredPreferences({ preferences: { app: { dynamicTitle: false }, widget: { order: ['preferences'] } } });
    expect(migrated.app.dynamicTitle).toBe(false);
    expect(migrated.app.enableStickyPreferencesNavigationBar).toBe(defaultPreferences.app.enableStickyPreferencesNavigationBar);
    expect(migrated.widget.order).toEqual(['preferences']);
    expect(migrated.widget.timezoneButtonPosition).toBe(defaultPreferences.widget.timezoneButtonPosition);
    expect(migrated.sidebar.extraCollapsedWidth).toBe(defaultPreferences.sidebar.extraCollapsedWidth);
  });

  it('migrates the removed full-content layout to sidebar-nav', () => {
    const migrated = migrateStoredPreferences({ preferences: { app: { layout: 'full-content' } } });
    expect(migrated.app.layout).toBe('sidebar-nav');
  });

  it('migrates auto header modes to fixed', () => {
    expect(migrateStoredPreferences({ preferences: { header: { mode: 'auto' } } } as never).header.mode).toBe('fixed');
    expect(migrateStoredPreferences({ preferences: { header: { mode: 'auto-scroll' } } } as never).header.mode).toBe('fixed');
    expect(migrateStoredPreferences({ preferences: { header: { mode: 'static' } } }).header.mode).toBe('static');
  });

  it('migrates the former shortcutKeys group name', () => {
    const migrated = migrateStoredPreferences({ shortcutKeys: { enable: false, escape: true } });
    expect(migrated.shortcut.enable).toBe(false);
    expect(migrated.shortcut.escape).toBe(true);
    expect(migrated.shortcut.globalSearch).toBe(defaultPreferences.shortcut.globalSearch);
  });

  it('resolves explicit theme mode', () => {
    expect(effectiveDark({ ...defaultPreferences, theme: { ...defaultPreferences.theme, mode: 'dark' } })).toBe(true);
    expect(effectiveDark({ ...defaultPreferences, theme: { ...defaultPreferences.theme, mode: 'light' } })).toBe(false);
  });

  it('resolves follow-system theme from prefers-color-scheme', () => {
    const autoPrefs = { ...defaultPreferences, theme: { ...defaultPreferences.theme, mode: 'auto' as const } };
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: (query: string) => ({
        matches: query.includes('prefers-color-scheme: dark'),
        media: query,
        onchange: null,
        addEventListener: () => undefined,
        removeEventListener: () => undefined,
        addListener: () => undefined,
        removeListener: () => undefined,
        dispatchEvent: () => false,
      }),
    });
    expect(effectiveDark(autoPrefs)).toBe(true);
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: (query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addEventListener: () => undefined,
        removeEventListener: () => undefined,
        addListener: () => undefined,
        removeListener: () => undefined,
        dispatchEvent: () => false,
      }),
    });
    expect(effectiveDark(autoPrefs)).toBe(false);
  });

  it('switches neutral preset primary color by theme mode', () => {
    const darkPrefs = {
      ...defaultPreferences,
      theme: { ...defaultPreferences.theme, builtinType: 'zinc', mode: 'dark' as const },
    };
    const lightPrefs = {
      ...defaultPreferences,
      theme: { ...defaultPreferences.theme, builtinType: 'zinc', mode: 'light' as const },
    };

    expect(resolveThemePrimaryColor(darkPrefs)).toBe('hsl(0 0% 98%)');
    expect(resolveThemePrimaryColor(lightPrefs)).toBe('hsl(240 5.9% 10%)');
  });
});
