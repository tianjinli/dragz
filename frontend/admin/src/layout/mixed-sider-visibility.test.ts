import { describe, expect, it } from 'vitest';
import { resolveMixedSiderVisibility } from './mixed-sider-visibility';

describe('resolveMixedSiderVisibility', () => {
  it('keeps the icon rail and hasSider when dual-column has no submenu children', () => {
    const result = resolveMixedSiderVisibility({
      siderBase: true,
      sideMixed: true,
      sidebarNeedSplit: true,
      sidebarMenuCount: 0,
    });
    expect(result).toEqual({
      showMixedRail: true,
      showSubMenuColumn: false,
      showSider: true,
    });
  });

  it('shows both rail and secondary column when submenu children exist', () => {
    const result = resolveMixedSiderVisibility({
      siderBase: true,
      sideMixed: true,
      sidebarNeedSplit: true,
      sidebarMenuCount: 2,
    });
    expect(result).toEqual({
      showMixedRail: true,
      showSubMenuColumn: true,
      showSider: true,
    });
  });

  it('hides both columns when the sidebar base is off', () => {
    const result = resolveMixedSiderVisibility({
      siderBase: false,
      sideMixed: true,
      sidebarNeedSplit: true,
      sidebarMenuCount: 2,
    });
    expect(result.showMixedRail).toBe(false);
    expect(result.showSubMenuColumn).toBe(false);
    expect(result.showSider).toBe(false);
  });

  it('shows only the secondary sider for non-mixed layouts', () => {
    const result = resolveMixedSiderVisibility({
      siderBase: true,
      sideMixed: false,
      sidebarNeedSplit: false,
      sidebarMenuCount: 0,
    });
    expect(result).toEqual({
      showMixedRail: false,
      showSubMenuColumn: true,
      showSider: true,
    });
  });

  it('for header-mixed: hides empty secondary rail but keeps tertiary column when present', () => {
    const emptyRail = resolveMixedSiderVisibility({
      siderBase: true,
      sideMixed: true,
      sidebarNeedSplit: true,
      sidebarMenuCount: 0,
      railMenuCount: 0,
      hideEmptyRail: true,
    });
    expect(emptyRail).toEqual({
      showMixedRail: false,
      showSubMenuColumn: false,
      showSider: false,
    });

    const railOnly = resolveMixedSiderVisibility({
      siderBase: true,
      sideMixed: true,
      sidebarNeedSplit: true,
      sidebarMenuCount: 0,
      railMenuCount: 3,
      hideEmptyRail: true,
    });
    expect(railOnly).toEqual({
      showMixedRail: true,
      showSubMenuColumn: false,
      showSider: true,
    });

    const both = resolveMixedSiderVisibility({
      siderBase: true,
      sideMixed: true,
      sidebarNeedSplit: true,
      sidebarMenuCount: 2,
      railMenuCount: 3,
      hideEmptyRail: true,
    });
    expect(both).toEqual({
      showMixedRail: true,
      showSubMenuColumn: true,
      showSider: true,
    });
  });
});
