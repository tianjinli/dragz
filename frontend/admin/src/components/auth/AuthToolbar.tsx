import { Dropdown } from 'antd';
import { Check, Languages, Palette } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { Preferences } from '@/types';
import { ThemeModeButton } from '@/components/ThemeModeButton';
import { LOCALE_CODES } from '@/constants/locales';
import { BUILT_IN_THEME_PRESETS, resolveThemePrimaryColor, usePreferencesStore } from '@/stores/preferences';
import { useLocaleStore } from '@/stores/locale';

/** Login toolbar color strip — first 8 built-in presets (excludes custom). */
const AUTH_COLOR_PRESETS = BUILT_IN_THEME_PRESETS.filter((preset) => preset.type !== 'custom').slice(0, 8);

export function AuthToolbar() {
  const { t } = useTranslation();
  const prefs = usePreferencesStore((state) => state.preferences);
  const update = usePreferencesStore((state) => state.update);
  const locale = useLocaleStore((state) => state.locale);
  const setLocale = useLocaleStore((state) => state.setLocale);

  const localeOptions = LOCALE_CODES.map((code) => ({ key: code, label: t(`locale.${code}`) }));

  const languageMenu = {
    selectedKeys: [locale],
    items: localeOptions.map((item) => ({
      key: item.key,
      label: item.label,
      onClick: () => {
        setLocale(item.key);
      },
    })),
  };

  const selectTheme = (type: string) => {
    const nextTheme = { ...prefs.theme, builtinType: type };
    update({ theme: { builtinType: type, colorPrimary: resolveThemePrimaryColor({ ...prefs, theme: nextTheme }) } } as Partial<Preferences>);
  };

  return (
    <div className="auth-toolbar auth-toolbar-grouped">
      <div className="auth-toolbar-desktop">
        <div className="auth-color-toggle">
          <div className="auth-color-presets">
            {AUTH_COLOR_PRESETS.map((preset) => {
              const selected = prefs.theme.builtinType === preset.type;
              return (
                <button
                  aria-label={preset.type}
                  aria-pressed={selected}
                  className={selected ? 'selected' : ''}
                  key={preset.type}
                  style={{ background: preset.color }}
                  type="button"
                  onClick={() => selectTheme(preset.type)}
                >
                  {selected ? <Check color="#fff" size={12} strokeWidth={3} /> : null}
                </button>
              );
            })}
          </div>
          <button aria-label={t('layout.themeColors')} className="auth-icon-button auth-palette-button" type="button">
            <Palette size={16} />
          </button>
        </div>
      </div>
      {prefs.widget.languageToggle ? (
        <Dropdown menu={languageMenu} trigger={['click']}>
          <button aria-label={t('layout.language')} className="auth-icon-button" type="button">
            <Languages size={16} />
          </button>
        </Dropdown>
      ) : null}
      {prefs.widget.themeToggle ? <ThemeModeButton /> : null}
    </div>
  );
}
