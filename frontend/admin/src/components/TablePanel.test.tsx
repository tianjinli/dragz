import { cleanup, fireEvent, render, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createRef } from 'react';
import type { TableRef } from 'antd/es/table';

import {
  TablePanel,
  measureBodyContentHeight,
  measureHeaderGutterPad,
} from './TablePanel';
import { ListPagePanel } from './ListPagePanel';

class ResizeObserverStub {
  disconnect() {}
  observe() {}
  unobserve() {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverStub);

const columns = [
  { title: '名称', dataIndex: 'name', fixed: 'start' as const },
  { title: '状态', dataIndex: 'status' },
  { title: '操作', fixed: 'end' as const, width: 140, render: () => <button type="button">修改</button> },
];
const rows = [
  { id: 1, name: '工程师', status: 1 },
  { id: 2, name: '设计师', status: 0 },
];

describe('TablePanel (antd sticky Table)', () => {
  afterEach(cleanup);

  it('honors native column defaults, selection, sorting, scroll, styles and ref', () => {
    const ref = createRef<TableRef>();
    const onChange = vi.fn();
    const onSelect = vi.fn();
    const { container } = render(
      <TablePanel
        ref={ref} rowKey="id" dataSource={rows} pagination={false}
        columns={[{ title: '状态', dataIndex: 'status', width: 90, sorter: (a, b) => a.status - b.status }]}
        column={{ align: 'right', ellipsis: false }}
        scroll={{ y: 200 }} sticky={false}
        rowSelection={{ onChange: onSelect }} onChange={onChange}
        classNames={{ body: { cell: 'native-cell' } }}
        styles={{ body: { cell: { color: 'red' } } }}
        title={() => '原生标题'} footer={() => '原生页脚'}
      />,
    );
    fireEvent.click(container.querySelector('.ant-table-row input[type="checkbox"]')!);
    expect(onSelect).toHaveBeenCalledWith([1], [rows[0]], { type: 'single' });
    fireEvent.click(container.querySelector('.ant-table-header .ant-table-column-title')!);
    expect(onChange).toHaveBeenCalledWith(expect.anything(), expect.anything(), expect.objectContaining({ order: 'ascend' }), expect.objectContaining({ action: 'sort' }));
    expect(container.querySelector('.ant-table-row .native-cell:last-child')).toHaveStyle({ textAlign: 'right', color: 'rgb(255, 0, 0)' });
    expect(container.querySelector('.ant-table-cell-ellipsis')).toBeNull();
    expect(container.querySelector('.ant-table-sticky-holder')).toBeNull();
    expect(container.querySelector('.ant-table-body')).toHaveStyle({ maxHeight: '200px' });
    expect(container.querySelector('col[style*="90px"]')).not.toBeNull();
    expect(container.querySelector('.ant-table-title')).toHaveTextContent('原生标题');
    expect(container.querySelector('.ant-table-footer')).toHaveTextContent('原生页脚');
    expect(ref.current?.nativeElement).toBe(container.querySelector('.ant-table-wrapper'));
    expect(ref.current?.scrollTo).toBeTypeOf('function');
  });

  it('renders data, empty state, loading and pagination through antd', () => {
    const { container, rerender } = render(
      <TablePanel rowKey="id" dataSource={rows} columns={columns} pagination={false} />,
    );
    expect(container.querySelector('.ant-table')).not.toBeNull();
    expect(container.querySelectorAll('.ant-table-tbody > tr').length).toBeGreaterThanOrEqual(2);
    rerender(<TablePanel rowKey="id" dataSource={[]} columns={columns} pagination={false} />);
    expect(container.querySelector('.ant-empty')).not.toBeNull();
    rerender(<TablePanel loading rowKey="id" dataSource={[]} columns={columns} pagination={false} />);
    expect(container.querySelector('.ant-table-wrapper')).not.toBeNull();
  });

  it('supports tree expansion and antd pagination callbacks', () => {
    const onChange = vi.fn();
    const { container } = render(
      <TablePanel
        rowKey="id"
        dataSource={[{ id: 1, name: '系统', children: [{ id: 2, name: '用户' }] }]}
        columns={[{ title: '名称', dataIndex: 'name' }]}
        expandable={{ defaultExpandAllRows: false }}
        pagination={{ current: 1, pageSize: 20, total: 41, onChange }}
      />,
    );
    expect(container.querySelector('.ant-table-tbody')).not.toBeNull();
    const expand = container.querySelector('.ant-table-row-expand-icon');
    expect(expand).not.toBeNull();
    expect(expand).toHaveClass('ant-table-row-expand-icon-collapsed');
    if (expand) fireEvent.click(expand);
    expect(container.querySelector('.ant-table-row-expand-icon-expanded')).not.toBeNull();
    expect(container.textContent).toContain('用户');
    const pageTwo = Array.from(container.querySelectorAll('.ant-pagination-item')).find((item) => item.textContent === '2');
    fireEvent.click(pageTwo!);
    expect(onChange).toHaveBeenCalledWith(2, 20);
  });

  it('ellipsis showTitle tip matches the truncated cell text on tree rows', () => {
    const longName = '超长部门名称用于验证省略号与 tips 对应关系';
    const { container } = render(
      <TablePanel
        rowKey="id"
        dataSource={[{ id: 1, name: longName, children: [{ id: 2, name: '子部门' }] }]}
        columns={[{ title: '名称', dataIndex: 'name', width: 120, ellipsis: { showTitle: true } }]}
        pagination={false}
      />,
    );
    const firstCell = container.querySelector('.ant-table-tbody tr.ant-table-row > td:first-child');
    expect(firstCell).not.toBeNull();
    expect(firstCell).toHaveClass('ant-table-cell-ellipsis');
    expect(firstCell).toHaveAttribute('title', longName);
    expect(firstCell!.querySelector('.ant-table-row-expand-icon')).not.toBeNull();
    expect(firstCell!.textContent).toContain(longName);
  });

  it('uses sticky + scroll.x with start/end fixed columns', async () => {
    const { container } = render(
      <TablePanel sticky scroll={{ x: 900 }} rowKey="id" dataSource={rows} columns={columns} pagination={false} />,
    );
    expect(container.querySelector('.ant-table-sticky-holder, .ant-table-header')).not.toBeNull();
    expect(container.querySelector('.list-table-measure-header')).not.toBeNull();
    await waitFor(() => {
      expect(
        container.querySelector('.ant-table-has-fix-start, .ant-table-has-fix-left, .ant-table-cell-fix-start'),
      ).not.toBeNull();
      expect(
        container.querySelector('.ant-table-has-fix-end, .ant-table-has-fix-right, .ant-table-cell-fix-end'),
      ).not.toBeNull();
    });
    expect(container.querySelector('.list-table-scroll-y-virtual')).toBeNull();
  });

  it('preserves official defaults and explicit bordered settings', () => {
    const { container, rerender } = render(
      <TablePanel rowKey="id" dataSource={rows} columns={[{ title: '名称', dataIndex: 'name', width: 123 }]} pagination={false} />,
    );
    expect(container.querySelector('.ant-table-bordered')).toBeNull();
    expect(container.querySelector('.ant-table-sticky-holder')).toBeNull();
    expect(container.querySelector('.ant-table-cell-ellipsis')).toBeNull();
    expect(container.querySelector('.ant-table-cell-fix-start')).toBeNull();
    expect(container.querySelector('col[style*="123px"]')).not.toBeNull();
    rerender(<TablePanel bordered rowKey="id" dataSource={rows} columns={columns} pagination={false} />);
    expect(container.querySelector('.ant-table-bordered')).not.toBeNull();
    rerender(<TablePanel bordered={false} rowKey="id" dataSource={rows} columns={columns} pagination={false} />);
    expect(container.querySelector('.ant-table-bordered')).toBeNull();
  });

  it('keeps configured column widths bounded when content grows', () => {
    const { container, rerender } = render(
      <TablePanel
        rowKey="id"
        dataSource={[{ id: 1, name: '十个汉字的单元格内容' }]}
        columns={[{ title: '名称', dataIndex: 'name', width: 60, ellipsis: true }]}
        pagination={false}
        scroll={{ x: 'max-content' }}
      />,
    );

    expect(container.querySelector('table')).toHaveStyle({ tableLayout: 'fixed' });
    expect(container.querySelector('col[style*="60px"]')).not.toBeNull();

    rerender(
      <TablePanel
        rowKey="id"
        dataSource={[{ id: 1, name: '汉'.repeat(100) }]}
        columns={[{ title: '名称', dataIndex: 'name', width: 60, ellipsis: true }]}
        pagination={false}
        scroll={{ x: 'max-content' }}
      />,
    );

    expect(container.querySelector('table')).toHaveStyle({ tableLayout: 'fixed' });
    expect(container.querySelector('col[style*="60px"]')).not.toBeNull();
    expect(container.querySelector('.ant-table-cell-ellipsis')).not.toBeNull();
  });

  it('keeps Ant Design scroll.y overflow (do not force auto — breaks fixed columns)', async () => {
    const { readFileSync } = await import('node:fs');
    const { dirname, resolve } = await import('node:path');
    const { fileURLToPath } = await import('node:url');
    const base = dirname(fileURLToPath(import.meta.url));
    const source = readFileSync(resolve(base, 'TablePanel.tsx'), 'utf8');
    const baseCss = readFileSync(resolve(base, '../styles/base.css'), 'utf8');

    const { container } = render(
      <TablePanel
        rowKey="id"
        dataSource={rows}
        columns={columns}
        pagination={false}
        scroll={{ y: 240 }}
      />,
    );
    expect(container.querySelector('.ant-table-body')).toHaveStyle({
      maxHeight: '240px',
      overflowY: 'scroll',
    });
    expect(source).not.toMatch(/overflowY\s*=\s*["']auto["']/);
    expect(source).not.toMatch(/overflowY:\s*["']auto["']/);
    expect(baseCss).not.toMatch(/\.ant-table-body(?:-inner)?[\s\S]{0,120}overflow-y:\s*auto/);
  });

  it('measures body content height from the split body table or flat tbody', () => {
    const split = document.createElement('div');
    const body = document.createElement('div');
    body.className = 'ant-table-body';
    const table = document.createElement('table');
    Object.defineProperty(table, 'scrollHeight', { configurable: true, value: 420 });
    body.append(table);
    split.append(body);
    expect(measureBodyContentHeight(split)).toBe(420);

    const flat = document.createElement('div');
    const tbody = document.createElement('tbody');
    tbody.className = 'ant-table-tbody';
    tbody.getBoundingClientRect = () =>
      ({
        height: 180,
        width: 0,
        top: 0,
        left: 0,
        bottom: 0,
        right: 0,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      }) as DOMRect;
    flat.append(tbody);
    expect(measureBodyContentHeight(flat)).toBe(180);
  });

  it('pads the sticky header when the body scrollbar exceeds the gutter cell', () => {
    const root = document.createElement('div');
    const header = document.createElement('div');
    header.className = 'ant-table-header';
    const body = document.createElement('div');
    body.className = 'ant-table-body';
    Object.defineProperty(body, 'offsetWidth', { configurable: true, value: 800 });
    Object.defineProperty(body, 'clientWidth', { configurable: true, value: 785 });
    root.append(header, body);

    // No Ant Design gutter cell yet (common when scroll.y is applied after mount).
    expect(measureHeaderGutterPad(root)).toBe(15);

    const gutter = document.createElement('div');
    gutter.className = 'ant-table-cell-scrollbar';
    header.append(gutter);
    gutter.getBoundingClientRect = () =>
      ({ width: 15, height: 0, top: 0, left: 0, bottom: 0, right: 0, x: 0, y: 0, toJSON: () => ({}) }) as DOMRect;
    expect(measureHeaderGutterPad(root)).toBe(0);

    gutter.getBoundingClientRect = () =>
      ({ width: 10, height: 0, top: 0, left: 0, bottom: 0, right: 0, x: 0, y: 0, toJSON: () => ({}) }) as DOMRect;
    expect(measureHeaderGutterPad(root)).toBe(5);
  });

  it('applies header paddingInlineEnd when the measured gutter is short', async () => {
    const observers: Array<{ callback: ResizeObserverCallback; element: Element }> = [];
    class MeasuringResizeObserver {
      callback: ResizeObserverCallback;
      constructor(callback: ResizeObserverCallback) {
        this.callback = callback;
      }
      observe(element: Element) {
        observers.push({ callback: this.callback, element });
      }
      disconnect() {}
      unobserve() {}
    }
    vi.stubGlobal('ResizeObserver', MeasuringResizeObserver);

    const { container } = render(
      <TablePanel
        sticky
        rowKey="id"
        dataSource={rows}
        columns={columns}
        pagination={false}
        scroll={{ y: 120 }}
      />,
    );

    const body = container.querySelector('.ant-table-body') as HTMLElement;
    const header = container.querySelector('.ant-table-header') as HTMLElement;
    expect(body).not.toBeNull();
    expect(header).not.toBeNull();

    Object.defineProperty(body, 'offsetWidth', { configurable: true, value: 800 });
    Object.defineProperty(body, 'clientWidth', { configurable: true, value: 784 });
    const gutter = header.querySelector('.ant-table-cell-scrollbar') as HTMLElement | null;
    if (gutter) {
      gutter.getBoundingClientRect = () =>
        ({
          width: 0,
          height: 0,
          top: 0,
          left: 0,
          bottom: 0,
          right: 0,
          x: 0,
          y: 0,
          toJSON: () => ({}),
        }) as DOMRect;
    }

    observers.forEach(({ callback, element }) => {
      callback(
        [{ target: element } as ResizeObserverEntry],
        {} as ResizeObserver,
      );
    });

    await waitFor(() => {
      expect(header.style.paddingInlineEnd).toBe('16px');
    });

    vi.stubGlobal('ResizeObserver', ResizeObserverStub);
  });

  it('enables measured scroll.y only when body content overflows the available height', async () => {
    const observers: Array<{ callback: ResizeObserverCallback; element: Element }> = [];
    class MeasuringResizeObserver {
      callback: ResizeObserverCallback;
      constructor(callback: ResizeObserverCallback) {
        this.callback = callback;
      }
      observe(element: Element) {
        observers.push({ callback: this.callback, element });
        this.callback([{ target: element } as ResizeObserverEntry], this as unknown as ResizeObserver);
      }
      disconnect() {}
      unobserve() {}
    }
    vi.stubGlobal('ResizeObserver', MeasuringResizeObserver);

    const { container, rerender } = render(
      <div style={{ height: 360 }}>
        <TablePanel rowKey="id" dataSource={rows} columns={columns} pagination={false} scroll={{ x: 'max-content' }} />
      </div>,
    );

    expect(container.querySelector('.list-table-measure-header')).not.toBeNull();
    const wrapper = container.querySelector('.ant-table-wrapper') as HTMLElement;
    Object.defineProperty(wrapper, 'getBoundingClientRect', {
      value: () => ({ height: 360, width: 800, top: 0, left: 0, bottom: 360, right: 800, x: 0, y: 0, toJSON: () => ({}) }),
    });
    Object.defineProperty(wrapper, 'clientHeight', { configurable: true, value: 360 });
    const header = container.querySelector('.list-table-measure-header') as HTMLElement | null;
    if (header) {
      Object.defineProperty(header, 'getBoundingClientRect', {
        value: () => ({ height: 55, width: 800, top: 0, left: 0, bottom: 55, right: 800, x: 0, y: 0, toJSON: () => ({}) }),
      });
    }
    const tbody = container.querySelector('.ant-table-tbody') as HTMLElement;
    tbody.getBoundingClientRect = () =>
      ({
        height: 80,
        width: 800,
        top: 0,
        left: 0,
        bottom: 80,
        right: 800,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      }) as DOMRect;

    observers[0]?.callback(
      [{ target: wrapper } as ResizeObserverEntry],
      observers[0] as unknown as ResizeObserver,
    );

    // Short content: no scroll.y, so Ant Design keeps a flat table (no body scroller).
    await waitFor(() => {
      expect(container.querySelector('.ant-table-body')).toBeNull();
    });

    tbody.getBoundingClientRect = () =>
      ({
        height: 480,
        width: 800,
        top: 0,
        left: 0,
        bottom: 480,
        right: 800,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      }) as DOMRect;
    rerender(
      <div style={{ height: 360 }}>
        <TablePanel rowKey="id" dataSource={rows} columns={columns} pagination={false} scroll={{ x: 'max-content' }} />
      </div>,
    );
    observers.forEach(({ callback, element }) => {
      callback(
        [{ target: element } as ResizeObserverEntry],
        {} as ResizeObserver,
      );
    });

    await waitFor(() => {
      expect(container.querySelector('.ant-table-body')).toHaveStyle({ maxHeight: '305px' });
    });

    vi.stubGlobal('ResizeObserver', ResizeObserverStub);
  });

  it('keeps the bordered start edge on table content instead of the full-height container', async () => {
    const { readFileSync } = await import('node:fs');
    const { dirname, resolve } = await import('node:path');
    const { fileURLToPath } = await import('node:url');
    const base = dirname(fileURLToPath(import.meta.url));
    const baseCss = readFileSync(resolve(base, '../styles/base.css'), 'utf8');
    expect(baseCss).toMatch(
      /\.ant-table-bordered\s*>\s*\.ant-table-container\s*\{[^}]*border-inline-start:\s*0/s,
    );
    expect(baseCss).toMatch(
      /\.ant-table-bordered\s*>\s*\.ant-table-container\s*>\s*\.ant-table-content\s*>\s*table[\s\S]*?border-inline-start:\s*1px\s+solid\s+var\(--border\)/s,
    );
    expect(baseCss).not.toMatch(
      /\.list-table-scroll-frame[\s\S]*?\.ant-table-bordered[\s\S]*?border-top:\s*0/s,
    );
  });

  it('supports toolbar refresh and fullscreen toggle', () => {
    const refresh = vi.fn();
    const { container } = render(
      <ListPagePanel title="列表" toolbar={{ onRefresh: refresh }}><TablePanel rowKey="id" dataSource={rows} columns={columns} pagination={false} /></ListPagePanel>,
    );
    fireEvent.click(container.querySelector('[aria-label="刷新"]')!);
    expect(refresh).toHaveBeenCalledOnce();
    fireEvent.click(container.querySelector('[aria-label="全屏"]')!);
    expect(container.querySelector('.list-table-render-wrapper')).toHaveClass('is-fullscreen');
  });
});
