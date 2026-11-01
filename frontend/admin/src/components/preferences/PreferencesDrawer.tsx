import { App, Button, Drawer, Tooltip } from 'antd';
import { Check, ChevronDown, CircleHelp, Copy, GripVertical, MoonStar, Pin, PinOff, RotateCw, Sun, SunMoon, UserRoundPen, X } from 'lucide-react';
import { Segmented } from 'antd';
import { useEffect, useMemo, useRef, useState, type CSSProperties } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '@/stores/auth';
import { LOCALE_CODES } from '@/constants/locales';
import { useLocaleStore } from '@/stores/locale';
import { NumberStepper } from '@/components/NumberStepper';
import {
  BUILT_IN_THEME_PRESETS,
  cssColorToHex,
  defaultPreferences,
  resolveThemePrimaryColor,
  TIMEZONE_OPTIONS,
  usePreferencesStore,
} from '@/stores/preferences';
import { clearAdminStorage } from '@/stores/storage';
import { PREFERENCES_DRAWER_WIDTH } from '@/constants/layout';
import type { LayoutType, LocaleCode, Preferences, WidgetKey, WidgetPosition } from '@/types';
import { ContentPreview, LayoutPreview } from './LayoutPreviews';
import { hasPreferenceDifference, serializePreferenceDifference } from './preferences-diff';
import { getPreferenceDisabledRules } from './preference-rules';
import './preferences.css';

export type PreferenceTab = 'appearance' | 'general' | 'layout';
type Option<T extends string | number> = { label: string; value: T };

function Block({ children, title }: { children: React.ReactNode; title: string }) {
  return <section className="pref-block"><h3>{title}</h3>{children}</section>;
}

function Row({ children, disabled = false, label, shortcut, tip }: { children: React.ReactNode; disabled?: boolean; label: string; shortcut?: React.ReactNode; tip?: string }) {
  return (
    <div className={`pref-row${disabled ? ' is-disabled' : ''}`}>
      <span className="pref-row-label">
        {label}
        {tip ? (
          <Tooltip mouseEnterDelay={0.15} placement="bottom" title={tip}>
            <button aria-label={tip} className="pref-tip" type="button">
              <CircleHelp size={14} strokeWidth={1.75} />
            </button>
          </Tooltip>
        ) : null}
      </span>
      {shortcut && <span className="pref-shortcut">{shortcut}</span>}
      <div className="pref-row-control">{children}</div>
    </div>
  );
}

function Switch({ checked, disabled, label, onChange }: { checked: boolean; disabled?: boolean; label: string; onChange: (value: boolean) => void }) {
  return <button aria-checked={checked} aria-label={label} className="pref-switch" disabled={disabled} onClick={() => onChange(!checked)} role="switch" type="button"><span /></button>;
}

function SwitchRow({ checked, disabled, label, onChange, shortcut, tip }: { checked: boolean; disabled?: boolean; label: string; onChange: (value: boolean) => void; shortcut?: React.ReactNode; tip?: string }) {
  return <Row disabled={disabled} label={label} shortcut={shortcut} tip={tip}><Switch checked={checked} disabled={disabled} label={label} onChange={onChange} /></Row>;
}

