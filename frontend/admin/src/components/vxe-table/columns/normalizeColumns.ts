import type { VxeColumnProps, VxeLeafColumn } from '../types';

/** One cell of the (possibly multi-row) table header. */
export interface HeaderCellMeta<Row> {
  /** react key, unique across the whole header */
  key: string;
  /** column definition as provided by the user */
  column: VxeColumnProps<Row>;
  /** set when this header cell maps to a leaf column */
  leaf?: VxeLeafColumn<Row>;
  colSpan: number;
  rowSpan: number;
  /** group header cells are centered by default and never sortable/resizable */
  isGroup: boolean;
  /**
   * Effective fixed mode. A group header is fixed only when ALL of its leaf
   * descendants share the same fixed mode; otherwise it scrolls (degrade).
   */
  fixed?: 'left' | 'right';
  /** leaf field used to look up the sticky offset for this cell */
  stickyField?: string;
}

export interface NormalizedColumns<Row> {
  /** flattened leaf columns in left-to-right order — used for colgroup/body */
  leafColumns: Array<VxeLeafColumn<Row>>;
  /** header rows top -> bottom, each a list of cells with colSpan/rowSpan */
  headerRows: Array<Array<HeaderCellMeta<Row>>>;
  /** number of header rows (1 = flat columns, no grouping) */
  headerLevel: number;
}

function visibleChildren<Row>(column: VxeColumnProps<Row>): Array<VxeColumnProps<Row>> {
  return (column.children ?? []).filter((child) => child.visible !== false);
}

function isVisible<Row>(column: VxeColumnProps<Row>): boolean {
  return column.visible !== false;
}

interface NodeInfo {
  fixed?: 'left' | 'right';
  firstLeafField?: string;
  leafCount: number;
  depth: number;
}

/** Bottom-up analysis: fixed mode of a group is the common fixed mode of its subtree. */
function analyze<Row>(column: VxeColumnProps<Row>): NodeInfo {
  const children = visibleChildren(column);
  if (children.length === 0) {
    return { fixed: column.fixed, firstLeafField: column.field, leafCount: 1, depth: 1 };
  }
  const infos = children.map(analyze);
  const firstFixed = infos[0].fixed;
  const sharedFixed = firstFixed && infos.every((info) => info.fixed === firstFixed) ? firstFixed : undefined;
  return {
    fixed: sharedFixed,
    firstLeafField: infos[0].firstLeafField,
    leafCount: infos.reduce((sum, info) => sum + info.leafCount, 0),
    depth: 1 + Math.max(...infos.map((info) => info.depth)),
  };
}

/**
 * Normalizes the user column tree:
 * - drops `visible: false` columns
 * - flattens leaf columns for the body/colgroup
 * - builds header rows with colSpan/rowSpan for group headers
 *
 * vxe-table semantics: a leaf in a nested group stretches across the remaining
 * header rows via rowSpan; a group spans its leaves via colSpan.
 */
export function normalizeColumns<Row>(columns: Array<VxeColumnProps<Row>>): NormalizedColumns<Row> {
  const roots = columns.filter(isVisible);
  const headerLevel = roots.reduce((max, col) => Math.max(max, analyze(col).depth), 1);
  const headerRows: Array<Array<HeaderCellMeta<Row>>> = Array.from({ length: headerLevel }, () => []);
  const leafColumns: Array<VxeLeafColumn<Row>> = [];

  const walk = (column: VxeColumnProps<Row>, level: number, path: string) => {
    const info = analyze(column);
    const children = visibleChildren(column);
    const key = column.field ?? `group:${path}`;
    if (children.length === 0) {
      const leaf = column as VxeLeafColumn<Row>;
      if (import.meta.env.DEV && !column.field) {
        // eslint-disable-next-line no-console
        console.warn('[vxe-table] leaf column is missing `field`:', column);
      }
      headerRows[level].push({
        key,
        column,
        leaf,
        colSpan: 1,
        rowSpan: headerLevel - level,
        isGroup: false,
        fixed: column.fixed,
        stickyField: column.field,
      });
      leafColumns.push(leaf);
      return;
    }
    headerRows[level].push({
      key,
      column,
      colSpan: info.leafCount,
      rowSpan: 1,
      isGroup: true,
      fixed: info.fixed,
      stickyField: info.firstLeafField,
    });
    children.forEach((child, index) => walk(child, level + 1, `${path}.${child.field ?? index}`));
  };

  roots.forEach((column, index) => walk(column, 0, String(column.field ?? index)));
  return { leafColumns, headerRows, headerLevel };
}
