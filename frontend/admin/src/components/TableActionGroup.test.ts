import { describe, expect, it, vi } from 'vitest';
import { actionColumnWidth, splitTableActions, type TableAction } from './TableActionGroup';

const action = (label: string): TableAction => ({ label, onClick: vi.fn() });

describe('splitTableActions', () => {
  it.each([0, 1, 2, 3])('keeps all actions visible when there are %i or fewer', (count) => {
    const result = splitTableActions(Array.from({ length: count }, (_, index) => action(String(index))));
    expect(result.overflowActions).toHaveLength(0);
    expect(result.visibleActions).toHaveLength(count);
  });

  it('keeps three actions visible', () => {
    const result = splitTableActions(['编辑', '授权', '删除'].map(action));
    expect(result.visibleActions.map((item) => item.label)).toEqual(['编辑', '授权', '删除']);
    expect(result.overflowActions).toHaveLength(0);
  });

  it('shows only the first two actions and overflows the rest', () => {
    const result = splitTableActions(['编辑', '授权', '删除', '复制'].map(action));
    expect(result.visibleActions.map((item) => item.label)).toEqual(['编辑', '授权']);
    expect(result.overflowActions.map((item) => item.label)).toEqual(['删除', '复制']);
  });

  it('preserves action order and metadata', () => {
    const remove = { ...action('删除'), danger: true, disabled: true };
    const result = splitTableActions([action('编辑'), action('授权'), remove, action('复制')]);
    expect(result.overflowActions[0]).toMatchObject({ label: '删除', danger: true, disabled: true });
  });

  it('forces inMore actions into the overflow menu', () => {
    const result = splitTableActions([
      action('编辑'),
      action('删除'),
      { ...action('改价历史'), inMore: true },
      { ...action('库存流水'), inMore: true },
    ]);
    expect(result.visibleActions.map((item) => item.label)).toEqual(['编辑', '删除']);
    expect(result.overflowActions.map((item) => item.label)).toEqual(['改价历史', '库存流水']);
  });

  it('shows only the more menu when all actions are inMore', () => {
    const result = splitTableActions([
      { ...action('改价历史'), inMore: true },
      { ...action('库存流水'), inMore: true },
    ]);
    expect(result.visibleActions).toHaveLength(0);
    expect(result.overflowActions.map((item) => item.label)).toEqual(['改价历史', '库存流水']);
  });
});

describe('actionColumnWidth', () => {
  it('returns the minimum width that fits the given link buttons', () => {
    expect(actionColumnWidth([{ label: '新增下级' }, { label: '修改' }, { label: '删除' }])).toBe(266);
  });

  it('is smaller for fewer actions', () => {
    expect(actionColumnWidth([{ label: '修改' }, { label: '删除' }])).toBeLessThan(actionColumnWidth([{ label: '新增下级' }, { label: '修改' }, { label: '删除' }]));
  });

  it('falls back to the minimum column width when empty', () => {
    expect(actionColumnWidth([])).toBe(128);
  });
});
