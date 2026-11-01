import { lazy, type ComponentType, type LazyExoticComponent } from 'react';

type LazyPage = LazyExoticComponent<ComponentType>;
const named = <T extends Record<string, ComponentType>>(loader: () => Promise<T>, name: keyof T) => lazy(async () => ({ default: (await loader())[name] }));

export const pageMap: Record<string, LazyPage> = {
  '/dashboard/analytics/index': named(() => import('@/pages/dashboard/AnalyticsPage'), 'AnalyticsPage'),
  '/dashboard/index': named(() => import('@/pages/dashboard/AnalyticsPage'), 'AnalyticsPage'),
  '/system/user/index': named(() => import('@/pages/system/users'), 'UserPage'),
  '/system/role/index': named(() => import('@/pages/system/users'), 'RolePage'),
  '/system/department/index': named(() => import('@/pages/system/organization'), 'DepartmentPage'),
  '/system/position/index': named(() => import('@/pages/system/organization'), 'PositionPage'),
  '/system/menu/index': named(() => import('@/pages/system/organization'), 'MenuPage'),
  '/system/dict/index': named(() => import('@/pages/system/settings'), 'DictionaryPage'),
  '/system/config/index': named(() => import('@/pages/system/settings'), 'ConfigPage'),
  '/system/auth-provider/index': named(() => import('@/pages/system/settings'), 'AuthProviderPage'),
  '/system/storage/index': named(() => import('@/pages/system/settings'), 'StorageConfigPage'),
  '/system/translation/index': named(() => import('@/pages/system/translation'), 'TranslationPage'),
  '/system/plugin/index': named(() => import('@/pages/system/plugins'), 'PluginPage'),
  '/example/index': named(() => import('@/pages/example'), 'ExamplePage'),
  '/system/notice/index': named(() => import('@/pages/system/content'), 'NoticePage'),
  '/system/message/index': named(() => import('@/pages/system/content'), 'MyMessagesPage'),
  '/system/file/index': named(() => import('@/pages/system/content'), 'FilePage'),
  '/system/file/recycle': named(() => import('@/pages/system/content'), 'FileRecyclePage'),
  '/system/file/references': named(() => import('@/pages/system/content'), 'FileReferencesPage'),
  '/system/security/online-user': named(() => import('@/pages/system/security'), 'OnlineUserPage'),
  '/system/security/token': named(() => import('@/pages/system/security'), 'TokenPage'),
  '/system/log/login': named(() => import('@/pages/system/security'), 'LoginLogPage'),
  '/system/log/operation': named(() => import('@/pages/system/security'), 'OperationLogPage'),
  '/system/log/error': named(() => import('@/pages/system/security'), 'ErrorLogPage'),
  '/system/job/index': named(() => import('@/pages/system/tools'), 'JobPage'),
  '/system/webhook/index': named(() => import('@/pages/system/tools'), 'WebhookPage'),
  '/system/cache/index': named(() => import('@/pages/system/tools'), 'CachePage'),
  '/profile/index': named(() => import('@/pages/profile/ProfilePage'), 'ProfilePage'),
};
