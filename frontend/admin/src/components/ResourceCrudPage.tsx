import { ListPagePanel } from "@/components/ListPagePanel";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  cloneElement,
  Fragment,
  isValidElement,
  type ReactElement,
  type ReactNode,
} from "react";
import { useTranslation } from "react-i18next";
import i18n from "@/i18n";
import {
  Button,
  Card,
  Form,
  Input,
  Segmented,
  Select,
  Space,
  Switch,
  Tag,
  Tooltip,
  TreeSelect,
  message,
  type FormRule,
  type TableColumnsType,
} from "antd";
import { Plus, RotateCcw, Search } from "lucide-react";
import {
  createResourceApi,
  deleteResourceApi,
  getResourceApi,
  listResourceApi,
  updateResourceApi,
  updateResourceStatusApi,
  type ResourceRecord,
} from "@/api/system";
import { hasAccessByCodes } from "@/stores/access";
import { usePreferencesStore } from "@/stores/preferences";
import { normalizeListColumns } from "@/utils/table-columns";
import { iconNode } from "@/router/menu-utils";
import { FormDrawer } from "./FormDrawer";
import { IconPicker } from "./IconPicker";
import { NumberStepper } from "./NumberStepper";
import { Page } from "./Page";
import { StorageImageUpload } from "./StorageImageUpload";
import { TablePanel } from "./TablePanel";
import {
  actionColumnWidth,
  TableActionGroup,
  type TableAction,
} from "./TableActionGroup";

const EMPTY_FILTERS: Record<string, unknown> = {};

export const STATUS_OPTIONS = (): SelectOption[] => [
  { label: i18n.t("common.enabled"), value: 1 },
  { label: i18n.t("crud.disabled"), value: 0 },
];

export interface SelectOption {
  className?: string;
  disabled?: boolean;
  icon?: string;
  label: string;
  parentId?: number;
  value: boolean | number | string;
}
interface FieldOptionLoaderContext {
  editing: ResourceRecord | null;
  keyword?: string;
  values: Record<string, unknown>;
}
interface FieldRulesContext {
  editing: ResourceRecord | null;
  getFieldValue: (name: string) => unknown;
}
interface FieldChangeContext {
  getFieldValue: (name: string) => unknown;
  setFieldValue: (name: string, value: unknown) => void;
}
/**
 * Context for a control rendered inside a text field's trailing slot. The
 * handlers read and write the form directly, so a button can act on the field it
 * sits in and on the field it fills.
 */
interface FieldSuffixContext extends FieldChangeContext {
  editing: ResourceRecord | null;
}
export type FieldType =
  | "icon"
  | "image"
  | "input"
  | "number"
  | "password"
  | "select"
  | "switch"
  | "textarea"
  | "tree-select"
  | "segmented";
