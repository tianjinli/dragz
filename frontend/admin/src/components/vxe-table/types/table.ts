import type { CSSProperties, ReactNode } from 'react';
import type { VxeColumnProps } from './column';
import type { VxeColumnConfig, VxeRowConfig, VxeSortConfig } from './configs';
import type { CellClickEvent, HeaderCellClickEvent, RowClickEvent, SortChangeEvent, SortOrder } from './events';

export interface VxeTableProps<Row> {
  /** rows to render */
  data: Row[];
  /** column definitions, ordered left -> right; supports `children` for group headers */
  columns: Array<VxeColumnProps<Row>>;
  /** fixed table height in px; header stays sticky while body scrolls */
  height?: number;
  /** max height in px; the table only scrolls when rows overflow */
  maxHeight?: number;
  /** zebra striped rows */
  stripe?: boolean;
  /** outer border + vertical cell borders */
  border?: boolean;
  /** show loading overlay */
  loading?: boolean;
  /** text/element for the empty state (defaults to the localized "no data") */
  emptyText?: ReactNode;
  /** ellipsis overflowing cell text with a title tooltip (vxe show-overflow) */
  showOverflow?: boolean;
  /** controlled sort state */
  sort?: { field: string; order: SortOrder };
  sortConfig?: VxeSortConfig;
  columnConfig?: VxeColumnConfig;
  rowConfig?: VxeRowConfig<Row>;
  /** extra class on the root wrapper */
  className?: string;
  /** extra style on the root wrapper */
  style?: CSSProperties;
  /** vxe-table `sort-change` event */
  onSortChange?: (event: SortChangeEvent) => void;
  /** vxe-table `cell-click` event */
  onCellClick?: (event: CellClickEvent<Row>) => void;
  /** vxe-table `row-click` event */
  onRowClick?: (event: RowClickEvent<Row>) => void;
  /** vxe-table `header-cell-click` event */
  onHeaderCellClick?: (event: HeaderCellClickEvent<Row>) => void;
  /** fired after column resize */
  onColumnWidthChange?: (widths: Record<string, number>) => void;
}
