import { describe, expect, it } from 'vitest';
import { findMenu, firstLeafPath, flattenMenus, menuAncestors, menuItems } from './menu-utils';
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

