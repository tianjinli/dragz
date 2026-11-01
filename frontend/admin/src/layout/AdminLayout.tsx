import { useEffect, useMemo, useRef, useState, type CSSProperties, type MouseEvent as ReactMouseEvent, type ReactNode } from 'react';
import { Avatar, Button, ConfigProvider, Drawer, Input, Layout, List, Menu, Modal, Typography, Watermark, theme as antdTheme } from 'antd';
import {
  Camera,
  ChevronLeft,
  ChevronRight,
  House,
  Menu as MenuIcon,
  Pin,
  PinOff,
  Search,
  Settings,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
import { AuthCopyright } from '@/components/auth/AuthCopyright';
import { CheckUpdates } from '@/components/CheckUpdates';
import { PreferencesDrawer } from '@/components/preferences/PreferencesDrawer';
import { useMixedMenu } from '@/hooks/useMixedMenu';
import { findMenu, firstLeafPath, flattenMenus, iconNode, menuAncestors, menuIcon, menuItems } from '@/router/menu-utils';
import { LOGOUT_REDIRECT_FLAG, useAuthStore } from '@/stores/auth';
import { useAccessStore } from '@/stores/access';
import { useLockScreenStore } from '@/stores/lock-screen';
import { effectiveDark, usePreferencesStore } from '@/stores/preferences';
import { syncTabsPersistence, useTabsStore } from '@/stores/tabs';
import type { MenuRoute } from '@/types';
import { confirmLogout } from '@/utils/confirm-logout';
import { CachedViews } from './CachedViews';
import { HeaderWidgets, shouldShowPreferencesFloat } from './HeaderWidgets';
import {
  isHeaderFullWidthLayout,
  isSidebarToggleEnabled,
  resolveHeaderChrome,
  shouldShowMobileHamburger,
} from './header-chrome';
import { resolveMixedSiderVisibility } from './mixed-sider-visibility';
import {
  resolveMobileDrawerWidth,
  SIDER_WIDTH_MAX,
  SIDER_WIDTH_MIN,
} from './resolve-mobile-drawer-width';
import { TabsBar } from './TabsBar';

const { Header, Sider, Content, Footer } = Layout;
const SEMI_DARK_BG = '#151a23';
/** Stable empty openKeys for collapsed inline menus — avoids antd Overflow/Menu update loops. */
const EMPTY_OPEN_KEYS: string[] = [];

function sameOpenKeys(prev: string[], next: string[]) {
  return prev.length === next.length && prev.every((key, index) => key === next[index]);
}

function SemiDarkScope({ active, children }: { active: boolean; children: ReactNode }) {
  if (!active) return children;
  return (
    <ConfigProvider
      theme={{
        algorithm: antdTheme.darkAlgorithm,
        token: {
          colorBgContainer: SEMI_DARK_BG,
          colorBgElevated: '#1c2330',
          colorBgLayout: SEMI_DARK_BG,
        },
        components: {
          Layout: { headerBg: SEMI_DARK_BG, siderBg: SEMI_DARK_BG },
          Menu: {
            darkItemBg: SEMI_DARK_BG,
            darkSubMenuItemBg: SEMI_DARK_BG,
            darkItemSelectedBg: 'rgba(255,255,255,0.08)',
            darkItemHoverBg: 'rgba(255,255,255,0.06)',
            itemBg: 'transparent',
            subMenuItemBg: 'transparent',
          },
        },
      }}
    >
      {children}
    </ConfigProvider>
  );
}

function Brand({ compact = false, titleOnly = false }: { compact?: boolean; titleOnly?: boolean }) {
  const prefs = usePreferencesStore((state) => state.preferences);
  const showLogo = !titleOnly;
  const showTitle = titleOnly || !compact;
  return (
    <div className={`brand ${compact ? 'brand-compact' : ''} ${titleOnly ? 'brand-title-only' : ''}`}>
      {showLogo ? <img alt={prefs.app.name} className="brand-mark" height={31} src="/favicon.svg" width={31} /> : null}
      {showTitle ? <strong>{prefs.app.name}</strong> : null}
    </div>
  );
}

function SidebarToggleButton({
  mobileMode = false,
  mobileOpen = false,
  onMobileToggle,
}: {
  mobileMode?: boolean;
  mobileOpen?: boolean;
  onMobileToggle?: () => void;
}) {
  const { t } = useTranslation();
  const prefs = usePreferencesStore((state) => state.preferences);
  const update = usePreferencesStore((state) => state.update);
  // 显示侧边栏 off → toggle stays visible but looks disabled and does nothing.
  const enabled = isSidebarToggleEnabled(prefs.sidebar.enable);
  // Mobile: header toggle opens/closes the drawer (same control as vertical layout).
  const sidebarVisible = enabled && (mobileMode ? mobileOpen : !prefs.sidebar.hidden);
  const iconPath = sidebarVisible
    ? 'M896 192H128v128h768zm0 256H384v128h512zm0 256H128v128h768zM320 384L128 512l192 128z'
    : 'M128 192h768v128H128zm0 256h512v128H128zm0 256h768v128H128zm576-352l192 160l-192 128z';

  return (
    <button
      aria-disabled={!enabled}
      aria-label={sidebarVisible ? t('shell.hideSidebar') : t('shell.showSidebar')}
      aria-pressed={!sidebarVisible}
      className={`sidebar-toggle-btn${!enabled ? ' is-disabled' : ''}`}
      data-layout-action="toggle-sidebar"
      disabled={!enabled}
      type="button"
      onClick={() => {
        if (!enabled) return;
        if (mobileMode) {
          onMobileToggle?.();
          return;
        }
        update({ sidebar: { ...prefs.sidebar, hidden: !prefs.sidebar.hidden } });
      }}
    >
      <svg aria-hidden="true" className="iconify iconify--ep" height="1em" role="img" viewBox="0 0 1024 1024" width="1em" xmlns="http://www.w3.org/2000/svg">
        <path fill="currentColor" d={iconPath} />
      </svg>
    </button>
  );
}

function BreadcrumbBar({ menus }: { menus: MenuRoute[] }) {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const prefs = usePreferencesStore((state) => state.preferences);
  const trail = menuAncestors(menus, location.pathname);
  if (prefs.breadcrumb.hideOnlyOne && trail.length <= 1) return null;

  const crumbs: Array<{ key: string; icon?: ReactNode; label?: string; onClick?: () => void }> = trail.map((menu) => ({
    key: menu.path,
    icon: prefs.breadcrumb.showIcon && menu.meta?.icon ? iconNode(menu.meta.icon) : undefined,
    label: menu.meta?.title || menu.name,
  }));
  if (prefs.breadcrumb.showHome) {
    crumbs.unshift({
      key: 'home',
      icon: <House size={16} strokeWidth={1.75} />,
      onClick: () => navigate(prefs.app.defaultHomePath),
    });
  }

  const background = prefs.breadcrumb.styleType === 'background';

  return (
    <div className={`breadcrumb-row breadcrumb-${background ? 'background' : 'normal'}`}>
      <ul className={background ? 'breadcrumb-chevron' : 'breadcrumb-plain'}>
        {crumbs.map((crumb, index) => {
          const last = index === crumbs.length - 1;
          const inner = (
            <span className={background ? 'breadcrumb-chevron-inner' : 'breadcrumb-plain-inner'}>
              {crumb.icon ? <span className="breadcrumb-item-icon">{crumb.icon}</span> : null}
              {crumb.label ? <span className="breadcrumb-item-label">{crumb.label}</span> : null}
            </span>
          );
          return (
            <li key={crumb.key}>
              {crumb.onClick && !last ? (
                <button aria-label={crumb.label || t('shell.home')} className={background ? 'breadcrumb-chevron-link' : 'breadcrumb-plain-link'} type="button" onClick={crumb.onClick}>
                  {inner}
                </button>
              ) : (
                <span className={`${background ? 'breadcrumb-chevron-link' : 'breadcrumb-plain-link'}${last ? ' is-current' : ''}`}>{inner}</span>
              )}
              {!last && !background ? (
                <span aria-hidden className="breadcrumb-plain-sep">
                  <ChevronRight size={14} strokeWidth={2} />
                </span>
              ) : null}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function GlobalSearch({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const menus = useAccessStore((state) => state.menus);
  const navigate = useNavigate();
  const [value, setValue] = useState('');
  const result = flattenMenus(menus).filter(
    (menu) => menu.component && `${menu.meta?.title || menu.name}${menu.path}`.toLowerCase().includes(value.toLowerCase()),
  );
  return (
    <Modal className="search-modal" footer={null} mask={{ closable: true }} open={open} title={t('shell.globalSearchTitle')} onCancel={onClose}>
      <Input
        autoFocus
        placeholder={t('shell.globalSearchPlaceholder')}
        prefix={<Search />}
        size="large"
        value={value}
        onChange={(event) => setValue(event.target.value)}
      />
      <List
        dataSource={result}
        locale={{ emptyText: t('shell.noMatchingMenu') }}
        renderItem={(menu) => (
          <List.Item
            className="search-result"
            onClick={() => {
              navigate(menu.path);
              onClose();
            }}
          >
            <List.Item.Meta avatar={iconNode(menu.meta?.icon)} description={menu.path} title={menu.meta?.title || menu.name} />
          </List.Item>
        )}
      />
    </Modal>
  );
}

function LockScreen() {
  const { t } = useTranslation();
  const locked = useLockScreenStore((state) => state.locked);
  const lock = useLockScreenStore((state) => state.lock);
  const unlock = useLockScreenStore((state) => state.unlock);
  const user = useAccessStore((state) => state.userInfo);
  const [setup, setSetup] = useState(false);
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const fn = () => setSetup(true);
    window.addEventListener('admin:lock', fn);
    return () => window.removeEventListener('admin:lock', fn);
  }, []);

  const doUnlock = async () => {
    setBusy(true);
    try {
      if (await unlock(password)) setPassword('');
      else void Modal.error({ title: t('shell.wrongPassword'), content: t('shell.wrongPasswordDesc') });
    } finally {
      setBusy(false);
    }
  };

  const doLock = async () => {
    if (!password) return;
    setBusy(true);
    try {
      await lock(password);
      setSetup(false);
      setPassword('');
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <Modal
        confirmLoading={busy}
        open={setup}
        title={t('shell.setLockPassword')}
        onCancel={() => setSetup(false)}
        onOk={() => void doLock()}
      >
        <Input.Password
          autoFocus
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          onPressEnter={() => void doLock()}
        />
      </Modal>
      {locked ? (
        <div className="lock-screen">
          <div className="lock-card">
            <Avatar icon={<Camera />} size={80} src={user?.avatar || undefined} />
            <Typography.Title level={3}>{user?.realName || user?.username}</Typography.Title>
            <Typography.Paragraph type="secondary">{t('shell.screenLocked')}</Typography.Paragraph>
            <Input.Password
              autoFocus
              placeholder={t('shell.lockPassword')}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              onPressEnter={() => void doUnlock()}
            />
            <Button block loading={busy} type="primary" onClick={() => void doUnlock()}>
              {t('unlock')}
            </Button>
          </div>
        </div>
      ) : null}
    </>
  );
}

export function AdminLayout() {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const menus = useAccessStore((state) => state.menus);
  const prefs = usePreferencesStore((state) => state.preferences);
  const updatePrefs = usePreferencesStore((state) => state.update);
  const tabs = useTabsStore((state) => state.tabs);
  const maximized = useTabsStore((state) => state.maximized);
  const addTab = useTabsStore((state) => state.add);
  const dark = effectiveDark(prefs);
  const semiDarkSidebar = !dark && prefs.theme.semiDarkSidebar;
  const semiDarkSidebarSub = !dark && prefs.theme.semiDarkSidebarSub;
  const semiDarkHeader = !dark && prefs.theme.semiDarkHeader;
  const isHeaderNavLayout = prefs.app.layout === 'header-nav';
  const isHeaderSidebarNavLayout = prefs.app.layout === 'header-sidebar-nav';
  const {
    showHeaderBrand,
    showHeaderBreadcrumbInMenuSlot,
    showHeaderBreadcrumbInHeaderLeft,
    showHeaderSidebarToggle,
    sidebarToggleAtFarLeft,
    sidebarToggleInHeaderLeft,
  } = resolveHeaderChrome({
    layout: prefs.app.layout,
    breadcrumbEnable: prefs.breadcrumb.enable,
    maximized,
    sidebarToggleWidget: prefs.widget.sidebarToggle,
  });
  const showMobileHamburger = shouldShowMobileHamburger(showHeaderSidebarToggle);
  const siderMenuTheme = dark || semiDarkSidebar ? 'dark' : 'light';
  const headerMenuTheme = dark || semiDarkHeader ? 'dark' : 'light';
  const [prefOpen, setPrefOpen] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [isMobileLayout, setIsMobileLayout] = useState(() =>
    typeof window !== 'undefined' ? window.matchMedia('(max-width: 900px)').matches : false,
  );
  const [siderWidth, setSiderWidth] = useState(prefs.sidebar.width);
  const [hoverExpanded, setHoverExpanded] = useState(false);
  const [openKeys, setOpenKeys] = useState<string[]>([]);
  const siderWidthRef = useRef(siderWidth);
  siderWidthRef.current = siderWidth;
  const mixed = useMixedMenu();
  const sideMixed = mixed.isSidebarMixedNav || mixed.isHeaderMixedNav;

  const currentMenu = findMenu(menus, location.pathname);
  const fullPath = location.pathname + location.search;
  const hoverExpandWhole = hoverExpanded && !prefs.sidebar.expandOnHover;
  const collapsed = prefs.sidebar.collapsed && !hoverExpandWhole;
  const popupSubmenus = collapsed && prefs.sidebar.expandOnHover;
  const showHeader = prefs.header.enable && !prefs.header.hidden && !maximized;
  const siderBase =
    prefs.sidebar.enable && !prefs.sidebar.hidden && !maximized && prefs.app.layout !== 'header-nav';
  // Dual-column (双列): never show menu titles while collapsed.
  const showCollapsedTitle = prefs.sidebar.collapsedShowTitle && !mixed.isSidebarMixedNav;
  const { showMixedRail, showSubMenuColumn, showSider } = resolveMixedSiderVisibility({
    siderBase,
    sideMixed,
    sidebarNeedSplit: mixed.sidebarNeedSplit,
    sidebarMenuCount: mixed.sidebarMenus.length,
    // 混合双列 rail = 二级; hide when the active 一级 has no children.
    railMenuCount: mixed.railMenus.length,
    hideEmptyRail: mixed.isHeaderMixedNav,
  });
  const showSiderResizer =
    showSubMenuColumn &&
    prefs.sidebar.draggable &&
    !collapsed &&
    !isMobileLayout;
  const mobileDrawerWidth = resolveMobileDrawerWidth(siderWidth);
  const headerNav = ['header-nav', 'header-mixed-nav', 'mixed-nav'].includes(prefs.app.layout) && (prefs.app.layout !== 'mixed-nav' || mixed.needSplit);
  const headerMode = prefs.header.mode === 'static' ? 'static' : 'fixed';
  // header-mixed-nav: keep rail + tertiary full-height (header lives in workspace).
  const headerFullWidth = isHeaderFullWidthLayout(prefs.app.layout);
  const headerAboveWorkspace = showHeader && headerFullWidth && headerMode === 'fixed';
  const overlayRef = useRef(false);
  overlayRef.current = searchOpen || prefOpen || mobileOpen;

  useEffect(() => {
    setSiderWidth(prefs.sidebar.width);
  }, [prefs.sidebar.width]);

  useEffect(() => {
    syncTabsPersistence(prefs.tabbar.persist);
  }, [prefs.tabbar.persist]);

  const startSiderResize = (event: ReactMouseEvent) => {
    event.preventDefault();
    const startX = event.clientX;
    const startWidth = siderWidthRef.current;
    const onMove = (moveEvent: MouseEvent) => {
      const next = Math.min(
        SIDER_WIDTH_MAX,
        Math.max(SIDER_WIDTH_MIN, startWidth + (moveEvent.clientX - startX)),
      );
      siderWidthRef.current = next;
      setSiderWidth(next);
    };
    const onUp = () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
      document.body.style.removeProperty('cursor');
      document.body.style.removeProperty('user-select');
      const width = siderWidthRef.current;
      const sidebar = usePreferencesStore.getState().preferences.sidebar;
      updatePrefs({ sidebar: { ...sidebar, width } });
    };
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  };

  // Match the 900px CSS breakpoint: hide horizontal menu in React (not only CSS)
  // so Ant Design Overflow remounts cleanly when returning to desktop width.
  useEffect(() => {
    const media = window.matchMedia('(max-width: 900px)');
    const onChange = () => {
      const mobile = media.matches;
      setIsMobileLayout(mobile);
      if (!mobile) setMobileOpen(false);
    };
    onChange();
    media.addEventListener('change', onChange);
    return () => media.removeEventListener('change', onChange);
  }, []);

  useEffect(() => {
    if (location.pathname === '/' && menus.length) {
      navigate(prefs.app.defaultHomePath, { replace: true });
    }
  }, [location.pathname, menus.length, navigate, prefs.app.defaultHomePath]);

  useEffect(() => {
    const title = currentMenu?.meta?.title || (location.pathname === '/profile' ? t('profile') : t('shell.page'));
    addTab(
      {
        affix: !!currentMenu?.meta?.affixTab,
        icon: currentMenu?.meta?.icon || (location.pathname === '/profile' ? 'lucide:user-round' : undefined),
        key: fullPath,
        keepAlive: currentMenu?.meta?.keepAlive !== false,
        path: fullPath,
        title,
      },
      prefs.tabbar.maxCount,
    );
    document.title = prefs.app.dynamicTitle ? `${title} - ${prefs.app.name}` : prefs.app.name;
  }, [
    addTab,
    currentMenu?.meta?.affixTab,
    currentMenu?.meta?.icon,
    currentMenu?.meta?.keepAlive,
    currentMenu?.meta?.title,
    fullPath,
    location.pathname,
    prefs.app.dynamicTitle,
    prefs.app.name,
    prefs.tabbar.maxCount,
    t,
  ]);

  useEffect(() => {
    const ancestors = menuAncestors(menus, location.pathname).map((menu) => menu.path);
    setOpenKeys((prev) => {
      const next = prefs.navigation.accordion
        ? ancestors
        : Array.from(new Set([...prev, ...ancestors]));
      return sameOpenKeys(prev, next) ? prev : next;
    });
  }, [location.pathname, menus, prefs.navigation.accordion]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (useLockScreenStore.getState().locked) return;
      const shortcuts = usePreferencesStore.getState().preferences.shortcut;
      if (!shortcuts.enable) return;
      const mod = event.ctrlKey || event.metaKey;
      if (mod && event.key.toLowerCase() === 'k' && shortcuts.globalSearch) {
        event.preventDefault();
        setSearchOpen(true);
      }
      if (mod && event.key === ',' && shortcuts.preferences && prefs.app.enablePreferences) {
        event.preventDefault();
        setPrefOpen(true);
      }
      if (event.altKey && event.key.toLowerCase() === 'l' && shortcuts.lockScreen) {
        event.preventDefault();
        window.dispatchEvent(new Event('admin:lock'));
      }
      if (event.altKey && event.key.toLowerCase() === 'q' && shortcuts.logout) {
        event.preventDefault();
        confirmLogout(async () => {
          await useAuthStore.getState().logout();
          navigate('/auth/login', { replace: true });
        });
      }
      if (event.key === 'Escape') {
        const store = useTabsStore.getState();
        if (store.maximized) {
          store.setMaximized(false);
          return;
        }
        if (overlayRef.current) {
          setSearchOpen(false);
          setPrefOpen(false);
          setMobileOpen(false);
        } else if (shortcuts.escape) {
          if (store.tabs.length > 1) {
            const active = store.tabs.find((tab) => tab.key === store.activeKey);
            if (active) navigate(store.close(active.key));
          }
        }
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [navigate, prefs.app.enablePreferences]);

  useEffect(() => {
    const onStorage = (event: StorageEvent) => {
      if (!event.key?.includes('-access')) return;
      if (!useAccessStore.getState().accessToken && event.newValue) return;
      const next = event.newValue ? (JSON.parse(event.newValue) as { state?: { accessToken?: string | null } }) : null;
      if (!next?.state?.accessToken && useAccessStore.getState().accessToken) {
        sessionStorage.setItem(LOGOUT_REDIRECT_FLAG, '1');
        useAccessStore.setState({
          accessCodes: [],
          accessToken: null,
          checked: true,
          menus: [],
          userInfo: null,
        });
        useTabsStore.setState({ activeKey: '', tabs: [], visitHistory: [], refreshKeys: {}, maximized: false });
        navigate('/auth/login', { replace: true });
      }
    };
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, [navigate]);

  const selectedKeys = useMemo(() => [location.pathname], [location.pathname]);
  const headerSelectedKeys = useMemo(
    () => (mixed.needSplit ? [mixed.activeRootPath] : selectedKeys),
    [mixed.activeRootPath, mixed.needSplit, selectedKeys],
  );
  const headerNavMenuStyle =
    isHeaderNavLayout || mixed.isHeaderMixedNav || (mixed.isMixedNav && mixed.needSplit);
  const sideMenuClass = `admin-menu nav-${prefs.navigation.styleType}`;
  const menuClass = `${sideMenuClass}${headerNavMenuStyle ? ' header-nav-menu' : ''}`;
  const sideMenuSource = mixed.sidebarNeedSplit || sideMixed ? mixed.sidebarMenus : menus;
  const topMenuSource = headerNav ? mixed.headerMenus : menus;
  // Narrow drawer always mirrors vertical layout: full menu tree (icon + label + chevrons).
  const mobileMenuSource = menus;
  // Memoize items: antd horizontal Menu Overflow dead-loops when `data` is a new array every render.
  const sideMenuItems = useMemo(() => menuItems(sideMenuSource), [sideMenuSource]);
  const topMenuItems = useMemo(() => menuItems(topMenuSource), [topMenuSource]);
  const mobileMenuItems = useMemo(() => menuItems(mobileMenuSource), [mobileMenuSource]);
  const menuOpenKeys = collapsed && !popupSubmenus ? EMPTY_OPEN_KEYS : openKeys;

  const onOpenChange = (keys: string[]) => {
    const latest = keys.find((key) => !openKeys.includes(key));
    if (latest && prefs.sidebar.autoActivateChild) {
      const menu = findMenu(menus, latest);
      if (menu?.children?.some((child) => !child.meta?.hideInMenu)) {
        navigate(firstLeafPath(menu));
      }
    }
    const applyKeys = (next: string[]) => {
      setOpenKeys((prev) => (sameOpenKeys(prev, next) ? prev : next));
    };
    if (!prefs.navigation.accordion) {
      applyKeys(keys);
      return;
    }
    if (!latest) {
      applyKeys(keys);
      return;
    }
    // Drawer uses the full tree; desktop sider uses the active column source.
    const accordionMenus = isMobileLayout ? menus : sideMenuSource;
    const root = accordionMenus.find(
      (menu) => menu.path === latest || (menu.children || []).some((child) => child.path === latest),
    );
    if (root && latest === root.path) {
      applyKeys([latest]);
      return;
    }
    applyKeys(keys);
  };

  const menuClick = ({ key }: { key: string }) => {
    // Desktop header split only — narrow drawer uses the full vertical tree.
    if (
      !isMobileLayout &&
      headerNav &&
      mixed.needSplit &&
      mixed.headerMenus.some((menu) => menu.path === key)
    ) {
      navigate(mixed.resolveRootNavigation(key));
      setMobileOpen(false);
      return;
    }
    if (prefs.sidebar.autoActivateChild) {
      const menu = findMenu(menus, key);
      if (menu?.children?.some((child) => !child.meta?.hideInMenu)) {
        navigate(firstLeafPath(menu));
        setMobileOpen(false);
        return;
      }
    }
    navigate(key);
    setMobileOpen(false);
  };

  const sideMenu = (
    <Menu
      className={sideMenuClass}
      inlineCollapsed={showSider && collapsed}
      items={sideMenuItems}
      mode="inline"
      openKeys={menuOpenKeys}
      selectedKeys={selectedKeys}
      subMenuOpenDelay={0.08}
      subMenuCloseDelay={0.12}
      theme={siderMenuTheme}
      triggerSubMenuAction={popupSubmenus ? 'hover' : 'click'}
      onClick={menuClick}
      onOpenChange={onOpenChange}
    />
  );

  const topMenu = (
    <Menu
      className={menuClass}
      items={topMenuItems}
      mode="horizontal"
      selectedKeys={headerSelectedKeys}
      theme={headerMenuTheme}
      onClick={menuClick}
    />
  );

  const mobileDrawerMenu = (
    <Menu
      className={sideMenuClass}
      items={mobileMenuItems}
      mode="inline"
      openKeys={openKeys}
      selectedKeys={selectedKeys}
      theme={siderMenuTheme}
      onClick={menuClick}
      onOpenChange={onOpenChange}
    />
  );

  const sidebarToggle = (
    <SidebarToggleButton
      mobileMode={isMobileLayout}
      mobileOpen={mobileOpen}
      onMobileToggle={() => setMobileOpen((open) => !open)}
    />
  );

  const header = showHeader ? (
    <SemiDarkScope active={semiDarkHeader}>
      <Header
        className={`admin-header mode-${headerMode} ${semiDarkHeader ? 'semi-dark' : ''}`}
        style={{
          height: prefs.header.height,
          lineHeight: 'normal',
          ['--header-height' as string]: `${prefs.header.height}px`,
          ...(semiDarkHeader ? { background: SEMI_DARK_BG } : {}),
        }}
      >
        {showMobileHamburger ? (
          <Button className="mobile-menu" icon={<MenuIcon />} type="text" onClick={() => setMobileOpen(true)} />
        ) : null}
        {/* mixed-nav / header-mixed-nav: collapse toggle left of primary (一级) menu */}
        {sidebarToggleAtFarLeft ? (
          <div className="header-left header-left-toggle-only">
            {sidebarToggle}
          </div>
        ) : null}
        {showHeaderBrand ? <Brand /> : null}
        {headerNav && !isMobileLayout ? (
          <div className={`header-menu align-${prefs.header.menuAlign}`}>{topMenu}</div>
        ) : null}
        {showHeaderBreadcrumbInMenuSlot ? (
          <div className="header-breadcrumb">
            <BreadcrumbBar menus={menus} />
          </div>
        ) : null}
        {sidebarToggleInHeaderLeft || showHeaderBreadcrumbInHeaderLeft ? (
          <div className="header-left">
            {sidebarToggleInHeaderLeft ? sidebarToggle : null}
            {showHeaderBreadcrumbInHeaderLeft ? <BreadcrumbBar menus={menus} /> : null}
          </div>
        ) : null}
        <div className="header-widgets">
          <HeaderWidgets onPreferences={() => setPrefOpen(true)} onSearch={() => setSearchOpen(true)} />
        </div>
      </Header>
    </SemiDarkScope>
  ) : null;

  const content = (
    <Content className="admin-content" style={{ padding: prefs.app.contentPadding }}>
      <div
        className={prefs.app.contentCompact === 'compact' ? 'content-compact' : ''}
        style={{ maxWidth: prefs.app.contentCompact === 'compact' ? prefs.app.contentCompactWidth : undefined }}
      >
        <CachedViews activePath={fullPath} menus={menus} tabs={tabs} />
      </div>
    </Content>
  );

  const footerFixed = Boolean(prefs.footer.enable && prefs.footer.fixed && !maximized);
  const footerEl =
    prefs.footer.enable && !maximized ? (
      <Footer className={footerFixed ? 'footer-fixed' : ''} style={{ height: prefs.footer.height }}>
        {prefs.copyright.enable ? <AuthCopyright {...prefs.copyright} /> : null}
      </Footer>
    ) : null;
  const chrome = (
    <>
      {showHeader && !headerAboveWorkspace ? header : null}
      {prefs.tabbar.enable ? <TabsBar /> : null}
    </>
  );
  const body = content;
  const workspace = (
    <Layout className={`workspace header-mode-${headerMode}${footerFixed ? ' footer-is-fixed' : ''}`}>
      {headerMode === 'static' && footerFixed ? (
        <>
          <div className="workspace-scroll">
            {chrome}
            {body}
          </div>
          {footerEl}
        </>
      ) : headerMode === 'fixed' && footerEl && !footerFixed ? (
        <>
          {chrome}
          <div className="workspace-scroll">
            {body}
            {footerEl}
          </div>
        </>
      ) : (
        <>
          {chrome}
          {body}
          {footerEl}
        </>
      )}
    </Layout>
  );

  const watermarkContent = (prefs.app.watermarkContent || '').trim() || prefs.app.name;
  const shell = (
    <Layout
      className={`admin-layout layout-${prefs.app.layout}`}
      style={
        isHeaderNavLayout || isHeaderSidebarNavLayout
          ? ({ ['--sider-width' as string]: `${siderWidth}px` } as CSSProperties)
          : undefined
      }
    >
      {headerAboveWorkspace ? header : null}
      {/*
        Force hasSider when only the mixed icon rail is shown (secondary Sider hidden).
        Ant Design Layout defaults to flex-direction:column unless a Sider is present;
        without hasSider the rail stacks above the header/workspace.
      */}
      <Layout hasSider={showSider || undefined}>
        {showSider ? (
          <>
            {/* In dual-column layouts the rail is the main sidebar; the
                adjacent submenu Sider is the sidebar-sub region. */}
            {showMixedRail ? (
              <SemiDarkScope active={sideMixed ? semiDarkSidebar : semiDarkSidebarSub}>
                <div
                  className={`mixed-menu-rail ${
                    mixed.isSidebarMixedNav || mixed.isHeaderMixedNav ? 'mixed-menu-rail-sidebar' : ''
                  } ${(sideMixed ? semiDarkSidebar : semiDarkSidebarSub) ? 'semi-dark' : ''}`}
                  style={{
                    width: prefs.sidebar.mixedWidth,
                    flexBasis: prefs.sidebar.mixedWidth,
                    ...((sideMixed ? semiDarkSidebar : semiDarkSidebarSub) ? { background: SEMI_DARK_BG } : {}),
                  }}
                >
                  <Brand compact />
                  <div className="mixed-rail-dots">
                    {mixed.railMenus.map((item) => (
                      <button
                        aria-current={mixed.activeRailPath === item.path ? 'page' : undefined}
                        aria-label={item.meta?.title || item.name}
                        className={mixed.activeRailPath === item.path ? 'active' : ''}
                        key={item.path}
                        type="button"
                        onClick={() => navigate(mixed.resolveRootNavigation(item.path))}
                      >
                        {menuIcon(item, true)}
                        {/* 双列 / 混合双列: always show icon+label; other rails follow collapsedShowTitle. */}
                        {mixed.isSidebarMixedNav || mixed.isHeaderMixedNav || showCollapsedTitle ? (
                          <small className="mixed-rail-label">{item.meta?.title || item.name}</small>
                        ) : null}
                      </button>
                    ))}
                  </div>
                </div>
              </SemiDarkScope>
            ) : null}
            {showSubMenuColumn ? (
            <SemiDarkScope active={sideMixed ? semiDarkSidebarSub : semiDarkSidebar}>
              <Sider
                className={`${(sideMixed ? semiDarkSidebarSub : semiDarkSidebar) ? 'semi-dark' : ''} ${showCollapsedTitle ? 'show-title' : ''}`}
                collapsed={collapsed}
                collapsedWidth={
                  showCollapsedTitle && collapsed
                    ? Math.max(prefs.sidebar.collapseWidth, 80)
                    : prefs.sidebar.collapseWidth
                }
                style={(sideMixed ? semiDarkSidebarSub : semiDarkSidebar) ? { background: SEMI_DARK_BG } : undefined}
                trigger={null}
                width={siderWidth}
                onMouseEnter={() => {
                  // Unpinned: hover expands the whole collapsed sidebar.
                  if (!prefs.sidebar.expandOnHover && prefs.sidebar.collapsed) setHoverExpanded(true);
                }}
                onMouseLeave={() => {
                  if (!prefs.sidebar.expandOnHover) {
                    setHoverExpanded(false);
                    if (!prefs.sidebar.collapsed) {
                      updatePrefs({ sidebar: { ...prefs.sidebar, collapsed: true } });
                    }
                  }
                }}
              >
                <div className="sider-inner">
                  {!headerNav && !showHeaderBrand ? (
                    mixed.isSidebarMixedNav
                      ? // Dual-column secondary: title only when expanded; hide entirely when collapsed.
                        collapsed
                          ? null
                          : <Brand titleOnly />
                      : <Brand compact={collapsed} />
                  ) : mixed.isHeaderMixedNav && !collapsed ? (
                    // 混合双列 tertiary column mirrors dual-column secondary title strip.
                    <Brand titleOnly />
                  ) : null}
                  <div className="sider-menu-scroll">{sideMenu}</div>
                  {prefs.sidebar.collapsedButton || (prefs.sidebar.fixedButton && !collapsed) ? (
                    <div className={`sider-actions${collapsed ? ' is-collapsed' : ''}`}>
                      {prefs.sidebar.collapsedButton ? (
                        <Button
                          className="sider-collapse"
                          icon={collapsed ? <ChevronRight size={16} /> : <ChevronLeft size={16} />}
                          type="text"
                          onClick={() => {
                            setHoverExpanded(false);
                            updatePrefs({ sidebar: { ...prefs.sidebar, collapsed: !collapsed } });
                          }}
                        />
                      ) : (
                        <span />
                      )}
                      {prefs.sidebar.fixedButton && !collapsed ? (
                        <Button
                          className="sider-fixed"
                          icon={prefs.sidebar.expandOnHover ? <Pin size={14} /> : <PinOff size={14} />}
                          type="text"
                          onClick={() => {
                            const nextPinned = !prefs.sidebar.expandOnHover;
                            setHoverExpanded(false);
                            updatePrefs({
                              sidebar: {
                                ...prefs.sidebar,
                                expandOnHover: nextPinned,
                                ...(nextPinned && hoverExpanded ? { collapsed: false } : {}),
                              },
                            });
                          }}
                        />
                      ) : null}
                    </div>
                  ) : null}
                </div>
                {showSiderResizer ? (
                  <div
                    aria-label={t('shell.resizeSidebar')}
                    className="sider-resizer"
                    role="separator"
                    onMouseDown={startSiderResize}
                  />
                ) : null}
              </Sider>
            </SemiDarkScope>
            ) : null}
          </>
        ) : null}
        {workspace}
      </Layout>
      {shouldShowPreferencesFloat(prefs) && !prefOpen ? (
        <Button className="pref-float" icon={<Settings size={17} />} type="text" onClick={() => setPrefOpen(true)} />
      ) : null}
      <Drawer
        className="mobile-drawer"
        closable={false}
        destroyOnHidden
        open={mobileOpen}
        placement="left"
        rootClassName="mobile-drawer-root"
        rootStyle={{ ['--mobile-drawer-width' as string]: `${mobileDrawerWidth}px` }}
        size={mobileDrawerWidth}
        styles={{ wrapper: { width: mobileDrawerWidth } }}
        title={null}
        onClose={() => setMobileOpen(false)}
      >
        <SemiDarkScope active={semiDarkSidebar}>
          <div
            className={`sider-inner mobile-drawer-sider${semiDarkSidebar ? ' semi-dark' : ''}`}
            style={semiDarkSidebar ? { background: SEMI_DARK_BG, height: '100%' } : { height: '100%' }}
          >
            {/* Match vertical sider: brand + menu only — no collapse/pin actions in the drawer. */}
            <Brand />
            <div className="sider-menu-scroll">{mobileDrawerMenu}</div>
          </div>
        </SemiDarkScope>
      </Drawer>
      <GlobalSearch open={searchOpen} onClose={() => setSearchOpen(false)} />
      <LockScreen />
      {prefs.app.enableCheckUpdates ? <CheckUpdates /> : null}
    </Layout>
  );

  return (
    <>
      {prefs.app.watermark ? (
        <Watermark className="app-watermark" content={watermarkContent}>
          {shell}
        </Watermark>
      ) : shell}
      {/* This must remain outside the conditional Watermark wrapper. Switching
          watermark on/off then keeps the open drawer mounted. */}
      {prefs.app.enablePreferences ? <PreferencesDrawer open={prefOpen} onClose={() => setPrefOpen(false)} /> : null}
    </>
  );
}
