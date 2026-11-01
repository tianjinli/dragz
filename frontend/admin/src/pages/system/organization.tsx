import { type ComponentProps, useMemo } from "react";
import {
  ResourceCrudPage,
  type ColumnConfig,
  type FieldConfig,
  type RowAction,
  type SearchFilterConfig,
  type SelectOption,
  type SelectTreeNode,
} from "@/components/ResourceCrudPage";
import { listDepartmentsApi, listMenusApi, listUsersApi } from "@/api/system";
import i18n from "@/i18n";
import { iconNode } from "@/router/menu-utils";
import type { ResourceRecord } from "@/api/system";

const status = (): FieldConfig => ({
  key: "status",
  label: i18n.t("common.status"),
  numeric: true,
  type: "switch",
});
const sort = (): FieldConfig => ({
  key: "sort",
  label: i18n.t("common.sort"),
  numeric: true,
  type: "number",
});
function departmentPathIds(department: ResourceRecord | null): number[] {
  if (!department) return [];
  const ids = String(department.ancestors || "")
    .split(",")
    .map((id) => Number(id))
    .filter((id) => Number.isFinite(id) && id > 0);
  ids.push(Number(department.id));
  return Array.from(new Set(ids));
}
async function resolveLeaderDepartmentIds(
  editing: ResourceRecord | null,
  parentId: unknown,
) {
  if (editing) return departmentPathIds(editing).reverse();
  const selectedParentId = Number(parentId || 0);
  if (!selectedParentId) return [];
  const departments = await listDepartmentsApi();
  const parent =
    departments.find((item) => Number(item.id) === selectedParentId) || null;
  return departmentPathIds(parent).reverse();
}
function currentLeaderDepartmentId(
  editing: ResourceRecord | null,
  parentId: unknown,
) {
  return Number(editing?.id || parentId || 0);
}
interface ResourceOptions {
  cascadeStatus?: boolean;
  /** Guard for the delete action (rows owned by the route-table sync). */
  deleteDisabled?: (row: ResourceRecord) => boolean;
  extraSearchFilters?: SearchFilterConfig[];
  /**
   * manage: the module is guarded by a single `${code}:manage` permission
   * instead of the create/update/delete trio. Used where the backend route
   * table declares one manage code for all three verbs.
   */
  manage?: boolean;
  paged?: boolean;
  rowActions?: RowAction[];
  searchMode?: "auto" | "local" | "none" | "submit";
  sidePanel?: ComponentProps<typeof ResourceCrudPage>["sidePanel"];
  statusFilterAfterExtra?: boolean;
  statusSwitchDisabled?: (row: ResourceRecord) => boolean;
  tree?: boolean;
}

interface ResourceConfig {
  code: string;
  columns: ColumnConfig[];
  endpoint: string;
  fields: FieldConfig[];
  options?: ResourceOptions;
  title: string;
}

/**
 * The config is produced per render (not at import) so a language switch
 * relabels an already-mounted page instead of showing the strings captured
 * when the module was first evaluated.
 */
