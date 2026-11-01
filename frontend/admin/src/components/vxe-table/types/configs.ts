import type { ReactNode } from 'react';

export interface VxeSortConfig {
  /** initial sort state */
  defaultSort?: { field: string; order: 'asc' | 'desc' };
}

export interface VxeSeqConfig {
  /** offset added before numbering; default 0 so the first row shows 1 */
  startIndex?: number;
  /** custom seq cell content, wins over the number */
  seqMethod?: (params: { rowIndex: number }) => ReactNode;
}

export interface VxeColumnConfig {
  /** allow dragging header edge to resize columns (default true when width set on any column) */
  resizable?: boolean;
}

export interface VxeRowConfig<Row> {
  /** predicate for the highlight (current-row style) class */
  isCurrent?: (row: Row) => boolean;
  /** extra class names per row, aligned with vxe `row-class-name` */
  className?: string | ((params: { row: Row; rowIndex: number }) => string | undefined);
}
