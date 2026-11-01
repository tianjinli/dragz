import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { TabDefinition } from '@/types';
import { usePreferencesStore } from './preferences';
import { storageKey } from './storage';

interface TabsStore {
  activeKey: string;
  maximized: boolean;
  refreshKeys: Record<string, number>;
  /** Bumped on every language change; keep-alive pages remount when it moves. */
  localeEpoch: number;
  tabs: TabDefinition[];
  visitHistory: string[];
  add: (tab: TabDefinition, maxCount?: number) => void;
  close: (key: string) => string;
  closeAll: () => string;
  closeLeft: (key: string) => string;
  closeOthers: (key: string) => string;
  closeRight: (key: string) => string;
  refresh: (key: string) => void;
  /** Replace tab titles resolved from backend menus (language switch). */
  retitles: (titles: Record<string, string>) => void;
  bumpLocaleEpoch: () => void;
  reorder: (from: number, to: number) => void;
  setActive: (key: string) => void;
  setMaximized: (value: boolean) => void;
  clearRefreshKey: (key: string) => void;
}

function homePath() {
  return usePreferencesStore.getState().preferences.app.defaultHomePath || '/analytics';
}

function nearest(tabs: TabDefinition[], index: number) {
  return tabs[Math.min(index, tabs.length - 1)]?.key || homePath();
}

function resolveCloseTarget(
  previousActive: string,
  tabs: TabDefinition[],
  removedKeys: string[],
  fallbackIndex: number,
  visitHistory: string[],
): { activeKey: string; visitHistory: string[] } {
  const history = visitHistory.filter((key) => !removedKeys.includes(key) && tabs.some((tab) => tab.key === key));
  if (!removedKeys.includes(previousActive)) {
    return { activeKey: previousActive, visitHistory: history };
  }

  const useHistory = usePreferencesStore.getState().preferences.tabbar.visitHistory;
  if (useHistory && history.length > 0) {
    return { activeKey: history[history.length - 1]!, visitHistory: history };
  }

  return { activeKey: nearest(tabs, fallbackIndex), visitHistory: history };
}

function pruneRefreshKeys(refreshKeys: Record<string, number>, keys: string[]) {
  if (!keys.length) return refreshKeys;
  const next = { ...refreshKeys };
  for (const key of keys) delete next[key];
  return next;
}

