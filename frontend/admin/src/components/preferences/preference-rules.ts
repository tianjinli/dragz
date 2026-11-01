import { effectiveDark } from '@/stores/preferences';
import type { Preferences } from '@/types';

export interface PreferenceDisabledRules {
  breadcrumb: boolean;
  copyright: boolean;
  copyrightFields: boolean;
  darkHeader: boolean;
  darkSidebar: boolean;
  darkSidebarSub: boolean;
  footerFixed: boolean;
  header: boolean;
  navigationSplit: boolean;
  shortcutChildren: boolean;
  sidebar: boolean;
  tabbarChildren: boolean;
}

/**
 * Semi-dark sidebar/header only apply under light (effective) theme.
 * Explicit `dark` or `auto` resolving to system dark disables those switches (vben parity).
 */
export function getPreferenceDisabledRules(preferences: Preferences): PreferenceDisabledRules {
  const { layout } = preferences.app;
  const dark = effectiveDark(preferences);
  const sidebar = layout === 'header-nav';
  const breadcrumb = false;
  return {
    breadcrumb,
    copyright: !preferences.footer.enable,
    copyrightFields: !preferences.footer.enable || !preferences.copyright.enable,
    darkHeader: dark,
    darkSidebar: dark || sidebar,
    darkSidebarSub:
      dark ||
      !preferences.theme.semiDarkSidebar ||
      !['header-mixed-nav', 'sidebar-mixed-nav'].includes(layout),
    footerFixed: !preferences.footer.enable,
    header: false,
    navigationSplit: layout !== 'mixed-nav',
    shortcutChildren: !preferences.shortcut.enable,
    sidebar,
    tabbarChildren: !preferences.tabbar.enable,
  };
}