export interface FieldConfig {
  allowClear?: boolean;
  dependencies?: string[];
  /**
   * Read-only while editing. `true` locks the field for every record; a
   * function decides per record, for rows the backend owns only in some cases
   * (e.g. a route-synced permission button vs. one an admin created).
   * Locked fields are still sent: the update endpoint owns a full record, and
   * the server keeps its own copy of anything it owns.
   */
  disabledOnEdit?: boolean | ((editing: ResourceRecord | null) => boolean);
  hiddenOnCreate?: boolean;
  /** Hide this field while editing (e.g. create-only password). */
  hiddenOnEdit?: boolean;
  key: string;
  label: string;
  mode?: "multiple" | "tags";
  numeric?: boolean;
  /**
   * Called after this field's value changes, with helpers for writing sibling
   * fields — e.g. fill a source-text field from the option that was picked.
   */
  onChange?: (value: unknown, helpers: FieldChangeContext) => void;
  optionDependencies?: string[];
  optionLoader?: (context: FieldOptionLoaderContext) => Promise<SelectOption[]>;
  options?: SelectOption[];
  placeholder?: string;
  remoteSearch?: boolean;
  required?: boolean;
  rules?: FormRule[] | ((context: FieldRulesContext) => FormRule[]);
  /**
   * Renders a control inside the text field's trailing slot — e.g. the translate
   * button the translation page puts in both language columns. It sits in the
   * input rather than in a row of its own so the action reads as belonging to
   * that field.
   */
  suffix?: (context: FieldSuffixContext) => ReactNode;
  treeTitleRender?: (node: SelectTreeNode) => ReactNode;
  type?: FieldType;
  /**
   * Render as this control while creating, and as `type` while editing — e.g. a
   * business key that is picked from a list on create and locked afterwards.
   */
  typeOnCreate?: FieldType;
  /** Hide the field based on current form values (re-evaluated via dependencies). */
  visibleWhen?: (getFieldValue: (name: string) => unknown) => boolean;
}
export interface ColumnConfig {
  align?: "center" | "left" | "right";
  dataIndex: string;
  title: string;
  width?: number;
  render?: (value: unknown, record: ResourceRecord) => ReactNode;
  /** dataIndex of the icon field on the record. Renders an icon node before the cell text. */
  iconDataIndex?: string;
  /** Maps raw cell values to display labels (e.g. { directory: "目录", menu: "菜单" }). */
  labelMap?: Record<string, string>;
  /**
   * Wrap the cell text instead of truncating it with an ellipsis. For columns
   * whose value is a whole sentence the reader has to see at once (translations,
   * remarks): a tip on hover is no substitute for the text itself.
   */
  wrap?: boolean;
}
export interface SearchFilterConfig {
  key: string;
  label: string;
  options: SelectOption[];
  placeholder?: string;
  width?: number;
}
export interface RowAction {
  danger?: boolean;
  /** 返回 true 时该行操作按钮置灰但仍显示。 */
  disabled?: (row: ResourceRecord) => boolean;
  hidden?: (row: ResourceRecord) => boolean;
  label: string;
  onClick?: (row: ResourceRecord) => Promise<void> | void;
  /** When false, skip list reload after onClick (e.g. open a secondary drawer). */
  reload?: boolean;
  /** 打开新增弹窗并预填由该行派生出的默认值（如“新增下级”）。 */
  createValues?: (row: ResourceRecord) => Record<string, unknown>;
}
interface Props {
  columns: ColumnConfig[];
  cascadeStatus?: boolean;
  createCode: string;
  /** Merge side-panel / filter values into create-form defaults. */
  createDefaultsFromFilters?: (
    filters: Record<string, unknown>,
  ) => Promise<Record<string, unknown>> | Record<string, unknown>;
  defaultValues?: Record<string, unknown>;
  deleteCode: string;
  /** One-line explanation under the page heading (page scope, not a field hint). */
  description?: ReactNode;
  endpoint: string;
  extraSearchFilters?: SearchFilterConfig[];
  fields: FieldConfig[];
  /** Hide the "add" button even when the account may create (e.g. a list that
   *  already contains every possible row, so there is nothing left to add). */
  hideCreate?: boolean;
  /** Hide the delete action even when the account may delete. */
  hideDelete?: boolean;
  /** When true, edit loads GET `${endpoint}/:id` and fills the form from detail. */
  loadDetailOnEdit?: boolean;
  /** Map API record values into form fields (e.g. bytes → MB). */
  mapRecordValues?: (
    values: Record<string, unknown>,
    record: ResourceRecord,
  ) => Record<string, unknown>;
  /** Extra guard for the delete action (e.g. rows owned by a startup sync). */
  deleteDisabled?: (record: ResourceRecord) => boolean;
  /** Map form values before create/update submit (e.g. MB → bytes). */
  mapSubmitValues?: (
    values: Record<string, unknown>,
    editing: ResourceRecord | null,
  ) => Record<string, unknown>;
  /**
   * Replace the built-in `DELETE ${endpoint}/:id` for rows addressed by a
   * business key rather than the id the list displays (e.g. a list whose rows
   * are preset entities and whose id is a display key).
   */
  onDelete?: (record: ResourceRecord) => Promise<void>;
  paged?: boolean;
  parentKey?: string;
  protectedField?: string;
  rowActions?: RowAction[];
  searchMode?: "auto" | "local" | "none" | "submit";
  searchPlaceholder?: string;
  sidePanel?: (context: {
    filters: Record<string, unknown>;
    items: ResourceRecord[];
    searchedItems: ResourceRecord[];
    setFilter: (key: string, value: unknown) => void;
  }) => ReactNode;
  statusFilterAfterExtra?: boolean;
  /** Extra guard for the status Switch (e.g. block self-disable). */
  statusSwitchDisabled?: (record: ResourceRecord) => boolean;
  title: string;
  tree?: boolean;
  updateCode: string;
}

function mapStatusInPlace(
  items: ResourceRecord[],
  id: number,
  status: number,
  cascade: boolean,
): ResourceRecord[] {
  return items.map((item) => {
    if (item.id === id) {
      const next: ResourceRecord = { ...item, status };
      if (cascade && status === 0 && Array.isArray(item.children)) {
        next.children = disableStatusTree(item.children as ResourceRecord[]);
      }
      return next;
    }
    return Array.isArray(item.children)
      ? {
          ...item,
          children: mapStatusInPlace(
            item.children as ResourceRecord[],
            id,
            status,
            cascade,
          ),
        }
      : item;
  });
}

function mapRecordInPlace(
  items: ResourceRecord[],
  id: number,
  values: Record<string, unknown>,
): ResourceRecord[] {
  return items.map((item) => {
    if (item.id === id) return { ...item, ...values };
    return Array.isArray(item.children)
      ? {
          ...item,
          children: mapRecordInPlace(
            item.children as ResourceRecord[],
            id,
            values,
          ),
        }
      : item;
  });
}

function disableStatusTree(items: ResourceRecord[]): ResourceRecord[] {
  return items.map((item) => ({
    ...item,
    status: 0,
    ...(Array.isArray(item.children)
      ? { children: disableStatusTree(item.children as ResourceRecord[]) }
      : {}),
  }));
}

