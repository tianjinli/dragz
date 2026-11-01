import { afterEach, describe, expect, it, vi } from 'vitest';
import { defaultPreferences } from '@/stores/preferences';
import type { Preferences } from '@/types';
import { getPreferenceDisabledRules } from './preference-rules';

const withPreference = (patch: Partial<Preferences>): Preferences => {
  const value = structuredClone(defaultPreferences);
  for (const key of Object.keys(patch) as (keyof Preferences)[]) Object.assign(value[key], patch[key]);
  return value;
};

function mockPrefersColorScheme(dark: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: query.includes('prefers-color-scheme: dark') ? dark : false,
    media: query,
    onchange: null,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    addListener: () => undefined,
    removeListener: () => undefined,
    dispatchEvent: () => false,
  }));
}

describe('Golden Master disabled rules', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('disables dark variants in dark mode', () => {
    const rules = getPreferenceDisabledRules(withPreference({ theme: { ...defaultPreferences.theme, mode: 'dark' } }));
    expect(rules.darkHeader).toBe(true);
    expect(rules.darkSidebar).toBe(true);
    expect(rules.darkSidebarSub).toBe(true);
  });

  it('disables dark variants when follow-system resolves to dark', () => {
    mockPrefersColorScheme(true);
    const rules = getPreferenceDisabledRules(withPreference({ theme: { ...defaultPreferences.theme, mode: 'auto' } }));
    expect(rules.darkHeader).toBe(true);
    expect(rules.darkSidebar).toBe(true);
    expect(rules.darkSidebarSub).toBe(true);
  });

  it('keeps dark variants available when follow-system resolves to light', () => {
    mockPrefersColorScheme(false);
    const rules = getPreferenceDisabledRules(withPreference({
      theme: { ...defaultPreferences.theme, mode: 'auto', semiDarkSidebar: true },
      app: { ...defaultPreferences.app, layout: 'sidebar-mixed-nav' },
    }));
    expect(rules.darkHeader).toBe(false);
    expect(rules.darkSidebar).toBe(false);
    expect(rules.darkSidebarSub).toBe(false);
  });

  it('disables sidebar for header-nav', () => {
    expect(getPreferenceDisabledRules(withPreference({ app: { ...defaultPreferences.app, layout: 'header-nav' } })).sidebar).toBe(true);
  });

  it('allows the semi-dark sub sidebar only in mixed two-column layouts', () => {
    for (const layout of ['sidebar-mixed-nav', 'header-mixed-nav'] as const) {
      const rules = getPreferenceDisabledRules(withPreference({
        app: { ...defaultPreferences.app, layout },
        theme: { ...defaultPreferences.theme, mode: 'light', semiDarkSidebar: true },
      }));
      expect(rules.darkSidebarSub).toBe(false);
    }
  });

  it('models all parent-child disabled conditions', () => {
    const preferences = withPreference({
      app: { ...defaultPreferences.app, layout: 'mixed-nav' },
      footer: { ...defaultPreferences.footer, enable: false },
      header: { ...defaultPreferences.header, enable: false },
      shortcut: { ...defaultPreferences.shortcut, enable: false },
      tabbar: { ...defaultPreferences.tabbar, enable: false },
    });
    const rules = getPreferenceDisabledRules(preferences);
    expect(rules.breadcrumb).toBe(false);
    expect(rules.footerFixed).toBe(true);
    expect(rules.copyright).toBe(true);
    expect(rules.tabbarChildren).toBe(true);
    expect(rules.shortcutChildren).toBe(true);
    expect(rules.navigationSplit).toBe(false);
  });
});