export const useTabsStore = create<TabsStore>()(
  persist(
    (set, get) => ({
      activeKey: '',
      maximized: false,
      refreshKeys: {},
      localeEpoch: 0,
      tabs: [],
      visitHistory: [],
      add: (tab, maxCount = 0) =>
        set((state) => {
          const existing = state.tabs.find((item) => item.key === tab.key);
          if (
            existing &&
            state.activeKey === tab.key &&
            existing.title === tab.title &&
            existing.path === tab.path &&
            existing.icon === tab.icon &&
            existing.affix === tab.affix &&
            existing.keepAlive === tab.keepAlive &&
            !(maxCount > 0 && state.tabs.length > maxCount)
          ) {
            // Avoid churning tabs/visitHistory on every AdminLayout effect re-run
            // (feeds antd horizontal Menu Overflow update loops).
            return state;
          }
          let tabs = existing
            ? state.tabs.map((item) => (item.key === tab.key ? { ...item, ...tab } : item))
            : [...state.tabs, tab];
          let removedKeys: string[] = [];
          if (maxCount > 0 && tabs.length > maxCount) {
            const removable = tabs.findIndex((item) => item.key !== tab.key);
            if (removable >= 0) {
              removedKeys = [tabs[removable]!.key];
              tabs = tabs.filter((_, index) => index !== removable);
            }
          }
          return {
            activeKey: tab.key,
            refreshKeys: pruneRefreshKeys(state.refreshKeys, removedKeys),
            tabs,
            visitHistory: [...state.visitHistory.filter((key) => key !== tab.key && !removedKeys.includes(key)), tab.key].slice(-50),
          };
        }),
      close: (key) => {
        const state = get();
        if (state.tabs.length <= 1) return state.activeKey;
        const index = state.tabs.findIndex((tab) => tab.key === key);
        if (index < 0) return state.activeKey;
        const tabs = state.tabs.filter((tab) => tab.key !== key);
        const { activeKey, visitHistory } = resolveCloseTarget(state.activeKey, tabs, [key], index, state.visitHistory);
        set({
          tabs,
          activeKey,
          visitHistory,
          refreshKeys: pruneRefreshKeys(state.refreshKeys, [key]),
        });
        return activeKey;
      },
      closeAll: () => {
        const state = get();
        const keep = state.tabs[0];
        if (!keep) return state.activeKey || homePath();
        const removedKeys = state.tabs.slice(1).map((tab) => tab.key);
        set({
          tabs: [keep],
          activeKey: keep.key,
          visitHistory: [keep.key],
          refreshKeys: pruneRefreshKeys(state.refreshKeys, removedKeys),
          maximized: false,
        });
        return keep.key;
      },
      closeLeft: (key) => {
        const state = get();
        const index = state.tabs.findIndex((tab) => tab.key === key);
        if (index <= 0) return state.activeKey;
        const removedKeys = state.tabs.slice(0, index).map((tab) => tab.key);
        const tabs = state.tabs.slice(index);
        const visitHistory = state.visitHistory.filter((item) => !removedKeys.includes(item) && tabs.some((tab) => tab.key === item));
        set({
          tabs,
          activeKey: key,
          visitHistory: [...visitHistory.filter((item) => item !== key), key].slice(-50),
          refreshKeys: pruneRefreshKeys(state.refreshKeys, removedKeys),
        });
        return key;
      },
      closeOthers: (key) => {
        const state = get();
        const keep = state.tabs.find((tab) => tab.key === key);
        if (!keep) return state.activeKey;
        const removedKeys = state.tabs.filter((tab) => tab.key !== key).map((tab) => tab.key);
        set({
          tabs: [keep],
          activeKey: key,
          visitHistory: [key],
          refreshKeys: pruneRefreshKeys(state.refreshKeys, removedKeys),
        });
        return key;
      },
      closeRight: (key) => {
        const state = get();
        const index = state.tabs.findIndex((tab) => tab.key === key);
        if (index < 0 || index === state.tabs.length - 1) return state.activeKey;
        const removedKeys = state.tabs.slice(index + 1).map((tab) => tab.key);
        const tabs = state.tabs.slice(0, index + 1);
        const visitHistory = state.visitHistory.filter((item) => !removedKeys.includes(item) && tabs.some((tab) => tab.key === item));
        set({
          tabs,
          activeKey: key,
          visitHistory: [...visitHistory.filter((item) => item !== key), key].slice(-50),
          refreshKeys: pruneRefreshKeys(state.refreshKeys, removedKeys),
        });
        return key;
      },
      refresh: (key) =>
        set((state) => ({
          refreshKeys: { ...state.refreshKeys, [key]: (state.refreshKeys[key] || 0) + 1 },
        })),
      bumpLocaleEpoch: () => set((state) => ({ localeEpoch: state.localeEpoch + 1 })),
      retitles: (titles) =>
        set((state) => {
          const tabs = state.tabs.map((tab) => {
            const title = titles[tab.path.split('?')[0]!];
            return title && title !== tab.title ? { ...tab, title } : tab;
          });
          return tabs.every((tab, index) => tab === state.tabs[index]) ? state : { tabs };
        }),
      clearRefreshKey: (key) =>
        set((state) => {
          if (!(key in state.refreshKeys)) return state;
          const refreshKeys = { ...state.refreshKeys };
          delete refreshKeys[key];
          return { refreshKeys };
        }),
      reorder: (from, to) =>
        set((state) => {
          if (from === to || from < 0 || to < 0 || from >= state.tabs.length || to >= state.tabs.length) {
            return state;
          }
          const tabs = [...state.tabs];
          const [item] = tabs.splice(from, 1);
          if (!item) return state;
          tabs.splice(to, 0, item);
          return { tabs };
        }),
      setActive: (activeKey) => set({ activeKey }),
      setMaximized: (maximized) => set({ maximized }),
    }),
    {
      name: storageKey('tabs'),
      partialize: (state) => {
        const persistTabs = usePreferencesStore.getState().preferences.tabbar.persist;
        if (!persistTabs) return { activeKey: '', tabs: [], visitHistory: [] };
        return { activeKey: state.activeKey, tabs: state.tabs, visitHistory: state.visitHistory };
      },
    },
  ),
);

/** Clear persisted tabs storage immediately when persist preference is turned off. */
export function syncTabsPersistence(persistEnabled: boolean) {
  if (persistEnabled) return;
  localStorage.removeItem(storageKey('tabs'));
}
