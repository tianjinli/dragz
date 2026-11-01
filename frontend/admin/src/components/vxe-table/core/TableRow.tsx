import { memo } from 'react';
import type { CellClickEvent, RowClickEvent, VxeLeafColumn, VxeRowConfig } from '../types';
import { joinClassNames } from './renderCell';
import type { StickyOffsets } from './sticky';
import { stickyCellStyle } from './sticky';
import { TableCell } from './TableCell';

export interface TableRowProps<Row> {
  row: Row;
  rowIndex: number;
  columns: Array<VxeLeafColumn<Row>>;
  offsets: StickyOffsets;
  stripe: boolean;
  showOverflow: boolean;
  rowConfig?: VxeRowConfig<Row>;
  onCellClick?: (event: CellClickEvent<Row>) => void;
  onRowClick?: (event: RowClickEvent<Row>) => void;
}

function TableRowInner<Row>(props: TableRowProps<Row>) {
  const { row, rowIndex, columns, offsets, stripe, showOverflow, rowConfig, onCellClick, onRowClick } = props;

  const extraClass = rowConfig?.className;
  const resolvedExtra = typeof extraClass === 'function' ? extraClass({ row, rowIndex }) : extraClass;

  return (
    <tr
      className={joinClassNames(
        'vxe-body--row',
        stripe && rowIndex % 2 === 1 && 'row--stripe',
        rowConfig?.isCurrent?.(row) && 'row--current',
        resolvedExtra || undefined,
      )}
      onClick={() => onRowClick?.({ row, rowIndex })}
    >
      {columns.map((column) => (
        <TableCell
          key={column.field}
          column={column}
          row={row}
          rowIndex={rowIndex}
          showOverflow={showOverflow}
          stickyStyle={stickyCellStyle(column.fixed, column.field, offsets)}
          onCellClick={onCellClick}
        />
      ))}
    </tr>
  );
}

export const TableRow = memo(TableRowInner) as typeof TableRowInner;
