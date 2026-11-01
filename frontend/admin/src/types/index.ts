export type LayoutType = 'header-mixed-nav' | 'header-nav' | 'header-sidebar-nav' | 'mixed-nav' | 'sidebar-mixed-nav' | 'sidebar-nav';
export type ThemeMode = 'auto' | 'dark' | 'light';
export type LocaleCode = 'en-US' | 'zh-CN';
export type TabStyle = 'brisk' | 'card' | 'chrome' | 'plain';

export interface ApiEnvelope<T> { code: number; data: T; message: string }
export interface PagedResult<T> { items: T[]; page: number; pageSize: number; total: number }
export interface ListQuery { keyword?: string; page?: number; pageSize?: number; [key: string]: unknown }

export interface RouteMeta {
  affixTab?: boolean; hideInMenu?: boolean; icon?: string;
  keepAlive?: boolean; order?: number; title?: string;
}
export interface MenuRoute {
  children?: MenuRoute[]; component?: string; meta?: RouteMeta; name?: string;
  path: string; redirect?: string;
}
export interface AdminUserInfo {
  avatar: string; departmentName: string; email: string; homePath: string;
  isSuperAdmin: boolean; mobile: string; mustChangePassword: boolean; passwordMinLength: number;
  permissions: string[]; realName: string; roleNames: string[]; roles: string[];
  userId: string; username: string;
}
export interface TabDefinition {
  affix?: boolean; icon?: string; key: string; keepAlive?: boolean;
  path: string; title: string;
}

export type WidgetPosition = 'header' | 'none' | 'user-dropdown';
export type PreferencesButtonPosition = 'auto' | 'fixed' | 'header' | 'none' | 'user-dropdown';
export type WidgetKey =
  | 'fullscreen' | 'globalSearch' | 'languageToggle' | 'lockScreenBtn' | 'logoutBtn'
  | 'notification' | 'preferences' | 'refresh' | 'themeToggle' | 'timezone';

export interface Preferences {
  app: {
    authPageLayout: 'panel-center' | 'panel-left' | 'panel-right'; colorGrayMode: boolean;
    colorWeakMode: boolean; compact: boolean; contentCompact: 'compact' | 'wide';
    contentCompactWidth: number; contentPadding: number; defaultHomePath: string;
    dynamicTitle: boolean; enableCheckUpdates: boolean; enableCopyPreferences: boolean; enablePreferences: boolean;
    enableRefreshToken: boolean; enableStickyPreferencesNavigationBar: boolean; layout: LayoutType;
    locale: LocaleCode; loginExpiredMode: 'modal' | 'page'; name: string;
    preferencesButtonPosition: PreferencesButtonPosition; timezone: string;
    watermark: boolean; watermarkContent: string;
  };
  breadcrumb: { enable: boolean; hideOnlyOne: boolean; showHome: boolean; showIcon: boolean; styleType: 'background' | 'normal' };
  copyright: { companyName: string; companySiteLink: string; date: string; enable: boolean; icp: string; icpLink: string; settingShow: boolean };
  footer: { enable: boolean; fixed: boolean; height: number };
  header: { enable: boolean; height: number; hidden: boolean; menuAlign: 'center' | 'end' | 'start'; mode: 'fixed' | 'static' };
  navigation: { accordion: boolean; split: boolean; styleType: 'plain' | 'rounded' };
  sidebar: { autoActivateChild: boolean; collapsed: boolean; collapsedButton: boolean; collapsedShowTitle: boolean; collapseWidth: number; draggable: boolean; enable: boolean; expandOnHover: boolean; extraCollapse: boolean; extraCollapsedWidth: number; fixedButton: boolean; hidden: boolean; mixedWidth: number; width: number };
  tabbar: { draggable: boolean; enable: boolean; height: number; keepAlive: boolean; maxCount: number; middleClickToClose: boolean; persist: boolean; showIcon: boolean; showMaximize: boolean; showMore: boolean; showRefresh: boolean; styleType: TabStyle; visitHistory: boolean; wheelable: boolean };
  theme: { builtinType: string; colorDestructive: string; colorPrimary: string; colorSuccess: string; colorWarning: string; fontSize: number; mode: ThemeMode; radius: string; semiDarkHeader: boolean; semiDarkSidebar: boolean; semiDarkSidebarSub: boolean };
  transition: { enable: boolean; loading: boolean; name: string; progress: boolean };
  widget: {
    fullscreen: boolean; fullscreenButtonPosition: WidgetPosition; globalSearch: boolean; globalSearchButtonPosition: WidgetPosition;
    languageToggle: boolean; languageToggleButtonPosition: WidgetPosition; lockScreen: boolean; lockScreenButtonPosition: WidgetPosition;
    logoutButtonPosition: WidgetPosition; notificationButtonPosition: WidgetPosition;
    notification: boolean; order: WidgetKey[]; refresh: boolean; refreshButtonPosition: WidgetPosition; sidebarToggle: boolean;
    themeToggle: boolean; themeToggleButtonPosition: WidgetPosition; timezone: boolean; timezoneButtonPosition: WidgetPosition;
  };
  shortcut: { enable: boolean; escape: boolean; globalSearch: boolean; lockScreen: boolean; logout: boolean; preferences: boolean };
  custom: { defaultTableSize: number; enableFormFullscreen: boolean; reportTitle: string; tenantMode: 'multi' | 'single' };
}
