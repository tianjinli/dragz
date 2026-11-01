import type { VxeColumnProps, VxeLeafColumn } from './column';

/** Sort order, mirrors vxe-table: null -> 'asc' -> 'desc' -> null */
export type SortOrder = 'asc' | 'desc' | null;

/** Sort change params, aligned with vxe-table `sort-change` event. */
export interface SortChangeEvent {
  /** sorted column field */
  field: string;
  /** new sort order */
  order: SortOrder;
  /** full ordered sort list (single-column mode has at most one entry) */
  sortList: Array<{ field: string; order: 'asc' | 'desc' }>;
}

/** Cell click params, aligned with vxe-table `cell-click` event. */
export interface CellClickEvent<Row> {
  row: Row;
  rowIndex: number;
  column: VxeLeafColumn<Row>;
  field: string;
}

/** Row click params, aligned with vxe-table `row-click` event. */
export interface RowClickEvent<Row> {
  row: Row;
  rowIndex: number;
}

/** Header cell click params, aligned with vxe-table `header-cell-click` event. */
export interface HeaderCellClickEvent<Row> {
  column: VxeColumnProps<Row>;
}
