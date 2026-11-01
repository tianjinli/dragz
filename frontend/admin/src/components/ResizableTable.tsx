import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Table, type TableColumnType, type TableColumnsType, type TableProps } from 'antd';
import { useTranslation } from 'react-i18next';
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type ColumnSizingState,
  type Header,
  type Row,
} from '@tanstack/react-table';

type AnyHeader<TData> = Header<TData, unknown>;
type ResizeHandleTitleProps<TData> = { header: AnyHeader<TData> };

function readStoredSizing(storageKey: string | undefined): ColumnSizingState {
  if (!storageKey || typeof window === 'undefined') return {};
  try {
    const raw = window.localStorage.getItem(storageKey);
    if (!raw) return {};
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {};
    return parsed as ColumnSizingState;
  } catch {
    return {};
  }
}

function writeStoredSizing(storageKey: string | undefined, sizing: ColumnSizingState) {
  if (!storageKey || typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(sizing));
  } catch {
    /* storage disabled or full — column sizing simply will not persist */
  }
}

/** Drag handle for one header. TanStack owns the drag math; we only own the chrome. */
function ResizeHandle<TData extends object>({ header }: ResizeHandleTitleProps<TData>) {
  const { t } = useTranslation();
  const column = header.column;
  const canResize = column.getCanResize();
  const label = typeof column.columnDef.header === 'string' ? column.columnDef.header : column.id;

  const startResize = (event: unknown) => {
    document.body.classList.add('is-column-resizing');
    header.getResizeHandler()(event);
  };

  if (!canResize) return null;
  return (
    <span
      aria-label={t('common.resizeColumn', { label })}
      className="resizable-th-handle"
      data-resizing={column.getIsResizing() ? 'true' : undefined}
      onDoubleClick={(event) => {
        event.stopPropagation();
        column.resetSize();
      }}
      onMouseDown={startResize}
      onTouchStart={startResize}
      role="separator"
      title={t('common.resizeColumnTip')}
    />
  );
}

function HeaderTitle<TData extends object>({ header }: ResizeHandleTitleProps<TData>) {
  const content = header.isPlaceholder
    ? null
    : flexRender(header.column.columnDef.header, header.getContext());
  return (
    <span className="resizable-th-inner">
      <span className="resizable-th-label">{content}</span>
      <ResizeHandle header={header} />
    </span>
  );
}

/** Escape hatch for antd-only column props (align / ellipsis / fixed / …) via columnDef.meta.antd. */
function antdColumnMeta<TData extends object>(
  header: AnyHeader<TData>,
): Partial<TableColumnType<TData>> {
  const meta = header.column.columnDef.meta as { antd?: Partial<TableColumnType<TData>> } | undefined;
  return meta?.antd ?? {};
}

function mapHeadersToAntdColumns<TData extends object>(
  headers: Array<AnyHeader<TData>>,
  rows: Array<Row<TData>>,
): TableColumnsType<TData> {
  return headers.map((header) => {
    const column = header.column;
    const isGroup = header.subHeaders.length > 0;
    const cell = (rowIndex: number, fallback: unknown): ReactNode => {
      const row = rows[rowIndex];
      const found = row?.getVisibleCells().find((item) => item.column.id === column.id);
      if (!found) return fallback as ReactNode;
      return flexRender(found.column.columnDef.cell, found.getContext());
    };
    const base = {
      ...antdColumnMeta(header),
      key: column.id,
      onHeaderCell: () => ({ className: 'resizable-th' }),
      title: <HeaderTitle header={header} />,
    };
    if (isGroup) {
      return {
        ...base,
        children: mapHeadersToAntdColumns(header.subHeaders, rows),
      } as TableColumnsType<TData>[number];
    }
    return {
      ...base,
      dataIndex:
        'accessorKey' in column.columnDef && typeof column.columnDef.accessorKey === 'string'
          ? column.columnDef.accessorKey
          : undefined,
      render: (value: unknown, _record: TData, index: number) => cell(index, value),
      width: header.getSize(),
    } as TableColumnsType<TData>[number];
  });
}

