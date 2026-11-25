import { useLayoutEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, UIEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { normalizeColumns } from '../columns/normalizeColumns';
import { useColumnWidths } from '../state/useColumnWidths';
import { useSort } from '../state/useSort';
import { useStableCallback } from '../state/useStableCallback';
import type { VxeLeafColumn, VxeTableProps } from '../types';
import { joinClassNames } from './renderCell';
import { computeFixedEdges, computeStickyOffsets, fixedBlockEdgeClasses } from './sticky';
import { TableBody } from './TableBody';
import { TableHeader } from './TableHeader';

/** Shared <colgroup> so header and body tables keep identical column widths. */
function TableColGroup<Row>(props: { columns: Array<VxeLeafColumn<Row>>; widths: Record<string, number> }) {
  return (
    <colgroup>
      {props.columns.map((column) => (
        <col key={column.field} style={{ width: props.widths[column.field] || undefined }} />
      ))}
    </colgroup>
  );
}

/**
 * React replication of vxe-table (`<vxe-table>` + `<vxe-column>`).
 *
 * Pipeline: columns -> normalizeColumns (group headers) -> useSort -> render.
 * Row/cell components are memoized; event callbacks are stabilized so inline
 * handlers do not bust the memo cache.
 */
export function VxeTable<Row>(props: VxeTableProps<Row>) {
  const { t } = useTranslation();
  const {
    data,
    columns,
    height,
    maxHeight,
    stripe = false,
    border = false,
    loading = false,
    emptyText,
    showOverflow = false,
    sort,
    sortConfig,
    columnConfig,
    rowConfig,
    className,
    style,
    onSortChange,
    onCellClick,
    onRowClick,
    onHeaderCellClick,
    onColumnWidthChange,
  } = props;

  const { leafColumns, headerRows } = useMemo(() => normalizeColumns(columns), [columns]);
  const resetKey = leafColumns.map((col) => col.field).join('|');

  const resizable = columnConfig?.resizable ?? leafColumns.some((col) => col.width !== undefined);
  const { widths, resizingField, startResize } = useColumnWidths({ columns: leafColumns, resizable, resetKey, onColumnWidthChange });
  const { sortState, cycleSort, sortedData } = useSort({
    columns: leafColumns,
    data,
    sort,
    defaultSort: sortConfig?.defaultSort,
    resetKey,
    onSortChange,
  });

  const offsets = useMemo(() => computeStickyOffsets(leafColumns, widths), [leafColumns, widths]);
  const totalWidth = useMemo(
    () => leafColumns.reduce((sum, col) => sum + (widths[col.field] ?? col.width ?? 0), 0),
    [leafColumns, widths],
  );

  // Outline cells of each fixed block (its leftmost and rightmost edge); the
  // lines themselves are only painted while the table scrolls horizontally
  // (`.has-h-scroll` on the root).
  const edges = useMemo(() => computeFixedEdges(leafColumns), [leafColumns]);
  const headerEdgeClasses = useMemo(() => {
    const map: Record<string, string | undefined> = {};
    for (const cells of headerRows) {
      for (const cell of cells) {
        // header rows are mapped to leaves via leafStart: a row under a
        // rowSpan cell skips that cell, so counting cells in the row would
        // shift every later cell by the number of spanning cells above it.
        map[cell.key] = fixedBlockEdgeClasses(
          cell.fixed,
          leafColumns[cell.leafStart]?.field,
          leafColumns[cell.leafStart + cell.colSpan - 1]?.field,
          edges,
        );
      }
    }
    return map;
  }, [headerRows, leafColumns, edges]);

  const handleCellClick = useStableCallback(onCellClick);
  const handleRowClick = useStableCallback(onRowClick);
  const handleHeaderCellClick = useStableCallback(onHeaderCellClick);

  // ---- split scrolling: header is fixed, the vertical scrollbar lives on the
  // body wrapper only. Header horizontal scroll is driven by the body, and the
  // header gets a right padding equal to the body's vertical scrollbar width so
  // columns (especially right-fixed ones) stay aligned.
  const headerWrapperRef = useRef<HTMLDivElement>(null);
  const bodyWrapperRef = useRef<HTMLDivElement>(null);
  const [scrollbarWidth, setScrollbarWidth] = useState(0);
  const [hasHScroll, setHasHScroll] = useState(false);

  const handleBodyScroll = (event: UIEvent<HTMLDivElement>) => {
    const body = event.currentTarget;
    if (headerWrapperRef.current && headerWrapperRef.current.scrollLeft !== body.scrollLeft) {
      headerWrapperRef.current.scrollLeft = body.scrollLeft;
    }
  };

  useLayoutEffect(() => {
    const body = bodyWrapperRef.current;
    if (!body) return;
    const measure = () => {
      const hasVerticalScrollbar = body.scrollHeight > body.clientHeight + 1;
      const width = hasVerticalScrollbar ? body.offsetWidth - body.clientWidth : 0;
      setScrollbarWidth((prev) => (prev === width ? prev : width));
      const hScroll = body.scrollWidth > body.clientWidth + 1;
      setHasHScroll((prev) => (prev === hScroll ? prev : hScroll));
    };
    measure();
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(measure);
    observer.observe(body);
    const table = body.querySelector('table');
    if (table) observer.observe(table);
    return () => observer.disconnect();
  }, []);

  const wrapperStyle: CSSProperties = { ...style, height: height ?? undefined, maxHeight: maxHeight ?? undefined };

  return (
    <div
      className={joinClassNames(
        'vxe-table',
        border && 'vxe-table--border',
        stripe && 'vxe-table--stripe',
        hasHScroll && 'has-h-scroll',
        loading && 'is-loading',
        className,
      )}
      style={wrapperStyle}
    >
      <div className="vxe-table--header-wrapper" ref={headerWrapperRef} style={{ paddingRight: scrollbarWidth || undefined }}>
        <table style={{ tableLayout: 'fixed', width: totalWidth || '100%' }}>
          <TableColGroup columns={leafColumns} widths={widths} />
          <TableHeader
            headerRows={headerRows}
            edgeClasses={headerEdgeClasses}
            offsets={offsets}
            resizable={resizable}
            resizingField={resizingField}
            showOverflow={showOverflow}
            sortState={sortState}
            onHeaderCellClick={handleHeaderCellClick}
            onResizeStart={startResize}
            onSortClick={cycleSort}
          />
        </table>
      </div>
      <div className="vxe-table--body-wrapper" ref={bodyWrapperRef} onScroll={handleBodyScroll}>
        <table style={{ tableLayout: 'fixed', width: totalWidth || '100%' }}>
          <TableColGroup columns={leafColumns} widths={widths} />
          <TableBody
            columns={leafColumns}
            emptyText={emptyText ?? t('vxe.emptyText')}
            edges={edges}
            offsets={offsets}
            rowConfig={rowConfig}
            rows={sortedData}
            showOverflow={showOverflow}
            stripe={stripe}
            onCellClick={handleCellClick}
            onRowClick={handleRowClick}
          />
        </table>
      </div>
      {loading ? (
        <div className="vxe-table--loading">
          <span className="vxe-loading--spinner" />
        </div>
      ) : null}
    </div>
  );
}
