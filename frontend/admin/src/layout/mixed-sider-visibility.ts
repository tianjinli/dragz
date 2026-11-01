/** Dual-column / mixed sidebar visibility: icon rail vs secondary submenu column. */
export type MixedSiderVisibility = {
  showMixedRail: boolean;
  showSubMenuColumn: boolean;
  /** Pass to Ant Design Layout `hasSider` so rail-only mode stays row layout. */
  showSider: boolean;
};

export function resolveMixedSiderVisibility(input: {
  siderBase: boolean;
  sideMixed: boolean;
  sidebarNeedSplit: boolean;
  sidebarMenuCount: number;
  /**
   * Header-mixed rail lists secondary menus; hide the rail when none exist.
   * Omit / leave undefined for sidebar-mixed (rail always shows roots).
   */
  railMenuCount?: number;
  /** When true, empty railMenuCount hides the icon rail (header-mixed-nav). */
  hideEmptyRail?: boolean;
}): MixedSiderVisibility {
  // Two-column layouts only need the submenu sider when the active parent has children.
  // Keep the icon rail even when the submenu column is empty (unless hideEmptyRail).
  const hasSubMenuColumn = !input.sidebarNeedSplit || input.sidebarMenuCount > 0;
  const railHasItems = !input.hideEmptyRail || (input.railMenuCount ?? 0) > 0;
  const showMixedRail = input.sideMixed && input.siderBase && railHasItems;
  const showSubMenuColumn = input.siderBase && hasSubMenuColumn;
  return {
    showMixedRail,
    showSubMenuColumn,
    showSider: showMixedRail || showSubMenuColumn,
  };
}
