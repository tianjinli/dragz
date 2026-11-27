import { describe, expect, it, vi } from 'vitest';
import { findMenu, firstLeafPath, flattenMenus, isExternalPath, menuAncestors, menuItems, openExternalPath } from './menu-utils';
import type { MenuRoute } from '@/types';

const menus: MenuRoute[] = [
  {
    path: '/organization',
    name: 'Organization',
    meta: { title: '组织权限' },
    children: [{ path: '/organization/users', name: 'Users', component: '/system/user/index', meta: { title: '用户管理', icon: 'lucide:users' } }],
  },
];

describe('dynamic menus', () => {
  it('flattens and resolves backend component routes', () => {
    expect(flattenMenus(menus)).toHaveLength(2);
    expect(findMenu(menus, '/organization/users')?.component).toBe('/system/user/index');
  });

  it('builds breadcrumb ancestors and menu items', () => {
    expect(menuAncestors(menus, '/organization/users').map((menu) => menu.path)).toEqual(['/organization', '/organization/users']);
    expect(menuItems(menus)).toHaveLength(1);
  });

  it('assigns a first-level icon when the backend icon is missing', () => {
    const items = menuItems(menus) || [];
    const first = items[0];
    expect(first).toMatchObject({ key: '/organization' });
    expect(first && 'icon' in first && first.icon).toBeTruthy();
  });

  it('resolves the first leaf path through nested children', () => {
    const tree: MenuRoute = {
      path: '/root',
      name: 'Root',
      children: [
        {
          path: '/root/child',
          name: 'Child',
          children: [{ path: '/root/child/leaf', name: 'Leaf', component: '/x' }],
        },
      ],
    };
    expect(firstLeafPath(tree)).toBe('/root/child/leaf');
  });
});

describe('external menu targets', () => {
  it('recognises absolute http(s) paths only', () => {
    expect(isExternalPath('https://example.com/docs')).toBe(true);
    expect(isExternalPath('http://example.com')).toBe(true);
    expect(isExternalPath('HTTPS://EXAMPLE.COM')).toBe(true);
    expect(isExternalPath('/organization/users')).toBe(false);
    expect(isExternalPath('/api/system/users')).toBe(false);
    expect(isExternalPath('')).toBe(false);
  });

  it('opens the target in a new tab without handing over the opener', () => {
    const open = vi.spyOn(window, 'open').mockImplementation(() => null);
    openExternalPath('https://example.com/docs');
    expect(open).toHaveBeenCalledWith('https://example.com/docs', '_blank', 'noopener,noreferrer');
    open.mockRestore();
  });

  it('renders an off-site label as an anchor so the browser previews the URL', () => {
    const items = menuItems([
      { path: 'https://example.com/docs', name: 'Docs', meta: { title: '外部文档' } },
      { path: '/organization', name: 'Org', meta: { title: '组织' } },
    ])!;
    const external = items[0] as unknown as {
      label: { type: string; props: Record<string, unknown> };
    };
    expect(external.label.type).toBe('a');
    expect(external.label.props.href).toBe('https://example.com/docs');
    expect(external.label.props.target).toBe('_blank');
    expect(external.label.props.rel).toBe('noopener noreferrer');

    // The anchor cancels its own navigation; the menu's onClick opens the tab.
    const preventDefault = vi.fn();
    (external.label.props.onClick as (event: { preventDefault: () => void }) => void)({ preventDefault });
    expect(preventDefault).toHaveBeenCalled();

    expect((items[1] as unknown as { label: string }).label).toBe('组织');
  });
});

