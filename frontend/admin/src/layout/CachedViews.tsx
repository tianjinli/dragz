import { Suspense, useEffect, useMemo } from 'react';
import { KeepAlive, useKeepAliveRef } from 'keepalive-for-react';
import { useTranslation } from 'react-i18next';
import { dropPendingGetDedupes } from '@/api/request';
import { ForbiddenPage, NotFoundPage, UnknownComponentPage } from '@/pages/fallback/FallbackPages';
import { pageMap } from '@/router/page-map';
import { findMenu, menuTitlesByPath } from '@/router/menu-utils';
import { useAccessStore } from '@/stores/access';
import { useAuthStore } from '@/stores/auth';
import { usePreferencesStore } from '@/stores/preferences';
import { useTabsStore } from '@/stores/tabs';
import type { MenuRoute, TabDefinition } from '@/types';

function componentFor(path: string, menus: MenuRoute[]) {
  if (path.split('?')[0] === '/profile') return pageMap['/profile/index'];
  const menu = findMenu(menus, path.split('?')[0]!);
  return menu?.component ? pageMap[menu.component] : undefined;
}

function PageNode({
  menus,
  refreshKey,
  tab,
}: {
  menus: MenuRoute[];
  refreshKey: number;
  tab: TabDefinition;
}) {
  const { t } = useTranslation();
  const transition = usePreferencesStore((state) => state.preferences.transition);
  const pathname = tab.path.split('?')[0]!;
  const Component = componentFor(tab.path, menus);
  const menu = findMenu(menus, pathname);
  const isProfile = pathname === '/profile';

  return (
    <div className={transition.enable ? `transition-${transition.name}` : undefined} key={refreshKey}>
      <Suspense
        fallback={
          transition.loading ? (
            <div className="view-loading">
              {transition.progress ? <div className="route-progress" /> : null}
              {t('shell.loadingPage')}
            </div>
          ) : null
        }
      >
        {Component ? (
          <Component />
        ) : menu ? (
          <UnknownComponentPage component={menu.component} />
        ) : isProfile ? (
          <NotFoundPage />
        ) : (
          <ForbiddenPage />
        )}
      </Suspense>
    </div>
  );
}

export function CachedViews({
  activePath,
  menus,
  tabs,
}: {
  activePath: string;
  menus: MenuRoute[];
  tabs: TabDefinition[];
}) {
  const prefs = usePreferencesStore((state) => state.preferences);
  const refreshKeys = useTabsStore((state) => state.refreshKeys);
  const aliveRef = useKeepAliveRef();
  const activeTab = tabs.find((tab) => tab.key === activePath) || tabs.find((tab) => tab.path === activePath);
  const globalKeepAlive = prefs.tabbar.keepAlive;
  const tabKeepAlive = activeTab?.keepAlive !== false;
  const shouldCache = globalKeepAlive && tabKeepAlive;
  const max = prefs.tabbar.maxCount > 0 ? prefs.tabbar.maxCount : 20;
  const exclude = useMemo(
    () => tabs.filter((tab) => tab.keepAlive === false).map((tab) => tab.key),
    [tabs],
  );

  useEffect(() => {
    const unsubscribe = useTabsStore.subscribe((state, prev) => {
      if (state.localeEpoch === prev.localeEpoch) return;
      void (async () => {
        if (!useAccessStore.getState().accessToken) return;
        // Menus, permission codes and the profile are backend-rendered text:
        // refetch them under the new Accept-Language before the pages remount.
        dropPendingGetDedupes();
        await useAuthStore.getState().loadAccess().catch(() => undefined);
        // Cached pages hold strings resolved under the previous language, so
        // every open tab remounts (the active cache entry must survive, hence
        // refresh per key rather than destroyAll).
        for (const tab of useTabsStore.getState().tabs) aliveRef.current?.refresh(tab.key);
      })();
    });
    return unsubscribe;
  }, [aliveRef]);

  // Tab titles are plain strings snapshotted into storage, so they are renewed
  // from the (language-aware) menu tree rather than re-added per navigation.
  useEffect(() => {
    useTabsStore.getState().retitles(menuTitlesByPath(menus));
  }, [menus]);

  useEffect(() => {
    const unsubscribe = useTabsStore.subscribe((state, prev) => {
      const removed = prev.tabs.filter((tab) => !state.tabs.some((item) => item.key === tab.key)).map((tab) => tab.key);
      if (removed.length) void aliveRef.current?.destroy(removed);
      if (state.tabs.length === 0) void aliveRef.current?.destroyAll();
    });
    return unsubscribe;
  }, [aliveRef]);

  useEffect(() => {
    const key = activeTab?.key;
    if (!key) return;
    const version = refreshKeys[key];
    if (!version) return;
    aliveRef.current?.refresh(key);
  }, [activeTab?.key, aliveRef, refreshKeys]);

  if (!activeTab) {
    return <PageNode menus={menus} refreshKey={0} tab={{ key: activePath, path: activePath, title: '' }} />;
  }

  if (!shouldCache) {
    const node = (
      <PageNode menus={menus} refreshKey={refreshKeys[activeTab.key] || 0} tab={activeTab} />
    );
    return <div className="cached-view is-active">{node}</div>;
  }

  const node = (
    <PageNode menus={menus} refreshKey={0} tab={activeTab} />
  );

  return (
    <KeepAlive
      activeCacheKey={activeTab.key}
      aliveRef={aliveRef}
      cacheNodeClassName="cached-view"
      containerClassName="keepalive-container"
      exclude={exclude}
      max={max}
      transition={prefs.transition.enable}
    >
      {node}
    </KeepAlive>
  );
}