function resource(config: () => ResourceConfig) {
  return function ResourcePage() {
    const { code, columns, endpoint, fields, options, title } = useMemo(
      config,
      [config, i18n.language],
    );
    const createCode = options?.manage ? `${code}:manage` : `${code}:create`;
    const updateCode = options?.manage ? `${code}:manage` : `${code}:update`;
    const deleteCode = options?.manage ? `${code}:manage` : `${code}:delete`;
    return (
      <ResourceCrudPage
        cascadeStatus={options?.cascadeStatus}
        columns={columns}
        createCode={createCode}
        deleteCode={deleteCode}
        deleteDisabled={options?.deleteDisabled}
        endpoint={endpoint}
        extraSearchFilters={options?.extraSearchFilters}
        fields={fields}
        paged={options?.paged}
        rowActions={options?.rowActions}
        searchMode={options?.searchMode}
        sidePanel={options?.sidePanel}
        statusFilterAfterExtra={options?.statusFilterAfterExtra}
        statusSwitchDisabled={options?.statusSwitchDisabled}
        title={title}
        tree={options?.tree}
        updateCode={updateCode}
      />
    );
  };
}
export const DepartmentPage = resource(() => ({
  title: i18n.t("system.org.department"),
  endpoint: "/system/departments",
  code: "system:department",
  columns: [
    {
      title: i18n.t("system.org.departmentName"),
      dataIndex: "name",
      width: 220,
      align: "left",
    },
    {
      title: i18n.t("system.org.departmentCode"),
      dataIndex: "code",
      width: 120,
    },
    {
      title: i18n.t("system.org.ancestorPath"),
      dataIndex: "ancestorPath",
      width: 280,
    },
    { title: i18n.t("system.org.leader"), dataIndex: "leaderName", width: 140 },
    { title: i18n.t("common.sort"), dataIndex: "sort", width: 90 },
    { title: i18n.t("common.status"), dataIndex: "status", width: 90 },
  ],
  fields: [
    {
      key: "parentId",
      label: i18n.t("system.org.parentDepartment"),
      type: "tree-select",
      optionLoader: async () =>
        (await listDepartmentsApi()).map((item) => ({
          label: String(item.name),
          parentId: Number(item.parentId || 0),
          value: item.id,
        })),
    },
    {
      key: "name",
      label: i18n.t("system.org.departmentName"),
      required: true,
    },
    { key: "code", label: i18n.t("system.org.departmentCode"), required: true },
    {
      key: "leaderUserId",
      label: i18n.t("system.org.leader"),
      type: "select",
      allowClear: true,
      optionDependencies: ["parentId"],
      placeholder: i18n.t("system.org.leaderSearchPlaceholder"),
      remoteSearch: true,
      optionLoader: async ({ editing, keyword, values }) => {
        const departmentIds = await resolveLeaderDepartmentIds(
          editing,
          values.parentId,
        );
        const currentLeaderId = Number(editing?.leaderUserId || 0);
        if (!departmentIds.length)
          return currentLeaderId
            ? [
                {
                  label: String(editing?.leaderName || currentLeaderId),
                  value: currentLeaderId,
                },
              ]
            : [];
        const currentDepartmentId = currentLeaderDepartmentId(
          editing,
          values.parentId,
        );
        const results = await Promise.all(
          departmentIds.map((departmentId) =>
            listUsersApi({
              departmentId,
              keyword,
              page: 1,
              pageSize: 500,
              status: 1,
            }),
          ),
        );
        const users = new Map<number, ResourceRecord>();
        results
          .flatMap((result) => result.items)
          .forEach((item) => users.set(Number(item.id), item));
        const options = Array.from(users.values()).map((item) => {
          const username = String(item.username || "");
          const displayName = String(item.displayName || username || item.id);
          const outsideCurrentDepartment =
            Number(item.departmentId || 0) !== currentDepartmentId;
          return {
            className: outsideCurrentDepartment
              ? "department-leader-option-outside-current"
              : undefined,
            label: displayName,
            value: item.id,
          };
        });
        if (currentLeaderId && !users.has(currentLeaderId))
          options.unshift({
            className: undefined,
            label: String(editing?.leaderName || currentLeaderId),
            value: currentLeaderId,
          });
        return options;
      },
    },
    sort(),
    status(),
  ],
  options: {
    tree: true,
    cascadeStatus: true,
    rowActions: [
      {
        label: i18n.t("system.org.addChild"),
        createValues: (row: ResourceRecord) => ({ parentId: row.id }),
      },
    ],
    searchMode: "local",
  },
}));
export const PositionPage = resource(() => ({
  title: i18n.t("system.org.position"),
  endpoint: "/system/positions",
  code: "system:position",
  columns: [
    { title: i18n.t("system.org.positionName"), dataIndex: "name", width: 220 },
    { title: i18n.t("system.org.positionCode"), dataIndex: "code", width: 180 },
    { title: i18n.t("system.org.userCount"), dataIndex: "userCount", width: 100 },
    { title: i18n.t("common.sort"), dataIndex: "sort", width: 90 },
    { title: i18n.t("common.status"), dataIndex: "status", width: 90 },
    { title: i18n.t("common.remark"), dataIndex: "remark", width: 260 },
  ],
  fields: [
    { key: "name", label: i18n.t("system.org.positionName"), required: true },
    { key: "code", label: i18n.t("system.org.positionCode"), required: true },
    sort(),
    status(),
    { key: "remark", label: i18n.t("common.remark"), type: "textarea" },
  ],
  options: { paged: true, searchMode: "submit" },
}));
const menuTypeLabels = (): Record<string, string> => ({
  directory: i18n.t("system.org.menuType.directory"),
  page: i18n.t("system.org.menuType.page"),
  button: i18n.t("system.org.menuType.button"),
  external: i18n.t("system.org.menuType.external"),
});
function menuChildrenOf(menus: ResourceRecord[], id: number): ResourceRecord[] {
  return menus.filter((menu) => Number(menu.parentId) === id);
}
function menuDescendantIds(menus: ResourceRecord[], id: number): number[] {
  return menuChildrenOf(menus, id).flatMap((child) => [
    Number(child.id),
    ...menuDescendantIds(menus, Number(child.id)),
  ]);
}
/**
 * A permission button the startup sync owns: its code, endpoint and parent come
 * from the route table, so the form locks them. A button an administrator
 * created by hand (managed = 0) is an ordinary row and stays fully editable.
 */
