import {
  forwardRef,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type ReactElement,
  type Ref,
} from "react";
import { Table, type TableColumnGroupType, type TableColumnsType, type TableProps } from "antd";
import type { TableRef } from "antd/es/table";
import { MIN_LIST_COLUMN_WIDTH } from "@/utils/table-columns";

export type TablePanelProps<RecordType> = TableProps<RecordType>;

const MEASURE_HEADER = "list-table-measure-header";
const MEASURE_PAGINATION = "list-table-measure-pagination";
/** Ant Design skips the pager when total is 0. Middle size is 32px plus 16px margin on each side. */
const EMPTY_PAGINATION_BLOCK = 64;

function paginationBlockHeight(
  root: HTMLElement,
  pagination: TableProps<unknown>["pagination"],
): number {
  const measured = getOuterHeight(MEASURE_PAGINATION, root);
  if (measured > 0) return measured;
  if (!pagination) return 0;
  return EMPTY_PAGINATION_BLOCK;
}

function listTableWidth<RecordType>(columns: TableColumnsType<RecordType> | undefined): number | undefined {
  if (!columns?.length) return undefined;
  return columns.reduce((total, column) => {
    if ('children' in column && Array.isArray((column as TableColumnGroupType<RecordType>).children)) {
      return total + (listTableWidth((column as TableColumnGroupType<RecordType>).children) ?? 0);
    }
    return total + (typeof column.width === 'number' ? column.width : MIN_LIST_COLUMN_WIDTH);
  }, 0);
}

/**
 * `max-content` lets a newly saved long value participate in intrinsic table
 * sizing and can expand its column far beyond the configured width. Resolve it
 * to the declared/default column widths so content never controls table width.
 */
function withBoundedHorizontalScroll<RecordType>(
  scroll: TableProps<RecordType>["scroll"],
  columns: TableColumnsType<RecordType> | undefined,
): TableProps<RecordType>["scroll"] {
  if (scroll?.x !== 'max-content') return scroll;
  const x = listTableWidth(columns);
  return x == null ? scroll : { ...scroll, x };
}

type TableClassNames<RecordType extends object> = NonNullable<
  TableProps<RecordType>["classNames"]
>;
type TableStyles<RecordType extends object> = NonNullable<
  TableProps<RecordType>["styles"]
>;
type TableClassNamesObject<RecordType extends object> = Exclude<
  TableClassNames<RecordType>,
  (...args: never[]) => unknown
>;
type TableStylesObject<RecordType extends object> = Exclude<
  TableStyles<RecordType>,
  (...args: never[]) => unknown
>;

function assignRef<T>(ref: Ref<T> | undefined, value: T | null) {
  if (typeof ref === "function") ref(value);
  else if (ref) (ref as { current: T | null }).current = value;
}

function getOuterHeight(
  target: string | HTMLElement | null | undefined,
  root?: HTMLElement | null,
) {
  const ele =
    typeof target === "string"
      ? root?.querySelector<HTMLElement>(`.${target}`)
      : target;
  if (!ele) return 0;
  const styles = getComputedStyle(ele);
  const marginTop = Number.parseFloat(styles.marginTop) || 0;
  const marginBottom = Number.parseFloat(styles.marginBottom) || 0;
  return ele.getBoundingClientRect().height + marginTop + marginBottom;
}

function joinClass(...parts: Array<string | undefined>) {
  const value = parts.filter(Boolean).join(" ");
  return value || undefined;
}

/**
 * Ant Design skips the pager entirely when `total` is 0 (InternalTable gates
 * it on a truthy `mergedPagination.total`). Empty lists still need the page
 * controls, so show one placeholder page to Ant Design while keeping the real
 * total for `showTotal` and change callbacks.
 */
function withVisibleEmptyPagination<RecordType extends object>(
  pagination: TableProps<RecordType>["pagination"],
): TableProps<RecordType>["pagination"] {
  if (!pagination || pagination.total !== 0) return pagination;
  const { showTotal } = pagination;
  return {
    ...pagination,
    total: 1,
    current: 1,
    ...(showTotal
      ? {
          showTotal: (_total: number, range: [number, number]) =>
            showTotal(0, range),
        }
      : null),
  };
}

