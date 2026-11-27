import { useCallback, useEffect, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from "antd";
import { CreditCard, Download, Globe, KeyRound, Plus, TimerReset, Undo2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  addTenantDomain,
  cancelTenantPurge,
  createTenant,
  exportTenant,
  impersonateTenant,
  listPlans,
  listTenantDomains,
  listTenants,
  removeTenantDomain,
  scheduleTenantPurge,
  updateTenantSlug,
  updateTenantStatus,
  updateTenantSubscription,
  verifyTenantDomain,
  type PlatformPlan,
  type PlatformTenant,
  type TenantDomain,
} from "@/api/platform";
import { FormDrawer } from "@/components/FormDrawer";
import { ListPagePanel } from "@/components/ListPagePanel";
import { Page } from "@/components/Page";
import { TableActionGroup, actionColumnWidth } from "@/components/TableActionGroup";
import { TablePanel } from "@/components/TablePanel";
import { useAccessStore } from "@/stores/access";
import { normalizeListColumns } from "@/utils/table-columns";
import { formatDateTime } from "@/utils/format-time";
import { SuperAdminDenied } from "./super-admin";

interface CreateValues {
  code: string;
  name: string;
}

interface SubscriptionValues {
  planId: number;
  status: string;
}

interface ImpersonateValues {
  hours: number;
  reason: string;
}

interface PurgeValues {
  days: number;
}

const CODE_PATTERN = /^[A-Za-z0-9_-]+$/;
const SLUG_PATTERN = /^[a-z][a-z0-9-]{1,62}$/;
const RESERVED_SLUGS = new Set(["www", "api", "admin", "app", "static"]);

export function PlatformTenantsPage() {
  const isSuperAdmin = useAccessStore((state) => !!state.userInfo?.isSuperAdmin);
  if (!isSuperAdmin) return <SuperAdminDenied />;
  return <TenantConsole />;
}

function TenantConsole() {
  const { t } = useTranslation();
  const [items, setItems] = useState<PlatformTenant[]>([]);
  const [plans, setPlans] = useState<PlatformPlan[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [pending, setPending] = useState<string>();
  const [createOpen, setCreateOpen] = useState(false);
  const [subscriptionTarget, setSubscriptionTarget] = useState<PlatformTenant>();
  const [impersonateTarget, setImpersonateTarget] = useState<PlatformTenant>();
  const [purgeTarget, setPurgeTarget] = useState<PlatformTenant>();
  const [domainTarget, setDomainTarget] = useState<PlatformTenant>();
  const [domains, setDomains] = useState<TenantDomain[]>([]);
  const [domainLoading, setDomainLoading] = useState(false);
  const [hostDraft, setHostDraft] = useState("");
  const [accessToken, setAccessToken] = useState<string>();
  const [createForm] = Form.useForm<CreateValues>();
  const [subscriptionForm] = Form.useForm<SubscriptionValues>();
  const [impersonateForm] = Form.useForm<ImpersonateValues>();
  const [purgeForm] = Form.useForm<PurgeValues>();
  const [slugForm] = Form.useForm<{ slug: string }>();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [tenantRows, planRows] = await Promise.all([listTenants(), listPlans()]);
      setItems(tenantRows);
      setPlans(planRows);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const toggleStatus = async (row: PlatformTenant, next: boolean) => {
    const key = `${row.id}:status`;
    setPending(key);
    try {
      await updateTenantStatus(row.id, next ? 1 : 0);
      void message.success(t("platform.statusUpdated"));
      await load();
    } catch {
      await load();
    } finally {
      setPending(undefined);
    }
  };

  const submitCreate = async () => {
    let values: CreateValues;
    try {
      values = await createForm.validateFields();
    } catch {
      return;
    }
    setSaving(true);
    try {
      await createTenant({ code: values.code.trim(), name: values.name.trim() });
      void message.success(t("platform.created"));
      setCreateOpen(false);
      await load();
    } catch {
      /* interceptor surfaces the message */
    } finally {
      setSaving(false);
    }
  };

  const submitSubscription = async () => {
    if (!subscriptionTarget) return;
    let values: SubscriptionValues;
    try {
      values = await subscriptionForm.validateFields();
    } catch {
      return;
    }
    setSaving(true);
    try {
      await updateTenantSubscription(subscriptionTarget.id, {
        planId: values.planId,
        status: values.status,
      });
      void message.success(t("platform.subscriptionUpdated"));
      setSubscriptionTarget(undefined);
      await load();
    } catch {
      /* interceptor surfaces the message */
    } finally {
      setSaving(false);
    }
  };

  const submitImpersonate = async () => {
    if (!impersonateTarget) return;
    let values: ImpersonateValues;
    try {
      values = await impersonateForm.validateFields();
    } catch {
      return;
    }
    setSaving(true);
    try {
      const issued = await impersonateTenant(impersonateTarget.id, {
        reason: values.reason.trim(),
        hours: values.hours,
      });
      setImpersonateTarget(undefined);
      setAccessToken(issued.accessToken);
    } catch {
      /* interceptor surfaces the message */
    } finally {
      setSaving(false);
    }
  };

  const downloadExport = async (row: PlatformTenant) => {
    const key = `${row.id}:export`;
    setPending(key);
    try {
      const snapshot = await exportTenant(row.id);
      const blob = new Blob([JSON.stringify(snapshot, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = `${row.code || row.id}-export.json`;
      anchor.click();
      URL.revokeObjectURL(url);
      void message.success(t("platform.exported"));
    } catch {
      /* interceptor surfaces the message */
    } finally {
      setPending(undefined);
    }
  };

  const submitPurge = async () => {
    if (!purgeTarget) return;
    let values: PurgeValues;
    try {
      values = await purgeForm.validateFields();
    } catch {
      return;
    }
    setSaving(true);
    try {
      await scheduleTenantPurge(purgeTarget.id, values.days);
      void message.success(t("platform.purgeScheduled"));
      setPurgeTarget(undefined);
      await load();
    } catch {
      /* interceptor surfaces the message */
    } finally {
      setSaving(false);
    }
  };

  const openDomains = async (row: PlatformTenant) => {
    setDomainTarget(row);
    setHostDraft("");
    setDomainLoading(true);
    try {
      setDomains(await listTenantDomains(row.id));
    } catch {
      setDomains([]);
    } finally {
      setDomainLoading(false);
    }
  };

  const refreshDomains = async (tenantId: number) => {
    setDomains(await listTenantDomains(tenantId));
  };

  const submitSlug = async () => {
    if (!domainTarget) return;
    let values: { slug: string };
    try {
      values = await slugForm.validateFields();
    } catch {
      return;
    }
    setSaving(true);
    try {
      await updateTenantSlug(domainTarget.id, values.slug.trim());
      void message.success(t("platform.slugUpdated"));
      setDomainTarget(undefined);
      await load();
    } catch {
      /* interceptor surfaces the message */
    } finally {
      setSaving(false);
    }
  };

  const submitDomain = async () => {
    if (!domainTarget) return;
    const host = hostDraft.trim();
    if (!host) return;
    setSaving(true);
    try {
      await addTenantDomain(domainTarget.id, host);
      void message.success(t("platform.domainAdded"));
      setHostDraft("");
      await refreshDomains(domainTarget.id);
    } catch {
      /* interceptor surfaces the message */
    } finally {
      setSaving(false);
    }
  };

  const markDomainVerified = async (domainId: number) => {
    if (!domainTarget) return;
    setSaving(true);
    try {
      await verifyTenantDomain(domainTarget.id, domainId);
      void message.success(t("platform.domainVerified"));
      await refreshDomains(domainTarget.id);
    } catch {
      /* interceptor surfaces the message */
    } finally {
      setSaving(false);
    }
  };

  const deleteDomain = async (domainId: number) => {
    if (!domainTarget) return;
    setSaving(true);
    try {
      await removeTenantDomain(domainTarget.id, domainId);
      void message.success(t("platform.domainRemoved"));
      await refreshDomains(domainTarget.id);
    } catch {
      /* interceptor surfaces the message */
    } finally {
      setSaving(false);
    }
  };

  const confirmCancelPurge = (row: PlatformTenant) => {
    Modal.confirm({
      title: t("platform.cancelPurge"),
      content: t("platform.cancelPurgeConfirm", { name: row.name }),
      okText: t("common.confirm"),
      cancelText: t("common.cancel"),
      onOk: async () => {
        await cancelTenantPurge(row.id);
        void message.success(t("platform.purgeCancelled"));
        await load();
      },
    });
  };

  return (
    <Page title={t("platform.tenantsTitle")}>
      <Card className="table-card fit-list">
        <ListPagePanel
          title={t("platform.tenantsList")}
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
            columns={normalizeListColumns<PlatformTenant>([
              { title: t("platform.id"), dataIndex: "id", width: 80 },
              { title: t("platform.code"), dataIndex: "code", width: 160 },
              { title: t("platform.slug"), dataIndex: "slug", width: 160 },
              { title: t("common.name"), dataIndex: "name", width: 200 },
              {
                title: t("common.status"),
                dataIndex: "status",
                width: 120,
                ellipsis: false,
                render: (value: number, row: PlatformTenant) => (
                  <Switch
                    checked={value === 1}
                    loading={pending === `${row.id}:status`}
                    disabled={pending === `${row.id}:status`}
                    onChange={(next) => void toggleStatus(row, next)}
                  />
                ),
              },
              {
                title: t("platform.purgeAfter"),
                dataIndex: "purgeAfter",
                width: 200,
                render: (_: unknown, row: PlatformTenant) => {
                  if (!row.purgeAfter && !row.purgeStatus) return "—";
                  const when = row.purgeAfter ? formatDateTime(row.purgeAfter) : "";
                  const label = row.purgeStatus === "pending_delete" ? t("platform.purgePending") : row.purgeStatus;
                  return [label, when].filter(Boolean).join(" ");
                },
              },
              {
                title: t("common.actions"),
                key: "actions",
                width: actionColumnWidth(4),
                ellipsis: false,
                render: (_: unknown, row: PlatformTenant) => (
                  <TableActionGroup
                    actions={[
                      {
                        icon: <CreditCard size={16} />,
                        label: t("platform.subscription"),
                        onClick: () => setSubscriptionTarget(row),
                      },
                      {
                        icon: <KeyRound size={16} />,
                        label: t("platform.impersonate"),
                        onClick: () => setImpersonateTarget(row),
                      },
                      {
                        icon: <Globe size={16} />,
                        label: t("platform.domains"),
                        inMore: true,
                        onClick: () => void openDomains(row),
                      },
                      {
                        icon: <Download size={16} />,
                        label: t("platform.export"),
                        inMore: true,
                        disabled: pending === `${row.id}:export`,
                        onClick: () => void downloadExport(row),
                      },
                      {
                        icon: <TimerReset size={16} />,
                        label: t("platform.schedulePurge"),
                        inMore: true,
                        disabled: row.id === 1,
                        onClick: () => setPurgeTarget(row),
                      },
                      {
                        icon: <Undo2 size={16} />,
                        label: t("platform.cancelPurge"),
                        inMore: true,
                        disabled: row.id === 1 || row.purgeStatus !== "pending_delete",
                        onClick: () => confirmCancelPurge(row),
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
        title={t("platform.createTenant")}
        onClose={() => setCreateOpen(false)}
        onOk={() => void submitCreate()}
      >
        <Form
          key={createOpen ? "create-open" : "create-closed"}
          form={createForm}
          layout="vertical"
          preserve={false}
        >
          <Form.Item
            name="code"
            label={t("platform.code")}
            rules={[
              { required: true, whitespace: true, message: t("platform.codeRequired") },
              { max: 64, message: t("platform.codeInvalid") },
              { pattern: CODE_PATTERN, message: t("platform.codeInvalid") },
            ]}
          >
            <Input maxLength={64} />
          </Form.Item>
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
        </Form>
      </FormDrawer>

      <FormDrawer
        confirmLoading={saving}
        form={subscriptionForm}
        open={!!subscriptionTarget}
        title={t("platform.subscriptionFor", { name: subscriptionTarget?.name ?? "" })}
        onClose={() => setSubscriptionTarget(undefined)}
        onOk={() => void submitSubscription()}
      >
        <Form
          key={subscriptionTarget ? `subscription-${subscriptionTarget.id}` : "subscription-closed"}
          form={subscriptionForm}
          layout="vertical"
          preserve={false}
          initialValues={{ status: "active", planId: plans[0]?.id }}
        >
          <Form.Item
            name="planId"
            label={t("platform.plan")}
            rules={[{ required: true, message: t("platform.planRequired") }]}
          >
            <Select
              options={plans.map((plan) => ({
                label: `${plan.name} (${plan.code})`,
                value: plan.id,
              }))}
              placeholder={t("common.pleaseSelect")}
            />
          </Form.Item>
          <Form.Item
            name="status"
            label={t("platform.subscriptionStatus")}
            extra={t("platform.subscriptionHint")}
            rules={[{ required: true, message: t("platform.subscriptionStatus") }]}
          >
            <Select
              options={[
                { label: t("platform.statusActive"), value: "active" },
                { label: t("platform.statusTrial"), value: "trial" },
                { label: t("platform.statusOverdue"), value: "overdue" },
                { label: t("platform.statusCancelled"), value: "cancelled" },
              ]}
            />
          </Form.Item>
        </Form>
      </FormDrawer>

      <FormDrawer
        confirmLoading={saving}
        form={impersonateForm}
        open={!!impersonateTarget}
        title={t("platform.impersonateFor", { name: impersonateTarget?.name ?? "" })}
        onClose={() => setImpersonateTarget(undefined)}
        onOk={() => void submitImpersonate()}
      >
        <Form
          key={impersonateTarget ? `impersonate-${impersonateTarget.id}` : "impersonate-closed"}
          form={impersonateForm}
          layout="vertical"
          preserve={false}
          initialValues={{ hours: 1 }}
        >
          <Form.Item
            name="reason"
            label={t("platform.reason")}
            rules={[
              { required: true, whitespace: true, message: t("platform.reasonRequired") },
              { max: 500, message: t("platform.reasonRequired") },
            ]}
          >
            <Input.TextArea maxLength={500} rows={3} />
          </Form.Item>
          <Form.Item
            name="hours"
            label={t("platform.hours")}
            rules={[
              { required: true, message: t("platform.hoursRange") },
              { type: "number", min: 1, max: 24, message: t("platform.hoursRange") },
            ]}
          >
            <InputNumber className="full-width" min={1} max={24} precision={0} />
          </Form.Item>
        </Form>
      </FormDrawer>

      <FormDrawer
        confirmLoading={saving}
        form={purgeForm}
        open={!!purgeTarget}
        title={t("platform.schedulePurgeFor", { name: purgeTarget?.name ?? "" })}
        onClose={() => setPurgeTarget(undefined)}
        onOk={() => void submitPurge()}
      >
        <Form
          key={purgeTarget ? `purge-${purgeTarget.id}` : "purge-closed"}
          form={purgeForm}
          layout="vertical"
          preserve={false}
          initialValues={{ days: 7 }}
        >
          <Form.Item
            name="days"
            label={t("platform.purgeDays")}
            rules={[
              { required: true, message: t("platform.purgeDaysRange") },
              { type: "number", min: 1, max: 3650, message: t("platform.purgeDaysRange") },
            ]}
          >
            <InputNumber className="full-width" min={1} max={3650} precision={0} />
          </Form.Item>
        </Form>
      </FormDrawer>

      <FormDrawer
        confirmLoading={saving}
        form={slugForm}
        open={!!domainTarget}
        title={t("platform.domainsFor", { name: domainTarget?.name ?? "" })}
        onClose={() => setDomainTarget(undefined)}
        onOk={() => void submitSlug()}
      >
        <Form
          key={domainTarget ? `domains-${domainTarget.id}` : "domains-closed"}
          form={slugForm}
          initialValues={{ slug: domainTarget?.slug ?? "" }}
          layout="vertical"
          preserve={false}
        >
          <Form.Item
            name="slug"
            label={t("platform.slug")}
            rules={[
              { required: true, whitespace: true, message: t("platform.slugInvalid") },
              {
                validator: async (_, value: string) => {
                  const slug = (value ?? "").trim();
                  if (!SLUG_PATTERN.test(slug) || RESERVED_SLUGS.has(slug)) {
                    throw new Error(t("platform.slugInvalid"));
                  }
                },
              },
            ]}
          >
            <Input maxLength={63} />
          </Form.Item>
        </Form>
        <Space.Compact className="full-width" style={{ marginBottom: 16 }}>
          <Input
            maxLength={255}
            placeholder={t("platform.hostPlaceholder")}
            value={hostDraft}
            onChange={(event) => setHostDraft(event.target.value)}
            onPressEnter={() => void submitDomain()}
          />
          <Button disabled={!hostDraft.trim() || saving} onClick={() => void submitDomain()}>
            {t("platform.addDomain")}
          </Button>
        </Space.Compact>
        <Table<TenantDomain>
          className="fit-list"
          columns={[
            { title: t("platform.host"), dataIndex: "host" },
            {
              title: t("common.status"),
              dataIndex: "verifiedAt",
              width: 120,
              render: (value: string | null | undefined) =>
                value ? <Tag color="success">{t("platform.verified")}</Tag> : <Tag>{t("platform.unverified")}</Tag>,
            },
            {
              title: t("common.actions"),
              key: "actions",
              width: 200,
              render: (_: unknown, row: TenantDomain) => (
                <Space>
                  <Button disabled={!!row.verifiedAt || saving} size="small" onClick={() => void markDomainVerified(row.id)}>
                    {t("platform.verify")}
                  </Button>
                  <Popconfirm
                    cancelText={t("common.cancel")}
                    okText={t("common.confirm")}
                    title={t("platform.removeDomainConfirm", { host: row.host })}
                    onConfirm={() => deleteDomain(row.id)}
                  >
                    <Button danger disabled={saving} size="small">
                      {t("platform.removeDomain")}
                    </Button>
                  </Popconfirm>
                </Space>
              ),
            },
          ]}
          dataSource={domains}
          loading={domainLoading}
          pagination={false}
          rowKey="id"
          size="small"
        />
      </FormDrawer>

      <Modal
        open={accessToken !== undefined}
        title={t("platform.tokenTitle")}
        onCancel={() => setAccessToken(undefined)}
        footer={
          <Button type="primary" onClick={() => setAccessToken(undefined)}>
            {t("common.close")}
          </Button>
        }
      >
        <Alert
          type="warning"
          showIcon
          message={t("platform.tokenWarning")}
          style={{ marginBottom: 16 }}
        />
        <Typography.Paragraph copyable code>
          {accessToken}
        </Typography.Paragraph>
      </Modal>
    </Page>
  );
}
