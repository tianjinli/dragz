import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { message } from 'antd';
import i18n from '@/i18n';
import { dropPendingGetDedupes } from '@/api/request';
import type { LocaleCode } from '@/types';
import { useTabsStore } from './tabs';
import { storageKey } from './storage';
import { usePreferencesStore } from './preferences';

interface LocaleStore {
  locale: LocaleCode;
  setLocale: (locale: LocaleCode) => void;
}

/**
 * Single entry point for a language change.
 *
 * `preferences.app.locale` is what goes out as `Accept-Language`, so the copy the
 * backend renders (menus, roles, department names) follows the same value as the
 * UI; `tabs.localeEpoch` is how keep-alive pages learn they must remount.
 */
function applyLocale(locale: LocaleCode) {
  void i18n.changeLanguage(locale);
  const prefs = usePreferencesStore.getState();
  if (prefs.preferences.app.locale !== locale) {
    prefs.update({ app: { ...prefs.preferences.app, locale } });
  }
  dropPendingGetDedupes();
  // Imperative toasts are rendered outside React and keep the old wording.
  message.destroy();
  useTabsStore.getState().bumpLocaleEpoch();
}

/** Language used to live in persisted preferences only; adopt it on first run. */
function legacyLocale(): LocaleCode | null {
  try {
    const raw = localStorage.getItem(storageKey('preferences'));
    const stored = raw ? (JSON.parse(raw) as { state?: { preferences?: { app?: { locale?: string } } } }) : null;
    const value = stored?.state?.preferences?.app?.locale;
    return value === 'zh-CN' || value === 'en-US' ? value : null;
  } catch {
    return null;
  }
}

export const useLocaleStore = create<LocaleStore>()(
  persist(
    (set) => ({
      locale: 'zh-CN',
      setLocale: (locale) => {
        set({ locale });
        applyLocale(locale);
      },
    }),
    {
      name: storageKey('locale'),
      onRehydrateStorage: () => (state) => {
        if (state) applyLocale(state.locale);
      },
      skipHydration: true,
    },
  ),
);

export async function initLocale() {
  await useLocaleStore.persist.rehydrate();
  const stored = useLocaleStore.getState().locale;
  const locale = stored === 'zh-CN' ? (legacyLocale() ?? 'zh-CN') : stored;
  useLocaleStore.setState({ locale });
  applyLocale(locale);
}