export interface ResizableTableProps<TData extends object> {
  /** TanStack column defs. Use `meta.antd` for antd-only props (align, ellipsis, fixed…). */
  columns: Array<ColumnDef<TData, unknown>>;
  data: TData[];
  /** localStorage key that persists column widths across reloads. */
  storageKey?: string;
  /** Initial widths keyed by column id (also the reset target of a double-clicked handle). */
  defaultColumnSizing?: ColumnSizingState;
  /** Notified with the committed column widths whenever the user drags a handle. */
  onColumnSizingChange?: (sizing: ColumnSizingState) => void;
  /** Pass-through antd Table props (pagination, rowSelection, bordered…). */
  tableProps?: Partial<TableProps<TData>>;
}

/**
 * antd Table whose header columns can be dragged wider/narrower.
 *
 * TanStack Table is the sizing engine: it owns the drag math (`columnSizing`
 * state, min/max clamping, double-click reset) while antd renders the DOM.
 * Widths are applied to each column's `width`, and the drag handle is injected
 * through the column `title` + `onHeaderCell`.
 */
export function ResizableTable<TData extends object>({
  columns,
  data,
  storageKey,
  defaultColumnSizing,
  onColumnSizingChange,
  tableProps,
}: ResizableTableProps<TData>) {
  const [columnSizing, setColumnSizing] = useState<ColumnSizingState>(() => {
    const stored = readStoredSizing(storageKey);
    return Object.keys(stored).length > 0 ? stored : { ...defaultColumnSizing };
  });

  const table = useReactTable<TData>({
    columns,
    data,
    defaultColumn: { minSize: 80 },
    columnResizeDirection: 'ltr',
    columnResizeMode: 'onChange',
    enableColumnResizing: true,
    getCoreRowModel: getCoreRowModel(),
    onColumnSizingChange: (updater) => {
      setColumnSizing((previous) => (typeof updater === 'function' ? updater(previous) : updater));
    },
    state: { columnSizing },
  });

  const rows = table.getRowModel().rows;
  // Rebuilt on every render on purpose: `column.getIsResizing()` reads the
  // transient resize info, which changes on mousedown before any commit.
  const antdColumns = mapHeadersToAntdColumns(table.getHeaderGroups()[0]?.headers ?? [], rows);

  // Keep the latest callback in a ref so an inline arrow never re-triggers the effect.
  const notifySizingChange = useRef(onColumnSizingChange);
  useEffect(() => {
    notifySizingChange.current = onColumnSizingChange;
  }, [onColumnSizingChange]);

  useEffect(() => {
    notifySizingChange.current?.(columnSizing);
  }, [columnSizing]);

  useEffect(() => {
    writeStoredSizing(storageKey, columnSizing);
  }, [storageKey, columnSizing]);

  // Keep a global `col-resize` cursor + disable text selection while dragging.
  useEffect(() => {
    const stop = () => document.body.classList.remove('is-column-resizing');
    window.addEventListener('mouseup', stop);
    window.addEventListener('touchend', stop);
    window.addEventListener('touchcancel', stop);
    return () => {
      window.removeEventListener('mouseup', stop);
      window.removeEventListener('touchend', stop);
      window.removeEventListener('touchcancel', stop);
    };
  }, []);

  return (
    <Table<TData>
      {...tableProps}
      className={tableProps?.className ? `resizable-table ${tableProps.className}` : 'resizable-table'}
      columns={antdColumns}
      dataSource={data}
      rowKey={(_record, index) => rows[index ?? 0]?.id ?? String(index ?? 0)}
      scroll={{ x: table.getTotalSize(), ...tableProps?.scroll }}
      tableLayout="fixed"
    />
  );
}
