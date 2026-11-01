import { describe, expect, it } from 'vitest';
import {
  resolveMobileDrawerWidth,
  SIDER_WIDTH_MAX,
  SIDER_WIDTH_MIN,
} from './resolve-mobile-drawer-width';

describe('resolveMobileDrawerWidth', () => {
  it('uses the current sider width (preference / drag value)', () => {
    expect(resolveMobileDrawerWidth(224)).toBe(224);
    expect(resolveMobileDrawerWidth(268)).toBe(268);
  });

  it('clamps to the same min/max as desktop sider drag', () => {
    expect(resolveMobileDrawerWidth(SIDER_WIDTH_MIN - 40)).toBe(SIDER_WIDTH_MIN);
    expect(resolveMobileDrawerWidth(SIDER_WIDTH_MAX + 80)).toBe(SIDER_WIDTH_MAX);
  });

  it('ignores mixed rail width — caller passes secondary/main sider width only', () => {
    // Dual-column desktop uses mixedWidth (rail) + siderWidth (submenu).
    // Mobile drawer renders a flat menu, so width is siderWidth alone.
    expect(resolveMobileDrawerWidth(224)).toBe(224);
  });
});
