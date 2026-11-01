import { memo } from 'react';
import type { MouseEvent as ReactMouseEvent } from 'react';
import type { HeaderCellMeta } from '../columns/normalizeColumns';
import type { HeaderCellClickEvent, SortOrder, VxeLeafColumn } from '../types';
import { ALIGN_CLASS, joinClassNames, renderHeaderContent } from './renderCell';
import type { StickyOffsets } from './sticky';
import { stickyCellStyle } from './sticky';

export interface TableHeaderProps<Row> {
  headerRows: Array<Array<HeaderCellMeta<Row>>>;
  offsets: StickyOffsets;
  sortState: { field: string; order: SortOrder };
  resizable: boolean;
  resizingField: string | null;
  showOverflow: boolean;
  /** click on any header cell (group or leaf) */
  onHeaderCellClick?: (event: HeaderCellClickEvent<Row>) => void;
  /** click on a sortable leaf header — triggers the sort cycle */
  onSortClick: (column: VxeLeafColumn<Row>) => void;
  onResizeStart: (event: ReactMouseEvent, column: VxeLeafColumn<Row>) => void;
}

function SortMark<Row>(props: { column: VxeLeafColumn<Row>; sortState: { field: string; order: SortOrder } }) {
  const { column, sortState } = props;
  const active = sortState.field === column.field;
  return (
    <span className={joinClassNames('vxe-sort--wrapper', active && 'is-active')}>
      <span className="vxe-sort--asc" data-active={active && sortState.order === 'asc' ? 'true' : undefined}>
        ▲
      </span>
      <span className="vxe-sort--desc" data-active={active && sortState.order === 'desc' ? 'true' : undefined}>
        ▼
      </span>
    </span>
  );
}

function TableHeaderInner<Row>(props: TableHeaderProps<Row>) {
  const { headerRows, offsets, sortState, resizable, resizingField, showOverflow, onHeaderCellClick, onSortClick, onResizeStart } = props;

  const handleClick = (meta: HeaderCellMeta<Row>) => {
    onHeaderCellClick?.({ column: meta.column });
    if (meta.leaf?.sortable) onSortClick(meta.leaf);
  };

  return (
    <thead className="vxe-table--header">
      {headerRows.map((cells, rowIndex) => (
        <tr key={rowIndex}>
          {cells.map((meta) => {
            const { column, leaf } = meta;
            const align = column.headerAlign ?? column.align ?? (meta.isGroup ? 'center' : undefined);
            return (
              <th
                key={meta.key}
                className={joinClassNames(
                  'vxe-header--column',
                  meta.isGroup && 'is-group',
                  align ? ALIGN_CLASS[align] : undefined,
                  meta.fixed ? 'is-fixed' : undefined,
                  column.headerClassName,
                  leaf?.sortable && 'is-sortable',
                  leaf && resizingField === leaf.field && 'is-resizing',
                )}
                colSpan={meta.colSpan}
                rowSpan={meta.rowSpan}
                style={stickyCellStyle(meta.fixed, meta.stickyField, offsets)}
                onClick={() => handleClick(meta)}
              >
                <div className={joinClassNames('vxe-cell', showOverflow && 'vxe-cell--ellipsis')}>
                  {renderHeaderContent(column)}
                  {leaf?.sortable ? <SortMark column={leaf} sortState={sortState} /> : null}
                </div>
                {leaf && resizable ? (
                  <span
                    aria-hidden="true"
                    className="vxe-header--resizable"
                    data-resizing={resizingField === leaf.field ? 'true' : undefined}
                    role="separator"
                    onClick={(event) => event.stopPropagation()}
                    onMouseDown={(event) => onResizeStart(event, leaf)}
                  />
                ) : null}
              </th>
            );
          })}
        </tr>
      ))}
    </thead>
  );
}

export const TableHeader = memo(TableHeaderInner) as typeof TableHeaderInner;
