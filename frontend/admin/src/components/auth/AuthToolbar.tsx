import { Dropdown } from 'antd';
import { Check, InspectionPanel, Languages, Palette, PanelLeft, PanelRight } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { Preferences } from '@/types';
import { ThemeModeButton } from '@/components/ThemeModeButton';
import { LOCALE_CODES } from '@/constants/locales';
import { PALETTE_THEME_PRESETS, resolveThemePrimaryColor, usePreferencesStore } from '@/stores/preferences';
import { useLocaleStore } from '@/stores/locale';

/** Login toolbar color strip — mainstream presets only (excludes custom). */
const AUTH_COLOR_PRESETS = PALETTE_THEME_PRESETS.filter((preset) => preset.type !== 'custom');

const AUTH_LAYOUT_OPTIONS = [
  { icon: PanelLeft, label: 'layout.authLeft', value: 'panel-left' },
  { icon: InspectionPanel, label: 'layout.authCenter', value: 'panel-center' },
  { icon: PanelRight, label: 'layout.authRight', value: 'panel-right' },
] as const;

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

  const currentLayout = AUTH_LAYOUT_OPTIONS.find((option) => option.value === prefs.app.authPageLayout) ?? AUTH_LAYOUT_OPTIONS[1];
  const CurrentLayoutIcon = currentLayout.icon;
  const layoutMenu = {
    selectedKeys: [prefs.app.authPageLayout],
    items: AUTH_LAYOUT_OPTIONS.map(({ icon: Icon, label, value }) => ({
      icon: <Icon size={14} />,
      key: value,
      label: t(label),
      onClick: () => selectLayout(value),
    })),
  };

  const selectTheme = (type: string) => {
    const nextTheme = { ...prefs.theme, builtinType: type };
    update({ theme: { builtinType: type, colorPrimary: resolveThemePrimaryColor({ ...prefs, theme: nextTheme }) } } as Partial<Preferences>);
  };

  const selectLayout = (value: Preferences['app']['authPageLayout']) => {
    update({ app: { authPageLayout: value } } as Partial<Preferences>);
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
        <Dropdown menu={layoutMenu} trigger={['click']}>
          <button aria-label={t('layout.authLayout')} className="auth-icon-button auth-layout-button" title={t('layout.authLayout')} type="button">
            <CurrentLayoutIcon size={16} />
          </button>
        </Dropdown>
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
