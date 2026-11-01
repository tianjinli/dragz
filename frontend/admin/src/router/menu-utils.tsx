import {
  Bell,
  BookKey,
  BriefcaseBusiness,
  Clock3,
  DatabaseZap,
  Files,
  FolderOpen,
  History,
  KeyRound,
  Layers,
  LayoutDashboard,
  ListTree,
  LogIn,
  Mail,
  Megaphone,
  Menu as MenuIcon,
  MonitorSmartphone,
  Network,
  Settings,
  Shield,
  ShieldCheck,
  SlidersHorizontal,
  Trash2,
  TriangleAlert,
  Unplug,
  UserRound,
  UserRoundCog,
  Users,
  Webhook,
  Wrench,
} from 'lucide-react';
import { icons as lucideIconsSource } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import type { MenuProps } from 'antd';
import type { MenuRoute } from '@/types';

const iconMap: Record<string, React.ComponentType<{ size?: number }>> = {
  bell: Bell,
  'book-key': BookKey,
  'briefcase-business': BriefcaseBusiness,
  'clock-3': Clock3,
  'database-zap': DatabaseZap,
  files: Files,
  'folder-open': FolderOpen,
  history: History,
  'key-round': KeyRound,
  layers: Layers,
  'layout-dashboard': LayoutDashboard,
  'list-tree': ListTree,
  'log-in': LogIn,
  mail: Mail,
  megaphone: Megaphone,
  'monitor-smartphone': MonitorSmartphone,
  network: Network,
  settings: Settings,
  shield: Shield,
  'shield-check': ShieldCheck,
  'sliders-horizontal': SlidersHorizontal,
  'trash-2': Trash2,
  'triangle-alert': TriangleAlert,
  unplug: Unplug,
  'user-round': UserRound,
  'user-round-cog': UserRoundCog,
  users: Users,
  webhook: Webhook,
  wrench: Wrench,
};

const rootIconByPath: Record<string, string> = {
  '/analytics': 'layout-dashboard',
  '/organization': 'shield-check',
  '/settings': 'settings',
  '/messages': 'bell',
  '/files': 'folder-open',
  '/security': 'shield',
  '/tools': 'wrench',
};

/* i18n-exempt-start: these needles match backend menu titles (data, not UI copy) */
const rootIconHints: Array<[string, string]> = [
  ['工作台', 'layout-dashboard'],
  ['analytics', 'layout-dashboard'],
  ['dashboard', 'layout-dashboard'],
  ['组织', 'shield-check'],
  ['权限', 'shield-check'],
  ['organization', 'shield-check'],
  ['设置', 'settings'],
  ['settings', 'settings'],
  ['消息', 'bell'],
  ['通知', 'bell'],
  ['message', 'bell'],
  ['文件', 'folder-open'],
  ['file', 'folder-open'],
  ['安全', 'shield'],
  ['审计', 'shield'],
  ['security', 'shield'],
  ['工具', 'wrench'],
  ['tool', 'wrench'],
];
/* i18n-exempt-end */

function iconKey(name?: string) {
  return (name || '').replace(/^lucide:/, '');
}

function mappedIcon(name?: string) {
  const key = iconKey(name);
  return key && iconMap[key] ? key : undefined;
}

function pascalToKebab(name: string) {
  return name.replace(/([a-z0-9])([A-Z])/g, '$1-$2').replace(/([A-Z])([A-Z][a-z])/g, '$1-$2').toLowerCase();
}

function kebabToPascal(name: string) {
  return name.replace(/(^|-)([a-z0-9])/g, (_, __, ch) => ch.toUpperCase());
}

export interface LucideIconOption { Component: LucideIcon; search: string; value: string }

const lucideIcons = lucideIconsSource as Record<string, LucideIcon>;

// 全量 lucide 图标清单（供图标选择器使用，值为 `lucide:<kebab-case>`）。
const lucideIconOptions: LucideIconOption[] = Object.keys(lucideIcons).map((pascal) => {
  const kebab = pascalToKebab(pascal);
  return { Component: lucideIcons[pascal], search: `${kebab} ${pascal}`, value: `lucide:${kebab}` };
});

export function getLucideIconList(): LucideIconOption[] {
  return lucideIconOptions;
}

function fallbackRootIcon(menu: MenuRoute) {
  if (rootIconByPath[menu.path]) return rootIconByPath[menu.path];
  const haystack = `${menu.meta?.title || ''} ${menu.name || ''} ${menu.path}`.toLowerCase();
  const hit = rootIconHints.find(([needle]) => haystack.includes(needle.toLowerCase()));
  return hit?.[1] || 'layers';
}

export function iconNode(name?: string) {
  const Icon = iconMap[iconKey(name)] || lucideIcons[kebabToPascal(iconKey(name))] || MenuIcon;
  return <Icon size={17} />;
}

export function menuIcon(menu: MenuRoute, root = false) {
  const mapped = mappedIcon(menu.meta?.icon);
  if (mapped) return iconNode(mapped);
  if (root) return iconNode(fallbackRootIcon(menu));
  return iconNode(menu.meta?.icon);
}

export function flattenMenus(menus: MenuRoute[]): MenuRoute[] {
  return menus.flatMap((menu) => [menu, ...flattenMenus(menu.children || [])]);
}

export function findMenu(menus: MenuRoute[], path: string) {
  return flattenMenus(menus).find((menu) => menu.path === path);
}

/** path -> menu title, used to relabel open tabs after a language change. */
export function menuTitlesByPath(menus: MenuRoute[]): Record<string, string> {
  const titles: Record<string, string> = {};
  for (const menu of flattenMenus(menus)) {
    if (menu.meta?.title) titles[menu.path] = menu.meta.title;
  }
  return titles;
}

/** Deepest first visible leaf under a menu (child → grandchild → …). */
export function firstLeafPath(menu: MenuRoute): string {
  const children = (menu.children || []).filter((child) => !child.meta?.hideInMenu);
  if (!children.length) return menu.path;
  return firstLeafPath(children[0]!);
}

export function menuItems(menus: MenuRoute[], root = true): MenuProps['items'] {
  return menus
    .filter((menu) => !menu.meta?.hideInMenu)
    .map((menu) => ({
      key: menu.path,
      icon: menuIcon(menu, root),
      label: menu.meta?.title || menu.name || menu.path,
      children: menu.children?.length ? menuItems(menu.children, false) : undefined,
    }));
}

export function menuAncestors(menus: MenuRoute[], path: string, trail: MenuRoute[] = []): MenuRoute[] {
  for (const menu of menus) {
    const next = [...trail, menu];
    if (menu.path === path) return next;
    const found = menuAncestors(menu.children || [], path, next);
    if (found.length) return found;
  }
  return [];
}
