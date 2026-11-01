import { Button, Dropdown, Tooltip } from 'antd';
import { ChevronsLeft, ChevronsRight, Maximize2, Minimize2, MoreHorizontal, RefreshCw, X } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import i18n from '@/i18n';
import { iconNode } from '@/router/menu-utils';
import { usePreferencesStore } from '@/stores/preferences';
import { useTabsStore } from '@/stores/tabs';
import type { TabDefinition } from '@/types';

export function tabCloseAvailability(tab: TabDefinition, tabs: TabDefinition[], _homePath = '/analytics') {
  const index = tabs.findIndex((item) => item.key === tab.key);
  return {
    canClose: tabs.length > 1,
    canCloseAll: tabs.length > 1,
    canCloseLeft: index > 0,
    canCloseOthers: tabs.length > 1,
    canCloseRight: index >= 0 && index < tabs.length - 1,
  };
}

function moreItems(
  tab: TabDefinition,
  tabs: TabDefinition[],
  actions: {
    closeAll: () => string;
    closeLeft: (key: string) => string;
    closeOthers: (key: string) => string;
    closeRight: (key: string) => string;
  },
  navigate: (path: string) => void,
  homePath: string,
) {
  const { canCloseAll, canCloseLeft, canCloseOthers, canCloseRight } = tabCloseAvailability(tab, tabs, homePath);
  return [
    {
      key: 'left',
      label: i18n.t('shell.closeLeft'),
      disabled: !canCloseLeft,
      onClick: canCloseLeft ? () => navigate(actions.closeLeft(tab.key)) : undefined,
    },
    {
      key: 'right',
      label: i18n.t('shell.closeRight'),
      disabled: !canCloseRight,
      onClick: canCloseRight ? () => navigate(actions.closeRight(tab.key)) : undefined,
    },
    {
      key: 'others',
      label: i18n.t('shell.closeOthers'),
      disabled: !canCloseOthers,
      onClick: canCloseOthers ? () => navigate(actions.closeOthers(tab.key)) : undefined,
    },
    {
      key: 'all',
      label: i18n.t('shell.closeAll'),
      disabled: !canCloseAll,
      onClick: canCloseAll ? () => navigate(actions.closeAll()) : undefined,
    },
  ];
}

