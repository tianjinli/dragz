import { useCallback, useEffect, useState } from "react";
import { Button, Drawer, Form, Input, Modal, Switch, Tag, message } from "antd";
import { useTranslation } from "react-i18next";
import {
  clearCacheApi,
  deleteCacheKeyApi,
  listCacheKeysApi,
  listJobLogsApi,
  listJobsApi,
  runJobApi,
  testWebhookApi,
  updateJobApi,
  type ResourceRecord,
} from "@/api/system";
import { DataTablePage } from "@/components/DataTablePage";
import { ResourceCrudPage } from "@/components/ResourceCrudPage";

export function JobPage() {
  const { t } = useTranslation();
  const [job, setJob] = useState<ResourceRecord | null>(null);
  const [version, setVersion] = useState(0);
  const [logRefreshKey, setLogRefreshKey] = useState(0);
  const [statusUpdatingId, setStatusUpdatingId] = useState<number | null>(null);
  const load = useCallback(async () => {
    const value = await listJobsApi();
    return value.items;
  }, [version]);
  const changeStatus = useCallback(async (row: ResourceRecord, checked: boolean) => {
    setStatusUpdatingId(row.id);
    try {
      await updateJobApi(row.id, {
        name: row.name,
        code: row.code,
        handler: row.handler,
        cronExpression: row.cronExpression,
        payload: (row.payload as unknown) ?? {},
        timeoutSeconds: row.timeoutSeconds,
        maxRetries: row.maxRetries,
        status: checked ? 1 : 0,
      });
      void message.success(checked ? t("system.tools.jobEnabled") : t("system.tools.jobDisabled"));
      setVersion((v) => v + 1);
    } catch {
      void message.error(t("system.tools.statusUpdateFailed"));
    } finally {
      setStatusUpdatingId(null);
    }
  }, [t]);
  return (
    <>
      <DataTablePage
        title={t("system.tools.title")}
        paged={false}
        loadData={load}
        columns={[
          { title: t("system.tools.taskName"), dataIndex: "name" },
          { title: t("system.tools.handler"), dataIndex: "handler" },
          { title: t("system.tools.cron"), dataIndex: "cronExpression" },
          {
            title: t("common.status"),
            dataIndex: "status",
            render: (value: unknown, row: ResourceRecord) => (
              <Switch
                checked={Number(value ?? 0) === 1}
                loading={statusUpdatingId === row.id}
                disabled={statusUpdatingId === row.id}
                onChange={(checked) => void changeStatus(row, checked)}
              />
            ),
          },
          { title: t("system.tools.nextRunAt"), dataIndex: "nextRunAt" },
        ]}
        actions={[
          {
            label: t("system.tools.runNow"),
            onClick: async (row, reload) => {
              await runJobApi(row.id);
              void message.success(t("system.tools.jobSubmitted"));
              await reload();
              setLogRefreshKey((key) => key + 1);
              setJob(row);
            },
          },
          { label: t("system.tools.logs"), onClick: (row) => setJob(row) },
        ]}
      />
      <Drawer
        size={760}
        open={!!job}
        title={t("system.tools.drawerTitle", {
          name: String(job?.name || t("system.tools.jobFallback")),
        })}
        onClose={() => setJob(null)}
      >
        {job && <JobLogList id={job.id} refreshKey={logRefreshKey} />}
      </Drawer>
    </>
  );
}
function durationMs(row: ResourceRecord): string {
  const started = row.startedAt ? new Date(String(row.startedAt)).getTime() : Number.NaN;
  const finished = row.finishedAt ? new Date(String(row.finishedAt)).getTime() : Number.NaN;
  if (!Number.isFinite(started) || !Number.isFinite(finished)) return "—";
  return String(finished - started);
}
function JobLogList({ id, refreshKey }: { id: number; refreshKey: number }) {
  const { t } = useTranslation();
  const [tick, setTick] = useState(0);
  useEffect(() => {
    const timer = window.setInterval(() => setTick((value) => value + 1), 3000);
    return () => window.clearInterval(timer);
  }, []);
  const load = useCallback(
    (q: { keyword: string; page: number; pageSize: number }) =>
      listJobLogsApi(id, q),
    [id, refreshKey, tick],
  );
  return (
    <DataTablePage
      title=""
      loadData={load}
      columns={[
        {
          title: t("system.tools.triggerType"),
          dataIndex: "triggerType",
          render: (value: unknown) =>
            String(value) === "manual"
              ? t("system.tools.triggerManual")
              : t("system.tools.triggerScheduled"),
        },
        {
          title: t("common.status"),
          dataIndex: "success",
          render: (value: unknown) =>
            value === true ? (
              <Tag color="success">{t("common.success")}</Tag>
            ) : value === false ? (
              <Tag color="error">{t("common.failed")}</Tag>
            ) : (
              <Tag color="processing">{t("system.tools.running")}</Tag>
            ),
        },
        { title: t("system.tools.attempt"), dataIndex: "attempt" },
        { title: t("system.tools.startedAt"), dataIndex: "startedAt" },
        { title: t("system.tools.finishedAt"), dataIndex: "finishedAt" },
        {
          title: t("system.tools.duration"),
          key: "durationMs",
          render: (_: unknown, row: ResourceRecord) => durationMs(row),
        },
        { title: t("system.tools.output"), dataIndex: "output" },
        { title: t("system.tools.error"), dataIndex: "errorMessage" },
      ]}
    />
  );
}
export function WebhookPage() {
  const { t } = useTranslation();
  const actions = [
    {
      label: t("system.tools.test"),
      onClick: async (row: ResourceRecord) => {
        await testWebhookApi(row.id);
        void message.success(t("system.tools.testSent"));
      },
    },
  ];
  return (
    <ResourceCrudPage
      title={t("system.tools.webhookTitle")}
      endpoint="/system/webhooks"
      createCode="system:webhook:manage"
      updateCode="system:webhook:manage"
      deleteCode="system:webhook:manage"
      rowActions={actions}
      columns={[
        { title: t("common.name"), dataIndex: "name" },
        { title: t("system.tools.url"), dataIndex: "url", width: 300 },
        { title: t("system.tools.events"), dataIndex: "events" },
        { title: t("common.status"), dataIndex: "status" },
      ]}
      fields={[
        { key: "name", label: t("common.name"), required: true },
        { key: "url", label: t("system.tools.url"), required: true },
        {
          key: "secret",
          label: t("system.tools.secret"),
          type: "password",
          placeholder: t("system.tools.secretPlaceholder"),
        },
        { key: "events", label: t("system.tools.events"), type: "select", mode: "tags" },
        { key: "status", label: t("common.enabled"), type: "switch", numeric: true },
      ]}
    />
  );
}
export function CachePage() {
  const { t } = useTranslation();
  const [version, setVersion] = useState(0);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm();
  const load = useCallback(
    (q: { keyword: string; page: number; pageSize: number }) =>
      listCacheKeysApi({ ...q, pattern: q.keyword, cursor: 0, version }),
    [version],
  );
  const clear = async () => {
    const values = await form.validateFields();
    await clearCacheApi(values);
    setOpen(false);
    setVersion((v) => v + 1);
    void message.success(t("system.tools.cacheCleared"));
  };
  return (
    <>
      <DataTablePage
        title={t("system.tools.cacheTitle")}
        loadData={load}
        columns={[
          { title: t("system.tools.cacheKey"), dataIndex: "key" },
          { title: t("common.type"), dataIndex: "type" },
          { title: t("system.tools.ttl"), dataIndex: "ttl" },
          { title: t("system.tools.size"), dataIndex: "size" },
        ]}
        actions={[
          {
            label: t("common.delete"),
            danger: true,
            onClick: async (row) => {
              await deleteCacheKeyApi(String(row.key));
              void message.success(t("system.tools.keyDeleted"));
            },
          },
        ]}
        extra={
          <Button danger onClick={() => setOpen(true)}>
            {t("system.tools.bulkClear")}
          </Button>
        }
      />
      <Modal
        open={open}
        title={t("system.tools.bulkClearTitle")}
        onCancel={() => setOpen(false)}
        onOk={() => void clear()}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="pattern"
            label={t("system.tools.pattern")}
            initialValue=""
            rules={[{ required: true }]}
          >
            <Input placeholder={t("system.tools.patternPlaceholder")} />
          </Form.Item>
          <Form.Item
            name="confirmation"
            label={t("system.tools.confirmation")}
            rules={[{ required: true }]}
          >
            <Input placeholder={t("system.tools.confirmationPlaceholder")} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
