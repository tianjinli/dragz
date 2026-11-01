import {
  forwardRef,
  useEffect,
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

  useEffect(() => {
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
      const paginationHeight = getOuterHeight(MEASURE_PAGINATION, element);
      setScrollY(
        Math.max(0, Math.floor(totalHeight - headerHeight - paginationHeight)),
      );
      setSectionHeight(Math.max(0, totalHeight - paginationHeight));
    };

    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(container);
    if (container !== element) observer.observe(element);
    return () => observer.disconnect();
  }, [
    hasExplicitY,
    restProps.dataSource,
    restProps.loading,
    restProps.pagination,
    restProps.columns,
  ]);

  return (
    <Table<RecordType>
      {...restProps}
      ref={(instance) => {
        rootRef.current = instance;
        assignRef(ref, instance);
      }}
      classNames={withMeasureClassNames(classNames)}
      tableLayout={restProps.tableLayout ?? (scroll?.x === 'max-content' ? 'fixed' : undefined)}
      scroll={
        hasExplicitY
          ? boundedScroll
          : scrollY > 0
            ? { ...boundedScroll, y: scrollY }
            : boundedScroll
      }
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

