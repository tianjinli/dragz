import { useCallback, useEffect, useState } from "react";
import { Button, Card, Form, Input, InputNumber, Space, Switch, Tag, message } from "antd";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  createPlan,
  listPlans,
  updatePlan,
  updatePlanStatus,
  type PlanEntitlement,
  type PlatformPlan,
} from "@/api/platform";
import { FormDrawer } from "@/components/FormDrawer";
import { ListPagePanel } from "@/components/ListPagePanel";
import { Page } from "@/components/Page";
import { TableActionGroup, actionColumnWidth } from "@/components/TableActionGroup";
import { TablePanel } from "@/components/TablePanel";
import { useAccessStore } from "@/stores/access";
import { normalizeListColumns } from "@/utils/table-columns";
import { SuperAdminDenied } from "./super-admin";

interface EntitlementValue {
  key: string;
  value: string;
}

interface PlanFormValues {
  code: string;
  name: string;
  description?: string;
  status: number;
  sort: number;
  entitlements?: EntitlementValue[];
}

const CODE_PATTERN = /^[A-Za-z0-9_-]+$/;
const ENTITLEMENT_KEY = /^[A-Za-z0-9._-]+$/;

export function PlatformPlansPage() {
  const isSuperAdmin = useAccessStore((state) => !!state.userInfo?.isSuperAdmin);
  if (!isSuperAdmin) return <SuperAdminDenied />;
  return <PlanCatalog />;
}

