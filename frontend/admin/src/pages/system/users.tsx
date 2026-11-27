import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ComponentProps,
  type Key,
  type ReactNode,
} from "react";
import { useTranslation } from "react-i18next";
import {
  Button,
  Checkbox,
  Form,
  Input,
  Modal,
  Select,
  Space,
  Tag,
  Tooltip,
  Tree as AntTree,
  TreeSelect,
  message,
} from "antd";
import {
  ChevronDown,
  ChevronRight,
  FoldVertical,
  KeyRound,
  Search,
  ShieldCheck,
  UnfoldVertical,
} from "lucide-react";
import {
  getRoleGrantsApi,
  listDepartmentsApi,
  listMenusApi,
  listPositionsApi,
  listRolesApi,
  resetUserPasswordApi,
  saveRoleGrantsApi,
  type ResourceRecord,
} from "@/api/system";
import { FormDrawer } from "@/components/FormDrawer";
import {
  ResourceCrudPage,
  type ColumnConfig,
  type FieldConfig,
  type RowAction,
  type SelectOption,
} from "@/components/ResourceCrudPage";
import { iconNode } from "@/router/menu-utils";
import { formatDateTime } from "@/utils/format-time";
import i18n from "@/i18n";
import { useAccessStore } from "@/stores/access";
import {
  emailError,
  formValidator,
  mobileError,
  passwordError,
  usernameError,
} from "@/utils/user-validation";

const Tree = (props: ComponentProps<typeof AntTree>) => (
  <AntTree expandAction="doubleClick" {...props} />
);

function statusField(): FieldConfig {
  return {
    key: "status",
    label: i18n.t("common.status"),
    numeric: true,
    type: "switch",
  };
}

function isEnabledStatus(status?: number) {
  return Number(status ?? 1) === 1;
}

/** Created-at columns use the configured `system.dateTimeFormat`. */
function createdAtColumn(): ColumnConfig {
  return {
    title: i18n.t("common.createdAt"),
    dataIndex: "createdAt",
    width: 180,
    render: (value) =>
      value == null || value === ""
        ? null
        : formatDateTime(value as string | number | Date),
  };
}

// Deterministic palette so a given role/position keeps the same colour across
// rows and reloads. antd preset colours read well on both light surfaces.
const NAME_TAG_COLORS = [
  "blue",
  "green",
  "gold",
  "purple",
  "cyan",
  "magenta",
  "volcano",
  "geekblue",
  "lime",
  "orange",
];

function nameTagColor(name: string): string {
  let hash = 0;
  for (let index = 0; index < name.length; index += 1)
    hash = (hash * 31 + name.charCodeAt(index)) >>> 0;
  return NAME_TAG_COLORS[hash % NAME_TAG_COLORS.length];
}

function NameTag({ name }: { name: string }) {
  return <Tag color={nameTagColor(name)}>{name}</Tag>;
}

