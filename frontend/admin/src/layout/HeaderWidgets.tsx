import { Avatar, Badge, Button, Dropdown, Empty, List, message, Popover, Select, Space, Tooltip } from 'antd';
import type { MenuProps } from 'antd';
import {
  Bell,
  Camera,
  Clock,
  Languages,
  LockKeyhole,
  LogOut,
  Maximize2,
  Moon,
  RefreshCw,
  Search,
  Settings,
  Sun,
  UserRound,
} from 'lucide-react';
import { useState } from 'react';
import type { ReactElement } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { ThemeModeButton } from '@/components/ThemeModeButton';
import { LOCALE_CODES } from '@/constants/locales';
import { useAccessStore } from '@/stores/access';
import { useAuthStore } from '@/stores/auth';
import { useLocaleStore } from '@/stores/locale';
import { useMessagesStore } from '@/stores/messages';
import { effectiveDark, TIMEZONE_OPTIONS, usePreferencesStore } from '@/stores/preferences';
import { useTabsStore } from '@/stores/tabs';
import { formatDateTime } from '@/utils/format-time';
import { confirmLogout } from '@/utils/confirm-logout';
import { normalizeNoticePriority } from '@/utils/notice';
import { applyPreferencesThemeNow, runThemeViewTransition } from '@/utils/theme-transition';
import type { LocaleCode, Preferences, WidgetKey, WidgetPosition } from '@/types';

const widgetMeta: Array<{ key: WidgetKey; labelKey: string }> = [
  { key: 'globalSearch', labelKey: 'shell.globalSearch' },
  { key: 'preferences', labelKey: 'preferences' },
  { key: 'themeToggle', labelKey: 'shell.themeToggle' },
  { key: 'languageToggle', labelKey: 'shell.languageToggle' },
  { key: 'timezone', labelKey: 'timezone.label' },
  { key: 'fullscreen', labelKey: 'common.fullscreen' },
  { key: 'refresh', labelKey: 'refresh' },
  { key: 'notification', labelKey: 'shell.notificationCenter' },
  { key: 'lockScreenBtn', labelKey: 'lock' },
  { key: 'logoutBtn', labelKey: 'logout' },
];

function widgetEnabled(prefs: Preferences, key: WidgetKey) {
  switch (key) {
    case 'fullscreen':
      return prefs.widget.fullscreen;
    case 'globalSearch':
      return prefs.widget.globalSearch;
    case 'languageToggle':
      return prefs.widget.languageToggle;
    case 'lockScreenBtn':
      return prefs.widget.lockScreen;
    case 'logoutBtn':
      return true;
    case 'notification':
      return prefs.widget.notification;
    case 'preferences':
      return prefs.app.enablePreferences;
    case 'refresh':
      return prefs.widget.refresh;
    case 'themeToggle':
      return prefs.widget.themeToggle;
    case 'timezone':
      return prefs.widget.timezone;
    default:
      return false;
  }
}

function widgetPositionOf(prefs: Preferences, key: WidgetKey): string {
  if (key === 'preferences') return prefs.app.preferencesButtonPosition;
  const map: Record<Exclude<WidgetKey, 'preferences'>, WidgetPosition> = {
    fullscreen: prefs.widget.fullscreenButtonPosition,
    globalSearch: prefs.widget.globalSearchButtonPosition,
    languageToggle: prefs.widget.languageToggleButtonPosition,
    lockScreenBtn: prefs.widget.lockScreenButtonPosition,
    logoutBtn: prefs.widget.logoutButtonPosition,
    notification: prefs.widget.notificationButtonPosition,
    refresh: prefs.widget.refreshButtonPosition,
    themeToggle: prefs.widget.themeToggleButtonPosition,
    timezone: prefs.widget.timezoneButtonPosition,
  };
  return map[key];
}

function fullWidgetOrder(prefs: Preferences): WidgetKey[] {
  const seen = new Set<WidgetKey>();
  const order: WidgetKey[] = [];
  for (const key of prefs.widget.order) {
    if (widgetMeta.some((meta) => meta.key === key) && !seen.has(key)) {
      seen.add(key);
      order.push(key);
    }
  }
  for (const meta of widgetMeta) {
    if (!seen.has(meta.key)) order.push(meta.key);
  }
  return order;
}

