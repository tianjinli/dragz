import { describe, expect, it } from 'vitest';
import { normalizeColumns } from './normalizeColumns';
import type { VxeColumnProps } from '../types';

interface Row {
  a: number;
  b: string;
  c: string;
  d: string;
}

describe('normalizeColumns', () => {
  it('flattens plain columns into a single header row', () => {
    const columns: Array<VxeColumnProps<Row>> = [
      { field: 'a', title: 'A' },
      { field: 'b', title: 'B' },
    ];
    const { leafColumns, headerRows, headerLevel } = normalizeColumns(columns);
    expect(headerLevel).toBe(1);
    expect(leafColumns.map((col) => col.field)).toEqual(['a', 'b']);
    expect(headerRows).toHaveLength(1);
    expect(headerRows[0].map((cell) => [cell.colSpan, cell.rowSpan])).toEqual([
      [1, 1],
      [1, 1],
    ]);
  });

  it('builds colSpan/rowSpan for nested group headers', () => {
    const columns: Array<VxeColumnProps<Row>> = [
      { field: 'a', title: 'A' },
      {
        title: 'Group',
        children: [
          { field: 'b', title: 'B' },
          {
            title: 'Sub',
            children: [
              { field: 'c', title: 'C' },
              { field: 'd', title: 'D' },
            ],
          },
        ],
      },
    ];
    const { leafColumns, headerRows, headerLevel } = normalizeColumns(columns);
    expect(headerLevel).toBe(3);
    expect(leafColumns.map((col) => col.field)).toEqual(['a', 'b', 'c', 'd']);
    // row 0: A (rowSpan 3), Group (colSpan 3) — group keys use the index path
    expect(headerRows[0].map((cell) => [cell.key, cell.colSpan, cell.rowSpan])).toEqual([
      ['a', 1, 3],
      ['group:1', 3, 1],
    ]);
    // row 1: B (rowSpan 2), Sub (colSpan 2)
    expect(headerRows[1].map((cell) => [cell.key, cell.colSpan, cell.rowSpan])).toEqual([
      ['b', 1, 2],
      ['group:1.1', 2, 1],
    ]);
    // row 2: C, D
    expect(headerRows[2].map((cell) => [cell.key, cell.colSpan, cell.rowSpan])).toEqual([
      ['c', 1, 1],
      ['d', 1, 1],
    ]);
  });

  it('drops columns with visible: false', () => {
    const columns: Array<VxeColumnProps<Row>> = [
      { field: 'a', title: 'A' },
      { field: 'b', title: 'B', visible: false },
      {
        title: 'Group',
        children: [
          { field: 'c', title: 'C', visible: false },
          { field: 'd', title: 'D' },
        ],
      },
    ];
    const { leafColumns, headerRows } = normalizeColumns(columns);
    expect(leafColumns.map((col) => col.field)).toEqual(['a', 'd']);
    expect(headerRows[0][1].colSpan).toBe(1);
  });

  it('marks a group fixed only when all leaves share the fixed mode', () => {
    const columns: Array<VxeColumnProps<Row>> = [
      {
        title: 'FixedGroup',
        children: [
          { field: 'a', title: 'A', fixed: 'left' },
          { field: 'b', title: 'B', fixed: 'left' },
        ],
      },
      {
        title: 'MixedGroup',
        children: [
          { field: 'c', title: 'C', fixed: 'right' },
          { field: 'd', title: 'D' },
        ],
      },
    ];
    const { headerRows } = normalizeColumns(columns);
    expect(headerRows[0][0].fixed).toBe('left');
    expect(headerRows[0][0].stickyField).toBe('a');
    expect(headerRows[0][1].fixed).toBeUndefined();
  });
});
