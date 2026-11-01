import { afterEach, describe, expect, it } from 'vitest';
import i18n from '@/i18n';
import { useLocaleStore } from './locale';
import { usePreferencesStore } from './preferences';
import { useTabsStore } from './tabs';

afterEach(async () => {
  useLocaleStore.setState({ locale: 'zh-CN' });
  await i18n.changeLanguage('zh-CN');
});

describe('locale store', () => {
  it('applies the language everywhere at once', () => {
    const before = useTabsStore.getState().localeEpoch;
    useLocaleStore.getState().setLocale('en-US');
    expect(i18n.language).toBe('en-US');
    expect(usePreferencesStore.getState().preferences.app.locale).toBe('en-US');
    expect(useTabsStore.getState().localeEpoch).toBe(before + 1);
  });

  it('bumps the epoch on every switch, even back to the same language', () => {
    const before = useTabsStore.getState().localeEpoch;
    useLocaleStore.getState().setLocale('zh-CN');
    expect(useTabsStore.getState().localeEpoch).toBe(before + 1);
  });
});
