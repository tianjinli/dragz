/** Same bounds as desktop sider edge-drag in AdminLayout. */
export const SIDER_WIDTH_MIN = 160;
export const SIDER_WIDTH_MAX = 320;

/**
 * Width for the mobile hamburger Drawer.
 * Uses the main / secondary sider width (preference or post-drag), not the mixed icon-rail width.
 */
export function resolveMobileDrawerWidth(siderWidth: number): number {
  return Math.min(SIDER_WIDTH_MAX, Math.max(SIDER_WIDTH_MIN, siderWidth));
}
