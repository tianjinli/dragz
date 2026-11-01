import { beforeEach, describe, expect, it } from 'vitest';
import { usePreferencesStore } from './preferences';
import { useTabsStore } from './tabs';

const tab = (key: string, affix = false) => ({ affix, key, path: key, title: key });

describe('tabs store', () => {
  beforeEach(() => {
    useTabsStore.setState({ activeKey: '', maximized: false, refreshKeys: {}, tabs: [], visitHistory: [] });
    usePreferencesStore.getState().update({
      tabbar: { ...usePreferencesStore.getState().preferences.tabbar, visitHistory: false },
      app: { ...usePreferencesStore.getState().preferences.app, defaultHomePath: '/analytics' },
    });
  });

  it('adds, de-duplicates and tracks history', () => {
    const s = useTabsStore.getState();
    s.add(tab('/a'));
    s.add(tab('/b'));
    s.add({ ...tab('/a'), title: 'A' });
    expect(useTabsStore.getState().tabs).toHaveLength(2);
    expect(useTabsStore.getState().activeKey).toBe('/a');
    expect(useTabsStore.getState().visitHistory.at(-1)).toBe('/a');
  });

  it('closes every tab including affix, but keeps the last tab', () => {
    const s = useTabsStore.getState();
    s.add(tab('/home', true));
    s.add(tab('/a'));
    s.add(tab('/b'));
    expect(s.close('/home')).toBe('/b');
    expect(useTabsStore.getState().tabs.map((t) => t.key)).toEqual(['/a', '/b']);
    expect(s.closeOthers('/a')).toBe('/a');
    expect(useTabsStore.getState().tabs.map((t) => t.key)).toEqual(['/a']);
    expect(s.close('/a')).toBe('/a');
    expect(useTabsStore.getState().tabs.map((t) => t.key)).toEqual(['/a']);
  });

  it('close-all keeps only the first tab', () => {
    const s = useTabsStore.getState();
    s.add(tab('/home', true));
    s.add(tab('/a'));
    s.add(tab('/b'));
    expect(s.closeAll()).toBe('/home');
    expect(useTabsStore.getState().tabs.map((t) => t.key)).toEqual(['/home']);
    expect(useTabsStore.getState().activeKey).toBe('/home');
  });

  it('navigates after bulk close removes the active tab', () => {
    const s = useTabsStore.getState();
    s.add(tab('/a'));
    s.add(tab('/b'));
    s.add(tab('/c'));
    expect(useTabsStore.getState().activeKey).toBe('/c');
    expect(s.closeLeft('/c')).toBe('/c');
    expect(useTabsStore.getState().tabs.map((t) => t.key)).toEqual(['/c']);

    s.add(tab('/a'));
    s.add(tab('/b'));
    expect(s.closeRight('/a')).toBe('/a');
    expect(useTabsStore.getState().activeKey).toBe('/a');
  });

  it('uses visit history when closing the active tab', () => {
    usePreferencesStore.getState().update({
      tabbar: { ...usePreferencesStore.getState().preferences.tabbar, visitHistory: true },
    });
    const s = useTabsStore.getState();
    s.add(tab('/a'));
    s.add(tab('/b'));
    s.add(tab('/c'));
    s.add(tab('/a'));
    expect(s.close('/a')).toBe('/c');
  });

  it('refreshes only the selected cache instance and reorders', () => {
    const s = useTabsStore.getState();
    s.add(tab('/a'));
    s.add(tab('/b'));
    s.refresh('/a');
    expect(useTabsStore.getState().refreshKeys).toEqual({ '/a': 1 });
    s.reorder(1, 0);
    expect(useTabsStore.getState().tabs[0]?.key).toBe('/b');
    s.reorder(-1, 0);
    expect(useTabsStore.getState().tabs[0]?.key).toBe('/b');
  });

  it('retitles open tabs from the menu tree and leaves unknown paths alone', () => {
    const s = useTabsStore.getState();
    s.add({ ...tab('/a'), title: '旧名' });
    s.add({ ...tab('/b'), title: '旧名二' });
    s.retitles({ '/a': 'New name', '/c': 'irrelevant' });
    expect(useTabsStore.getState().tabs.map((item) => item.title)).toEqual(['New name', '旧名二']);
  });

  it('bumps the locale epoch without touching tabs', () => {
    const s = useTabsStore.getState();
    s.add(tab('/a'));
    const before = useTabsStore.getState();
    s.bumpLocaleEpoch();
    const after = useTabsStore.getState();
    expect(after.localeEpoch).toBe(before.localeEpoch + 1);
    expect(after.tabs).toBe(before.tabs);
  });

  it('enforces maximum tab count and clears refresh keys on close', () => {
    const s = useTabsStore.getState();
    s.add(tab('/home', true), 2);
    s.add(tab('/a'), 2);
    s.refresh('/a');
    s.add(tab('/b'), 2);
    expect(useTabsStore.getState().tabs.map((t) => t.key)).toEqual(['/a', '/b']);
    expect(useTabsStore.getState().refreshKeys['/a']).toBe(1);
    expect(s.close('/a')).toBe('/b');
    expect(useTabsStore.getState().refreshKeys['/a']).toBeUndefined();
  });
});
