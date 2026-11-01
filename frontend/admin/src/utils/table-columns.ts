import type { TableColumnGroupType, TableColumnType, TableColumnsType } from 'antd';
import i18n from '@/i18n';
import { formatDateTime, isDateTimeField } from '@/utils/format-time';

type AnyColumn<T> = TableColumnGroupType<T> | TableColumnType<T>;

export const MIN_LIST_COLUMN_WIDTH = 128;

/** Map antd Table `column.fixed` aliases (`left`/`right`) onto v6 `start`/`end`. */
export function toAntdColumnFixed(fixed: TableColumnType<unknown>['fixed']): 'end' | 'start' | undefined {
  if (fixed === true || fixed === 'left' || fixed === 'start') return 'start';
  if (fixed === 'right' || fixed === 'end') return 'end';
  return undefined;
}

/** True for the row-actions column (not a data field titled 操作). */
export function isActionsColumn<T>(column: TableColumnType<T>): boolean {
  if (column.key === 'actions') return true;
  // Title match is only a fallback for keyless columns (a few inline tables
  // declare 操作 without a key); keyed columns are identified structurally.
  return (
    column.key == null &&
    column.title === i18n.t('common.actions') &&
    (column.dataIndex == null || column.dataIndex === '')
  );
}

function isGroupColumn<T>(column: AnyColumn<T>): column is TableColumnGroupType<T> {
  return 'children' in column && Array.isArray(column.children);
}

/** Stable key for column order / width / drag identity. */
export function getColumnKey<T>(column: AnyColumn<T>, index = 0): string {
  if (column.key != null && column.key !== '') return String(column.key);
  if (!isGroupColumn(column)) {
    const dataIndex = column.dataIndex;
    if (dataIndex != null && dataIndex !== '') {
      return Array.isArray(dataIndex) ? dataIndex.map(String).join('.') : String(dataIndex);
    }
  }
  if (typeof column.title === 'string' || typeof column.title === 'number') return String(column.title);
  return `col-${index}`;
}

/** Ensure every column has a stable `key` (and recurse into groups). */
export function withStableColumnKeys<T>(columns: TableColumnsType<T>): TableColumnsType<T> {
  return columns.map((col, index) => {
    if (!col || typeof col !== 'object') return col;
    if (isGroupColumn(col)) {
      const group = col as TableColumnGroupType<T>;
      return {
        ...group,
        key: group.key ?? getColumnKey(group, index),
        children: withStableColumnKeys(group.children) as TableColumnGroupType<T>['children'],
      };
    }
    const column = col as TableColumnType<T>;
    return { ...column, key: column.key ?? getColumnKey(column, index) };
  });
}

/**
 * Normalize list-table columns: center headers and cells, including actions,
 * ellipsis+tooltip for text fields, and pin the 操作 column to `fixed: 'end'`.
 * Pair with `scroll.x` + `sticky` on Table (antd sticky / fixed-columns demo).
 *
 * An explicit column width is an explicit overflow boundary, so it also
 * enables ellipsis for rendered columns. Callers can opt out with
 * `ellipsis: false` for controls that must remain fully visible.
 */
export function normalizeListColumns<T>(columns: TableColumnsType<T>): TableColumnsType<T> {
  return withStableColumnKeys(
    columns.map((col) => {
      if (!col || typeof col !== 'object') return col;
      if (isGroupColumn(col)) {
        const group = col as TableColumnGroupType<T>;
        return { ...group, children: normalizeListColumns(group.children) as TableColumnGroupType<T>['children'] };
      }
      const column = { ...(col as TableColumnType<T>) };
      if (isActionsColumn(column)) {
        column.key = column.key ?? 'actions';
        column.fixed = toAntdColumnFixed(column.fixed) ?? 'end';
        column.width = column.width ?? 140;
        column.align = column.align ?? 'center';
        column.ellipsis = column.ellipsis ?? false;
        // Attach stable classes so CSS can force-center header + cells
        // even if a caller overrides `align` elsewhere.
        const prevOnHeaderCell = column.onHeaderCell;
        const prevOnCell = column.onCell;
        column.onHeaderCell = ((cell: unknown) => {
          const extra =
            typeof prevOnHeaderCell === 'function'
              ? (prevOnHeaderCell(cell as never) as Record<string, unknown> | undefined)
              : undefined;
          const prevClass = extra?.className as string | undefined;
          return {
            ...extra,
            className: prevClass
              ? `${prevClass} list-table-actions-header`
              : 'list-table-actions-header',
          };
        }) as TableColumnType<T>['onHeaderCell'];
        column.onCell = ((record: unknown, index?: number) => {
          const extra =
            typeof prevOnCell === 'function'
              ? (prevOnCell(record as never, index as never) as Record<string, unknown> | undefined)
              : undefined;
          const prevClass = extra?.className as string | undefined;
          return {
            ...extra,
            className: prevClass ? `${prevClass} list-table-actions-cell` : 'list-table-actions-cell',
          };
        }) as TableColumnType<T>['onCell'];
      } else {
        column.align = column.align ?? 'center';
        const mappedFixed = toAntdColumnFixed(column.fixed);
        if (mappedFixed) column.fixed = mappedFixed;
        if (column.ellipsis === undefined && (column.render == null || column.width != null)) {
          // Official Column ellipsis; ResourceCrudPage overlays Tooltip + showTitle:false.
          column.ellipsis = { showTitle: true };
        }
        if (column.render == null && isDateTimeField(column.dataIndex)) {
          // Uniform time display: yyyy-MM-dd HH:mm:ss.
          column.render = (value: unknown) => formatDateTime(value as string | null | undefined);
        }
      }
      return column as AnyColumn<T>;
    }),
  );
}