function PlanCatalog() {
  const { t } = useTranslation();
  const [items, setItems] = useState<PlatformPlan[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [pending, setPending] = useState<string>();
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<PlatformPlan>();
  const [createForm] = Form.useForm<PlanFormValues>();
  const [editForm] = Form.useForm<PlanFormValues>();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setItems(await listPlans());
    } catch {
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const toggleStatus = async (row: PlatformPlan, next: boolean) => {
    const key = `${row.id}:status`;
    setPending(key);
    try {
      await updatePlanStatus(row.id, next ? 1 : 0);
      void message.success(t("platform.statusUpdated"));
      await load();
    } catch {
      await load();
    } finally {
      setPending(undefined);
    }
  };

  const submitCreate = async () => {
    let values: PlanFormValues;
    try {
      values = await createForm.validateFields();
    } catch {
      return;
    }
    setSaving(true);
    try {
      await createPlan({
        code: values.code.trim(),
        name: values.name.trim(),
        description: values.description?.trim() ?? "",
        status: values.status,
        sort: values.sort,
        entitlements: cleanEntitlements(values.entitlements),
      });
      void message.success(t("platform.planCreated"));
      setCreateOpen(false);
      await load();
    } catch {
      /* interceptor surfaces the message */
    } finally {
      setSaving(false);
    }
  };

  const submitEdit = async () => {
    if (!editing) return;
    let values: PlanFormValues;
    try {
      values = await editForm.validateFields();
    } catch {
      return;
    }
    setSaving(true);
    try {
      await updatePlan(editing.id, {
        code: editing.code,
        name: values.name.trim(),
        description: values.description?.trim() ?? "",
        status: values.status,
        sort: values.sort,
        entitlements: cleanEntitlements(values.entitlements),
      });
      void message.success(t("platform.planUpdated"));
      setEditing(undefined);
      await load();
    } catch {
      /* interceptor surfaces the message */
    } finally {
      setSaving(false);
    }
  };

  return (
    <Page title={t("platform.plansTitle")}>
      <Card className="table-card fit-list">
        <ListPagePanel
          title={t("platform.plansList")}
          toolbar={{
            onRefresh: () => void load(),
            actions: (
              <Button type="primary" icon={<Plus size={16} />} onClick={() => setCreateOpen(true)}>
                {t("common.add")}
              </Button>
            ),
          }}
        >
          <TablePanel
            className="list-page-table"
            bordered
            sticky
            scroll={{ x: "max-content" }}
            rowKey="id"
            loading={loading}
            pagination={false}
            dataSource={items}
            columns={normalizeListColumns<PlatformPlan>([
              { title: t("platform.id"), dataIndex: "id", width: 80 },
              { title: t("platform.code"), dataIndex: "code", width: 140 },
              { title: t("common.name"), dataIndex: "name", width: 180 },
              { title: t("platform.description"), dataIndex: "description" },
              {
                title: t("common.status"),
                dataIndex: "status",
                width: 120,
                ellipsis: false,
                render: (value: number, row: PlatformPlan) => (
                  <Switch
                    checked={value === 1}
                    loading={pending === `${row.id}:status`}
                    disabled={pending === `${row.id}:status`}
                    onChange={(next) => void toggleStatus(row, next)}
                  />
                ),
              },
              { title: t("platform.sort"), dataIndex: "sort", width: 90 },
              {
                title: t("platform.entitlements"),
                dataIndex: "entitlements",
                render: (value: PlanEntitlement[] | null) => {
                  const rows = value ?? [];
                  if (rows.length === 0) return "-";
                  return rows.map((item) => (
                    <Tag key={item.key}>
                      {item.key}={item.value}
                    </Tag>
                  ));
                },
              },
              {
                title: t("common.actions"),
                key: "actions",
                width: actionColumnWidth(1),
                ellipsis: false,
                render: (_: unknown, row: PlatformPlan) => (
                  <TableActionGroup
                    actions={[
                      {
                        icon: <Pencil size={16} />,
                        label: t("common.edit"),
                        onClick: () => setEditing(row),
                      },
                    ]}
                  />
                ),
              },
            ])}
          />
        </ListPagePanel>
      </Card>

      <FormDrawer
        confirmLoading={saving}
        form={createForm}
        open={createOpen}
        title={t("platform.createPlan")}
        onClose={() => setCreateOpen(false)}
        onOk={() => void submitCreate()}
      >
        <Form
          key={createOpen ? "create-open" : "create-closed"}
          form={createForm}
          layout="vertical"
          preserve={false}
          initialValues={{ status: 1, sort: 0, entitlements: [] }}
        >
          <PlanFields includeCode />
        </Form>
      </FormDrawer>

      <FormDrawer
        confirmLoading={saving}
        form={editForm}
        open={!!editing}
        title={t("platform.editPlan")}
        onClose={() => setEditing(undefined)}
        onOk={() => void submitEdit()}
      >
        <Form
          key={editing ? `edit-${editing.id}` : "edit-closed"}
          form={editForm}
          layout="vertical"
          preserve={false}
          initialValues={{
            name: editing?.name,
            description: editing?.description ?? "",
            status: editing?.status ?? 1,
            sort: editing?.sort ?? 0,
            entitlements: editing?.entitlements ?? [],
          }}
        >
          <PlanFields />
        </Form>
      </FormDrawer>
    </Page>
  );
}

function cleanEntitlements(items: EntitlementValue[] | undefined): PlanEntitlement[] {
  const rows = items ?? [];
  const seen = new Map<string, string>();
  const order: string[] = [];
  for (const item of rows) {
    const key = item.key.trim();
    if (!key) continue;
    if (!seen.has(key)) order.push(key);
    seen.set(key, (item.value ?? "").trim());
  }
  return order.map((key) => ({ key, value: seen.get(key) ?? "" }));
}

function PlanFields({ includeCode = false }: { includeCode?: boolean }) {
  const { t } = useTranslation();
  return (
    <>
      {includeCode ? (
        <Form.Item
          name="code"
          label={t("platform.code")}
          rules={[
            { required: true, whitespace: true, message: t("platform.planCodeRequired") },
            { max: 64, message: t("platform.codeInvalid") },
            { pattern: CODE_PATTERN, message: t("platform.codeInvalid") },
          ]}
        >
          <Input maxLength={64} />
        </Form.Item>
      ) : null}
      <Form.Item
        name="name"
        label={t("common.name")}
        rules={[
          { required: true, whitespace: true, message: t("platform.nameRequired") },
          { max: 128, message: t("platform.nameRequired") },
        ]}
      >
        <Input maxLength={128} />
      </Form.Item>
      <Form.Item
        name="description"
        label={t("platform.description")}
        rules={[{ max: 500, message: t("platform.descriptionTooLong") }]}
      >
        <Input.TextArea maxLength={500} rows={3} showCount />
      </Form.Item>
      <Form.Item
        name="status"
        label={t("common.status")}
        valuePropName="checked"
        getValueFromEvent={(checked: boolean) => (checked ? 1 : 0)}
        getValueProps={(value: number) => ({ checked: value === 1 })}
      >
        <Switch />
      </Form.Item>
      <Form.Item
        name="sort"
        label={t("platform.sort")}
        rules={[
          { required: true, message: t("platform.sortRequired") },
          { type: "number", min: -1000000, max: 1000000, message: t("platform.sortRequired") },
        ]}
      >
        <InputNumber className="full-width" precision={0} />
      </Form.Item>
      <EntitlementFields />
    </>
  );
}

function EntitlementFields() {
  const { t } = useTranslation();
  return (
    <Form.Item label={t("platform.entitlements")} style={{ marginBottom: 0 }}>
      <Form.List name="entitlements">
        {(fields, { add, remove }) => (
          <Space direction="vertical" style={{ width: "100%" }}>
            {fields.map(({ key, name, ...restField }) => (
              <Space key={key} align="baseline" style={{ display: "flex" }}>
                <Form.Item
                  {...restField}
                  name={[name, "key"]}
                  rules={[
                    { required: true, whitespace: true, message: t("platform.entitlementKeyRequired") },
                    { max: 128, message: t("platform.entitlementKeyInvalid") },
                    { pattern: ENTITLEMENT_KEY, message: t("platform.entitlementKeyInvalid") },
                  ]}
                >
                  <Input maxLength={128} placeholder={t("platform.entitlementKey")} />
                </Form.Item>
                <Form.Item
                  {...restField}
                  name={[name, "value"]}
                  rules={[{ max: 2000, message: t("platform.entitlementValueInvalid") }]}
                >
                  <Input maxLength={2000} placeholder={t("platform.entitlementValue")} />
                </Form.Item>
                <Button
                  aria-label={t("platform.removeEntitlement")}
                  icon={<Trash2 size={16} />}
                  type="text"
                  onClick={() => remove(name)}
                />
              </Space>
            ))}
            <Button type="dashed" icon={<Plus size={16} />} onClick={() => add({ key: "", value: "" })}>
              {t("platform.addEntitlement")}
            </Button>
          </Space>
        )}
      </Form.List>
    </Form.Item>
  );
}
