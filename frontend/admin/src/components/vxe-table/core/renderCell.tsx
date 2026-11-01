import type { ReactNode } from 'react';
import type { VxeColumnProps, VxeLeafColumn } from '../types';

/** Plain-text representation of a raw cell value. */
export function cellText(value: unknown): string {
  if (value === null || value === undefined) return '';
  return typeof value === 'object' ? JSON.stringify(value) : String(value);
}

/** Header cell content: slots.header > title > field. */
export function renderHeaderContent<Row>(column: VxeColumnProps<Row>): ReactNode {
  if (column.slots?.header) return column.slots.header({ column });
  return column.title ?? column.field ?? '';
}

/** Body cell content: slots.default > formatter > raw value. */
export function renderBodyContent<Row>(column: VxeLeafColumn<Row>, row: Row, rowIndex: number): ReactNode {
  const value = (row as Record<string, unknown>)[column.field];
  if (column.slots?.default) return column.slots.default({ row, rowIndex, column, value });
  if (column.formatter) return column.formatter(value, row, rowIndex);
  return cellText(value);
}

export const ALIGN_CLASS = {
  left: 'is-align-left',
  center: 'is-align-center',
  right: 'is-align-right',
} as const;

export function joinClassNames(...names: Array<string | false | null | undefined>): string {
  return names.filter(Boolean).join(' ');
}