function SelectControl<T extends string | number>({ disabled, label, onChange, options, value }: { disabled?: boolean; label: string; onChange: (value: T) => void; options: Option<T>[]; value: T }) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!open) return;
    const pointerDown = (event: MouseEvent) => { if (!rootRef.current?.contains(event.target as Node)) setOpen(false); };
    const keyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') setOpen(false); };
    document.addEventListener('mousedown', pointerDown);
    document.addEventListener('keydown', keyDown);
    return () => { document.removeEventListener('mousedown', pointerDown); document.removeEventListener('keydown', keyDown); };
  }, [open]);
  useEffect(() => {
    if (!open) return;
    const root = rootRef.current;
    const popup = root?.querySelector<HTMLElement>('.pref-select-popup');
    const scroller = root?.closest<HTMLElement>('.pref-scroll');
    if (!root || !popup || !scroller) return;
    const spaceBelow = scroller.getBoundingClientRect().bottom - root.getBoundingClientRect().bottom;
    popup.classList.toggle('is-upward', popup.offsetHeight + 8 > spaceBelow);
  }, [open]);
  const selected = options.find((option) => option.value === value);
  return (
    <div className={`pref-select${open ? ' is-open' : ''}`} ref={rootRef}>
      <button aria-expanded={open} aria-haspopup="listbox" aria-label={label} className="pref-select-trigger" disabled={disabled} type="button" onClick={() => setOpen((previous) => !previous)}>
        <span className="pref-select-value">{selected?.label ?? ''}</span>
        <ChevronDown aria-hidden className="pref-select-chevron" size={14} strokeWidth={2} />
      </button>
      {open && !disabled && (
        <div aria-label={label} className="pref-select-popup" role="listbox">
          {options.map((option) => (
            <button
              aria-selected={option.value === value}
              className="pref-select-option"
              key={option.value}
              role="option"
              type="button"
              onClick={() => { setOpen(false); if (option.value !== value) onChange(option.value); }}
            >
              <span className="pref-select-option-label">{option.label}</span>
              {option.value === value && <Check aria-hidden size={14} strokeWidth={2.5} />}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function Toggle<T extends string | number>({ disabled, label, onChange, options, value }: { disabled?: boolean; label: string; onChange: (value: T) => void; options: Option<T>[]; value: T }) {
  const activeIndex = Math.max(0, options.findIndex((option) => option.value === value));
  const count = Math.max(1, options.length);
  return (
    <div
      aria-label={label}
      className="pref-toggle"
      role="group"
      style={{ '--pref-toggle-count': count, '--pref-toggle-index': activeIndex } as CSSProperties}
    >
      <i aria-hidden className="pref-toggle-indicator" />
      {options.map((option) => (
        <button aria-pressed={value === option.value} disabled={disabled} key={option.value} type="button" onClick={() => onChange(option.value)}>
          {option.label}
        </button>
      ))}
    </div>
  );
}

/** Matches the single-choice control used by the menu type field. */
function ChoiceControl<T extends string | number>({ disabled, label, onChange, options, value }: { disabled?: boolean; label: string; onChange: (value: T) => void; options: Option<T>[]; value: T }) {
  return <Segmented aria-label={label} disabled={disabled} onChange={(next) => onChange(next as T)} options={options} value={value} />;
}

function CheckButtons({ disabled, items }: { disabled?: boolean; items: Array<{ checked: boolean; label: string; onChange: (value: boolean) => void }> }) {
  return (
    <div className="pref-check-buttons" role="group">
      {items.map((item) => (
        <button
          aria-pressed={item.checked}
          disabled={disabled}
          key={item.label}
          type="button"
          onClick={() => item.onChange(!item.checked)}
        >
          <span className="pref-check-mark" aria-hidden="true">
            {item.checked ? <Check size={12} strokeWidth={2.6} /> : null}
          </span>
          <span>{item.label}</span>
        </button>
      ))}
    </div>
  );
}

function InputControl({ disabled, label, max, min, onChange, placeholder, step, type = 'text', value }: { disabled?: boolean; label: string; max?: number; min?: number; onChange: (value: string) => void; placeholder?: string; step?: number; type?: 'number' | 'text'; value: string | number }) {
  return <input aria-label={label} className="pref-input" disabled={disabled} max={max} min={min} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} step={step} type={type} value={value} />;
}

const layouts: Array<{ label: string; tip: string; value: LayoutType }> = [
  { label: 'sidebar', tip: 'preferences.layoutTip.sidebar-nav', value: 'sidebar-nav' },
  { label: 'layout', tip: 'preferences.layoutTip.sidebar-mixed-nav', value: 'sidebar-mixed-nav' },
  { label: 'header', tip: 'preferences.layoutTip.header-nav', value: 'header-nav' },
  { label: 'navigation', tip: 'preferences.layoutTip.header-sidebar-nav', value: 'header-sidebar-nav' },
  { label: 'content', tip: 'preferences.layoutTip.mixed-nav', value: 'mixed-nav' },
  { label: 'tabbar', tip: 'preferences.layoutTip.header-mixed-nav', value: 'header-mixed-nav' },
];

function widgetPosition(preferences: Preferences, key: WidgetKey) {
  if (key === 'preferences') return preferences.app.preferencesButtonPosition;
  const map: Record<Exclude<WidgetKey, 'preferences'>, WidgetPosition> = {
    fullscreen: preferences.widget.fullscreenButtonPosition, globalSearch: preferences.widget.globalSearchButtonPosition,
    languageToggle: preferences.widget.languageToggleButtonPosition, lockScreenBtn: preferences.widget.lockScreenButtonPosition,
    logoutBtn: preferences.widget.logoutButtonPosition, notification: preferences.widget.notificationButtonPosition,
    refresh: preferences.widget.refreshButtonPosition, themeToggle: preferences.widget.themeToggleButtonPosition, timezone: preferences.widget.timezoneButtonPosition,
  };
  return map[key];
}

function widgetPositionKey(key: Exclude<WidgetKey, 'preferences'>): keyof Preferences['widget'] {
  if (key === 'lockScreenBtn') return 'lockScreenButtonPosition';
  if (key === 'logoutBtn') return 'logoutButtonPosition';
  return `${key}ButtonPosition` as keyof Preferences['widget'];
}

export function PreferencesDrawer({ initialTab = 'appearance', onClose, open }: { initialTab?: PreferenceTab; onClose: () => void; open: boolean }) {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const preferences = usePreferencesStore((state) => state.preferences);
  const update = usePreferencesStore((state) => state.update);
  const reset = usePreferencesStore((state) => state.reset);
  const locale = useLocaleStore((state) => state.locale);
  const setLocale = useLocaleStore((state) => state.setLocale);
  const logout = useAuthStore((state) => state.logout);
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<PreferenceTab>(initialTab);
  const [tabAnimKey, setTabAnimKey] = useState(0);
  const dragIndex = useRef<number | null>(null);
  useEffect(() => setActiveTab(initialTab), [initialTab]);
  const changed = useMemo(() => hasPreferenceDifference(preferences, defaultPreferences), [preferences]);
  const patch = <K extends keyof Preferences>(key: K, value: Partial<Preferences[K]>) => update({ [key]: { ...preferences[key], ...value } } as Partial<Preferences>);
  const rules = getPreferenceDisabledRules(preferences);
  const sidebarMode = !rules.sidebar;
  const mixedSide = ['header-mixed-nav', 'sidebar-mixed-nav'].includes(preferences.app.layout);
  const tabOrder = ['appearance', 'layout', 'general'] as const;
  const activeTabIndex = Math.max(0, tabOrder.indexOf(activeTab));
  const paneTransition = preferences.transition.enable ? preferences.transition.name : 'fade-slide';
  const switchTab = (tab: PreferenceTab) => {
    if (tab === activeTab) return;
    setActiveTab(tab);
    setTabAnimKey((value) => value + 1);
  };
  const positionOptions: Option<WidgetPosition>[] = [{ label: t('preferences.headerPlace'), value: 'header' }, { label: t('preferences.userDropdown'), value: 'user-dropdown' }, { label: t('preferences.hidden'), value: 'none' }];
  const preferencePositionOptions: Option<Preferences['app']['preferencesButtonPosition']>[] = [{ label: t('preferences.positionAuto'), value: 'auto' }, { label: t('preferences.headerPlace'), value: 'header' }, { label: t('preferences.positionFixed'), value: 'fixed' }, { label: t('preferences.userDropdown'), value: 'user-dropdown' }, { label: t('preferences.hidden'), value: 'none' }];

  const selectBuiltinTheme = (type: string) => {
    const theme = { ...preferences.theme, builtinType: type };
    patch('theme', { builtinType: type, colorPrimary: resolveThemePrimaryColor({ ...preferences, theme }) });
  };
  const updateWidgetPosition = (key: WidgetKey, value: string) => {
    if (key === 'preferences') patch('app', { preferencesButtonPosition: value as Preferences['app']['preferencesButtonPosition'] });
    else patch('widget', { [widgetPositionKey(key)]: value } as Partial<Preferences['widget']>);
  };
  const reorderWidget = (from: number, to: number) => {
    const order = [...preferences.widget.order]; const [item] = order.splice(from, 1); if (!item) return; order.splice(to, 0, item); patch('widget', { order });
  };
  const copyPreferences = async () => {
    await navigator.clipboard.writeText(serializePreferenceDifference(preferences, defaultPreferences));
    message.success({ content: t('preferences.copiedBody'), key: 'preferences-copy' });
  };
  const clearAndLogout = () => {
    void (async () => {
      onClose();
      clearAdminStorage();
      reset();
      await logout();
      navigate('/auth/login', { replace: true });
    })();
  };

  return <Drawer className="golden-preferences-drawer" closeIcon={false} footer={<div className="pref-footer">{preferences.app.enableCopyPreferences && <button className="pref-primary-button" disabled={!changed} onClick={() => void copyPreferences()} type="button"><Copy size={13} />{t('preferences.copy')}</button>}<button className="pref-ghost-button" onClick={clearAndLogout} type="button">{t('preferences.clear')}</button></div>} mask={{ closable: true }} onClose={onClose} open={open} placement="right" size={PREFERENCES_DRAWER_WIDTH} title={null}>
    <header className="pref-header">
      <div>
        <h2>{t('preferences.title')}</h2>
        <p>{t('preferences.subtitle')}</p>
      </div>
      <div className="pref-header-actions">
        <Button
          aria-label={t('preferences.resetTip')}
          className={`pref-icon-button${changed ? ' has-change' : ''}`}
          disabled={!changed}
          icon={<RotateCw size={16} />}
          title={t('preferences.resetTip')}
          type="text"
          onClick={reset}
        />
        <Button
          aria-label={preferences.app.enableStickyPreferencesNavigationBar ? t('preferences.pinOff') : t('preferences.pinOn')}
          className="pref-icon-button"
          icon={preferences.app.enableStickyPreferencesNavigationBar ? <PinOff size={16} /> : <Pin size={16} />}
          title={preferences.app.enableStickyPreferencesNavigationBar ? t('preferences.pinOff') : t('preferences.pinOn')}
          type="text"
          onClick={() => patch('app', { enableStickyPreferencesNavigationBar: !preferences.app.enableStickyPreferencesNavigationBar })}
        />
        <Button aria-label={t('close')} className="pref-icon-button" icon={<X size={16} />} type="text" onClick={onClose} />
      </div>
    </header>
    <main className="pref-scroll">
      <div className={preferences.app.enableStickyPreferencesNavigationBar ? 'pref-tabs-sticky' : undefined}>
        <nav className="pref-tabs" role="tablist">
          <i aria-hidden className="pref-tabs-indicator" style={{ transform: `translate3d(calc(${activeTabIndex} * (100% + 2px)), 0, 0)` }} />
          {tabOrder.map((tab) => (
            <button aria-selected={activeTab === tab} key={tab} onClick={() => switchTab(tab)} role="tab" type="button">{t('preferences.' + tab)}</button>
          ))}
        </nav>
      </div>
      <div className={`pref-pane pref-pane-${paneTransition}`} key={`${activeTab}-${tabAnimKey}`}>
      {activeTab === 'appearance' && <>
        <Block title={t('preferences.theme')}><div className="theme-mode-grid">{([{ icon: Sun, label: t('preferences.light'), value: 'light' }, { icon: MoonStar, label: t('preferences.dark'), value: 'dark' }, { icon: SunMoon, label: t('preferences.auto'), value: 'auto' }] as const).map(({ icon: Icon, label, value }) => <button className={preferences.theme.mode === value ? 'selected' : ''} key={value} onClick={() => patch('theme', { mode: value })} type="button"><span className={`outline-box${preferences.theme.mode === value ? ' is-active' : ''}`}><Icon size={20} /></span><small>{label}</small></button>)}</div><SwitchRow checked={preferences.theme.semiDarkSidebar} disabled={rules.darkSidebar} label={t('preferences.semiSidebar')} onChange={(value) => patch('theme', { semiDarkSidebar: value, ...(!value ? { semiDarkSidebarSub: false } : {}) })} /><SwitchRow checked={preferences.theme.semiDarkSidebarSub} disabled={rules.darkSidebarSub || !mixedSide} label={t('preferences.semiSub')} onChange={(value) => patch('theme', { semiDarkSidebarSub: value })} /><SwitchRow checked={preferences.theme.semiDarkHeader} disabled={rules.darkHeader} label={t('preferences.semiHeader')} onChange={(value) => patch('theme', { semiDarkHeader: value })} /></Block>
        <Block title={t('preferences.builtin')}><div className="builtin-theme-grid">{BUILT_IN_THEME_PRESETS.map((preset) => {const label = t('preferences.themeName.' + preset.type);return <button aria-label={label} className={preferences.theme.builtinType === preset.type ? 'selected' : ''} key={preset.type} onClick={() => selectBuiltinTheme(preset.type)} title={label} type="button"><span className={`builtin-card outline-box${preferences.theme.builtinType === preset.type ? ' is-active' : ''}`}>{preset.type === 'custom' ? <><UserRoundPen size={20}/><input aria-label={t('preferences.themeName.custom')} type="color" value={cssColorToHex(preferences.theme.colorPrimary)} onClick={(event)=>event.stopPropagation()} onChange={(event) => patch('theme', { builtinType:'custom',colorPrimary: event.target.value })} /></> : <i style={{ background: preset.color }} />}</span><small>{label}</small></button>})}</div></Block>
        <Block title={t('preferences.radius')}><Toggle label={t('preferences.radius')} onChange={(value) => patch('theme', { radius: value })} options={['0', '0.25', '0.5', '0.75', '1'].map((value) => ({ label: value, value }))} value={preferences.theme.radius} /></Block>
        <Block title={t('preferences.other')}><Row label={t('preferences.fontSize')}><NumberStepper label={t('preferences.fontSize')} max={22} min={15} onChange={(value) => patch('theme', { fontSize: value })} suffix="px" value={preferences.theme.fontSize} /></Row><SwitchRow checked={preferences.app.colorWeakMode} label={t('preferences.weak')} onChange={(value) => patch('app', { colorWeakMode: value })} /><SwitchRow checked={preferences.app.colorGrayMode} label={t('preferences.gray')} onChange={(value) => patch('app', { colorGrayMode: value })} /></Block>
        <Block title={t('preferences.footer')}><SwitchRow checked={preferences.footer.enable} label={t('preferences.showFooter')} onChange={(value) => patch('footer', { enable: value })} /><SwitchRow checked={preferences.footer.fixed} disabled={!preferences.footer.enable} label={t('preferences.fixedFooter')} onChange={(value) => patch('footer', { fixed: value })} /></Block>
        {preferences.copyright.settingShow && <Block title={t('preferences.copyright')}><SwitchRow checked={preferences.copyright.enable} disabled={!preferences.footer.enable} label={t('preferences.enableCopyright')} onChange={(value) => patch('copyright', { enable: value })} />{([{ key: 'companyName', label: t('preferences.companyName') }, { key: 'companySiteLink', label: t('preferences.companySite') }, { key: 'date', label: t('preferences.date') }, { key: 'icp', label: t('preferences.icp') }, { key: 'icpLink', label: t('preferences.icpLink') }] as const).map(({ key, label }) => <Row disabled={!preferences.footer.enable || !preferences.copyright.enable} key={key} label={label}><InputControl disabled={!preferences.footer.enable || !preferences.copyright.enable} label={label} onChange={(value) => patch('copyright', { [key]: value })} value={preferences.copyright[key]} /></Row>)}</Block>}
      </>}
      {activeTab === 'layout' && <>
        <Block title={t('preferences.layout')}><div className="layout-card-grid">{layouts.map(({ tip, value }) => {
          const label = t('preferences.layoutName.' + value);
          return (
            <button
              className={preferences.app.layout === value ? 'selected' : ''}
              key={value}
              title={label}
              type="button"
              onClick={() => {
                // Dual-column never shows collapsed menu titles — clear the preference when selecting it.
                if (value === 'sidebar-mixed-nav') {
                  update({
                    app: { ...preferences.app, layout: value },
                    sidebar: { ...preferences.sidebar, collapsedShowTitle: false },
                  });
                  return;
                }
                patch('app', { layout: value });
              }}
            >
              <span className={`outline-box${preferences.app.layout === value ? ' is-active' : ''}`}>
                <LayoutPreview type={value} />
              </span>
              <span className="layout-card-label">
                {label}
                <Tooltip mouseEnterDelay={0.15} placement="bottom" title={tip}>
                  <span aria-hidden className="pref-tip" onClick={(event) => event.stopPropagation()}>
                    <CircleHelp size={12} strokeWidth={1.75} />
                  </span>
                </Tooltip>
              </span>
            </button>
          );
        })}</div></Block>
        <Block title={t('preferences.content')}><div className="content-card-grid">{(['wide', 'compact'] as const).map((value) => <button className={preferences.app.contentCompact === value ? 'selected' : ''} key={value} onClick={() => patch('app', { contentCompact: value })} type="button"><span className={`outline-box${preferences.app.contentCompact === value ? ' is-active' : ''}`}><ContentPreview type={value} /></span><small>{t('preferences.' + value)}</small></button>)}</div></Block>
        <Block title={t('preferences.sidebar')}><SwitchRow checked={preferences.sidebar.enable} disabled={!sidebarMode} label={t('preferences.sidebarVisible')} onChange={(value) => patch('sidebar', { enable: value })} /><SwitchRow checked={preferences.sidebar.draggable} disabled={!sidebarMode || !preferences.sidebar.enable} label={t('preferences.sidebarEdgeDrag')} onChange={(value) => patch('sidebar', { draggable: value })} /><SwitchRow checked={preferences.sidebar.collapsed} disabled={!sidebarMode || !preferences.sidebar.enable} label={t('preferences.sidebarCollapsed')} onChange={(value) => patch('sidebar', { collapsed: value, ...(!value ? { expandOnHover: false, collapsedShowTitle: false } : {}) })} /><SwitchRow checked={preferences.sidebar.expandOnHover} disabled={!sidebarMode || !preferences.sidebar.enable || !preferences.sidebar.collapsed} label={t('preferences.hoverExpand')} onChange={(value) => patch('sidebar', { expandOnHover: value })} /><SwitchRow checked={preferences.app.layout === 'sidebar-mixed-nav' ? false : preferences.sidebar.collapsedShowTitle} disabled={!sidebarMode || !preferences.sidebar.enable || !preferences.sidebar.collapsed || preferences.app.layout === 'sidebar-mixed-nav'} label={t('preferences.collapsedTitle')} onChange={(value) => patch('sidebar', { collapsedShowTitle: value })} /><SwitchRow checked={preferences.sidebar.autoActivateChild} disabled={!sidebarMode || !preferences.sidebar.enable} label={t('preferences.autoChild')} onChange={(value) => patch('sidebar', { autoActivateChild: value })} /><Row label={t('preferences.showButtons')}><CheckButtons items={[{checked:preferences.sidebar.collapsedButton,label:t('preferences.collapseButton'),onChange:(value)=>patch('sidebar',{collapsedButton:value})},{checked:preferences.sidebar.fixedButton,label:t('preferences.fixedButton'),onChange:(value)=>patch('sidebar',{fixedButton:value})}]}/></Row><Row disabled={!sidebarMode || !preferences.sidebar.enable} label={t('preferences.sidebarWidth')}><NumberStepper disabled={!sidebarMode || !preferences.sidebar.enable} label={t('preferences.sidebarWidth')} max={320} min={160} onChange={(value) => patch('sidebar', { width: value })} step={10} value={preferences.sidebar.width} /></Row></Block>
        <Block title={t('preferences.header')}><SwitchRow checked={preferences.header.enable} label={t('preferences.headerVisible')} onChange={(value) => patch('header', { enable: value })} /><Row disabled={!preferences.header.enable} label={t('preferences.headerMode')}><ChoiceControl disabled={!preferences.header.enable} label={t('preferences.headerMode')} onChange={(value) => patch('header', { mode: value })} options={[{ label: t('preferences.static'), value: 'static' }, { label: t('preferences.fixed'), value: 'fixed' }]} value={preferences.header.mode === 'static' ? 'static' : 'fixed'} /></Row><Row disabled label={t('preferences.menuAlign')}><ChoiceControl disabled label={t('preferences.menuAlign')} onChange={() => undefined} options={[{ label: t('preferences.start'), value: 'start' }, { label: t('preferences.center'), value: 'center' }, { label: t('preferences.end'), value: 'end' }]} value={preferences.header.menuAlign} /></Row></Block>
        <Block title={t('preferences.navigation')}><Row label={t('preferences.navStyle')}><ChoiceControl label={t('preferences.navStyle')} onChange={(value) => patch('navigation', { styleType: value })} options={[{ label: t('preferences.rounded'), value: 'rounded' }, { label: t('preferences.plain'), value: 'plain' }]} value={preferences.navigation.styleType} /></Row><SwitchRow checked={preferences.navigation.split} disabled={preferences.app.layout !== 'mixed-nav'} label={t('preferences.split')} onChange={(value) => patch('navigation', { split: value })} /><SwitchRow checked={preferences.navigation.accordion} label={t('preferences.accordion')} onChange={(value) => patch('navigation', { accordion: value })} /></Block>
        <Block title={t('preferences.breadcrumb')}><SwitchRow checked={preferences.breadcrumb.enable} label={t('preferences.breadcrumbEnable')} onChange={(value) => patch('breadcrumb', { enable: value })} /><SwitchRow checked={preferences.breadcrumb.hideOnlyOne} disabled={!preferences.breadcrumb.enable} label={t('preferences.hideOne')} onChange={(value) => patch('breadcrumb', { hideOnlyOne: value })} /><SwitchRow checked={preferences.breadcrumb.showIcon} disabled={!preferences.breadcrumb.enable} label={t('preferences.icon')} onChange={(value) => patch('breadcrumb', { showIcon: value })} /><SwitchRow checked={preferences.breadcrumb.showHome} disabled={!preferences.breadcrumb.enable} label={t('preferences.home')} onChange={(value) => patch('breadcrumb', { showHome: value })} /><Row disabled={!preferences.breadcrumb.enable} label={t('preferences.breadcrumbStyle')}><ChoiceControl disabled={!preferences.breadcrumb.enable} label={t('preferences.breadcrumbStyle')} onChange={(value) => patch('breadcrumb', { styleType: value })} options={[{ label: t('preferences.normal'), value: 'normal' }, { label: t('preferences.background'), value: 'background' }]} value={preferences.breadcrumb.styleType} /></Row></Block>
        <Block title={t('preferences.tabbar')}><SwitchRow checked={preferences.tabbar.enable} label={t('preferences.tabEnable')} onChange={(value) => patch('tabbar', { enable: value })} /><SwitchRow checked={preferences.tabbar.persist} disabled={!preferences.tabbar.enable} label={t('preferences.persist')} onChange={(value) => patch('tabbar', { persist: value })} /><SwitchRow checked={preferences.tabbar.visitHistory} disabled={!preferences.tabbar.enable} label={t('preferences.history')} tip={t('preferences.historyTip')} onChange={(value) => patch('tabbar', { visitHistory: value })} /><Row disabled={!preferences.tabbar.enable} label={t('preferences.maxCount')} tip={t('preferences.maxCountTip')}><NumberStepper disabled={!preferences.tabbar.enable} label={t('preferences.maxCount')} min={0} onChange={(value) => patch('tabbar', { maxCount: value })} value={preferences.tabbar.maxCount} /></Row><SwitchRow checked={preferences.tabbar.draggable} disabled={!preferences.tabbar.enable} label={t('preferences.draggable')} onChange={(value) => patch('tabbar', { draggable: value })} /><SwitchRow checked={preferences.tabbar.wheelable} disabled={!preferences.tabbar.enable} label={t('preferences.wheel')} tip={t('preferences.wheelTip')} onChange={(value) => patch('tabbar', { wheelable: value })} /><SwitchRow checked={preferences.tabbar.middleClickToClose} disabled={!preferences.tabbar.enable} label={t('preferences.middle')} onChange={(value) => patch('tabbar', { middleClickToClose: value })} /><SwitchRow checked={preferences.tabbar.showIcon} disabled={!preferences.tabbar.enable} label={t('preferences.showIcon')} onChange={(value) => patch('tabbar', { showIcon: value })} /><SwitchRow checked={preferences.tabbar.showMore} disabled={!preferences.tabbar.enable} label={t('preferences.showMore')} onChange={(value) => patch('tabbar', { showMore: value })} /><SwitchRow checked={preferences.tabbar.showMaximize} disabled={!preferences.tabbar.enable} label={t('preferences.showMax')} onChange={(value) => patch('tabbar', { showMaximize: value })} /><Row disabled={!preferences.tabbar.enable} label={t('preferences.tabStyle')}><ChoiceControl disabled={!preferences.tabbar.enable} label={t('preferences.tabStyle')} onChange={(value) => patch('tabbar', { styleType: value })} options={[{ label: t('preferences.chrome'), value: 'chrome' }, { label: t('preferences.card'), value: 'card' }, { label: t('preferences.brisk'), value: 'brisk' }, { label: t('preferences.plain'), value: 'plain' }]} value={preferences.tabbar.styleType} /></Row></Block>
        <Block title={t('preferences.widget')}><div className="widget-list">{preferences.widget.order.map((key, index) => <div className="widget-item" draggable key={key} onDragOver={(event) => event.preventDefault()} onDragStart={() => { dragIndex.current = index; }} onDrop={() => { if (dragIndex.current !== null) reorderWidget(dragIndex.current, index); dragIndex.current = null; }}><GripVertical size={15} /><span>{t('preferences.widgetName.' + key)}</span><SelectControl label={t('preferences.widgetName.' + key)} onChange={(value) => updateWidgetPosition(key, value)} options={key === 'preferences' ? preferencePositionOptions : positionOptions} value={widgetPosition(preferences, key)} /></div>)}</div></Block>
      </>}
      {activeTab === 'general' && <>
        <Block title={t('preferences.general')}><Row label={t('preferences.language')}><SelectControl label={t('preferences.language')} onChange={(value) => setLocale(value)} options={LOCALE_CODES.map((code) => ({ label: t(`locale.${code}`), value: code }))} value={locale as LocaleCode} /></Row><Row label={t('timezone.label')}><SelectControl label={t('timezone.label')} onChange={(value) => patch('app', { timezone: value })} options={TIMEZONE_OPTIONS.map((item) => ({ label: t(`timezone.${item.value}`), value: item.value }))} value={preferences.app.timezone} /></Row><SwitchRow checked={preferences.app.dynamicTitle} label={t('preferences.dynamicTitle')} onChange={(value) => patch('app', { dynamicTitle: value })} /><SwitchRow checked={preferences.app.watermark} label={t('preferences.watermark')} onChange={(value) => patch('app', { watermark: value, ...(!value ? { watermarkContent: '' } : {}) })} />{preferences.app.watermark && <Row label={t('preferences.watermarkText')}><InputControl label={t('preferences.watermarkText')} onChange={(value) => patch('app', { watermarkContent: value })} placeholder={t('preferences.watermarkText')} value={preferences.app.watermarkContent} /></Row>}<SwitchRow checked={preferences.app.enableCheckUpdates} label={t('preferences.updates')} onChange={(value) => patch('app', { enableCheckUpdates: value })} /><SwitchRow checked={preferences.app.enableCopyPreferences} label={t('preferences.copyEnable')} onChange={(value) => patch('app', { enableCopyPreferences: value })} /></Block>
        <Block title={t('preferences.animation')}><SwitchRow checked={preferences.transition.progress} label={t('preferences.progress')} onChange={(value) => patch('transition', { progress: value })} /><SwitchRow checked={preferences.transition.loading} label={t('preferences.loading')} onChange={(value) => patch('transition', { loading: value })} /><SwitchRow checked={preferences.transition.enable} label={t('preferences.transition')} onChange={(value) => patch('transition', { enable: value })} />{preferences.transition.enable && <div className="transition-grid">{['fade', 'fade-slide', 'fade-up', 'fade-down'].map((value) => <button aria-label={value} className={`outline-box${preferences.transition.name === value ? ' is-active' : ''}`} key={value} onClick={() => patch('transition', { name: value })} type="button"><span className={`transition-demo ${value}`} /></button>)}</div>}</Block>
        <Block title={t('preferences.shortcuts')}><SwitchRow checked={preferences.shortcut.enable} label={t('preferences.shortcutMain')} onChange={(value) => patch('shortcut', { enable: value })} /><SwitchRow checked={preferences.shortcut.globalSearch} disabled={!preferences.shortcut.enable} label={t('preferences.search')} onChange={(value) => patch('shortcut', { globalSearch: value })} shortcut={<>Ctrl <kbd>K</kbd></>} /><SwitchRow checked={preferences.shortcut.logout} disabled={!preferences.shortcut.enable} label={t('preferences.logout')} onChange={(value) => patch('shortcut', { logout: value })} shortcut={<>Alt Q</>} /><SwitchRow checked={preferences.shortcut.lockScreen} disabled={!preferences.shortcut.enable} label={t('preferences.lock')} onChange={(value) => patch('shortcut', { lockScreen: value })} shortcut={<>Alt L</>} /></Block>
      </>}
      </div>
    </main>
  </Drawer>;
}
