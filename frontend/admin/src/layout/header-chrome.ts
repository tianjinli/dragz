import type { LayoutType } from '@/types';

/** Header brand / breadcrumb / sider-toggle visibility by layout mode. */
export type HeaderChrome = {
  showHeaderBrand: boolean;
  showHeaderBreadcrumbInMenuSlot: boolean;
  showHeaderBreadcrumbInHeaderLeft: boolean;
  showHeaderSidebarToggle: boolean;
  /** mixed-nav / header-mixed-nav: toggle before brand or primary menu (far left). */
  sidebarToggleAtFarLeft: boolean;
  /** Other layouts: toggle lives in header-left with optional breadcrumb. */
  sidebarToggleInHeaderLeft: boolean;
};

export function resolveHeaderChrome(input: {
  layout: LayoutType;
  breadcrumbEnable: boolean;
  maximized: boolean;
  sidebarToggleWidget: boolean;
}): HeaderChrome {
  const isHeaderNav = input.layout === 'header-nav';
  const isHeaderSidebarNav = input.layout === 'header-sidebar-nav';
  const isMixedNav = input.layout === 'mixed-nav';
  const isHeaderMixedNav = input.layout === 'header-mixed-nav';

  const showHeaderBrand = isHeaderNav || isHeaderSidebarNav || isMixedNav;
  const showHeaderBreadcrumbInMenuSlot =
    input.breadcrumbEnable && !input.maximized && isHeaderSidebarNav;
  // mixed-nav / header-mixed-nav: never show breadcrumb — primary menu takes that space.
  const showHeaderBreadcrumbInHeaderLeft =
    input.breadcrumbEnable &&
    !input.maximized &&
    !isHeaderNav &&
    !isHeaderSidebarNav &&
    !isMixedNav &&
    !isHeaderMixedNav;
  const showHeaderSidebarToggle =
    input.sidebarToggleWidget &&
    !input.maximized &&
    !['header-nav', 'sidebar-mixed-nav', 'header-sidebar-nav'].includes(input.layout);

  // 混合垂直 / 混合双列: collapse toggle sits left of the primary (一级) menu.
  const sidebarToggleAtFarLeft =
    (isMixedNav || isHeaderMixedNav) && showHeaderSidebarToggle;

  return {
    showHeaderBrand,
    showHeaderBreadcrumbInMenuSlot,
    showHeaderBreadcrumbInHeaderLeft,
    showHeaderSidebarToggle,
    sidebarToggleAtFarLeft,
    sidebarToggleInHeaderLeft: showHeaderSidebarToggle && !sidebarToggleAtFarLeft,
  };
}

/**
 * Whether a fixed header spans the full viewport width above sider + workspace.
 *
 * `header-mixed-nav` (混合双列) is excluded so the icon rail and tertiary column
 * stay full-height beside the header — same shell as 双列菜单 / 侧边导航.
 */
export function isHeaderFullWidthLayout(layout: LayoutType): boolean {
  return layout === 'header-nav' || layout === 'header-sidebar-nav' || layout === 'mixed-nav';
}

/**
 * Mobile hamburger opens the drawer only when the vertical-layout sidebar toggle
 * is not already in the header (that toggle opens/closes the drawer instead).
 */
export function shouldShowMobileHamburger(showHeaderSidebarToggle: boolean): boolean {
  return !showHeaderSidebarToggle;
}

/**
 * Header expand/collapse (显示/隐藏侧边栏) toggle is interactive only when
 * preference 显示侧边栏 (`sidebar.enable`) is on. Layouts without a sider
 * already hide the toggle via `resolveHeaderChrome`.
 */
export function isSidebarToggleEnabled(sidebarEnable: boolean): boolean {
  return sidebarEnable;
}