const isManagedButton = (row: ResourceRecord | null) =>
  String(row?.type ?? "") === "button" && Number(row?.managed ?? 0) === 1;
function menuAllowedTypes(
  menus: ResourceRecord[],
  editing: ResourceRecord | null,
  parentId: unknown,
): string[] {
  const editingType = editing ? String(editing.type) : "";
  // A button is a leaf: its type never changes and it may not own anything.
  if (editingType === "button") return ["button"];
  const parentType = menus.find(
    (menu) => Number(menu.id) === Number(parentId),
  )?.type;
  // Children may only be added below a directory or a page, and a page's
  // children are permission buttons.
  if (parentType === "button" || parentType === "external") return [];
  if (editing && menuChildrenOf(menus, Number(editing.id)).length)
    return [String(editing.type)];
  if (parentType === "page") return ["button"];
  return ["directory", "page"];
}
function menuTreeOption(menu: ResourceRecord): SelectOption {
  return {
    icon: String(menu.icon || ""),
    label: String(menu.name || ""),
    parentId: Number(menu.parentId || 0),
    value: menu.id,
  };
}
const menuParentField = (): FieldConfig => ({
  key: "parentId",
  label: i18n.t("system.org.parentMenu"),
  type: "tree-select",
  optionLoader: async ({ editing }) => {
    const menus = await listMenusApi();
    const excluded = new Set<number>(
      editing
        ? [Number(editing.id), ...menuDescendantIds(menus, Number(editing.id))]
        : [],
    );
    // Directories and pages may own children; a button may not. Pages are
    // included so an existing button can show its parent instead of an empty
    // field.
    return menus
      .filter(
        (menu) =>
          menu.type !== "button" && !excluded.has(Number(menu.id)),
      )
      .map(menuTreeOption);
  },
  treeTitleRender: (node: SelectTreeNode) => (
    <span className="department-tree-title">
      {node.icon ? iconNode(node.icon) : null}
      <span>{node.label}</span>
    </span>
  ),
});
const menuTypeField = (): FieldConfig => ({
  key: "type",
  label: i18n.t("common.type"),
  required: true,
  type: "select",
  optionDependencies: ["parentId"],
  optionLoader: async ({ editing, values }) => {
    const menus = await listMenusApi();
    const labels = menuTypeLabels();
    return menuAllowedTypes(menus, editing, values.parentId).map((type) => ({
      label: labels[type] ?? type,
      value: type,
    }));
  },
});
// Route-shaped fields only make sense for directories and pages.
const routeOnly = (getFieldValue: (name: string) => unknown) =>
  getFieldValue("type") !== "button";
