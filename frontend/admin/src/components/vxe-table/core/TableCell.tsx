import { memo } from 'react';
import type { CSSProperties } from 'react';
import type { CellClickEvent, VxeLeafColumn } from '../types';
import { ALIGN_CLASS, cellText, joinClassNames, renderBodyContent } from './renderCell';

export interface TableCellProps<Row> {
  column: VxeLeafColumn<Row>;
  row: Row;
  rowIndex: number;
  /** inline sticky style for fixed columns; undefined for scrolling cells */
  stickyStyle?: CSSProperties;
  showOverflow: boolean;
  onCellClick?: (event: CellClickEvent<Row>) => void;
}

function TableCellInner<Row>(props: TableCellProps<Row>) {
  const { column, row, rowIndex, stickyStyle, showOverflow, onCellClick } = props;
  const content = renderBodyContent(column, row, rowIndex);
  const title = showOverflow && typeof content === 'string' ? content : undefined;
  // tooltip needs plain text even when a formatter produced it
  const plainTitle = title ?? (showOverflow && !column.slots?.default ? cellText((row as Record<string, unknown>)[column.field]) : undefined);

  return (
    <td
      className={joinClassNames(
        'vxe-body--column',
        column.align ? ALIGN_CLASS[column.align] : undefined,
        column.fixed ? 'is-fixed' : undefined,
        column.className,
      )}
      style={stickyStyle}
      onClick={() => onCellClick?.({ row, rowIndex, column, field: column.field })}
    >
      <div className={joinClassNames('vxe-cell', showOverflow && 'vxe-cell--ellipsis')} title={plainTitle || undefined}>
        {content}
      </div>
    </td>
  );
}

export const TableCell = memo(TableCellInner) as typeof TableCellInner;
