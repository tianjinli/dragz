import { cleanup, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import type { ReactNode } from 'react';
import { useAccessStore } from '@/stores/access';
import { defaultPreferences, usePreferencesStore } from '@/stores/preferences';
import type { MenuRoute } from '@/types';
import { useMixedMenu } from './useMixedMenu';

const menus: MenuRoute[] = [
  {
    path: '/dashboard',
    name: 'Dashboard',
    meta: { title: '仪表盘' },
  },
  {
    path: '/system',
    name: 'System',
    meta: { title: '系统管理' },
    children: [
      { path: '/system/users', name: 'Users', meta: { title: '用户管理' } },
      { path: '/system/hidden', name: 'Hidden', meta: { title: '隐藏', hideInMenu: true } },
      {
        path: '/system/org',
        name: 'Org',
        meta: { title: '组织' },
        children: [
          { path: '/system/org/depts', name: 'Depts', meta: { title: '部门' } },
          { path: '/system/org/secret', name: 'Secret', meta: { title: '隐藏', hideInMenu: true } },
        ],
      },
    ],
  },
];

function wrapper(initialPath: string) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <MemoryRouter initialEntries={[initialPath]}>{children}</MemoryRouter>;
  };
}

describe('useMixedMenu', () => {
  beforeEach(() => {
    useAccessStore.setState({ menus });
    usePreferencesStore.setState({ preferences: structuredClone(defaultPreferences) });
  });

  afterEach(() => {
    cleanup();
    useAccessStore.setState({ menus: [] });
    usePreferencesStore.setState({ preferences: structuredClone(defaultPreferences) });
  });

  it('exposes visible submenu children for dual-column layouts', () => {
    usePreferencesStore.setState({
      preferences: {
        ...structuredClone(defaultPreferences),
        app: { ...defaultPreferences.app, layout: 'sidebar-mixed-nav' },
      },
    });
    const { result } = renderHook(() => useMixedMenu(), { wrapper: wrapper('/system/users') });
    expect(result.current.sidebarNeedSplit).toBe(true);
    expect(result.current.railMenus.map((menu) => menu.path)).toEqual(['/dashboard', '/system']);
    expect(result.current.sidebarMenus.map((menu) => menu.path)).toEqual([
      '/system/users',
      '/system/org',
    ]);
  });

  it('returns an empty submenu list when the active root has no children', () => {
    usePreferencesStore.setState({
      preferences: {
        ...structuredClone(defaultPreferences),
        app: { ...defaultPreferences.app, layout: 'sidebar-mixed-nav' },
      },
    });
    const { result } = renderHook(() => useMixedMenu(), { wrapper: wrapper('/dashboard') });
    expect(result.current.sidebarNeedSplit).toBe(true);
    expect(result.current.sidebarMenus).toEqual([]);
  });

  it('for header-mixed-nav: rail = secondary, sider = tertiary when present', () => {
    usePreferencesStore.setState({
      preferences: {
        ...structuredClone(defaultPreferences),
        app: { ...defaultPreferences.app, layout: 'header-mixed-nav' },
      },
    });
    const { result } = renderHook(() => useMixedMenu(), { wrapper: wrapper('/system/org/depts') });
    expect(result.current.needSplit).toBe(true);
    expect(result.current.railMenus.map((menu) => menu.path)).toEqual([
      '/system/users',
      '/system/org',
    ]);
    expect(result.current.activeRailPath).toBe('/system/org');
    expect(result.current.sidebarMenus.map((menu) => menu.path)).toEqual(['/system/org/depts']);
    expect(result.current.sidebarMenus.every((menu) => !menu.meta?.hideInMenu)).toBe(true);
  });

  it('for header-mixed-nav: hides tertiary column when secondary has no children', () => {
    usePreferencesStore.setState({
      preferences: {
        ...structuredClone(defaultPreferences),
        app: { ...defaultPreferences.app, layout: 'header-mixed-nav' },
      },
    });
    const { result } = renderHook(() => useMixedMenu(), { wrapper: wrapper('/system/users') });
    expect(result.current.railMenus.map((menu) => menu.path)).toEqual([
      '/system/users',
      '/system/org',
    ]);
    expect(result.current.activeRailPath).toBe('/system/users');
    expect(result.current.sidebarMenus).toEqual([]);
  });

  it('keeps full menus for vertical sidebar-nav', () => {
    const { result } = renderHook(() => useMixedMenu(), { wrapper: wrapper('/system/users') });
    expect(result.current.sidebarNeedSplit).toBe(false);
    expect(result.current.sidebarMenus).toEqual(menus);
    expect(result.current.railMenus).toEqual([]);
  });
});