function buildTree(
  items: ResourceRecord[],
  parentKey: string,
  depth = 0,
): ResourceRecord[] {
  const map = new Map<
    number,
    ResourceRecord & { children?: ResourceRecord[] }
  >();
  const roots: Array<ResourceRecord & { children?: ResourceRecord[] }> = [];
  items.forEach((item) => map.set(item.id, { ...item, __treeDepth: depth }));
  items.forEach((item) => {
    const node = map.get(item.id);
    if (!node) return;
    const parent = map.get(Number(item[parentKey] || 0));
    if (parent) (parent.children ||= []).push(node);
    else roots.push(node);
  });
  const assignDepth = (
    nodes: Array<ResourceRecord & { children?: ResourceRecord[] }>,
    level: number,
  ) =>
    nodes.forEach((node) => {
      node.__treeDepth = level;
      if (node.children)
        assignDepth(
          node.children as Array<
            ResourceRecord & { children?: ResourceRecord[] }
          >,
          level + 1,
        );
    });
  assignDepth(roots, depth);
  return roots;
}

function recordMatchesKeyword(item: ResourceRecord, keyword: string): boolean {
  const value = keyword.trim().toLowerCase();
  if (!value) return true;
  return ["name", "code"].some((key) =>
    String(item[key] ?? "")
      .toLowerCase()
      .includes(value),
  );
}

function recordMatchesFilters(
  item: ResourceRecord,
  keyword: string,
  statusFilter: number | undefined,
  filters: Record<string, unknown>,
): boolean {
  if (!recordMatchesKeyword(item, keyword)) return false;
  if (
    (statusFilter === 0 || statusFilter === 1) &&
    Number(item.status) !== statusFilter
  )
    return false;
  return Object.entries(filters).every(
    ([key, value]) =>
      value === undefined ||
      value === null ||
      value === "" ||
      String(item[key] ?? "") === String(value),
  );
}

function localFilterItems(
  items: ResourceRecord[],
  keyword: string,
  statusFilter: number | undefined,
  filters: Record<string, unknown>,
  parentKey: string,
): ResourceRecord[] {
  const hasFilters =
    keyword.trim() ||
    statusFilter === 0 ||
    statusFilter === 1 ||
    Object.values(filters).some(
      (value) => value !== undefined && value !== null && value !== "",
    );
  if (!hasFilters) return items;
  const byID = new Map(items.map((item) => [Number(item.id), item]));
  const included = new Set<number>();
  items.forEach((item) => {
    if (!recordMatchesFilters(item, keyword, statusFilter, filters)) return;
    let current: ResourceRecord | undefined = item;
    while (current) {
      const id = Number(current.id);
      if (!Number.isFinite(id) || included.has(id)) break;
      included.add(id);
      current = byID.get(Number(current[parentKey] || 0));
    }
  });
  return items.filter((item) => included.has(Number(item.id)));
}

export interface SelectTreeNode {
  children?: SelectTreeNode[];
  disabled?: boolean;
  icon?: string;
  key: string | number;
  label: string;
  title?: ReactNode;
  value: string | number;
}
function buildSelectTree(
  items: SelectOption[],
  parentId = 0,
  renderTitle?: (node: SelectTreeNode) => ReactNode,
): SelectTreeNode[] {
  return items
    .filter((item) => Number(item.parentId || 0) === parentId)
    .map((item) => {
      const children = buildSelectTree(items, Number(item.value), renderTitle);
      const value = item.value as string | number;
      const node: SelectTreeNode = {
        key: value,
        label: item.label,
        value,
        ...(item.icon ? { icon: item.icon } : {}),
        ...(item.disabled ? { disabled: true } : {}),
        ...(children.length ? { children } : {}),
      };
      node.title = renderTitle ? (
        renderTitle(node)
      ) : (
        <span className="department-tree-title">
          <span>{node.label}</span>
        </span>
      );
      return node;
    });
}

