import type { ReactNode } from 'react';

/** Slot renderers, aligned with vxe-table column slots. */
export interface VxeColumnSlots<Row> {
  /** custom body cell content; wins over formatter and raw value */
  default?: (params: { row: Row; rowIndex: number; column: VxeLeafColumn<Row>; value: unknown }) => ReactNode;
  /** custom header cell content */
  header?: (params: { column: VxeColumnProps<Row> }) => ReactNode;
  /** custom footer cell content */
  footer?: (params: { column: VxeColumnProps<Row>; value: unknown; rowIndex: number }) => ReactNode;
}

/** Column definition, aligned with vxe-table `<vxe-column>` props. */
export interface VxeColumnProps<Row> {
  /** built-in column type; 'seq' renders the row sequence number */
  type?: 'seq';
  /** data field; required on plain leaf columns, optional on `type` columns and group headers */
  field?: string;
  /** header text */
  title?: ReactNode;
  /** fixed column width in px */
  width?: number;
  /** minimal column width in px while resizing (default 40) */
  minWidth?: number;
  /** pin column to the left/right edge */
  fixed?: 'left' | 'right';
  /** enable header click sort cycling asc -> desc -> none */
  sortable?: boolean;
  /** custom sorter, defaults to string/number comparison */
  sorter?: (a: Row, b: Row) => number;
  /** text alignment of body cells (default 'left') */
  align?: 'left' | 'center' | 'right';
  /** text alignment of header cell, defaults to `align` */
  headerAlign?: 'left' | 'center' | 'right';
  /** text alignment of footer cells, defaults to `align` */
  footerAlign?: 'left' | 'center' | 'right';
  /** cell text formatter: value -> display */
  formatter?: (value: unknown, row: Row, rowIndex: number) => ReactNode;
  /** custom cell content; wins over formatter and raw value */
  slots?: VxeColumnSlots<Row>;
  /** custom cell className */
  className?: string;
  /** custom header cell className */
  headerClassName?: string;
  /** group header: nested child columns */
  children?: Array<VxeColumnProps<Row>>;
  /** set false to hide the column (default true) */
  visible?: boolean;
}

/** A column guaranteed to be a leaf with a field. */
export type VxeLeafColumn<Row> = VxeColumnProps<Row> & { field: string };
