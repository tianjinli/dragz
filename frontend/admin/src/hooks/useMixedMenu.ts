import { useMemo } from 'react';
import { useLocation } from 'react-router-dom';
import { findMenu, firstLeafPath, menuAncestors } from '@/router/menu-utils';
import { useAccessStore } from '@/stores/access';
import { usePreferencesStore } from '@/stores/preferences';
import type { MenuRoute } from '@/types';

function visibleChildren(menu?: MenuRoute): MenuRoute[] {
  return (menu?.children || []).filter((child) => !child.meta?.hideInMenu);
}

export function useMixedMenu() {
  const menus = useAccessStore((state) => state.menus);
  const location = useLocation();
  const prefs = usePreferencesStore((state) => state.preferences);
  const layout = prefs.app.layout;
  const isMixedNav = layout === 'mixed-nav';
  const isHeaderMixedNav = layout === 'header-mixed-nav';
  const isSidebarMixedNav = layout === 'sidebar-mixed-nav';
  const needSplit = (prefs.navigation.split && isMixedNav) || isHeaderMixedNav;
  /** Column-2 submenu: active root's children (mixed-nav split, sidebar two-column). */
  const sidebarNeedSplit = needSplit || isSidebarMixedNav;

  const trail = useMemo(() => menuAncestors(menus, location.pathname), [menus, location.pathname]);
  const rootMenu = useMemo(
    () => trail[0] || menus.find((menu) => menu.path === location.pathname),
    [trail, menus, location.pathname],
  );
  /** Header-mixed rail highlights the active secondary (二级) item. */
  const secondaryMenu = isHeaderMixedNav ? trail[1] : undefined;

  /** Visible children of the active root — dual-column submenu / header-mixed rail (二级). */
  const splitSideMenus = useMemo(() => visibleChildren(rootMenu), [rootMenu]);
  /** Header-mixed second column: tertiary (三级) under the active secondary. */
  const tertiaryMenus = useMemo(() => visibleChildren(secondaryMenu), [secondaryMenu]);

  const headerMenus = useMemo(() => {
    if (!needSplit) return menus;
    return menus.map((item) => ({ ...item, children: undefined }));
  }, [menus, needSplit]);

  /**
   * Icon rail items:
   * - sidebar-mixed-nav: top-level (一级)
   * - header-mixed-nav: secondary under active root (二级)
   */
  const railMenus = useMemo(() => {
    if (isSidebarMixedNav) return menus.filter((menu) => !menu.meta?.hideInMenu);
    if (isHeaderMixedNav) return splitSideMenus;
    return [];
  }, [isHeaderMixedNav, isSidebarMixedNav, menus, splitSideMenus]);

  const sidebarMenus = useMemo(() => {
    // 混合双列: secondary sider shows tertiary when the active 二级 has children.
    if (isHeaderMixedNav) return tertiaryMenus;
    if (sidebarNeedSplit) return splitSideMenus;
    return menus;
  }, [isHeaderMixedNav, menus, sidebarNeedSplit, splitSideMenus, tertiaryMenus]);

  const activeRootPath = rootMenu?.path || location.pathname;
  const activeRailPath = isHeaderMixedNav
    ? secondaryMenu?.path || ''
    : activeRootPath;

  const resolveRootNavigation = (rootPath: string) => {
    const root = findMenu(menus, rootPath) || menus.find((menu) => menu.path === rootPath);
    if (!root?.children?.some((child) => !child.meta?.hideInMenu)) return rootPath;
    if (prefs.sidebar.autoActivateChild) return firstLeafPath(root);
    const firstChild = (root.children || []).find((child) => !child.meta?.hideInMenu);
    return firstChild?.path || rootPath;
  };

  return {
    activeRailPath,
    activeRootPath,
    headerMenus,
    isHeaderMixedNav,
    isMixedNav,
    isSidebarMixedNav,
    needSplit,
    railMenus,
    resolveRootNavigation,
    sidebarMenus,
    sidebarNeedSplit,
    splitSideMenus,
    tertiaryMenus,
  };
}

export function useActiveMenu(path = ''): MenuRoute | undefined {
  const menus = useAccessStore((state) => state.menus);
  return findMenu(menus, path);
}
