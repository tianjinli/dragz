import { memo } from 'react';
import type { ReactNode } from 'react';
import type { CellClickEvent, RowClickEvent, VxeLeafColumn, VxeRowConfig } from '../types';
import type { StickyOffsets } from './sticky';
import { TableRow } from './TableRow';

export interface TableBodyProps<Row> {
  rows: Row[];
  columns: Array<VxeLeafColumn<Row>>;
  offsets: StickyOffsets;
  stripe: boolean;
  showOverflow: boolean;
  emptyText: ReactNode;
  rowConfig?: VxeRowConfig<Row>;
  onCellClick?: (event: CellClickEvent<Row>) => void;
  onRowClick?: (event: RowClickEvent<Row>) => void;
}

function TableBodyInner<Row>(props: TableBodyProps<Row>) {
  const { rows, columns, offsets, stripe, showOverflow, emptyText, rowConfig, onCellClick, onRowClick } = props;

  return (
    <tbody className="vxe-table--body">
      {rows.length === 0 ? (
        <tr className="vxe-body--row is-empty">
          <td className="vxe-body--column" colSpan={Math.max(columns.length, 1)}>
            <div className="vxe-cell vxe-cell--empty">{emptyText}</div>
          </td>
        </tr>
      ) : (
        rows.map((row, rowIndex) => (
          <TableRow
            key={rowIndex}
            columns={columns}
            offsets={offsets}
            row={row}
            rowConfig={rowConfig}
            rowIndex={rowIndex}
            showOverflow={showOverflow}
            stripe={stripe}
            onCellClick={onCellClick}
            onRowClick={onRowClick}
          />
        ))
      )}
    </tbody>
  );
}

export const TableBody = memo(TableBodyInner) as typeof TableBodyInner;
