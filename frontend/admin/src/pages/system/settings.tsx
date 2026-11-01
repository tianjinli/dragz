import { ListPagePanel } from "@/components/ListPagePanel";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  Button,
  Card,
  Drawer,
  Form,
  Input,
  InputNumber,
  Segmented,
  Select,
  Space,
  Switch,
  Tag,
  message,
} from "antd";
import { Maximize2, Minimize2, Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import i18n from "@/i18n";
import {
  createDictionaryItemApi,
  createDictionaryTypeApi,
  deleteDictionaryItemApi,
  deleteDictionaryTypeApi,
  listConfigsApi,
  listDictionaryItemsApi,
  listDictionaryTypesApi,
  testStorageConfigApi,
  updateConfigApi,
  updateDictionaryItemApi,
  updateDictionaryTypeApi,
  type ResourceRecord,
} from "@/api/system";
import { NumberStepper } from "@/components/NumberStepper";
import { Page } from "@/components/Page";
import { ResourceCrudPage } from "@/components/ResourceCrudPage";
import { normalizeListColumns } from "@/utils/table-columns";
import { TableActionGroup, type TableAction } from "@/components/TableActionGroup";
import { TablePanel } from "@/components/TablePanel";

function configValue(row: ResourceRecord) {
  return String(row.value ?? row.configValue ?? "");
}

function configType(row: ResourceRecord) {
  return String(row.valueType ?? row.type ?? "string");
}

function isSecretConfig(row: ResourceRecord) {
  return (
    row.isSecret === true ||
    row.isSecret === 1 ||
    row.is_secret === true ||
    row.is_secret === 1 ||
    row.sensitive === true ||
    row.sensitive === 1
  );
}

function parseBooleanConfigValue(value: string) {
  return value === "true" || value === "1";
}

const dictionaryDisplayTypeKeys: Record<string, string> = {
  segmented: "system.settings.dict.displaySegmented",
  dropdown: "system.settings.dict.displayDropdown",
  checkbox: "system.settings.dict.displayCheckbox",
};

function dictionaryDisplayTypeOptions() {
  return Object.entries(dictionaryDisplayTypeKeys).map(([value, key]) => ({
    label: i18n.t(key),
    value,
  }));
}

function dictionaryDisplayTypeLabel(value: unknown) {
  const key = String(value || "segmented");
  const labelKey = dictionaryDisplayTypeKeys[key];
  return labelKey ? i18n.t(labelKey) : key;
}

const dictionaryFormLayout = {
  labelCol: { flex: "92px" },
  wrapperCol: { flex: 1 },
};

export function DictionaryPage() {
  const { t } = useTranslation();
  const [types, setTypes] = useState<ResourceRecord[]>([]);
  const [selected, setSelected] = useState<ResourceRecord | null>(null);
  const [items, setItems] = useState<ResourceRecord[]>([]);
  const [typeOpen, setTypeOpen] = useState(false);
  const [itemOpen, setItemOpen] = useState(false);
  const [typeMaximized, setTypeMaximized] = useState(false);
  const [itemMaximized, setItemMaximized] = useState(false);
  const [editingType, setEditingType] = useState<ResourceRecord | null>(null);
  const [editingItem, setEditingItem] = useState<ResourceRecord | null>(null);
  const [typeForm] = Form.useForm();
  const [itemForm] = Form.useForm();
  const [typeStatusChangingId, setTypeStatusChangingId] = useState<
    number | null
  >(null);
  const [statusChangingId, setStatusChangingId] = useState<number | null>(null);

  const loadTypes = useCallback(async () => {
    const data = await listDictionaryTypesApi({ page: 1, pageSize: 500 });
    setTypes(data);
    setSelected((current) => {
      if (!current) return data[0] ?? null;
      return data.find((item) => item.id === current.id) ?? data[0] ?? null;
    });
  }, []);
  const loadItems = useCallback(async () => {
    if (!selected) {
      setItems([]);
      return;
    }
    setItems(await listDictionaryItemsApi(selected.id));
  }, [selected]);

  useEffect(() => {
    void loadTypes();
  }, [loadTypes]);
  useEffect(() => {
    void loadItems();
  }, [loadItems]);

  const openTypeEditor = (row: ResourceRecord | null) => {
    setEditingType(row);
    setTypeMaximized(false);
    setTypeOpen(true);
  };
  const openItemEditor = (row: ResourceRecord | null) => {
    setEditingItem(row);
    setItemMaximized(false);
    setItemOpen(true);
  };
  const saveType = async () => {
    const values = await typeForm.validateFields();
    const payload = {
      ...values,
      displayType: values.displayType ?? "segmented",
      status: values.status ? 1 : 0,
    };
    if (editingType) await updateDictionaryTypeApi(editingType.id, payload);
    else await createDictionaryTypeApi(payload);
    setTypeOpen(false);
    void message.success(t("common.saveSuccess"));
    await loadTypes();
  };
  const saveItem = async () => {
    const values = await itemForm.validateFields();
    const payload = {
      ...values,
      sort: Number(values.sort ?? 0),
      status: values.status ? 1 : 0,
      isDefault: !!values.isDefault,
    };
    if (!selected) return;
    if (editingItem) await updateDictionaryItemApi(editingItem.id, payload);
    else await createDictionaryItemApi(selected.id, payload);
    setItemOpen(false);
    void message.success(t("common.saveSuccess"));
    await loadItems();
    await loadTypes();
  };
  const changeTypeStatus = async (row: ResourceRecord, checked: boolean) => {
    setTypeStatusChangingId(row.id);
    try {
      await updateDictionaryTypeApi(row.id, {
        code: row.code,
        name: row.name,
        displayType: row.displayType ?? "segmented",
        status: checked ? 1 : 0,
        remark: row.remark ?? "",
      });
      void message.success(
        checked
          ? t("system.settings.dict.enabledToast")
          : t("system.settings.dict.disabledToast"),
      );
      await loadTypes();
    } finally {
      setTypeStatusChangingId(null);
    }
  };
  const changeItemStatus = async (row: ResourceRecord, checked: boolean) => {
    setStatusChangingId(row.id);
    try {
      await updateDictionaryItemApi(row.id, {
        ...row,
        status: checked ? 1 : 0,
        sort: Number(row.sort ?? 0),
        isDefault: !!row.isDefault,
      });
      void message.success(t("system.settings.dict.itemStatusUpdated"));
      await loadItems();
    } finally {
      setStatusChangingId(null);
    }
  };
  // The backend refuses to delete a system dictionary or one that still has
  // items (409 + errors.dict.protected); the toast explains why.
  const removeType = async (row: ResourceRecord) => {
    await deleteDictionaryTypeApi(row.id);
    void message.success(t("common.deleteSuccess"));
    await loadTypes();
  };
  const removeItem = async (row: ResourceRecord) => {
    await deleteDictionaryItemApi(row.id);
    void message.success(t("common.deleteSuccess"));
    await loadItems();
    await loadTypes();
  };

  return (
    <Page title={t("system.settings.dict.title")}>
      <div className="split-page dictionary-split-page">
        <Card className="split-master table-card dictionary-table-card">
          <ListPagePanel
            title={t("system.settings.dict.typeList")}
            toolbar={{
              actions: (
                <Button
                  type="primary"
                  icon={<Plus size={16} />}
                  onClick={() => openTypeEditor(null)}
                >
                  {t("common.add")}
                </Button>
              ),
              onRefresh: () => void loadTypes(),
            }}
          >
            <TablePanel
              className="list-page-table"
              bordered
              sticky
              scroll={{ x: "max-content" }}
              size="small"
              rowKey="id"
              dataSource={types}
              pagination={false}
              rowClassName={(row) =>
                row.id === selected?.id ? "ant-table-row-selected" : ""
              }
              onRow={(row) => ({ onClick: () => setSelected(row) })}
              columns={normalizeListColumns<ResourceRecord>([
                {
                  title: t("system.settings.dict.typeName"),
                  dataIndex: "name",
                },
                {
                  title: t("system.settings.dict.typeCode"),
                  dataIndex: "code",
                },
                {
                  title: t("system.settings.dict.displayType"),
                  dataIndex: "displayType",
                  render: (v) => dictionaryDisplayTypeLabel(v),
                },
                {
                  title: t("common.status"),
                  dataIndex: "status",
                  render: (v, row) => (
                    <span onClick={(event) => event.stopPropagation()}>
                      <Switch
                        checked={v === 1}
                        loading={typeStatusChangingId === row.id}
                        onChange={(checked) =>
                          void changeTypeStatus(row, checked)
                        }
                      />
                    </span>
                  ),
                },
                {
                  title: t("common.actions"),
                  key: "actions",
                  width: 136,
                  render: (_, row) => {
                    const actions: TableAction[] = [
                      {
                        label: t("common.edit"),
                        onClick: () => openTypeEditor(row),
                      },
                      {
                        danger: true,
                        disabled: !!row.isSystem,
                        label: t("common.delete"),
                        onClick: () => removeType(row),
                      },
                    ];
                    return (
                      <span onClick={(event) => event.stopPropagation()}>
                        <TableActionGroup actions={actions} />
                      </span>
                    );
                  },
                },
              ])}
            />
          </ListPagePanel>
        </Card>
        <Card className="split-detail table-card dictionary-table-card">
          <ListPagePanel
            title={t("system.settings.dict.itemList")}
            toolbar={{
              actions: selected && (
                <Button
                  type="primary"
                  icon={<Plus size={16} />}
                  onClick={() => openItemEditor(null)}
                >
                  {t("common.add")}
                </Button>
              ),
              onRefresh: () => void loadItems(),
            }}
          >
            <TablePanel
              className="list-page-table"
              bordered
              sticky
              scroll={{ x: "max-content" }}
              rowKey="id"
              dataSource={items}
              pagination={false}
              columns={normalizeListColumns<ResourceRecord>([
                { title: t("system.settings.dict.tag"), dataIndex: "label" },
                { title: t("system.settings.dict.value"), dataIndex: "value" },
                { title: t("common.sort"), dataIndex: "sort" },
                {
                  title: t("system.settings.dict.isDefault"),
                  dataIndex: "isDefault",
                  render: (v) =>
                    v ? (
                      <Tag color="blue">
                        {t("system.settings.common.default")}
                      </Tag>
                    ) : (
                      t("common.no")
                    ),
                },
                {
                  title: t("common.status"),
                  dataIndex: "status",
                  render: (v, row) => (
                    <Switch
                      checked={v === 1}
                      loading={statusChangingId === row.id}
                      onChange={(checked) =>
                        void changeItemStatus(row, checked)
                      }
                    />
                  ),
                },
                {
                  title: t("common.actions"),
                  key: "actions",
                  width: 136,
                  render: (_, row) => {
                    const actions: TableAction[] = [
                      {
                        label: t("common.edit"),
                        onClick: () => openItemEditor(row),
                      },
                      {
                        danger: true,
                        label: t("common.delete"),
                        onClick: () => removeItem(row),
                      },
                    ];
                    return <TableActionGroup actions={actions} />;
                  },
                },
              ])}
            />
          </ListPagePanel>
        </Card>
      </div>
      <Drawer
        className="form-drawer"
        closable={false}
        title={
          editingType
            ? t("system.settings.dict.editType")
            : t("system.settings.dict.createType")
        }
        open={typeOpen}
        size={typeMaximized ? "100vw" : 520}
        rootClassName={typeMaximized ? "form-drawer-maximized" : undefined}
        onClose={() => {
          setTypeMaximized(false);
          setTypeOpen(false);
        }}
        extra={
          <Button
            aria-label={
              typeMaximized
                ? t("system.settings.dict.restore")
                : t("common.fullscreen")
            }
            icon={
              typeMaximized ? <Minimize2 size={15} /> : <Maximize2 size={15} />
            }
            type="text"
            onClick={() => setTypeMaximized((value) => !value)}
          />
        }
        footer={
          <div className="form-drawer-footer">
            <Space>
              <Button onClick={() => setTypeOpen(false)}>
                {t("common.cancel")}
              </Button>
              <Button
                aria-label={t("common.save")}
                type="primary"
                onClick={() => void saveType()}
              >
                {t("common.save")}
              </Button>
            </Space>
          </div>
        }
      >
        <Form
          key={
            typeOpen
              ? editingType
                ? `edit-type-${editingType.id}`
                : "create-type"
              : "closed-type"
          }
          form={typeForm}
          layout="horizontal"
          {...dictionaryFormLayout}
          preserve={false}
          initialValues={
            editingType
              ? {
                  ...editingType,
                  displayType: editingType.displayType ?? "segmented",
                  status: Number(editingType.status ?? 1) === 1,
                }
              : { displayType: "segmented", status: true }
          }
        >
          <Form.Item
            name="name"
            label={t("system.settings.dict.typeName")}
            rules={[{ required: true, whitespace: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="code"
            label={t("system.settings.dict.typeCode")}
            rules={[{ required: true, whitespace: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="displayType"
            label={t("system.settings.dict.displayType")}
            rules={[{ required: true }]}
          >
            <Segmented
              className="dictionary-display-type-segmented"
              options={dictionaryDisplayTypeOptions()}
            />
          </Form.Item>
          <Form.Item
            name="status"
            label={t("common.status")}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item name="remark" label={t("common.remark")}>
            <Input.TextArea />
          </Form.Item>
        </Form>
      </Drawer>
      <Drawer
        className="form-drawer"
        closable={false}
        title={
          editingItem
            ? t("system.settings.dict.editItem")
            : t("system.settings.dict.createItem")
        }
        open={itemOpen}
        size={itemMaximized ? "100vw" : 520}
        rootClassName={itemMaximized ? "form-drawer-maximized" : undefined}
        onClose={() => {
          setItemMaximized(false);
          setItemOpen(false);
        }}
        extra={
          <Button
            aria-label={
              itemMaximized
                ? t("system.settings.dict.restore")
                : t("common.fullscreen")
            }
            icon={
              itemMaximized ? <Minimize2 size={15} /> : <Maximize2 size={15} />
            }
            type="text"
            onClick={() => setItemMaximized((value) => !value)}
          />
        }
        footer={
          <div className="form-drawer-footer">
            <Space>
              <Button onClick={() => setItemOpen(false)}>
                {t("common.cancel")}
              </Button>
              <Button
                aria-label={t("common.save")}
                type="primary"
                onClick={() => void saveItem()}
              >
                {t("common.save")}
              </Button>
            </Space>
          </div>
        }
      >
        <Form
          key={
            itemOpen
              ? editingItem
                ? `edit-item-${editingItem.id}`
                : "create-item"
              : "closed-item"
          }
          form={itemForm}
          layout="horizontal"
          {...dictionaryFormLayout}
          preserve={false}
          initialValues={
            editingItem
              ? {
                  ...editingItem,
                  status: Number(editingItem.status ?? 1) === 1,
                }
              : { status: true, sort: 0, isDefault: false }
          }
        >
          <Form.Item
            name="label"
            label={t("system.settings.dict.tag")}
            rules={[{ required: true, whitespace: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="value"
            label={t("system.settings.dict.value")}
            rules={[{ required: true, whitespace: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="sort" label={t("common.sort")}>
            <InputNumber className="full-width" min={0} precision={0} />
          </Form.Item>
          <Form.Item
            name="isDefault"
            label={t("system.settings.dict.isDefault")}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name="status"
            label={t("common.status")}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item name="remark" label={t("common.remark")}>
            <Input.TextArea />
          </Form.Item>
        </Form>
      </Drawer>
    </Page>
  );
}

export function ConfigPage() {
  const { t } = useTranslation();
  const [items, setItems] = useState<ResourceRecord[]>([]);
  const [values, setValues] = useState<Record<string, string>>({});
  const [group, setGroup] = useState<string>();
  const savedValuesRef = useRef<Record<string, string>>({});
  const load = useCallback(async () => {
    const data = await listConfigsApi(group);
    setItems(data);
    const next = Object.fromEntries(
      data.map((item) => [String(item.key), configValue(item)]),
    );
    setValues(next);
    savedValuesRef.current = next;
  }, [group]);
  useEffect(() => {
    void load();
  }, [load]);
  const saveConfig = useCallback(async (key: string, value: string) => {
    if ((savedValuesRef.current[key] ?? "") === value) return;
    await updateConfigApi(key, value);
    savedValuesRef.current = { ...savedValuesRef.current, [key]: value };
    void message.success(i18n.t("common.saveSuccess"));
  }, []);
  const renderValue = (row: ResourceRecord) => {
    const key = String(row.key);
    const value = values[key] ?? "";
    const setValue = (next: string) =>
      setValues((current) => ({ ...current, [key]: next }));
    const save = () => {
      if (isSecretConfig(row) && (!value || value === "********")) return;
      void saveConfig(key, value);
    };
    let control;
    switch (configType(row)) {
      case "number":
        control = (
          <NumberStepper
            className="config-value-stepper"
            label={String(row.name || row.key)}
            value={Number(value)}
            onChange={(next) => setValue(String(next))}
            onCommit={(next) => void saveConfig(key, String(next))}
            onBlur={save}
            onKeyDown={(event) => {
              if (event.key === "Enter") save();
            }}
          />
        );
        break;
      case "boolean":
        control = (
          <Switch
            checked={parseBooleanConfigValue(value)}
            onChange={(checked) => {
              setValue(String(checked));
              void saveConfig(key, String(checked));
            }}
          />
        );
        break;
      case "string":
      default:
        control = isSecretConfig(row) ? (
          <Input.Password
            className="config-value-input"
            visibilityToggle
            aria-label={String(row.name || row.key)}
            value={value}
            placeholder={t("system.settings.common.keepUnchanged")}
            onChange={(e) => setValue(e.target.value)}
            onPressEnter={save}
            onBlur={save}
          />
        ) : (
          <Input
            className="config-value-input"
            aria-label={String(row.name || row.key)}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onPressEnter={save}
            onBlur={save}
          />
        );
    }
    return <div className="config-value-cell">{control}</div>;
  };
  return (
    <Page title={t("system.settings.config.title")}>
      <Card className="table-card">
        <ListPagePanel
          title={t("system.settings.config.list")}
          toolbar={{
            actions: (
              <Select
                allowClear
                placeholder={t("system.settings.config.allGroups")}
                style={{ minWidth: 160 }}
                value={group}
                onChange={setGroup}
                options={[...new Set(items.map((i) => String(i.group)))].map(
                  (v) => ({ label: v, value: v }),
                )}
              />
            ),
            onRefresh: () => void load(),
          }}
        >
          <TablePanel
            className="list-page-table"
            bordered
            sticky
            scroll={{ x: "max-content" }}
            rowKey="id"
            pagination={false}
            dataSource={items}
            columns={normalizeListColumns<ResourceRecord>([
              { title: t("system.settings.config.group"), dataIndex: "group" },
              { title: t("common.name"), dataIndex: "name" },
              { title: t("system.settings.config.key"), dataIndex: "key" },
              {
                title: t("system.settings.config.value"),
                width: 220,
                ellipsis: false,
                render: (_, row) => renderValue(row),
              },
            ])}
          />
        </ListPagePanel>
      </Card>
    </Page>
  );
}

export const AuthProviderPage = () => {
  const { t } = useTranslation();
  return (
    <ResourceCrudPage
      title={t("system.settings.authProvider.title")}
      endpoint="/system/auth-providers"
      createCode="system:config:auth-provider"
      updateCode="system:config:auth-provider"
      deleteCode="system:config:auth-provider"
      columns={[
        { title: t("common.name"), dataIndex: "name" },
        { title: t("common.type"), dataIndex: "type" },
        {
          title: t("system.settings.authProvider.clientId"),
          dataIndex: "clientId",
        },
        { title: t("common.status"), dataIndex: "status" },
      ]}
      fields={[
        { key: "name", label: t("common.name"), required: true },
        { key: "type", label: t("common.type"), required: true },
        {
          key: "clientId",
          label: t("system.settings.authProvider.clientId"),
          required: true,
        },
        {
          key: "clientSecret",
          label: t("system.settings.authProvider.clientSecret"),
          type: "password",
          placeholder: t("system.settings.common.keepUnchanged"),
        },
        {
          key: "authorizeUrl",
          label: t("system.settings.authProvider.authorizeUrl"),
        },
        { key: "tokenUrl", label: t("system.settings.authProvider.tokenUrl") },
        {
          key: "userInfoUrl",
          label: t("system.settings.authProvider.userInfoUrl"),
        },
        { key: "scopes", label: t("system.security.scopesLabel") },
        {
          key: "status",
          label: t("common.enabled"),
          type: "switch",
          numeric: true,
        },
      ]}
    />
  );
};

const BYTES_PER_MB = 1024 * 1024;

function bytesToMb(bytes: unknown) {
  const size = Number(bytes);
  if (!Number.isFinite(size) || size <= 0) return 0;
  return Math.max(1, Math.ceil(size / BYTES_PER_MB));
}

function requireWhenStorageType(type: "local" | "s3", label: string) {
  return ({ getFieldValue }: { getFieldValue: (name: string) => unknown }) => [
    {
      validator: async (_: unknown, value: unknown) => {
        if (getFieldValue("storageType") !== type) return;
        if (value == null || String(value).trim() === "") {
          throw new Error(
            i18n.t("system.settings.validation.required", { field: label }),
          );
        }
      },
    },
  ];
}

export function StorageConfigPage() {
  const { t } = useTranslation();
  const storageTypeLabels: Record<string, string> = {
    local: t("system.settings.storage.local"),
    s3: "S3 / MinIO",
  };
  return (
    <ResourceCrudPage
      title={t("system.settings.storage.title")}
      endpoint="/system/storage-configs"
      createCode="system:config:storage"
      updateCode="system:config:storage"
      deleteCode="system:config:storage"
      paged={false}
      searchPlaceholder={t("system.settings.storage.searchPlaceholder")}
      mapRecordValues={(values) => ({
        ...values,
        maxFileSize: bytesToMb(values.maxFileSize),
      })}
      mapSubmitValues={(values) => ({
        ...values,
        maxFileSize: Number(values.maxFileSize || 0) * BYTES_PER_MB,
      })}
      defaultValues={{
        storageType: "local",
        endpoint: "",
        bucket: "",
        region: "",
        basePath: "./data/uploads",
        publicBaseUrl: "",
        useHttps: true,
        privateAccess: true,
        maxFileSize: 100,
        allowedExtensions: "",
        isDefault: false,
        status: 1,
      }}
      columns={[
        {
          title: t("common.name"),
          dataIndex: "name",
          width: 160,
          align: "left",
        },
        {
          title: t("common.type"),
          dataIndex: "storageType",
          width: 120,
          labelMap: storageTypeLabels,
        },
        {
          title: t("system.settings.storage.target"),
          dataIndex: "endpoint",
          width: 220,
          align: "left",
          render: (_value, record) => {
            const type = String(record.storageType || "");
            if (type === "local") return String(record.basePath || "—");
            const endpoint = String(record.endpoint || "");
            const bucket = String(record.bucket || "");
            return [endpoint, bucket].filter(Boolean).join(" / ") || "—";
          },
        },
        {
          title: t("system.settings.storage.maxFile"),
          dataIndex: "maxFileSize",
          width: 110,
          render: (value) => {
            const mb = bytesToMb(value);
            return mb > 0 ? `${mb} MB` : "—";
          },
        },
        {
          title: t("system.settings.common.default"),
          dataIndex: "isDefault",
          width: 90,
          render: (value) =>
            value === true || value === 1 ? (
              <Tag color="blue">{t("system.settings.common.default")}</Tag>
            ) : (
              "—"
            ),
        },
        {
          title: t("common.status"),
          dataIndex: "status",
          width: 90,
          render: (value) => (
            <Tag color={Number(value) === 1 ? "success" : "default"}>
              {Number(value) === 1
                ? t("common.enabled")
                : t("common.disabled")}
            </Tag>
          ),
        },
      ]}
      fields={[
        { key: "name", label: t("common.name"), required: true },
        {
          key: "storageType",
          label: t("system.settings.storage.storageType"),
          type: "select",
          required: true,
          options: [
            { label: t("system.settings.storage.local"), value: "local" },
            { label: "S3 / MinIO", value: "s3" },
          ],
        },
        {
          key: "basePath",
          label: t("system.settings.storage.path"),
          dependencies: ["storageType"],
          rules: requireWhenStorageType(
            "local",
            t("system.settings.storage.path"),
          ),
          placeholder: t("system.settings.storage.pathPlaceholder"),
        },
        {
          key: "endpoint",
          label: t("system.settings.storage.endpoint"),
          dependencies: ["storageType"],
          visibleWhen: (get) => get("storageType") === "s3",
          rules: requireWhenStorageType(
            "s3",
            t("system.settings.storage.endpoint"),
          ),
          placeholder: t("system.settings.storage.endpointPlaceholder"),
        },
        {
          key: "bucket",
          label: t("system.settings.storage.bucket"),
          dependencies: ["storageType"],
          visibleWhen: (get) => get("storageType") === "s3",
          rules: requireWhenStorageType(
            "s3",
            t("system.settings.storage.bucket"),
          ),
        },
        {
          key: "region",
          label: t("system.settings.storage.region"),
          dependencies: ["storageType"],
          visibleWhen: (get) => get("storageType") === "s3",
          placeholder: t("system.settings.storage.optional"),
        },
        {
          key: "accessKey",
          label: t("system.settings.storage.accessKey"),
          type: "password",
          dependencies: ["storageType"],
          visibleWhen: (get) => get("storageType") === "s3",
          placeholder: t("system.settings.common.keepUnchanged"),
          rules: ({ editing, getFieldValue }) => [
            {
              validator: async (_rule, value) => {
                if (getFieldValue("storageType") !== "s3") return;
                if (!editing && (!value || String(value).trim() === "")) {
                  throw new Error(
                    t("system.settings.validation.accessKeyRequired"),
                  );
                }
              },
            },
          ],
        },
        {
          key: "secretKey",
          label: t("system.settings.storage.secretKey"),
          type: "password",
          dependencies: ["storageType"],
          visibleWhen: (get) => get("storageType") === "s3",
          placeholder: t("system.settings.common.keepUnchanged"),
          rules: ({ editing, getFieldValue }) => [
            {
              validator: async (_rule, value) => {
                if (getFieldValue("storageType") !== "s3") return;
                if (!editing && (!value || String(value).trim() === "")) {
                  throw new Error(
                    t("system.settings.validation.secretKeyRequired"),
                  );
                }
              },
            },
          ],
        },
        {
          key: "publicBaseUrl",
          label: t("system.settings.storage.publicBaseUrl"),
          placeholder: "https://cdn.example.com",
        },
        {
          key: "useHttps",
          label: t("system.settings.storage.useHttps"),
          type: "switch",
        },
        {
          key: "privateAccess",
          label: t("system.settings.storage.privateAccess"),
          type: "switch",
        },
        {
          key: "maxFileSize",
          label: t("system.settings.storage.maxFileSizeMb"),
          type: "number",
          required: true,
        },
        {
          key: "allowedExtensions",
          label: t("system.settings.storage.allowedExtensions"),
          placeholder: t(
            "system.settings.storage.allowedExtensionsPlaceholder",
          ),
        },
        {
          key: "isDefault",
          label: t("system.settings.storage.setDefault"),
          type: "switch",
        },
        {
          key: "status",
          label: t("common.enabled"),
          type: "switch",
          numeric: true,
        },
      ]}
      rowActions={[
        {
          label: t("system.settings.storage.testConnection"),
          reload: false,
          onClick: async (row) => {
            await testStorageConfigApi(row.id);
            void message.success(t("system.settings.storage.testSuccess"));
          },
        },
      ]}
    />
  );
}
