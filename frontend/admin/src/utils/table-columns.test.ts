import { describe, expect, it } from 'vitest';
import {
  getColumnKey,
  isActionsColumn,
  normalizeListColumns,
  toAntdColumnFixed,
  withStableColumnKeys,
} from './table-columns';

describe('isActionsColumn', () => {
  it('matches key actions or title 操作 without dataIndex', () => {
    expect(isActionsColumn({ key: 'actions', title: '操作' })).toBe(true);
    expect(isActionsColumn({ title: '操作', render: () => null })).toBe(true);
  });

  it('ignores data columns that happen to be titled 操作', () => {
    expect(isActionsColumn({ title: '操作', dataIndex: 'action' })).toBe(false);
  });
});

describe('normalizeListColumns', () => {
  it('pins the actions column to the end with a default width', () => {
    const columns = normalizeListColumns([
      { title: '名称', dataIndex: 'name' },
      { title: '操作', render: () => null },
    ]);
    expect(columns[1]).toMatchObject({ title: '操作', fixed: 'end', width: 140, align: 'center' });
  });

  it('preserves an existing fixed/width on actions and maps right to end', () => {
    const columns = normalizeListColumns([
      { title: '操作', key: 'actions', fixed: 'right', width: 220, render: () => null },
    ]);
    expect(columns[0]).toMatchObject({ fixed: 'end', width: 220 });
  });

  it('does not pin analytics-style 操作 data columns', () => {
    const columns = normalizeListColumns([
      { title: '用户', dataIndex: 'user' },
      { title: '操作', dataIndex: 'action' },
    ]);
    expect(columns[1]).not.toHaveProperty('fixed');
  });

  it('assigns stable keys from dataIndex', () => {
    const columns = normalizeListColumns([{ title: '名称', dataIndex: 'name' }]);
    expect(columns[0]).toMatchObject({ key: 'name' });
  });

  it('centers data cells and ellipsizes text columns with hover title', () => {
    const columns = normalizeListColumns([{ title: '名称', dataIndex: 'name' }]);
    expect(columns[0]).toMatchObject({
      align: 'center',
      ellipsis: { showTitle: true },
    });
  });

  it('defaults ellipsis.showTitle for text and width-constrained rendered columns', () => {
    const columns = normalizeListColumns([
      { title: '名称', dataIndex: 'name' },
      { title: '状态', dataIndex: 'status', width: 60, render: () => null },
      { title: '操作', key: 'actions', render: () => null },
    ]);
    expect(columns[0]).toMatchObject({
      align: 'center',
      ellipsis: { showTitle: true },
    });
    expect(columns[1]).toMatchObject({
      align: 'center',
      width: 60,
      ellipsis: { showTitle: true },
    });
    expect(columns[2]).toMatchObject({
      align: 'center',
      ellipsis: false,
      fixed: 'end',
    });
  });

  it('preserves explicit alignment and ellipsis overrides', () => {
    const columns = normalizeListColumns([
      { title: '名称', dataIndex: 'name', align: 'left', ellipsis: false },
    ]);
    expect(columns[0]).toMatchObject({ align: 'left', ellipsis: false });
  });

  it('lets width-constrained controls opt out of ellipsis', () => {
    const columns = normalizeListColumns([
      { title: '状态', dataIndex: 'status', width: 60, ellipsis: false, render: () => null },
    ]);
    expect(columns[0]).toMatchObject({ width: 60, ellipsis: false });
  });

  it('formats timestamp columns as yyyy-MM-dd HH:mm:ss', () => {
    const columns = normalizeListColumns([{ title: '时间', dataIndex: 'createdAt' }]);
    const render = columns[0].render as (value: unknown) => string;
    expect(render('2026-09-01T10:00:00+08:00')).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/);
    expect(render(null)).toBe('—');
  });

  it('leaves non-timestamp columns without a default render', () => {
    const columns = normalizeListColumns([{ title: '名称', dataIndex: 'name' }]);
    expect(columns[0].render).toBeUndefined();
  });

  it('keeps an explicit render on timestamp columns', () => {
    const render = () => 'custom';
    const columns = normalizeListColumns([{ title: '时间', dataIndex: 'createdAt', render }]);
    expect(columns[0].render).toBe(render);
  });
});

describe('getColumnKey / withStableColumnKeys', () => {
  it('prefers key, then dataIndex, then title', () => {
    expect(getColumnKey({ key: 'k', dataIndex: 'd', title: 't' })).toBe('k');
    expect(getColumnKey({ dataIndex: 'd', title: 't' })).toBe('d');
    expect(getColumnKey({ title: '标题' })).toBe('标题');
  });

  it('fills missing keys', () => {
    const columns = withStableColumnKeys([{ title: 'A', dataIndex: 'a' }, { title: 'B' }]);
    expect(columns.map((col) => col.key)).toEqual(['a', 'B']);
  });
});

describe('toAntdColumnFixed', () => {
  it('maps left/right aliases onto start/end', () => {
    expect(toAntdColumnFixed('left')).toBe('start');
    expect(toAntdColumnFixed('right')).toBe('end');
    expect(toAntdColumnFixed(true)).toBe('start');
  });
});