function withMeasureClassNames<RecordType extends object>(
  classNames: TableProps<RecordType>["classNames"],
): TableClassNames<RecordType> {
  const merge = (resolved?: TableClassNamesObject<RecordType>) => ({
    ...resolved,
    header: {
      ...resolved?.header,
      wrapper: joinClass(MEASURE_HEADER, resolved?.header?.wrapper),
    },
    pagination: {
      ...resolved?.pagination,
      root: joinClass(MEASURE_PAGINATION, resolved?.pagination?.root),
    },
  });

  if (typeof classNames === "function") {
    return ((info) => merge(classNames(info))) as TableClassNames<RecordType>;
  }
  return merge(classNames) as TableClassNames<RecordType>;
}

function withAutoHeightStyles<RecordType extends object>(
  styles: TableProps<RecordType>["styles"],
  sectionHeight: number,
  fillHeight: boolean,
): TableProps<RecordType>["styles"] {
  const merge = (resolved?: TableStylesObject<RecordType>) => ({
    ...resolved,
    ...(fillHeight && sectionHeight > 0
      ? {
          section: {
            ...resolved?.section,
            height: sectionHeight,
          } satisfies CSSProperties,
        }
      : null),
  });

  if (typeof styles === "function") {
    return ((info) => merge(styles(info))) as TableStyles<RecordType>;
  }
  return merge(styles) as TableStyles<RecordType>;
}

/**
 * Height of the row content that scroll.y must cover. When Ant Design has
 * already split the body, read the inner table; otherwise read the flat tbody.
 */
export function measureBodyContentHeight(
  root: HTMLElement | null | undefined,
): number {
  if (!root) return 0;
  const body = root.querySelector<HTMLElement>(".ant-table-body");
  if (body) {
    const table = body.querySelector("table");
    return table?.scrollHeight ?? body.scrollHeight;
  }
  const tbody = root.querySelector<HTMLElement>(".ant-table-tbody");
  return tbody ? tbody.getBoundingClientRect().height : 0;
}

/**
 * How many pixels the sticky header still needs on the inline-end side so its
 * column borders line up with the body. Zero when Ant Design already reserved
 * a matching `.ant-table-cell-scrollbar`.
 */
export function measureHeaderGutterPad(
  root: HTMLElement | null | undefined,
): number {
  if (!root) return 0;
  const body = root.querySelector<HTMLElement>(".ant-table-body");
  const header = root.querySelector<HTMLElement>(".ant-table-header");
  if (!body || !header) return 0;
  const scrollbar = Math.max(0, body.offsetWidth - body.clientWidth);
  if (scrollbar <= 0) return 0;
  const gutterCell = header.querySelector<HTMLElement>(
    ".ant-table-cell-scrollbar",
  );
  const gutter = gutterCell ? gutterCell.getBoundingClientRect().width : 0;
  return Math.max(0, Math.round(scrollbar - gutter));
}

