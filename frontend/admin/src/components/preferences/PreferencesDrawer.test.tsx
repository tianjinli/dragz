import { App as AntApp } from 'antd';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { defaultPreferences, usePreferencesStore } from '@/stores/preferences';
import { PreferencesDrawer } from './PreferencesDrawer';

function renderDrawer() {
  return render(<MemoryRouter><AntApp><PreferencesDrawer onClose={() => undefined} open /></AntApp></MemoryRouter>);
}

function mockMatchMedia(dark: boolean) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: (query: string) => ({
      matches: query.includes('prefers-color-scheme: dark') ? dark : false,
      media: query,
      onchange: null,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      addListener: () => undefined,
      removeListener: () => undefined,
      dispatchEvent: () => false,
    }),
  });
}

describe('Golden Master preferences drawer', () => {
  beforeEach(() => {
    mockMatchMedia(false);
    usePreferencesStore.setState({ preferences: structuredClone(defaultPreferences) });
  });
  afterEach(cleanup);

  it('renders the three exact sections and tracks reset state', () => {
    renderDrawer();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['外观', '布局', '通用']);
    const reset = screen.getByRole('button', { name: '数据有变化，点击可进行重置' });
    expect(reset).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: '浅色' }));
    expect(reset).toBeEnabled();
    fireEvent.click(reset);
    expect(usePreferencesStore.getState().preferences).toEqual(defaultPreferences);
  });

  it('applies header-nav and parent-child disabled rules', () => {
    renderDrawer();
    fireEvent.click(screen.getByRole('tab', { name: '布局' }));
    expect(screen.queryByText('内容全屏')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '水平' }));
    expect(screen.getByRole('switch', { name: '显示侧边栏' })).toBeDisabled();
    expect(screen.getByRole('switch', { name: '侧边栏边缘拖拽' })).toBeDisabled();
    fireEvent.click(screen.getByRole('switch', { name: '启用标签栏' }));
    expect(screen.getByRole('switch', { name: '显示标签栏图标' })).toBeDisabled();
  });

  it('toggles sidebar edge drag preference below show sidebar', () => {
    renderDrawer();
    fireEvent.click(screen.getByRole('tab', { name: '布局' }));
    const edgeDrag = screen.getByRole('switch', { name: '侧边栏边缘拖拽' });
    expect(edgeDrag).toBeEnabled();
    expect(usePreferencesStore.getState().preferences.sidebar.draggable).toBe(true);
    fireEvent.click(edgeDrag);
    expect(usePreferencesStore.getState().preferences.sidebar.draggable).toBe(false);
    fireEvent.click(screen.getByRole('switch', { name: '显示侧边栏' }));
    expect(screen.getByRole('switch', { name: '侧边栏边缘拖拽' })).toBeDisabled();
  });

  it('disables semi-dark sidebar/header when follow-system resolves to dark', () => {
    mockMatchMedia(true);
    usePreferencesStore.setState({
      preferences: {
        ...structuredClone(defaultPreferences),
        theme: { ...defaultPreferences.theme, mode: 'auto' },
      },
    });
    renderDrawer();
    expect(screen.getByRole('switch', { name: '深色侧边栏' })).toBeDisabled();
    expect(screen.getByRole('switch', { name: '深色顶栏' })).toBeDisabled();
  });

  it('keeps semi-dark sidebar/header available when follow-system resolves to light', () => {
    mockMatchMedia(false);
    usePreferencesStore.setState({
      preferences: {
        ...structuredClone(defaultPreferences),
        theme: { ...defaultPreferences.theme, mode: 'auto' },
      },
    });
    renderDrawer();
    expect(screen.getByRole('switch', { name: '深色侧边栏' })).toBeEnabled();
    expect(screen.getByRole('switch', { name: '深色顶栏' })).toBeEnabled();
  });

  it('disables collapsed menu title for dual-column layout', () => {
    usePreferencesStore.setState({
      preferences: {
        ...structuredClone(defaultPreferences),
        app: { ...defaultPreferences.app, layout: 'sidebar-mixed-nav' },
        sidebar: { ...defaultPreferences.sidebar, collapsed: true, collapsedShowTitle: true },
      },
    });
    renderDrawer();
    fireEvent.click(screen.getByRole('tab', { name: '布局' }));
    const titleSwitch = screen.getByRole('switch', { name: '折叠显示菜单名' });
    expect(titleSwitch).toBeDisabled();
    expect(titleSwitch).not.toBeChecked();
  });

  it('clears collapsed menu title when switching to dual-column layout', () => {
    usePreferencesStore.setState({
      preferences: {
        ...structuredClone(defaultPreferences),
        sidebar: { ...defaultPreferences.sidebar, collapsed: true, collapsedShowTitle: true },
      },
    });
    renderDrawer();
    fireEvent.click(screen.getByRole('tab', { name: '布局' }));
    fireEvent.click(screen.getByRole('button', { name: '双列菜单' }));
    expect(usePreferencesStore.getState().preferences.app.layout).toBe('sidebar-mixed-nav');
    expect(usePreferencesStore.getState().preferences.sidebar.collapsedShowTitle).toBe(false);
  });

  it('opens the dropdown select with a check on the selected option and updates on pick', () => {
    renderDrawer();
    fireEvent.click(screen.getByRole('tab', { name: '布局' }));
    fireEvent.click(screen.getByRole('button', { name: '全局搜索' }));

    expect(screen.getByRole('listbox', { name: '全局搜索' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: '顶栏' })).toHaveAttribute('aria-selected', 'true');

    fireEvent.click(screen.getByRole('option', { name: '不显示' }));

    expect(usePreferencesStore.getState().preferences.widget.globalSearchButtonPosition).toBe('none');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('closes the dropdown select on outside click without changing the value', () => {
    renderDrawer();
    fireEvent.click(screen.getByRole('tab', { name: '布局' }));
    fireEvent.click(screen.getByRole('button', { name: '全局搜索' }));
    expect(screen.getByRole('listbox')).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole('heading', { name: '小部件' }));

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(usePreferencesStore.getState().preferences.widget.globalSearchButtonPosition).toBe('header');
  });
});
