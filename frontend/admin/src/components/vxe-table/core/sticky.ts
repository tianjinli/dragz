import type { CSSProperties } from 'react';
import type { VxeLeafColumn } from '../types';

export interface StickyOffsets {
  left: Record<string, number>;
  right: Record<string, number>;
}

/**
 * Offset (in px) of each sticky column from its pinning edge, so that
 * consecutive fixed columns stack correctly (vxe-table fixed left/right).
 */
export function computeStickyOffsets<Row>(
  columns: Array<VxeLeafColumn<Row>>,
  widths: Record<string, number>,
): StickyOffsets {
  const left: Record<string, number> = {};
  const right: Record<string, number> = {};
  let acc = 0;
  for (const col of columns) {
    if (col.fixed === 'left') {
      left[col.field] = acc;
      acc += widths[col.field] ?? col.width ?? 0;
    }
  }
  acc = 0;
  for (let i = columns.length - 1; i >= 0; i -= 1) {
    const col = columns[i];
    if (col.fixed === 'right') {
      right[col.field] = acc;
      acc += widths[col.field] ?? col.width ?? 0;
    }
  }
  return { left, right };
}

/** Inline style for a fixed cell; undefined for normal (scrolling) cells. */
export function stickyCellStyle(fixed: 'left' | 'right' | undefined, stickyField: string | undefined, offsets: StickyOffsets): CSSProperties | undefined {
  if (!fixed || !stickyField) return undefined;
  if (fixed === 'left') return { position: 'sticky', left: offsets.left[stickyField] ?? 0, zIndex: 1 };
  return { position: 'sticky', right: offsets.right[stickyField] ?? 0, zIndex: 1 };
}