function NameTags({ value }: { value: unknown }) {
  const text = String(value ?? "").trim();
  const names = useMemo(
    () =>
      text
        .split(/,\s*/)
        .map((name) => name.trim())
        .filter(Boolean),
    [text],
  );
  const [visibleCount, setVisibleCount] = useState(names.length);
  const containerRef = useRef<HTMLSpanElement>(null);
  const measureRef = useRef<HTMLSpanElement>(null);

  useLayoutEffect(() => {
    const container = containerRef.current;
    if (!container || !names.length) return;
    const update = () => {
      const availableWidth = container.clientWidth;
      if (availableWidth <= 0) return;
      const measured = Array.from(
        measureRef.current?.children ?? [],
      ) as HTMLElement[];
      const gap = Number.parseFloat(getComputedStyle(container).columnGap) || 0;
      const widths = names.map(
        (_, index) => measured[index]?.getBoundingClientRect().width ?? 0,
      );
      const fullWidth =
        widths.reduce((total, width) => total + width, 0) +
        gap * (names.length - 1);
      if (fullWidth <= availableWidth) {
        setVisibleCount(names.length);
        return;
      }
      let usedWidth = 0;
      let count = 0;
      for (const width of widths) {
        const overflowWidth =
          measured[names.length + count]?.getBoundingClientRect().width ?? 0;
        const nextWidth =
          usedWidth +
          (count > 0 ? gap : 0) +
          width +
          gap +
          overflowWidth;
        if (nextWidth > availableWidth) break;
        usedWidth += (count > 0 ? gap : 0) + width;
        count += 1;
      }
      setVisibleCount(count);
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(container);
    return () => observer.disconnect();
  }, [names]);

  if (!names.length) return null;
  const shownCount = Math.min(visibleCount, names.length);
  const hiddenNames = names.slice(shownCount);
  return (
    <span className="list-tag-cell" ref={containerRef}>
      <span aria-hidden className="list-tag-measure" ref={measureRef}>
        {names.map((name, index) => (
          <NameTag key={`${name}-${index}`} name={name} />
        ))}
        {names.slice(0, -1).map((_, index) => (
          <Tag key={`overflow-${index}`}>+{names.length - index - 1}</Tag>
        ))}
      </span>
      {names.slice(0, shownCount).map((name, index) => (
        <NameTag key={`${name}-${index}`} name={name} />
      ))}
      {hiddenNames.length > 0 && (
        <Tooltip
          placement="top"
          title={
            <span className="list-tag-tooltip">
              {hiddenNames.map((name, index) => (
                <NameTag key={`${name}-${index}`} name={name} />
              ))}
            </span>
          }
        >
          <Tag className="list-tag-overflow">+{hiddenNames.length}</Tag>
        </Tooltip>
      )}
    </span>
  );
}

function nameTags(value: unknown): ReactNode {
  return <NameTags value={value} />;
}

interface DepartmentTreeNode {
  children?: DepartmentTreeNode[];
  key: number;
  title: ReactNode;
}

function departmentSideTree(
  items: ResourceRecord[],
  parentId = 0,
  keyword = "",
): DepartmentTreeNode[] {
  const needle = keyword.trim().toLowerCase();
  const nodes: DepartmentTreeNode[] = [];
  items
    .filter((item) => Number(item.parentId || 0) === parentId)
    .forEach((item) => {
      const children = departmentSideTree(items, Number(item.id), keyword);
      const label = String(item.name || "");
      const matched = !needle || label.toLowerCase().includes(needle);
      if (!matched && !children.length) return;
      nodes.push({
        key: Number(item.id),
        title: <span className="department-tree-title">{label}</span>,
        ...(children.length ? { children } : {}),
      });
    });
  return nodes;
}

function expandableDepartmentKeys(items: ResourceRecord[]): number[] {
  const parentIds = new Set(items.map((item) => Number(item.parentId || 0)));
  return items
    .filter((item) => parentIds.has(Number(item.id)))
    .map((item) => Number(item.id));
}

function UserDepartmentSidePanel({
  filters,
  setFilter,
}: Parameters<
  NonNullable<ComponentProps<typeof ResourceCrudPage>["sidePanel"]>
>[0]) {
  const { t } = useTranslation();
  const [departments, setDepartments] = useState<ResourceRecord[]>([]);
  const [keyword, setKeyword] = useState("");
  const [expandedKeys, setExpandedKeys] = useState<number[]>([]);
  useEffect(() => {
    void listDepartmentsApi().then((items) => {
      setDepartments(items);
      setExpandedKeys(expandableDepartmentKeys(items));
    });
  }, []);
  useEffect(() => {
    setExpandedKeys(expandableDepartmentKeys(departments));
  }, [departments, keyword]);
  const treeData = useMemo(
    () => departmentSideTree(departments, 0, keyword),
    [departments, keyword],
  );
  const selectedId = Number(filters.departmentId || 0);
  return (
    <div className="side-panel-groups">
      <div className="side-panel-heading">
        <span>{t("system.users.department")}</span>
      </div>
      <Input
        allowClear
        className="side-panel-search"
        onChange={(event) => setKeyword(event.target.value)}
        placeholder={t("system.users.searchDepartment")}
        suffix={<Search size={16} />}
        value={keyword}
      />
      <Tree
        blockNode
        expandedKeys={expandedKeys}
        selectedKeys={selectedId > 0 ? [selectedId] : []}
        switcherIcon={(node) =>
          node.expanded ? (
            <ChevronDown aria-hidden size={16} strokeWidth={1.8} />
          ) : (
            <ChevronRight aria-hidden size={16} strokeWidth={1.8} />
          )
        }
        treeData={treeData}
        onExpand={(keys) =>
          setExpandedKeys(
            keys.map(Number).filter((key) => Number.isFinite(key)),
          )
        }
        onSelect={(keys) => {
          const id = Number(keys[0]);
          setFilter(
            "departmentId",
            Number.isFinite(id) && id > 0 ? id : undefined,
          );
        }}
      />
    </div>
  );
}

function userColumns(): ColumnConfig[] {
  return [
    { title: i18n.t("pages.profile.account"), dataIndex: "username", width: 140 },
    { title: i18n.t("pages.profile.realName"), dataIndex: "displayName", width: 120 },
    { title: i18n.t("pages.profile.mobile"), dataIndex: "mobile", width: 140 },
    {
      title: i18n.t("pages.profile.role"),
      dataIndex: "roleNames",
      width: 180,
      render: (value) => nameTags(value),
    },
    { title: i18n.t("system.users.department"), dataIndex: "departmentName", width: 160 },
    {
      title: i18n.t("system.users.position"),
      dataIndex: "positionNames",
      width: 180,
      render: (value) => nameTags(value),
    },
    { title: i18n.t("common.status"), dataIndex: "status", width: 90 },
    createdAtColumn(),
  ];
}

function userFields(passwordMinLength: number): FieldConfig[] {
  return [
    {
      key: "username",
      label: i18n.t("system.users.loginAccount"),
      required: true,
      disabledOnEdit: true,
      rules: [{ validator: formValidator(usernameError) }],
    },
    {
      key: "password",
      label: i18n.t("system.users.initialPassword"),
      type: "password",
      required: true,
      hiddenOnEdit: true,
      dependencies: ["username"],
      rules: ({ getFieldValue }) => [
        {
          validator: formValidator((value) =>
            passwordError(
              value || "",
              passwordMinLength,
              String(getFieldValue("username") || ""),
            ),
          ),
        },
      ],
    },
    {
      key: "displayName",
      label: i18n.t("system.users.displayName"),
      required: true,
    },
    {
      key: "gender",
      label: i18n.t("system.users.gender"),
      type: "segmented",
      options: [
        { label: i18n.t("system.users.genderSecret"), value: 0 },
        { label: i18n.t("system.users.genderMale"), value: 1 },
        { label: i18n.t("system.users.genderFemale"), value: 2 },
      ],
    },
    {
      key: "roleIds",
      label: i18n.t("pages.profile.role"),
      type: "select",
      mode: "multiple",
      optionLoader: async () =>
        (await listRolesApi()).map((item) => ({
          label: String(item.name),
          value: item.id,
        })),
    },
    {
      key: "departmentId",
      label: i18n.t("system.users.departmentField"),
      type: "tree-select",
      allowClear: true,
      optionLoader: async ({ editing }) => {
        const keepId = Number(editing?.departmentId || 0);
        return (await listDepartmentsApi()).map((item) => ({
          label: String(item.name),
          parentId: Number(item.parentId || 0),
          value: item.id,
          disabled:
            !isEnabledStatus(Number(item.status ?? 1)) &&
            Number(item.id) !== keepId,
        }));
      },
    },
    {
      key: "positionIds",
      label: i18n.t("system.users.position"),
      type: "select",
      mode: "multiple",
      optionLoader: async ({ editing }) => {
        const keep = new Set(
          (Array.isArray(editing?.positionIds) ? editing.positionIds : [])
            .map(Number)
            .filter((id) => Number.isFinite(id) && id > 0),
        );
        const result = await listPositionsApi({ page: 1, pageSize: 500 });
        return result.items.map((item) => ({
          label: String(item.name),
          value: item.id,
          disabled:
            !isEnabledStatus(Number(item.status ?? 1)) &&
            !keep.has(Number(item.id)),
        }));
      },
    },
    {
      key: "mobile",
      label: i18n.t("pages.profile.mobile"),
      rules: [{ validator: formValidator(mobileError) }],
    },
    {
      key: "email",
      label: i18n.t("pages.profile.email"),
      rules: [{ validator: formValidator(emailError) }],
    },
    statusField(),
    {
      key: "mustChangePassword",
      label: i18n.t("system.users.mustChangePassword"),
      type: "switch",
    },
    {
      key: "remark",
      label: i18n.t("common.remark"),
      type: "textarea",
    },
  ];
}

export function UserPage() {
  const { t } = useTranslation();
  const passwordMinLength = useAccessStore(
    (state) => state.userInfo?.passwordMinLength ?? 8,
  );
  const currentUserId = useAccessStore((state) => state.userInfo?.userId);
  const [resetTarget, setResetTarget] = useState<ResourceRecord | null>(null);
  const [resetSaving, setResetSaving] = useState(false);
  const [resetForm] = Form.useForm();
  const fields = useMemo(
    () => userFields(passwordMinLength),
    [passwordMinLength, i18n.language],
  );
  const columns = useMemo(() => userColumns(), [i18n.language]);
  const extraSearchFilters = useMemo(
    () => [
      {
        key: "createdAt",
        label: t("common.createdAt"),
        options: [],
        type: "dateRange" as const,
        rangeKeys: ["createdFrom", "createdTo"] as [string, string],
        width: 280,
      },
    ],
    [i18n.language],
  );

  const openReset = (row: ResourceRecord) => {
    setResetTarget(row);
    resetForm.resetFields();
  };
  const closeReset = () => {
    setResetTarget(null);
    resetForm.resetFields();
  };
  const submitReset = async () => {
    if (!resetTarget) return;
    const values = await resetForm.validateFields();
    setResetSaving(true);
    try {
      await resetUserPasswordApi(Number(resetTarget.id), values.password);
      void message.success(t("system.users.passwordReset"));
      closeReset();
    } finally {
      setResetSaving(false);
    }
  };

  const rowActions: RowAction[] = [
    {
      icon: <KeyRound size={16} />,
      label: t("system.users.resetPassword"),
      // The backend refuses to reset the built-in super administrator's
      // password (ResetUserPassword excludes is_super_admin). Keep this
      // action disabled so the UI never offers a guaranteed 404.
      disabled: (row) => !!row.isSuperAdmin,
      onClick: (row) => {
        openReset(row);
      },
      reload: false,
    },
  ];

  return (
    <>
      <ResourceCrudPage
        columns={columns}
        createCode="system:user:create"
        createDefaultsFromFilters={async (filters) => {
          const id = Number(filters.departmentId || 0);
          if (!id) return {};
          const departments = await listDepartmentsApi();
          const selected = departments.find((item) => Number(item.id) === id);
          if (!selected || !isEnabledStatus(Number(selected.status ?? 1)))
            return {};
          return { departmentId: id };
        }}
        defaultValues={{
          gender: 0,
          status: 1,
          mustChangePassword: true,
          roleIds: [],
          positionIds: [],
        }}
        deleteCode="system:user:delete"
        endpoint="/system/users"
        extraSearchFilters={extraSearchFilters}
        fields={fields}
        loadDetailOnEdit
        paged
        protectedField="isSuperAdmin"
        rowActions={rowActions}
        searchMode="submit"
        searchPlaceholder={t("system.users.searchPlaceholder")}
        sidePanel={(context) => <UserDepartmentSidePanel {...context} />}
        statusSwitchDisabled={(record) => String(record.id) === currentUserId}
        title={t("system.users.title")}
        updateCode="system:user:update"
      />
      <Modal
        confirmLoading={resetSaving}
        okText={t("common.confirm")}
        cancelText={t("common.cancel")}
        open={!!resetTarget}
        title={t("system.users.resetPasswordFor", { name: String(resetTarget?.displayName || resetTarget?.username || "") })}
        onCancel={closeReset}
        onOk={() => void submitReset()}
      >
        <Form form={resetForm} layout="vertical" preserve={false}>
          <Form.Item
            name="password"
            label={t("system.users.newPassword")}
            rules={[
              {
                validator: formValidator((value) =>
                  passwordError(String(value || ""), passwordMinLength),
                ),
              },
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}

interface GrantTreeNode {
  children?: GrantTreeNode[];
  key: number | string;
  title: ReactNode;
}

function treeOptions(
  items: ResourceRecord[],
  parentId = 0,
): Array<{
  children?: ReturnType<typeof treeOptions>;
  label: string;
  value: number;
}> {
  return items
    .filter((item) => Number(item.parentId || 0) === parentId)
    .map((item) => {
      const children = treeOptions(items, item.id);
      return {
        label: String(item.name),
        value: item.id,
        ...(children.length ? { children } : {}),
      };
    });
}

function menuGrantTreeOptions(
  items: ResourceRecord[],
  parentId = 0,
): GrantTreeNode[] {
  return items
    .filter((item) => Number(item.parentId || 0) === parentId)
    .map((item) => {
      const children = menuGrantTreeOptions(items, item.id);
      const icon = String(item.icon || "");
      const isButton = item.type === "button";
      return {
        key: item.id,
        title: (
          <span className="role-grant-tree-title">
            {icon ? iconNode(icon) : null}
            <span>{String(item.name)}</span>
            {isButton ? (
              <Tag className="role-grant-tree-button">
                {i18n.t("system.users.button")}
              </Tag>
            ) : null}
            {isButton ? (
              <span className="role-grant-tree-code">{String(item.code)}</span>
            ) : null}
          </span>
        ),
        ...(children.length ? { children } : {}),
      };
    });
}

function numericCheckedKeys(keys: Key[]): number[] {
  return keys
    .map((key) => Number(key))
    .filter((key) => Number.isFinite(key) && key > 0);
}

function expandSelectedMenuIds(
  items: ResourceRecord[],
  selectedIds: unknown[],
): number[] {
  const selected = new Set(
    selectedIds
      .map((id) => Number(id))
      .filter((id) => Number.isFinite(id) && id > 0),
  );
  const byParent = items.reduce((map, item) => {
    const parentId = Number(item.parentId || 0);
    map.set(parentId, [...(map.get(parentId) || []), Number(item.id)]);
    return map;
  }, new Map<number, number[]>());
  const parentById = items.reduce((map, item) => {
    map.set(Number(item.id), Number(item.parentId || 0));
    return map;
  }, new Map<number, number>());
  const visit = (id: number) => {
    selected.add(id);
    for (const childId of byParent.get(id) || []) visit(childId);
  };
  for (const id of Array.from(selected)) {
    for (
      let parentId = parentById.get(id) || 0;
      parentId > 0;
      parentId = parentById.get(parentId) || 0
    )
      selected.add(parentId);
    visit(id);
  }
  return Array.from(selected);
}

function grantTreeNumericKeys(nodes: GrantTreeNode[]): number[] {
  return nodes.flatMap((node) =>
    [
      typeof node.key === "number" ? node.key : null,
      ...(node.children ? grantTreeNumericKeys(node.children) : []),
    ].filter((key): key is number => key !== null),
  );
}

function grantTreeExpandableKeys(nodes: GrantTreeNode[]): Key[] {
  return nodes
    .filter((node) => node.children?.length)
    .flatMap((node) => [
      node.key,
      ...grantTreeExpandableKeys(node.children ?? []),
    ]);
}

function RoleGrantTree({
  treeData,
  value = [],
  onChange,
}: {
  onChange?: (value: number[]) => void;
  treeData: GrantTreeNode[];
  value?: Key[];
}) {
  const { t } = useTranslation();
  const [expandedKeys, setExpandedKeys] = useState<Key[] | null>(null);
  const expandableKeys = useMemo(
    () => grantTreeExpandableKeys(treeData),
    [treeData],
  );
  const expanded = expandedKeys ?? expandableKeys;
  const allKeys = useMemo(() => grantTreeNumericKeys(treeData), [treeData]);
  const allSelected =
    allKeys.length > 0 && allKeys.every((key) => value.includes(key));
  const allExpanded =
    expandableKeys.length > 0 &&
    expandableKeys.every((key) => expanded.includes(key));
  return (
    <div className="role-grant-tree-wrapper">
      <Space className="role-grant-tree-actions" size={16}>
        <Checkbox
          checked={allSelected}
          onChange={(event) => onChange?.(event.target.checked ? allKeys : [])}
        >
          {t("system.users.selectAll")}
        </Checkbox>
        <Button
          type="link"
          size="small"
          icon={
            allExpanded ? (
              <FoldVertical size={14} />
            ) : (
              <UnfoldVertical size={14} />
            )
          }
          onClick={() => setExpandedKeys(allExpanded ? [] : expandableKeys)}
        >
          {allExpanded
            ? t("system.users.collapse")
            : t("common.expand")}
        </Button>
      </Space>
      <div className="role-grant-tree">
        <Tree
          blockNode
          checkable
          checkedKeys={value}
          expandedKeys={expanded}
          selectable={false}
          treeData={treeData}
          onCheck={(keys) =>
            onChange?.(
              numericCheckedKeys(Array.isArray(keys) ? keys : keys.checked),
            )
          }
          onExpand={(keys) => setExpandedKeys([...keys])}
        />
      </div>
    </div>
  );
}

/** sys_role.data_scope enum: 0 ALL, 1 SELF, 2 DEPT, 3 DEPT_AND_CHILDREN, 4 CUSTOM. */
function dataScopeOptions(): SelectOption[] {
  return [
    { label: i18n.t("system.users.dataScopeAll"), value: 0 },
    { label: i18n.t("system.users.dataScopeSelf"), value: 1 },
    { label: i18n.t("system.users.dataScopeDept"), value: 2 },
    {
      label: i18n.t("system.users.dataScopeDeptChildren"),
      value: 3,
    },
    { label: i18n.t("system.users.dataScopeCustom"), value: 4 },
  ] satisfies SelectOption[];
}

function roleColumns(): ColumnConfig[] {
  return [
    { title: i18n.t("system.users.roleName"), dataIndex: "name", width: 180 },
    { title: i18n.t("system.users.roleCode"), dataIndex: "code", width: 160 },
    {
      title: i18n.t("system.users.dataScope"),
      dataIndex: "dataScope",
      width: 160,
      labelMap: Object.fromEntries(
        dataScopeOptions().map((option) => [String(option.value), option.label]),
      ),
    },
    { title: i18n.t("system.users.userCount"), dataIndex: "userCount", width: 100 },
    { title: i18n.t("common.status"), dataIndex: "status", width: 90 },
    { title: i18n.t("common.remark"), dataIndex: "remark", width: 160 },
    createdAtColumn(),
  ];
}

function roleFields(): FieldConfig[] {
  return [
    { key: "name", label: i18n.t("system.users.roleName"), required: true },
    { key: "code", label: i18n.t("system.users.roleCode"), required: true },
    {
      key: "dataScope",
      label: i18n.t("system.users.dataScope"),
      type: "select",
      options: dataScopeOptions(),
    },
    statusField(),
    {
      key: "remark",
      label: i18n.t("common.remark"),
      type: "textarea",
    },
  ];
}

export function RolePage() {
  const { t } = useTranslation();
  const [grant, setGrant] = useState<ResourceRecord | null>(null);
  const columns = useMemo(() => roleColumns(), [i18n.language]);
  const fields = useMemo(() => roleFields(), [i18n.language]);
  const [grantForm] = Form.useForm();
  const [menuResources, setMenuResources] = useState<ResourceRecord[]>([]);
  const [menuTree, setMenuTree] = useState<GrantTreeNode[]>([]);
  const [departments, setDepartments] = useState<
    ReturnType<typeof treeOptions>
  >([]);

  const openGrant = async (row: ResourceRecord) => {
    const [value, menus, depts] = await Promise.all([
      getRoleGrantsApi(row.id),
      listMenusApi(),
      listDepartmentsApi(),
    ]);
    setGrant(row);
    grantForm.setFieldsValue(value);
    setMenuResources(menus);
    setMenuTree(menuGrantTreeOptions(menus));
    setDepartments(treeOptions(depts));
  };

  const saveGrant = async () => {
    if (!grant) return;
    const value = await grantForm.validateFields();
    await saveRoleGrantsApi(grant.id, {
      ...value,
      menuIds: expandSelectedMenuIds(menuResources, value.menuIds || []),
    });
    setGrant(null);
    void message.success(t("system.users.grantUpdated"));
  };

  const rowActions: RowAction[] = [
    {
      icon: <ShieldCheck size={16} />,
      label: t("system.users.grant"),
      disabled: (row) => !!row.isSystem,
      reload: false,
      onClick: (row) => void openGrant(row),
    },
  ];

  return (
    <>
      <ResourceCrudPage
        columns={columns}
        createCode="system:role:create"
        defaultValues={{ dataScope: 1, sort: 0, status: 1 }}
        deleteCode="system:role:delete"
        endpoint="/system/roles"
        fields={fields}
        protectedField="isSystem"
        rowActions={rowActions}
        searchMode="local-submit"
        title={t("pages.profile.role")}
        updateCode="system:role:update"
      />
      <FormDrawer
        form={grantForm}
        open={!!grant}
        title={t("system.users.roleGrant")}
        width={720}
        okText={t("common.save")}
        onClose={() => setGrant(null)}
        onOk={saveGrant}
      >
        <Form form={grantForm} labelCol={{ span: 5 }}>
          <Form.Item
            name="dataScope"
            label={t("system.users.dataScopeRange")}
          >
            <Select
              options={[
                { label: t("system.users.dataScopeAll"), value: 0 },
                {
                  label: t("system.users.dataScopeSelfData"),
                  value: 1,
                },
                {
                  label: t("system.users.dataScopeDeptData"),
                  value: 2,
                },
                {
                  label: t("system.users.dataScopeDeptChildren"),
                  value: 3,
                },
                {
                  label: t("system.users.dataScopeCustom"),
                  value: 4,
                },
              ]}
            />
          </Form.Item>
          <Form.Item shouldUpdate noStyle>
            {({ getFieldValue }) =>
              getFieldValue("dataScope") === 4 && (
                <Form.Item
                  name="departmentIds"
                  label={t("system.users.grantDepartments")}
                >
                  <TreeSelect
                    treeCheckable
                    treeData={departments}
                    treeDefaultExpandAll
                  />
                </Form.Item>
              )
            }
          </Form.Item>
          <Form.Item
            name="menuIds"
            label={t("system.users.menusAndButtons")}
          >
            <RoleGrantTree treeData={menuTree} />
          </Form.Item>
        </Form>
      </FormDrawer>
    </>
  );
}