function TablePanelInner<RecordType extends object>(
  props: TablePanelProps<RecordType>,
  ref: Ref<TableRef>,
): ReactElement {
  const { scroll, style, styles, classNames, ...restProps } = props;
  const rootRef = useRef<TableRef>(null);
  const [scrollY, setScrollY] = useState(0);
  const [sectionHeight, setSectionHeight] = useState(0);
  const hasExplicitY = scroll?.y != null;
  const boundedScroll = withBoundedHorizontalScroll(scroll, restProps.columns);
  const activeScrollY = hasExplicitY
    ? boundedScroll
    : scrollY > 0
      ? { ...boundedScroll, y: scrollY }
      : boundedScroll;
  const bodyScrollActive = hasExplicitY || scrollY > 0;
  const paginationProps = withVisibleEmptyPagination(restProps.pagination);

  useLayoutEffect(() => {
    if (hasExplicitY) {
      setScrollY(0);
      setSectionHeight(0);
      return;
    }
    const element = rootRef.current?.nativeElement;
    if (!element || typeof ResizeObserver === "undefined") return;

    const container =
      element.parentElement?.classList.contains("list-table-main-wrapper")
        ? element.parentElement
        : element;

    const measure = () => {
      // Prefer the list main wrapper's content box so the table fills that height.
      let totalHeight: number;
      if (container !== element && container) {
        const cs = getComputedStyle(container);
        const padY =
          (Number.parseFloat(cs.paddingTop) || 0) +
          (Number.parseFloat(cs.paddingBottom) || 0);
        totalHeight = Math.max(0, container.clientHeight - padY);
      } else {
        totalHeight = getOuterHeight(element);
      }
      const headerHeight = getOuterHeight(MEASURE_HEADER, element);
      const paginationHeight = paginationBlockHeight(element, restProps.pagination);
      const available = Math.max(
        0,
        Math.floor(totalHeight - headerHeight - paginationHeight),
      );
      setSectionHeight(Math.max(0, totalHeight - paginationHeight));

      // Only enable scroll.y when rows actually overflow. Ant Design's scroll.y
      // always paints a vertical scrollbar gutter; forcing it on short lists is
      // what misaligns header/body borders (and a CSS overflow:auto override
      // breaks fixed columns — see ant-design#56479).
      const contentHeight = measureBodyContentHeight(element);
      // A split body can report 0 before layout. Keep the previous scroll
      // height instead of collapsing the scroller for one frame.
      if (contentHeight <= 0) return;
      const nextY = contentHeight > available + 1 ? available : 0;
      // Measure before paint so the horizontal bar does not jump in after mount.
      // Ignore a 1px oscillation between the scrollbar and the content box.
      setScrollY((previous) =>
        Math.abs(previous - nextY) <= 1 ? previous : nextY,
      );
    };

    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(container);
    if (container !== element) observer.observe(element);
    const body = element.querySelector(".ant-table-body");
    const tbody = element.querySelector(".ant-table-tbody");
    if (body) observer.observe(body);
    if (tbody) observer.observe(tbody);
    return () => observer.disconnect();
  }, [
    hasExplicitY,
    restProps.dataSource,
    restProps.loading,
    restProps.pagination,
    restProps.columns,
    scrollY,
  ]);

  useLayoutEffect(() => {
    const element = rootRef.current?.nativeElement;
    const header = element?.querySelector<HTMLElement>(".ant-table-header");
    if (!element || !header || !bodyScrollActive) {
      if (header) header.style.paddingInlineEnd = "";
      return;
    }

    // Keep Ant Design's overflow-y: scroll. Only pad the sticky holder when its
    // gutter cell is shorter than the real body scrollbar (common when y is
    // enabled after mount and rc-table's one-shot measure stayed at 0).
    const syncGutter = () => {
      const next = measureHeaderGutterPad(element);
      const value = next > 0 ? `${next}px` : "";
      if (header.style.paddingInlineEnd !== value) {
        header.style.paddingInlineEnd = value;
      }
    };

    syncGutter();
    if (typeof ResizeObserver === "undefined") return;
    const body = element.querySelector(".ant-table-body");
    const observer = new ResizeObserver(syncGutter);
    observer.observe(element);
    if (body) observer.observe(body);
    observer.observe(header);
    return () => {
      observer.disconnect();
      header.style.paddingInlineEnd = "";
    };
  }, [
    bodyScrollActive,
    activeScrollY?.y,
    restProps.dataSource,
    restProps.loading,
    restProps.columns,
  ]);

  return (
    <Table<RecordType>
      {...restProps}
      pagination={paginationProps}
      ref={(instance) => {
        rootRef.current = instance;
        assignRef(ref, instance);
      }}
      classNames={withMeasureClassNames(classNames)}
      tableLayout={restProps.tableLayout ?? (scroll?.x === 'max-content' ? 'fixed' : undefined)}
      scroll={activeScrollY}
      style={{
        ...style,
        ...(hasExplicitY ? null : { height: "100%" }),
      }}
      styles={withAutoHeightStyles(styles, sectionHeight, !hasExplicitY)}
    />
  );
}
const ForwardedTablePanel = forwardRef(TablePanelInner) as <
  RecordType extends object = Record<string, unknown>,
>(
  props: TablePanelProps<RecordType> & { ref?: Ref<TableRef> },
) => ReactElement;

export const TablePanel = Object.assign(ForwardedTablePanel, {
  Column: Table.Column,
  ColumnGroup: Table.ColumnGroup,
  EXPAND_COLUMN: Table.EXPAND_COLUMN,
  SELECTION_ALL: Table.SELECTION_ALL,
  SELECTION_COLUMN: Table.SELECTION_COLUMN,
  SELECTION_INVERT: Table.SELECTION_INVERT,
  SELECTION_NONE: Table.SELECTION_NONE,
  Summary: Table.Summary,
}) as typeof Table;
