import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { MouseEvent as ReactMouseEvent } from 'react';
import type { VxeLeafColumn } from '../types';

export const DEFAULT_MIN_COLUMN_WIDTH = 40;

export interface UseColumnWidthsOptions<Row> {
  columns: Array<VxeLeafColumn<Row>>;
  /** whether header-edge drag resizing is enabled */
  resizable: boolean;
  /** bump to drop local drag overrides (column signature) */
  resetKey: string;
  onColumnWidthChange?: (widths: Record<string, number>) => void;
}

/**
 * Column width bookkeeping: prop widths as defaults, local overrides while the
 * user drags the header edge (vxe column-config.resizable).
 */
export function useColumnWidths<Row>(options: UseColumnWidthsOptions<Row>) {
  const { columns, resizable, resetKey, onColumnWidthChange } = options;

  const [dragWidths, setDragWidths] = useState<Record<string, number>>({});
  useEffect(() => setDragWidths({}), [resetKey]);

  const widths = useMemo(() => {
    const map: Record<string, number> = {};
    for (const col of columns) map[col.field] = dragWidths[col.field] ?? col.width ?? 0;
    return map;
  }, [columns, dragWidths]);

  const [resizingField, setResizingField] = useState<string | null>(null);
  const drag = useRef<{ field: string; startX: number; startWidth: number; minWidth: number } | null>(null);

  const commit = useCallback(
    (next: Record<string, number>) => {
      setDragWidths(next);
      onColumnWidthChange?.(next);
    },
    [onColumnWidthChange],
  );

  const startResize = useCallback(
    (event: ReactMouseEvent, column: VxeLeafColumn<Row>) => {
      if (!resizable) return;
      event.preventDefault();
      event.stopPropagation();
      drag.current = {
        field: column.field,
        startX: event.clientX,
        startWidth: widths[column.field] ?? column.width ?? 0,
        minWidth: column.minWidth ?? DEFAULT_MIN_COLUMN_WIDTH,
      };
      setResizingField(column.field);
    },
    [resizable, widths],
  );

  useEffect(() => {
    if (!resizingField) return undefined;
    const move = (event: MouseEvent) => {
      const state = drag.current;
      if (!state) return;
      const nextWidth = Math.max(state.minWidth, state.startWidth + event.clientX - state.startX);
      commit({ ...widths, [state.field]: nextWidth });
    };
    const stop = () => {
      drag.current = null;
      setResizingField(null);
    };
    window.addEventListener('mousemove', move);
    window.addEventListener('mouseup', stop);
    return () => {
      window.removeEventListener('mousemove', move);
      window.removeEventListener('mouseup', stop);
    };
  }, [commit, resizingField, widths]);

  return { widths, resizingField, startResize };
}