export function ResourceCrudPage({
  cascadeStatus = false,
  columns,
  createCode,
  createDefaultsFromFilters,
  defaultValues = { sort: 0, status: 1 },
  deleteCode,
  deleteDisabled,
  description,
  endpoint,
  extraSearchFilters = [],
  fields,
  hideCreate = false,
  hideDelete = false,
  loadDetailOnEdit = false,
  mapRecordValues,
  mapSubmitValues,
  onDelete,
  paged = false,
  parentKey = "parentId",
  protectedField = "isSystem",
  rowActions = [],
  searchMode = "auto",
  searchPlaceholder,
  sidePanel,
  statusFilterAfterExtra = false,
  statusSwitchDisabled,
  title,
  tree = false,
  updateCode,
}: Props) {
  const { t } = useTranslation();
  const resolvedSearchPlaceholder =
    searchPlaceholder ?? t("crud.searchPlaceholder");
  const defaults = useMemo(() => defaultValues, []);
  const [form] = Form.useForm<Record<string, unknown>>();
  const [items, setItems] = useState<ResourceRecord[]>([]);
  const [sourceItems, setSourceItems] = useState<ResourceRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [keyword, setKeyword] = useState("");
  const [searchKeyword, setSearchKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState<number>();
  const [searchStatusFilter, setSearchStatusFilter] = useState<number>();
  const [filterValues, setFilterValues] = useState<Record<string, unknown>>({});
  const [searchFilterValues, setSearchFilterValues] = useState<
    Record<string, unknown>
  >({});
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(
    usePreferencesStore.getState().preferences.custom.defaultTableSize,
  );
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState<ResourceRecord | null>(null);
  const [open, setOpen] = useState(false);
  const [statusUpdatingId, setStatusUpdatingId] = useState<number | null>(null);
  const [createOverrides, setCreateOverrides] = useState<
    Record<string, unknown>
  >({});
  const [options, setOptions] = useState<Record<string, SelectOption[]>>({});
  const [optionSearch, setOptionSearch] = useState<Record<string, string>>({});
  const watchedValues = Form.useWatch([], form) as
    | Record<string, unknown>
    | undefined;
  const optionDependencyKey = JSON.stringify(
    fields.map((field) => [
      field.key,
      ...(field.optionDependencies || []).map(
        (dependency) => watchedValues?.[dependency],
      ),
    ]),
  );
  const optionSearchKey = JSON.stringify(
    fields.map((field) => [
      field.key,
      field.remoteSearch ? optionSearch[field.key] || "" : "",
    ]),
  );
  const setFilter = useCallback((key: string, value: unknown) => {
    setFilterValues((previous) => ({ ...previous, [key]: value }));
  }, []);
  const searchedItems = useMemo(
    () =>
      searchMode === "local"
        ? sourceItems.filter((item) =>
            recordMatchesFilters(
              item,
              keyword,
              statusFilter,
              Object.fromEntries(
                Object.entries(filterValues).filter(
                  ([key]) => key !== "groupName",
                ),
              ),
            ),
          )
        : items,
    [filterValues, items, keyword, searchMode, sourceItems, statusFilter],
  );
  const activeFilters = filterValues;
  const remoteKeyword =
    searchMode === "local" || searchMode === "none" ? "" : keyword;
  const remoteStatus =
    searchMode === "local" || searchMode === "none" ? undefined : statusFilter;
  const remoteFilters = useMemo(
    () =>
      searchMode === "local" || searchMode === "none"
        ? EMPTY_FILTERS
        : activeFilters,
    [activeFilters, searchMode],
  );
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listResourceApi<
        ResourceRecord[] | { items: ResourceRecord[]; total: number }
      >(endpoint, {
        keyword: remoteKeyword,
        page,
        pageSize,
        status: remoteStatus,
        ...remoteFilters,
      });
      const next = Array.isArray(result) ? result : result.items;
      setSourceItems(next);
      setItems(tree ? buildTree(next, parentKey) : next);
      setTotal(
        searchMode === "local"
          ? next.length
          : Array.isArray(result)
            ? result.length
            : result.total,
      );
    } finally {
      setLoading(false);
    }
  }, [
    endpoint,
    page,
    pageSize,
    parentKey,
    remoteFilters,
    remoteKeyword,
    remoteStatus,
    searchMode,
    tree,
  ]);
  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    if (searchMode !== "local") return;
    const filtered = localFilterItems(
      sourceItems,
      keyword,
      statusFilter,
      filterValues,
      parentKey,
    );
    setItems(tree ? buildTree(filtered, parentKey) : filtered);
    setTotal(filtered.length);
  }, [
    filterValues,
    keyword,
    parentKey,
    searchMode,
    sourceItems,
    statusFilter,
    tree,
  ]);
  useEffect(() => {
    if (!open) return;
    const values = form.getFieldsValue();
    void Promise.all(
      fields.map(async (field) => {
        if (field.hiddenOnCreate && !editing) return [field.key, []] as const;
        return [
          field.key,
          field.options ||
            (await field.optionLoader?.({
              editing,
              keyword: optionSearch[field.key],
              values,
            })) ||
            [],
        ] as const;
      }),
    ).then((entries) => {
      setOptions(Object.fromEntries(entries));
      // A required single select renders as a Segmented, which highlights its
      // first option without the form ever learning about it. Adopt what the
      // operator sees, otherwise the drawer shows 目录 and saving still
      // reports the field as unset. The same rule has to hold when the offered
      // options change underneath a value that was already adopted: picking a
      // page as the parent leaves 按钮 as the only type, and a stale 目录 must
      // not survive behind an unselected control and be submitted.
      entries.forEach(([key, available]) => {
        const field = fields.find((item) => item.key === key);
        if (
          !field?.required ||
          field.type !== "select" ||
          field.remoteSearch ||
          field.mode ||
          key === parentKey ||
          !available.length
        )
          return;
        const current = form.getFieldValue(key);
        const stillOffered = available.some((item) => item.value === current);
        if (current == null || !stillOffered)
          form.setFieldValue(key, available[0].value);
      });
    });
  }, [
    editing,
    fields,
    form,
    open,
    optionDependencyKey,
    optionSearch,
    optionSearchKey,
    parentKey,
  ]);
  const normalizeFormValues = useCallback(
    (record: ResourceRecord) => {
      const values: Record<string, unknown> = { ...defaults, ...record };
      fields.forEach((field) => {
        if (field.type === "switch" && field.numeric)
          values[field.key] = Number(values[field.key] ?? 0) === 1;
        if (field.type === "password") {
          const secret = values[field.key];
          if (secret === "********" || secret == null) values[field.key] = "";
        }
        if (
          (field.type === "select" || field.type === "tree-select") &&
          field.mode === "multiple"
        ) {
          const raw = values[field.key];
          values[field.key] = Array.isArray(raw)
            ? raw.map((item) => Number(item))
            : [];
        }
        if (field.key === parentKey) {
          const parent = Number(values[field.key]);
          if (!Number.isFinite(parent) || parent <= 0)
            values[field.key] = undefined;
        }
      });
      return mapRecordValues ? mapRecordValues(values, record) : values;
    },
    [defaults, fields, mapRecordValues, parentKey],
  );
  const editValues = useMemo(() => {
    if (!editing) return undefined;
    return normalizeFormValues(editing);
  }, [editing, normalizeFormValues]);
  const formValuesOf = useCallback(
    (record: ResourceRecord) => normalizeFormValues(record),
    [normalizeFormValues],
  );
  const openCreateWith = (overrides: Record<string, unknown>) => {
    const values = { ...defaults, ...overrides };
    setCreateOverrides(overrides);
    setEditing(null);
    setOptionSearch({});
    form.resetFields();
    form.setFieldsValue(values);
    setOpen(true);
  };
  const openCreate = () => {
    void Promise.resolve(createDefaultsFromFilters?.(filterValues) || {}).then(
      (overrides) => openCreateWith(overrides),
    );
  };
  const openEdit = async (record: ResourceRecord) => {
    setEditing(record);
    setOptionSearch({});
    form.resetFields();
    form.setFieldsValue(formValuesOf(record));
    setOpen(true);
    if (!loadDetailOnEdit) return;
    try {
      const detail = await getResourceApi<ResourceRecord>(
        `${endpoint}/${record.id}`,
      );
      setEditing(detail);
      form.setFieldsValue(formValuesOf(detail));
    } catch {
      void message.error(t("crud.detailLoadFailed"));
    }
  };
  const save = async () => {
    const values = await form.validateFields();
    fields.forEach((field) => {
      if (field.type === "switch" && field.numeric)
        values[field.key] = values[field.key] ? 1 : 0;
    });
    const payload = mapSubmitValues ? mapSubmitValues(values, editing) : values;
    setSaving(true);
    try {
      let savedValues: Record<string, unknown> | null = null;
      if (editing) {
        await updateResourceApi(endpoint, editing.id, payload);
        savedValues = payload;
        setEditing((current) =>
          current && current.id === editing.id ? { ...current, ...payload } : current,
        );
        setItems((previous) => mapRecordInPlace(previous, editing.id, payload));
      } else await createResourceApi(endpoint, payload);
      void message.success(t("common.saveSuccess"));
      setOpen(false);
      await load();
      if (editing && savedValues)
        setItems((previous) =>
          mapRecordInPlace(previous, editing.id, savedValues),
        );
    } finally {
      setSaving(false);
    }
  };
  const isFieldLocked = (field: FieldConfig) =>
    typeof field.disabledOnEdit === "function"
      ? field.disabledOnEdit(editing)
      : !!editing && !!field.disabledOnEdit;
  const remove = useCallback(
    async (record: ResourceRecord) => {
      try {
        if (onDelete) await onDelete(record);
        else await deleteResourceApi(endpoint, record.id);
      } catch {
        // The request layer already reported the failure; keep the row.
        return;
      }
      void message.success(t("common.deleteSuccess"));
      await load();
    },
    [endpoint, load, onDelete, t],
  );
  const statusSwitchEnabled = fields.some(
    (field) =>
      field.key === "status" && field.type === "switch" && field.numeric,
  );
  const changeStatus = useCallback(
    async (record: ResourceRecord, checked: boolean) => {
      setStatusUpdatingId(record.id);
      try {
        await updateResourceStatusApi(endpoint, record.id, checked ? 1 : 0);
        void message.success(
          t(checked ? "crud.enabledStatus" : "crud.disabledStatus"),
        );
        setItems((previous) =>
          mapStatusInPlace(previous, record.id, checked ? 1 : 0, cascadeStatus),
        );
      } catch {
        void message.error(t("crud.statusUpdateFailed"));
      } finally {
        setStatusUpdatingId(null);
      }
    },
    [cascadeStatus, endpoint, t],
  );
  const tableColumns = useMemo<TableColumnsType<ResourceRecord>>(
    () =>
      normalizeListColumns<ResourceRecord>([
        ...columns.map((column) => {
          // Wrap wins over the ellipsis path, including columns with their own
          // render output: the point is that nothing in the cell stays hidden.
          if (column.wrap) {
            const { wrap: _wrap, render, ...rest } = column;
            return {
              ...rest,
              // Turning antd's ellipsis off drops the nowrap class, so the cell
              // grows to a second line instead of cutting the text with “…”.
              ellipsis: false,
              render: (value: unknown, record: ResourceRecord) => (
                <span className="table-panel-wrap-cell">
                  {render ? render(value, record) : String(value ?? "")}
                </span>
              ),
            };
          }
          if (column.render) return { ...column };
          if (column.labelMap) {
            const { labelMap, ...rest } = column;
            return {
              ...rest,
              render: (value: unknown) => (
                <span>{labelMap[String(value)] ?? String(value ?? "—")}</span>
              ),
            };
          }
          if (column.dataIndex === "status") {
            return {
              ...column,
              render: (value: unknown, record: ResourceRecord) => {
                const isChecked = Number(value ?? 0) === 1;
                return statusSwitchEnabled ? (
                  <Switch
                    checked={isChecked}
                    disabled={
                      statusUpdatingId === record.id ||
                      !!record[protectedField] ||
                      !!statusSwitchDisabled?.(record)
                    }
                    loading={statusUpdatingId === record.id}
                    onChange={(checked) => void changeStatus(record, checked)}
                  />
                ) : (
                  <Tag color={isChecked ? "success" : "default"}>
                    {isChecked ? t("common.enabled") : t("common.disabled")}
                  </Tag>
                );
              },
            };
          }
          // Official Column ellipsis for layout; Ant Design Tooltip for the tip UI.
          // Trigger must be width-bound or getBoundingClientRect spills into the next column.
          if (column.iconDataIndex) {
            const { iconDataIndex, ...rest } = column;
            return {
              ...rest,
              ellipsis: { showTitle: false },
              render: (_: unknown, record: ResourceRecord) => {
                const name = String(record[rest.dataIndex] ?? "");
                const icon = String(record[iconDataIndex] || "");
                // Flex BFC (no width:100%) stays on one line with floated expand
                // icons and shrinks so .table-panel-ellipsis-tip can show “…”.
                return (
                  <Tooltip placement="top" title={name}>
                    <span className="menu-name-cell">
                      {icon ? iconNode(icon) : null}
                      <span className="table-panel-ellipsis-tip">{name}</span>
                    </span>
                  </Tooltip>
                );
              },
            };
          }
          return {
            ...column,
            ellipsis: { showTitle: false },
            render: (value: unknown) => {
              const text = String(value ?? "");
              return (
                <Tooltip placement="top" title={text}>
                  <span className="table-panel-ellipsis-tip">{text}</span>
                </Tooltip>
              );
            },
          };
        }),
        {
          title: t("common.actions"),
          key: "actions",
          width: actionColumnWidth([
            ...rowActions.map((action) => ({ label: action.label })),
            ...(hasAccessByCodes([updateCode])
              ? [{ label: t("crud.modify") }]
              : []),
            ...(!hideDelete && hasAccessByCodes([deleteCode])
              ? [{ label: t("common.delete") }]
              : []),
          ]),
          render: (_, record) => {
            const actions: TableAction[] = [
              ...rowActions
                .filter((action) => !action.hidden?.(record))
                .map((action) => ({
                  ...action,
                  disabled: action.disabled?.(record),
                  onClick: () => {
                    if (action.createValues) {
                      openCreateWith(action.createValues(record));
                      return;
                    }
                    void Promise.resolve(action.onClick?.(record)).then(() => {
                      if (action.reload === false) return;
                      return load();
                    });
                  },
                })),
              ...(hasAccessByCodes([updateCode])
                ? [
                    {
                      label: t("crud.modify"),
                      disabled: !!record[protectedField],
                      onClick: () => void openEdit(record),
                    },
                  ]
                : []),
              ...(!hideDelete && hasAccessByCodes([deleteCode])
                ? [
                    {
                      danger: true,
                      label: t("common.delete"),
                      disabled: !!record[protectedField] || !!deleteDisabled?.(record),
                      onClick: () => remove(record),
                    },
                  ]
                : []),
            ];
            return <TableActionGroup actions={actions} />;
          },
        },
      ]),
    [
      changeStatus,
      columns,
      deleteCode,
      deleteDisabled,
      hideDelete,
      load,
      protectedField,
      remove,
      rowActions,
      statusSwitchDisabled,
      statusSwitchEnabled,
      statusUpdatingId,
      t,
      updateCode,
    ],
  );
  const statusFilterNode = (
    <Select
      allowClear
      options={STATUS_OPTIONS()}
      placeholder={t("common.status")}
      style={{ width: 120 }}
      value={searchMode === "submit" ? searchStatusFilter : statusFilter}
      onChange={(value) => {
        if (searchMode === "submit") setSearchStatusFilter(value);
        else setStatusFilter(value);
      }}
    />
  );
  const extraFilterNodes = extraSearchFilters.map((filter) => (
    <Select
      allowClear
      key={filter.key}
      options={filter.options}
      placeholder={filter.placeholder || filter.label}
      style={{ width: filter.width || 140 }}
      value={
        (searchMode === "submit" ? searchFilterValues : filterValues)[
          filter.key
        ]
      }
      onChange={(value) => {
        const setter =
          searchMode === "submit" ? setSearchFilterValues : setFilterValues;
        setter((previous) => ({ ...previous, [filter.key]: value }));
      }}
    />
  ));
  const search =
    searchMode === "none" ? undefined : (
      <Space wrap>
        <Input
          allowClear
          placeholder={resolvedSearchPlaceholder}
          prefix={<Search size={16} />}
          value={searchMode === "submit" ? searchKeyword : keyword}
          onChange={(event) => {
            if (searchMode === "submit") setSearchKeyword(event.target.value);
            else setKeyword(event.target.value);
          }}
          onPressEnter={() => {
            if (searchMode === "submit") {
              setKeyword(searchKeyword);
              setStatusFilter(searchStatusFilter);
              setFilterValues((previous) => {
                const next = { ...previous };
                extraSearchFilters.forEach((filter) => {
                  const value = searchFilterValues[filter.key];
                  if (value === undefined || value === null || value === "")
                    delete next[filter.key];
                  else next[filter.key] = value;
                });
                return next;
              });
              setPage(1);
            } else void load();
          }}
        />
        {statusFilterAfterExtra ? extraFilterNodes : statusFilterNode}
        {statusFilterAfterExtra ? statusFilterNode : extraFilterNodes}
        {searchMode === "local" ? null : (
          <Button
            type="primary"
            icon={<Search size={16} />}
            onClick={() => {
              if (searchMode === "submit") {
                setKeyword(searchKeyword);
                setStatusFilter(searchStatusFilter);
                setFilterValues((previous) => {
                  const next = { ...previous };
                  extraSearchFilters.forEach((filter) => {
                    const value = searchFilterValues[filter.key];
                    if (value === undefined || value === null || value === "")
                      delete next[filter.key];
                    else next[filter.key] = value;
                  });
                  return next;
                });
                setPage(1);
              } else void load();
            }}
          >
            {t("crud.query")}
          </Button>
        )}
        {searchMode === "local" ? null : (
          <Button
            icon={<RotateCcw size={16} />}
            onClick={() => {
              setKeyword("");
              setSearchKeyword("");
              setStatusFilter(undefined);
              setSearchStatusFilter(undefined);
              setFilterValues({});
              setSearchFilterValues({});
              setPage(1);
            }}
          >
            {t("common.reset")}
          </Button>
        )}
      </Space>
    );
  const listToolbar = {
    search,
    actions: !hideCreate && hasAccessByCodes([createCode]) ? (
      <Button type="primary" icon={<Plus size={16} />} onClick={openCreate}>
        {t("common.add")}
      </Button>
    ) : undefined,
    onRefresh: () => void load(),
  };
  const listPagination = paged
    ? {
        current: page,
        pageSize,
        total,
        showSizeChanger: true,
        onChange: (nextPage: number, nextSize: number) => {
          setPage(nextPage);
          setPageSize(nextSize);
        },
      }
    : false;
  // Tree data uses nested `children`; Ant Design Table handles expand/collapse natively (no custom expandIcon).
  const tableCard = (
    <Card
      className={`table-card resource-grid-card${tree ? " department-resource-table" : ""}`}
    >
    <ListPagePanel
      title={t("common.listTitle", { title })}
      toolbar={listToolbar}
    >
      <TablePanel<ResourceRecord>
        className="list-page-table"
        bordered
        sticky
        scroll={{ x: "max-content" }}
        columns={tableColumns}
        dataSource={items}
        loading={loading}
        rowKey="id"
        pagination={listPagination}
      />
    </ListPagePanel>
    </Card>
  );
  return (
    <Page description={description} title={t("crud.pageTitle", { title })}>
      {sidePanel ? (
        <div className="user-list-layout">
          <Card className="department-tree-card side-panel-card">
            {sidePanel({
              filters: filterValues,
              items: sourceItems,
              searchedItems,
              setFilter,
            })}
          </Card>
          <div className="user-list-main">{tableCard}</div>
        </div>
      ) : (
        tableCard
      )}
      <FormDrawer
        confirmLoading={saving}
        onClose={() => setOpen(false)}
        onOk={save}
        open={open}
        title={t(editing ? "crud.editTitle" : "crud.createTitle", { title })}
      >
        <Form
          key={
            open
              ? editing
                ? `edit-${editing.id}`
                : `create-${JSON.stringify(createOverrides)}`
              : "closed"
          }
          form={form}
          labelCol={{ span: 6 }}
          preserve={false}
          initialValues={
            editing ? editValues : { ...defaults, ...createOverrides }
          }
        >
          {fields
            .filter(
              (field) =>
                !(field.hiddenOnCreate && !editing) &&
                !(field.hiddenOnEdit && editing),
            )
            .map((field) => {
              // A field may use a different control while creating: a key
              // picked from a list on create is locked to its value on edit.
              const type = editing ? field.type : (field.typeOnCreate ?? field.type);
              const control =
                type === "number" ? (
                  <NumberStepper
                    disabled={isFieldLocked(field)}
                    label={field.label}
                  />
                ) : type === "icon" ? (
                  <IconPicker disabled={isFieldLocked(field)} />
                ) : type === "image" ? (
                  <StorageImageUpload />
                ) : type === "switch" ? (
                  <Switch disabled={isFieldLocked(field)} />
                ) : type == "segmented" ? (
                  <Segmented
                    disabled={isFieldLocked(field)}
                    options={
                      field.options?.map((item) => ({
                        label: item.label,
                        value: item.value,
                      })) || []
                    }
                  />
                ) : type === "tree-select" ? (
                  <TreeSelect
                    allowClear={field.allowClear !== false}
                    className="full-width"
                    classNames={{
                      popup: { root: "department-tree-select-dropdown" },
                    }}
                    disabled={isFieldLocked(field)}
                    multiple={field.mode === "multiple"}
                    treeCheckable={field.mode === "multiple"}
                    showCheckedStrategy={
                      field.mode === "multiple"
                        ? TreeSelect.SHOW_CHILD
                        : undefined
                    }
                    showSearch
                    treeData={buildSelectTree(
                      (options[field.key] || []).filter(
                        (item) =>
                          field.key !== parentKey || item.value !== editing?.id,
                      ),
                      0,
                      (node) =>
                        field.treeTitleRender ? (
                          field.treeTitleRender(node)
                        ) : (
                          <span className="department-tree-title">
                            <span>{node.label}</span>
                          </span>
                        ),
                    )}
                    treeDefaultExpandAll
                    treeNodeFilterProp="label"
                    treeNodeLabelProp="label"
                    placeholder={field.placeholder}
                  />
                ) : type === "select" ? (
                  (() => {
                    const fieldOptions = (options[field.key] || []).filter(
                      (item) =>
                        field.key !== parentKey || item.value !== editing?.id,
                    );
                    if (
                      field.required &&
                      !field.remoteSearch &&
                      !field.mode &&
                      fieldOptions.length > 0
                    )
                      return (
                        <Segmented
                          disabled={isFieldLocked(field)}
                          options={fieldOptions.map((item) => ({
                            label: item.label,
                            value: item.value,
                          }))}
                        />
                      );
                    return (
                      <Select
                        allowClear={field.allowClear !== false}
                        disabled={isFieldLocked(field)}
                        filterOption={field.remoteSearch ? false : undefined}
                        mode={field.mode}
                        onSearch={
                          field.remoteSearch
                            ? (value) =>
                                setOptionSearch((previous) => ({
                                  ...previous,
                                  [field.key]: value,
                                }))
                            : undefined
                        }
                        options={fieldOptions}
                        optionFilterProp="label"
                        placeholder={field.placeholder}
                        showSearch
                      />
                    );
                  })()
                ) : type === "textarea" ? (
                  <Input.TextArea
                    rows={4}
                    disabled={isFieldLocked(field)}
                    placeholder={field.placeholder}
                  />
                ) : type === "password" ? (
                  <Input.Password
                    disabled={isFieldLocked(field)}
                    placeholder={field.placeholder}
                  />
                ) : (
                  <Input
                    disabled={isFieldLocked(field)}
                    placeholder={field.placeholder}
                    suffix={field.suffix?.({
                      editing,
                      getFieldValue: (name) => form.getFieldValue(name),
                      setFieldValue: (name, next) =>
                        form.setFieldValue(name, next),
                    })}
                  />
                );

              // A field that drives a sibling (picking an entity fills in its
              // source text) needs its onChange wrapped. antd's Form.Item keeps
              // the handler it finds on the child and calls it *after* updating
              // the store, so the helpers already see the new value.
              const linkedControl =
                field.onChange && isValidElement(control)
                  ? cloneElement(
                      control as ReactElement<{
                        onChange?: (...args: unknown[]) => void;
                      }>,
                      {
                        onChange: (value: unknown, ...rest: unknown[]) => {
                          (
                            control as ReactElement<{
                              onChange?: (...args: unknown[]) => void;
                            }>
                          ).props.onChange?.(value, ...rest);
                          field.onChange?.(value, {
                            getFieldValue: (name) => form.getFieldValue(name),
                            setFieldValue: (name, next) =>
                              form.setFieldValue(name, next),
                          });
                        },
                      },
                    )
                  : control;

              const formItem = (
                <Form.Item
                  dependencies={field.dependencies}
                  label={field.label}
                  name={field.key}
                  rules={[
                    ...(field.required
                      ? [
                          {
                            required: true,
                            message: t("crud.requiredField", {
                              label: field.label,
                            }),
                            ...(type === "input" || !type
                              ? { whitespace: true }
                              : {}),
                          } as FormRule,
                        ]
                      : []),
                    ...(typeof field.rules === "function"
                      ? field.rules({
                          editing,
                          getFieldValue: (name) => form.getFieldValue(name),
                        })
                      : field.rules || []),
                  ]}
                  valuePropName={type === "switch" ? "checked" : "value"}
                >
                  {linkedControl}
                </Form.Item>
              );

              if (!field.visibleWhen) {
                return <Fragment key={field.key}>{formItem}</Fragment>;
              }

              return (
                <Form.Item
                  key={field.key}
                  noStyle
                  shouldUpdate={(previous, current) => {
                    const keys = field.dependencies?.length
                      ? field.dependencies
                      : Object.keys({ ...previous, ...current });
                    return keys.some(
                      (key) => previous?.[key] !== current?.[key],
                    );
                  }}
                >
                  {() =>
                    field.visibleWhen?.((name) => form.getFieldValue(name))
                      ? formItem
                      : null
                  }
                </Form.Item>
              );
            })}
        </Form>
      </FormDrawer>
    </Page>
  );
}
