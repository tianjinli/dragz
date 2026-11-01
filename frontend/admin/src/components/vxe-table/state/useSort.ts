import { useCallback, useEffect, useMemo, useState } from 'react';
import type { SortChangeEvent, SortOrder, VxeLeafColumn } from '../types';

export interface SortState {
  field: string;
  order: SortOrder;
}

function defaultSorter(a: unknown, b: unknown): number {
  if (a === b) return 0;
  if (a === null || a === undefined) return -1;
  if (b === null || b === undefined) return 1;
  if (typeof a === 'number' && typeof b === 'number') return a - b;
  return String(a).localeCompare(String(b), 'zh-Hans-CN', { numeric: true });
}

export interface UseSortOptions<Row> {
  columns: Array<VxeLeafColumn<Row>>;
  data: Row[];
  /** controlled sort state; when provided the hook never mutates it */
  sort?: SortState;
  /** uncontrolled initial sort (vxe sort-config.default-sort) */
  defaultSort?: { field: string; order: 'asc' | 'desc' };
  /** bump to reset the uncontrolled sort (column signature) */
  resetKey: string;
  onSortChange?: (event: SortChangeEvent) => void;
}

/**
 * Single-column header-click sorting, mirroring vxe-table:
 * click cycles asc -> desc -> none; supports controlled `sort` prop,
 * uncontrolled `defaultSort`, and a `sort-change` event.
 */
export function useSort<Row>(options: UseSortOptions<Row>) {
  const { columns, data, sort, defaultSort, resetKey, onSortChange } = options;

  const [innerSort, setInnerSort] = useState<SortState>(() =>
    defaultSort ? { field: defaultSort.field, order: defaultSort.order } : { field: '', order: null },
  );
  useEffect(() => setInnerSort({ field: '', order: null }), [resetKey]);

  const sortState: SortState = sort ?? innerSort;

  const cycleSort = useCallback(
    (column: VxeLeafColumn<Row>) => {
      if (!column.sortable) return;
      const nextOrder: SortOrder =
        sortState.field === column.field
          ? sortState.order === 'asc'
            ? 'desc'
            : sortState.order === 'desc'
              ? null
              : 'asc'
          : 'asc';
      if (!sort) setInnerSort({ field: column.field, order: nextOrder });
      onSortChange?.({
        field: column.field,
        order: nextOrder,
        sortList: nextOrder ? [{ field: column.field, order: nextOrder }] : [],
      });
    },
    [onSortChange, sort, sortState],
  );

  const sortedData = useMemo(() => {
    if (!sortState.field || !sortState.order) return data;
    const column = columns.find((col) => col.field === sortState.field);
    if (!column) return data;
    const factor = sortState.order === 'asc' ? 1 : -1;
    return [...data].sort((a, b) => {
      if (column.sorter) return factor * column.sorter(a, b);
      return factor * defaultSorter((a as Record<string, unknown>)[column.field], (b as Record<string, unknown>)[column.field]);
    });
  }, [columns, data, sortState]);

  return { sortState, cycleSort, sortedData };
}
