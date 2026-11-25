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

/**
 * Edge leaf fields of each fixed block: the first/last leaf of the left-fixed
 * run and of the right-fixed run. The block outline (a line on its leftmost
 * and rightmost edge) is drawn on these cells while the table scrolls
 * horizontally.
 */
export interface FixedEdges {
  /** first (leftmost) left-fixed leaf — outer left edge of the block */
  leftFirst?: string;
  /** last left-fixed leaf — boundary towards the scrolling area */
  leftLast?: string;
  /** first (leftmost) right-fixed leaf — boundary towards the scrolling area */
  rightFirst?: string;
  /** last (rightmost) right-fixed leaf — outer right edge of the block */
  rightLast?: string;
}

export function computeFixedEdges<Row>(columns: Array<VxeLeafColumn<Row>>): FixedEdges {
  const edges: FixedEdges = {};
  for (const col of columns) {
    if (col.fixed === 'left') {
      edges.leftFirst ??= col.field;
      edges.leftLast = col.field;
    } else if (col.fixed === 'right') {
      edges.rightFirst ??= col.field;
      edges.rightLast = col.field;
    }
  }
  return edges;
}

/**
 * Marker classes for the outline of a fixed block: `is-fixed-left-first` /
 * `is-fixed-left-last` for the left run, `is-fixed-right-first` /
 * `is-fixed-right-last` for the right run. Cells report the fields of their
 * first and last leaf so group headers resolve to the block edges too.
 */
export function fixedBlockEdgeClasses(
  fixed: 'left' | 'right' | undefined,
  firstLeafField: string | undefined,
  lastLeafField: string | undefined,
  edges: FixedEdges,
): string | undefined {
  if (!fixed) return undefined;
  const classes: string[] = [];
  if (fixed === 'left') {
    if (firstLeafField !== undefined && firstLeafField === edges.leftFirst) classes.push('is-fixed-left-first');
    if (lastLeafField !== undefined && lastLeafField === edges.leftLast) classes.push('is-fixed-left-last');
  } else {
    if (firstLeafField !== undefined && firstLeafField === edges.rightFirst) classes.push('is-fixed-right-first');
    if (lastLeafField !== undefined && lastLeafField === edges.rightLast) classes.push('is-fixed-right-last');
  }
  return classes.length > 0 ? classes.join(' ') : undefined;
}
