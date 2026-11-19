import { describe, expect, it, vi } from 'vitest';
import {
  actionColumnSlotCount,
  actionColumnWidth,
  splitTableActions,
  type TableAction,
} from './TableActionGroup';

const action = (label: string): TableAction => ({
  icon: null,
  label,
  onClick: vi.fn(),
});

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

describe('actionColumnSlotCount', () => {
  it('counts every button when there are three or fewer', () => {
    expect(actionColumnSlotCount([{}, {}, {}])).toBe(3);
  });

  it('counts two primaries plus the more control when overflowing', () => {
    expect(actionColumnSlotCount([{}, {}, {}, {}])).toBe(3);
  });

  it('counts the more control when every action is inMore', () => {
    expect(actionColumnSlotCount([{ inMore: true }, { inMore: true }])).toBe(1);
  });
});

describe('actionColumnWidth', () => {
  it('treats a single button as two so 「操作」fits on one line', () => {
    expect(actionColumnWidth(1)).toBe(actionColumnWidth(2));
    expect(actionColumnWidth(1)).toBe(104);
  });

  it('grows for three icon buttons', () => {
    expect(actionColumnWidth(2)).toBe(104);
    expect(actionColumnWidth(3)).toBe(144);
  });

  it('caps overflow rows at three slots (2 actions + more)', () => {
    expect(actionColumnWidth(5)).toBe(actionColumnWidth(3));
  });

  it('sizes from an action list including inMore', () => {
    expect(
      actionColumnWidth([
        {},
        {},
        { inMore: true },
        { inMore: true },
      ]),
    ).toBe(actionColumnWidth(3));
  });

  it('falls back to the minimum column width when empty', () => {
    expect(actionColumnWidth(0)).toBe(128);
    expect(actionColumnWidth([])).toBe(128);
  });
});
