import {
  useEffect,
  useMemo,
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
  Select,
  Space,
  Tag,
  Tree as AntTree,
  TreeSelect,
  message,
} from "antd";
import {
  ChevronDown,
  ChevronRight,
  FoldVertical,
  Search,
  UnfoldVertical,
} from "lucide-react";
import {
  getRoleGrantsApi,
  listDepartmentsApi,
  listMenusApi,
  listPositionsApi,
  listRolesApi,
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
    { title: i18n.t("pages.profile.role"), dataIndex: "roleNames", width: 120 },
    { title: i18n.t("system.users.department"), dataIndex: "departmentName", width: 160 },
    { title: i18n.t("common.status"), dataIndex: "status", width: 90 },
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
  const fields = useMemo(
    () => userFields(passwordMinLength),
    [passwordMinLength, i18n.language],
  );
  const columns = useMemo(() => userColumns(), [i18n.language]);
  return (
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
      fields={fields}
      loadDetailOnEdit
      paged
      protectedField="isSuperAdmin"
      searchMode="submit"
      searchPlaceholder={t("system.users.searchPlaceholder")}
      sidePanel={(context) => <UserDepartmentSidePanel {...context} />}
      statusSwitchDisabled={(record) => String(record.id) === currentUserId}
      title={t("system.users.title")}
      updateCode="system:user:update"
    />
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

function roleColumns(): ColumnConfig[] {
  return [
    { title: i18n.t("system.users.roleName"), dataIndex: "name", width: 180 },
    { title: i18n.t("system.users.roleCode"), dataIndex: "code", width: 160 },
    { title: i18n.t("system.users.dataScope"), dataIndex: "dataScope", width: 160 },
    { title: i18n.t("system.users.userCount"), dataIndex: "userCount", width: 100 },
    { title: i18n.t("common.status"), dataIndex: "status", width: 90 },
    { title: i18n.t("common.remark"), dataIndex: "remark", width: 160 },
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
      options: [
        { label: i18n.t("system.users.dataScopeAll"), value: "all" },
        {
          label: i18n.t("system.users.dataScopeDeptChildren"),
          value: "department_and_children",
        },
        { label: i18n.t("system.users.dataScopeDept"), value: "department" },
        { label: i18n.t("system.users.dataScopeSelf"), value: "self" },
        { label: i18n.t("system.users.dataScopeCustom"), value: "custom" },
      ] satisfies SelectOption[],
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
        defaultValues={{ dataScope: "self", sort: 0, status: 1 }}
        deleteCode="system:role:delete"
        endpoint="/system/roles"
        fields={fields}
        protectedField="isSystem"
        rowActions={rowActions}
        searchMode="local"
        title={t("pages.profile.role")}
        updateCode="system:role:update"
      />
      <FormDrawer
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
                { label: t("system.users.dataScopeAll"), value: "all" },
                {
                  label: t("system.users.dataScopeDeptChildren"),
                  value: "department_and_children",
                },
                {
                  label: t("system.users.dataScopeDeptData"),
                  value: "department",
                },
                {
                  label: t("system.users.dataScopeSelfData"),
                  value: "self",
                },
                {
                  label: t("system.users.dataScopeCustom"),
                  value: "custom",
                },
              ]}
            />
          </Form.Item>
          <Form.Item shouldUpdate noStyle>
            {({ getFieldValue }) =>
              getFieldValue("dataScope") === "custom" && (
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