/**
 * Dropdown clones its trigger, so a Tooltip nested inside it keeps showing after
 * the menu opens. The header sits at the very top of the viewport, which leaves
 * no room for the default `top` placement — the tip flips to `bottom`, lands
 * exactly on the menu, and its higher z-index hides the first entries. Suppress
 * the tip while the menu is open; the button keeps its aria-label either way.
 */
export function TippedDropdown({
  title,
  items,
  selectedKeys,
  onSelect,
  placement = 'bottomRight',
  children,
}: {
  title: string;
  items: MenuProps['items'];
  selectedKeys?: string[];
  onSelect: NonNullable<MenuProps['onClick']>;
  placement?: 'bottomLeft' | 'bottomRight';
  children: ReactElement;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [hovered, setHovered] = useState(false);
  return (
    <Dropdown
      menu={{ items, selectedKeys, onClick: onSelect }}
      onOpenChange={(open) => {
        setMenuOpen(open);
        if (open) setHovered(false);
      }}
      placement={placement}
      trigger={['click']}
    >
      <Tooltip
        onOpenChange={(open) => {
          if (!menuOpen) setHovered(open);
        }}
        open={hovered && !menuOpen}
        title={title}
      >
        {children}
      </Tooltip>
    </Dropdown>
  );
}

function UserAvatar({ size, src }: { size: number; src?: string }) {
  const [failed, setFailed] = useState(false);
  const showImg = Boolean(src) && !failed;
  return (
    <span className="user-avatar" style={{ width: size, height: size }}>
      <span className="user-avatar-face">
        {showImg ? (
          <img alt="" src={src} onError={() => setFailed(true)} />
        ) : (
          <Camera size={Math.round(size * 0.5)} strokeWidth={1.75} />
        )}
      </span>
    </span>
  );
}

export function HeaderWidgets({
  onPreferences,
  onSearch,
}: {
  onPreferences: () => void;
  onSearch: () => void;
}) {
  const prefs = usePreferencesStore((state) => state.preferences);
  const update = usePreferencesStore((state) => state.update);
  const locale = useLocaleStore((state) => state.locale);
  const setLocale = useLocaleStore((state) => state.setLocale);
  const user = useAccessStore((state) => state.userInfo);
  const logout = useAuthStore((state) => state.logout);
  const switchTenant = useAuthStore((state) => state.switchTenant);
  const [switchingTenant, setSwitchingTenant] = useState(false);
  const inboxItems = useMessagesStore((state) => state.items);
  const unreadTotal = useMessagesStore((state) => state.unread.total);
  const markRead = useMessagesStore((state) => state.markRead);
  const navigate = useNavigate();
  const { t } = useTranslation();
  const dark = effectiveDark(prefs);

  const lock = () => window.dispatchEvent(new Event('admin:lock'));
  const doLogout = () => {
    confirmLogout(async () => {
      await logout();
      navigate('/auth/login', { replace: true });
    });
  };
  const refreshActive = () => {
    const store = useTabsStore.getState();
    store.refresh(store.activeKey);
  };
  const toggleFullscreen = () => {
    if (document.fullscreenElement) void document.exitFullscreen();
    else void document.documentElement.requestFullscreen();
  };

  const headerNode = (key: WidgetKey) => {
    switch (key) {
      case 'globalSearch':
        return (
          <Tooltip key={key} title={t('shell.globalSearchTip')}>
            <Button className="widget-globalSearch" icon={<Search />} type="text" onClick={onSearch} />
          </Tooltip>
        );
      case 'preferences':
        return (
          <Tooltip key={key} title={t('shell.preferences')}>
            <Button className="widget-preferences" icon={<Settings />} type="text" onClick={onPreferences} />
          </Tooltip>
        );
      case 'themeToggle':
        return (
          <Tooltip key={key} title={t('shell.themeToggle')}>
            <span className="header-theme-toggle">
              <ThemeModeButton className="widget-themeToggle" iconSize={18} />
            </span>
          </Tooltip>
        );
      case 'languageToggle':
        return (
          <TippedDropdown
            key={key}
            items={LOCALE_CODES.map((code) => ({ key: code, label: t(`locale.${code}`) }))}
            onSelect={({ key: value }) => setLocale(value as LocaleCode)}
            selectedKeys={[locale]}
            title={t('shell.languageToggle')}
          >
            <Button className="widget-languageToggle" icon={<Languages />} type="text" />
          </TippedDropdown>
        );
      case 'timezone':
        return (
          <TippedDropdown
            key={key}
            items={TIMEZONE_OPTIONS.map((item) => ({
              key: item.value,
              label: t(`timezone.${item.value}`),
            }))}
            onSelect={({ key: value }) => update({ app: { ...prefs.app, timezone: value } })}
            selectedKeys={[prefs.app.timezone]}
            title={t('timezone.setTimezone')}
          >
            <Button className="widget-timezone" icon={<Clock />} type="text" />
          </TippedDropdown>
        );
      case 'fullscreen':
        return (
          <Tooltip key={key} title={t('shell.fullscreenTip')}>
            <Button className="widget-fullscreen" icon={<Maximize2 />} type="text" onClick={toggleFullscreen} />
          </Tooltip>
        );
      case 'refresh':
        return (
          <Tooltip key={key} title={t('shell.reload')}>
            <Button className="widget-refresh" icon={<RefreshCw />} type="text" onClick={refreshActive} />
          </Tooltip>
        );
      case 'notification':
        return (
          <Tooltip key={key} title={t('shell.notificationCenter')}>
            <Popover
              content={
                <div>
                  <List
                    className="notification-list"
                    dataSource={inboxItems}
                    locale={{
                      emptyText: (
                        <Empty
                          image={Empty.PRESENTED_IMAGE_SIMPLE}
                          description={t('shell.notificationEmpty')}
                        />
                      ),
                    }}
                    renderItem={(item) => {
                      const priority =
                        item.kind === 'notice'
                          ? normalizeNoticePriority(item.priority)
                          : 'normal';
                      return (
                        <List.Item
                          className={[
                            item.read ? undefined : 'is-unread',
                            item.kind === 'notice' ? `is-priority-${priority}` : undefined,
                          ]
                            .filter(Boolean)
                            .join(' ')}
                          onClick={() => {
                            void markRead(item);
                            navigate('/messages/mine');
                          }}
                        >
                          <List.Item.Meta
                            avatar={
                              <Badge dot={!item.read}>
                                <Avatar icon={<Bell />} />
                              </Badge>
                            }
                            description={<small>{formatDateTime(item.at)}</small>}
                            title={item.title}
                          />
                        </List.Item>
                      );
                    }}
                  />
                  <Button type="link" block onClick={() => navigate('/messages/mine')}>
                    {t('shell.notificationViewAll')}
                  </Button>
                </div>
              }
              placement="bottomRight"
              title={t('shell.notificationPopup')}
              trigger="click"
            >
              <span className="widget-notification-trigger">
                <Badge count={unreadTotal} size="small" overflowCount={99}>
                  <Button className="widget-notification" icon={<Bell />} type="text" />
                </Badge>
              </span>
            </Popover>
          </Tooltip>
        );
      case 'lockScreenBtn':
        return (
          <Tooltip key={key} title={t('lock')}>
            <Button className="widget-lockScreenBtn" icon={<LockKeyhole />} type="text" onClick={lock} />
          </Tooltip>
        );
      case 'logoutBtn':
        return (
          <Tooltip key={key} title={t('logout')}>
            <Button className="widget-logoutBtn" icon={<LogOut />} type="text" onClick={doLogout} />
          </Tooltip>
        );
      default:
        return null;
    }
  };

  const orderKeys = fullWidgetOrder(prefs);
  const headerItems = orderKeys.map((key) => {
    if (!widgetEnabled(prefs, key)) return null;
    const pos = widgetPositionOf(prefs, key);
    const resolved = key === 'preferences' && pos === 'auto' ? 'header' : pos;
    return resolved === 'header' ? headerNode(key) : null;
  });

  const displayName = user?.username || user?.realName || '';
  const description = user?.email || '';
  const tenants = user?.tenants ?? [];
  const onSwitchTenant = (tenantId: number) => {
    if (!tenantId || tenantId === user?.tenantId) return;
    setSwitchingTenant(true);
    void switchTenant(tenantId)
      .then(() => {
        navigate(useAccessStore.getState().userInfo?.homePath || '/analytics', { replace: true });
      })
      .catch(() => {
        void message.error(t('shell.switchTenantFailed'));
      })
      .finally(() => setSwitchingTenant(false));
  };

  const dropdownItems: NonNullable<MenuProps['items']> = [
    { key: 'profile', icon: <UserRound size={15} />, label: t('profile'), onClick: () => navigate('/profile') },
  ];

  orderKeys.forEach((key) => {
    if (!widgetEnabled(prefs, key)) return;
    if (widgetPositionOf(prefs, key) !== 'user-dropdown') return;
    if (key === 'globalSearch') dropdownItems.push({ key, icon: <Search size={15} />, label: t('shell.globalSearch'), onClick: onSearch });
    if (key === 'preferences') dropdownItems.push({ key, icon: <Settings size={15} />, label: t('shell.preferences'), onClick: onPreferences });
    if (key === 'themeToggle') {
      dropdownItems.push({
        key,
        icon: dark ? <Sun size={15} /> : <Moon size={15} fill="currentColor" />,
        label: t('shell.themeToggle'),
        onClick: () => {
          const nextMode = dark ? ('light' as const) : ('dark' as const);
          const next = { ...prefs, theme: { ...prefs.theme, mode: nextMode } };
          runThemeViewTransition({ clientX: window.innerWidth - 48, clientY: 48 }, () => {
            applyPreferencesThemeNow(next);
            update({ theme: next.theme });
          });
        },
      });
    }
    if (key === 'languageToggle') {
      dropdownItems.push({
        key,
        icon: <Languages size={15} />,
        label: t('shell.languageToggle'),
        children: LOCALE_CODES.map((code) => ({
          key: code,
          label: t(`locale.${code}`),
          onClick: () => {
            setLocale(code);
          },
        })),
      });
    }
    if (key === 'timezone') {
      dropdownItems.push({
        key,
        icon: <Clock size={15} />,
        label: t(`timezone.${prefs.app.timezone}`, { defaultValue: prefs.app.timezone }),
      });
    }
    if (key === 'fullscreen') dropdownItems.push({ key, icon: <Maximize2 size={15} />, label: t('common.fullscreen'), onClick: toggleFullscreen });
    if (key === 'refresh') dropdownItems.push({ key, icon: <RefreshCw size={15} />, label: t('refresh'), onClick: refreshActive });
    if (key === 'notification') dropdownItems.push({ key, icon: <Bell size={15} />, label: t('shell.notificationCenter') });
    if (key === 'lockScreenBtn') dropdownItems.push({ key, icon: <LockKeyhole size={15} />, label: t('lock'), onClick: lock });
    if (key === 'logoutBtn') {
      dropdownItems.push({ type: 'divider' });
      dropdownItems.push({ key, icon: <LogOut size={15} />, label: t('logout'), onClick: doLogout });
    }
  });

  return (
    <Space align="center" className="header-widgets-inner" size={2}>
      {headerItems}
      {tenants.length > 1 ? (
        <Select
          aria-label={t('shell.switchTenant')}
          className="tenant-switcher"
          disabled={switchingTenant}
          loading={switchingTenant}
          options={tenants.map((item) => ({ label: item.name || item.code, value: item.id }))}
          popupMatchSelectWidth={false}
          size="small"
          value={user?.tenantId}
          variant="borderless"
          onChange={onSwitchTenant}
        />
      ) : null}
      <Dropdown
        getPopupContainer={() => document.body}
        menu={{ items: dropdownItems }}
        placement="bottomRight"
        popupRender={(menu) => (
          <div className="user-dropdown">
            <div className="user-dropdown-profile">
              <UserAvatar key={user?.avatar || 'profile'} size={32} src={user?.avatar} />
              <div className="user-dropdown-meta">
                <div className="user-dropdown-name">
                  <span className="user-dropdown-name-text" title={displayName}>{displayName}</span>
                </div>
                {description ? <div className="user-dropdown-desc" title={description}>{description}</div> : null}
              </div>
            </div>
            {menu}
          </div>
        )}
        rootClassName="user-dropdown-overlay"
        trigger={['click']}
      >
        <button aria-label={displayName || t('shell.userMenu')} className="user-avatar-trigger" type="button">
          <UserAvatar key={user?.avatar || 'header'} size={32} src={user?.avatar} />
        </button>
      </Dropdown>
    </Space>
  );
}

export function shouldShowPreferencesFloat(prefs: Preferences) {
  if (!prefs.app.enablePreferences) return false;
  const pos = prefs.app.preferencesButtonPosition;
  if (pos === 'none') return false;
  if (pos === 'fixed') return true;
  const headerHidden = !prefs.header.enable || prefs.header.hidden;
  return headerHidden;
}
