import { describe, expect, it } from 'vitest';
import {
  isHeaderFullWidthLayout,
  isSidebarToggleEnabled,
  resolveHeaderChrome,
  shouldShowMobileHamburger,
} from './header-chrome';
import type { LayoutType } from '@/types';

const base = {
  breadcrumbEnable: true,
  maximized: false,
  sidebarToggleWidget: true,
};

describe('resolveHeaderChrome', () => {
  it('for mixed-nav: hides breadcrumb, shows brand, places toggle at far left', () => {
    const result = resolveHeaderChrome({ ...base, layout: 'mixed-nav' });
    expect(result.showHeaderBrand).toBe(true);
    expect(result.showHeaderBreadcrumbInHeaderLeft).toBe(false);
    expect(result.showHeaderBreadcrumbInMenuSlot).toBe(false);
    expect(result.showHeaderSidebarToggle).toBe(true);
    expect(result.sidebarToggleAtFarLeft).toBe(true);
    expect(result.sidebarToggleInHeaderLeft).toBe(false);
  });

  it('for header-mixed-nav: toggle left of primary menu, no brand or breadcrumb', () => {
    const result = resolveHeaderChrome({ ...base, layout: 'header-mixed-nav' });
    expect(result.showHeaderBrand).toBe(false);
    expect(result.showHeaderSidebarToggle).toBe(true);
    expect(result.sidebarToggleAtFarLeft).toBe(true);
    expect(result.sidebarToggleInHeaderLeft).toBe(false);
    expect(result.showHeaderBreadcrumbInHeaderLeft).toBe(false);
    expect(result.showHeaderBreadcrumbInMenuSlot).toBe(false);
  });

  it('for sidebar-nav: keeps breadcrumb and toggle in header-left, no header brand', () => {
    const result = resolveHeaderChrome({ ...base, layout: 'sidebar-nav' });
    expect(result.showHeaderBrand).toBe(false);
    expect(result.showHeaderBreadcrumbInHeaderLeft).toBe(true);
    expect(result.sidebarToggleAtFarLeft).toBe(false);
    expect(result.sidebarToggleInHeaderLeft).toBe(true);
  });

  it('for header-nav: shows brand, hides breadcrumb and sidebar toggle', () => {
    const result = resolveHeaderChrome({ ...base, layout: 'header-nav' });
    expect(result.showHeaderBrand).toBe(true);
    expect(result.showHeaderBreadcrumbInHeaderLeft).toBe(false);
    expect(result.showHeaderSidebarToggle).toBe(false);
  });

  it('for header-sidebar-nav: brand + breadcrumb in menu slot, no toggle', () => {
    const result = resolveHeaderChrome({ ...base, layout: 'header-sidebar-nav' });
    expect(result.showHeaderBrand).toBe(true);
    expect(result.showHeaderBreadcrumbInMenuSlot).toBe(true);
    expect(result.showHeaderSidebarToggle).toBe(false);
  });

  it('for sidebar-mixed-nav: no header brand or toggle', () => {
    const result = resolveHeaderChrome({ ...base, layout: 'sidebar-mixed-nav' });
    expect(result.showHeaderBrand).toBe(false);
    expect(result.showHeaderSidebarToggle).toBe(false);
    expect(result.showHeaderBreadcrumbInHeaderLeft).toBe(true);
    expect(result.sidebarToggleAtFarLeft).toBe(false);
  });

  it('respects maximized and widget flags for mixed-nav', () => {
    expect(
      resolveHeaderChrome({
        ...base,
        layout: 'mixed-nav',
        maximized: true,
      }).showHeaderSidebarToggle,
    ).toBe(false);
    expect(
      resolveHeaderChrome({
        ...base,
        layout: 'mixed-nav',
        sidebarToggleWidget: false,
      }).showHeaderSidebarToggle,
    ).toBe(false);
  });

  it('does not treat sidebar-mixed-nav like mixed-nav toggle placement', () => {
    const result = resolveHeaderChrome({ ...base, layout: 'sidebar-mixed-nav' as LayoutType });
    expect(result.sidebarToggleAtFarLeft).toBe(false);
    expect(result.showHeaderBrand).toBe(false);
  });
});

describe('isHeaderFullWidthLayout', () => {
  it('keeps header above sider for header-nav, header-sidebar-nav, and mixed-nav', () => {
    expect(isHeaderFullWidthLayout('header-nav')).toBe(true);
    expect(isHeaderFullWidthLayout('header-sidebar-nav')).toBe(true);
    expect(isHeaderFullWidthLayout('mixed-nav')).toBe(true);
  });

  it('places header beside full-height rail/tertiary for header-mixed-nav', () => {
    expect(isHeaderFullWidthLayout('header-mixed-nav')).toBe(false);
  });

  it('matches sidebar layouts that already use full-height siders', () => {
    expect(isHeaderFullWidthLayout('sidebar-nav')).toBe(false);
    expect(isHeaderFullWidthLayout('sidebar-mixed-nav')).toBe(false);
  });
});

describe('shouldShowMobileHamburger', () => {
  it('hides hamburger when vertical header sidebar toggle is present', () => {
    expect(shouldShowMobileHamburger(true)).toBe(false);
  });

  it('shows hamburger when header has no sidebar toggle', () => {
    expect(shouldShowMobileHamburger(false)).toBe(true);
  });
});

describe('isSidebarToggleEnabled', () => {
  it('is interactive when 显示侧边栏 (sidebar.enable) is on', () => {
    expect(isSidebarToggleEnabled(true)).toBe(true);
  });

  it('is unavailable when 显示侧边栏 is off', () => {
    expect(isSidebarToggleEnabled(false)).toBe(false);
  });
});