export const MenuPage = resource(() => ({
  title: i18n.t("system.org.menu"),
  endpoint: "/system/menus",
  code: "system:menu",
  columns: [
    {
      title: i18n.t("system.org.menuName"),
      dataIndex: "name",
      align: "left",
      width: 220,
      iconDataIndex: "icon",
    },
    {
      title: i18n.t("common.type"),
      dataIndex: "type",
      width: 150,
      render: (value, row) => {
        const label = menuTypeLabels()[String(value)] ?? String(value);
        if (value !== "button") return <span>{label}</span>;
        return (
          <span className="menu-type-cell">
            <span>{label}</span>
            <span className="menu-type-source">
              {Number(row.managed ?? 0) === 1
                ? i18n.t("system.org.menuSourceSynced")
                : i18n.t("system.org.menuSourceManual")}
            </span>
          </span>
        );
      },
    },
    { title: i18n.t("system.org.route"), dataIndex: "routePath", width: 200 },
    {
      title: i18n.t("system.org.permissionCode"),
      dataIndex: "code",
      width: 220,
    },
    { title: i18n.t("system.org.method"), dataIndex: "method", width: 90 },
    { title: i18n.t("system.org.path"), dataIndex: "path", width: 240 },
    { title: i18n.t("common.sort"), dataIndex: "sort", width: 80 },
    { title: i18n.t("common.status"), dataIndex: "status", width: 80 },
  ],
  fields: [
    menuParentField(),
    menuTypeField(),
    { key: "name", label: i18n.t("system.org.menuName"), required: true },
    {
      key: "code",
      label: i18n.t("system.org.permissionCode"),
      required: true,
      // The route table owns a synced button's code; a hand-made one is free.
      disabledOnEdit: isManagedButton,
    },
    // Owned by the route table: shown for reference, never editable.
    {
      key: "method",
      label: i18n.t("system.org.requestMethod"),
      disabledOnEdit: isManagedButton,
      dependencies: ["type"],
      visibleWhen: (getFieldValue) => getFieldValue("type") === "button",
    },
    {
      key: "path",
      label: i18n.t("system.org.apiPath"),
      disabledOnEdit: isManagedButton,
      dependencies: ["type"],
      visibleWhen: (getFieldValue) => getFieldValue("type") === "button",
    },
    {
      key: "routeName",
      label: i18n.t("system.org.routeName"),
      dependencies: ["type"],
      visibleWhen: routeOnly,
    },
    {
      key: "routePath",
      label: i18n.t("system.org.routePath"),
      dependencies: ["type"],
      visibleWhen: routeOnly,
    },
    {
      key: "component",
      label: i18n.t("system.org.componentPath"),
      dependencies: ["type"],
      visibleWhen: routeOnly,
    },
    {
      key: "icon",
      label: i18n.t("system.org.icon"),
      type: "icon",
      dependencies: ["type"],
      visibleWhen: routeOnly,
    },
    sort(),
    status(),
  ],
  options: {
    deleteDisabled: isManagedButton,
    manage: true,
    tree: true,
    rowActions: [
      {
        label: i18n.t("system.org.addChild"),
        // Directories get sub-menus, pages get permission buttons.
        disabled: (row: ResourceRecord) => row.type !== "directory" && row.type !== "page",
        createValues: (row: ResourceRecord) => ({ parentId: row.id }),
      },
    ],
    searchMode: "none",
    statusSwitchDisabled: isManagedButton,
  },
}));