export function TabsBar() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const prefs = usePreferencesStore((state) => state.preferences);
  const tabs = useTabsStore((state) => state.tabs);
  const active = useTabsStore((state) => state.activeKey);
  const maximized = useTabsStore((state) => state.maximized);
  const close = useTabsStore((state) => state.close);
  const refresh = useTabsStore((state) => state.refresh);
  const reorder = useTabsStore((state) => state.reorder);
  const setMax = useTabsStore((state) => state.setMaximized);
  const closeAll = useTabsStore((state) => state.closeAll);
  const closeLeft = useTabsStore((state) => state.closeLeft);
  const closeRight = useTabsStore((state) => state.closeRight);
  const closeOthers = useTabsStore((state) => state.closeOthers);
  const drag = useRef<number | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const [overflow, setOverflow] = useState(false);
  const [atLeft, setAtLeft] = useState(true);
  const [atRight, setAtRight] = useState(true);
  const actions = { closeAll, closeLeft, closeOthers, closeRight };

  const activate = (key: string) => navigate(key);
  const current = tabs.find((tab) => tab.key === active) || tabs[0] || { key: '', path: '', title: '' };

  const updateScrollState = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    const maxScroll = el.scrollWidth - el.clientWidth;
    setOverflow(maxScroll > 1);
    setAtLeft(el.scrollLeft <= 1);
    setAtRight(el.scrollLeft >= maxScroll - 1);
  }, []);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    updateScrollState();
    const resizeObserver = new ResizeObserver(() => updateScrollState());
    resizeObserver.observe(el);
    const mutationObserver = new MutationObserver(() => updateScrollState());
    mutationObserver.observe(el, { childList: true, subtree: true });
    el.addEventListener('scroll', updateScrollState, { passive: true });
    window.addEventListener('resize', updateScrollState);
    return () => {
      resizeObserver.disconnect();
      mutationObserver.disconnect();
      el.removeEventListener('scroll', updateScrollState);
      window.removeEventListener('resize', updateScrollState);
    };
  }, [tabs, updateScrollState]);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const activeEl = el.querySelector<HTMLElement>('.tab-item.active');
    activeEl?.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'nearest' });
    const timer = window.setTimeout(updateScrollState, 280);
    return () => window.clearTimeout(timer);
  }, [active, tabs.length, updateScrollState]);

  const scrollByDir = (direction: 'left' | 'right') => {
    const el = scrollRef.current;
    if (!el) return;
    const amount = Math.max(120, el.clientWidth - 80);
    el.scrollBy({ behavior: 'smooth', left: direction === 'left' ? -amount : amount });
  };

  return (
    <div className={`tabs-bar tabs-${prefs.tabbar.styleType}`} style={{ height: prefs.tabbar.height }}>
      {overflow ? (
        <Button
          aria-label={t('shell.scrollLeft')}
          className="tabs-scroll-btn"
          disabled={atLeft}
          icon={<ChevronsLeft size={16} />}
          type="text"
          onClick={() => scrollByDir('left')}
        />
      ) : null}
      <div
        className="tabs-scroll"
        ref={scrollRef}
        onWheel={(event) => {
          if (!prefs.tabbar.wheelable) return;
          const index = tabs.findIndex((tab) => tab.key === active);
          const next = tabs[Math.max(0, Math.min(tabs.length - 1, index + (event.deltaY > 0 ? 1 : -1)))];
          if (next) activate(next.key);
        }}
      >
        {tabs.map((tab, index) => (
          <Dropdown key={tab.key} menu={{ items: moreItems(tab, tabs, actions, navigate, prefs.app.defaultHomePath) }} trigger={['contextMenu']}>
            <div
              className={`tab-item ${tab.key === active ? 'active' : ''}`}
              draggable={prefs.tabbar.draggable}
              onAuxClick={(event) => {
                if (event.button === 1 && prefs.tabbar.middleClickToClose && tabs.length > 1) {
                  navigate(close(tab.key));
                }
              }}
              onClick={() => activate(tab.key)}
              onDragOver={(event) => event.preventDefault()}
              onDragStart={() => {
                drag.current = index;
              }}
              onDrop={() => {
                if (drag.current !== null) reorder(drag.current, index);
                drag.current = null;
              }}
            >
              {prefs.tabbar.styleType === 'chrome' ? (
                <div aria-hidden className="tab-chrome-bg">
                  <div className="tab-chrome-bg-content" />
                  <svg className="tab-chrome-before" height="7" viewBox="0 0 7 7" width="7">
                    <path d="M 0 7 A 7 7 0 0 0 7 0 L 7 7 Z" />
                  </svg>
                  <svg className="tab-chrome-after" height="7" viewBox="0 0 7 7" width="7">
                    <path d="M 0 0 A 7 7 0 0 0 7 7 L 0 7 Z" />
                  </svg>
                </div>
              ) : null}
              <div className="tab-item-main">
                {prefs.tabbar.showIcon && tab.icon ? iconNode(tab.icon) : null}
                <span>{tab.title}</span>
                {tabs.length > 1 ? (
                  <X
                    size={13}
                    onClick={(event) => {
                      event.stopPropagation();
                      navigate(close(tab.key));
                    }}
                  />
                ) : null}
              </div>
            </div>
          </Dropdown>
        ))}
      </div>
      {overflow ? (
        <Button
          aria-label={t('shell.scrollRight')}
          className="tabs-scroll-btn"
          disabled={atRight}
          icon={<ChevronsRight size={16} />}
          type="text"
          onClick={() => scrollByDir('right')}
        />
      ) : null}
      <div className="tab-actions">
        {prefs.tabbar.showRefresh ? (
          <Tooltip title={t('refresh')}>
            <Button icon={<RefreshCw size={15} />} type="text" onClick={() => refresh(active)} />
          </Tooltip>
        ) : null}
        {prefs.tabbar.showMaximize ? (
          <Tooltip title={maximized ? t('tabbar.restore') : t('tabbar.maximize')}>
            <Button
              aria-label={maximized ? t('tabbar.restore') : t('tabbar.maximize')}
              icon={maximized ? <Minimize2 size={15} /> : <Maximize2 size={15} />}
              type="text"
              onClick={() => setMax(!maximized)}
            />
          </Tooltip>
        ) : null}
        {prefs.tabbar.showMore ? (
          <Dropdown menu={{ items: moreItems(current, tabs, actions, navigate, prefs.app.defaultHomePath) }} trigger={['click']}>
            <Button icon={<MoreHorizontal size={16} />} type="text" />
          </Dropdown>
        ) : null}
      </div>
    </div>
  );
}
