import { useCallback, useEffect, useState } from "react";
import { Button, Card, Form, Input, InputNumber, message, Modal, Select, Space, Switch, Table, Tabs, Tag, Typography } from "antd";
import type { TableProps } from "antd";
import { Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { apiDelete, apiGet, apiPost, apiPut } from "@/api/request";
import { Page } from "@/components/Page";

type ChannelType = "smtp" | "aliyun_sms" | "tencent_sms";

interface Channel {
  id: number;
  channelType: ChannelType;
  name: string;
  enabled: number;
  config: string;
}

interface Template {
  id: number;
  channelType: ChannelType;
  code: string;
  name: string;
  subject: string;
  content: string;
  status: number;
}

interface LogRow {
  id: number;
  channelType: string;
  channelName: string;
  templateCode: string;
  recipient: string;
  subject: string;
  status: number;
  errorMsg: string;
  createdAt: string;
}

interface Paged<T> {
  items: T[];
  total: number;
}

const CHANNEL_TYPES: { label: string; value: ChannelType }[] = [
  { label: "SMTP", value: "smtp" },
  { label: "Aliyun SMS", value: "aliyun_sms" },
  { label: "Tencent SMS", value: "tencent_sms" },
];

const CONFIG_FIELDS: Record<ChannelType, { key: string; label: string; secret?: boolean; numeric?: boolean }[]> = {
  smtp: [
    { key: "host", label: "SMTP host" },
    { key: "port", label: "Port", numeric: true },
    { key: "from", label: "From address" },
    { key: "username", label: "Username" },
    { key: "password", label: "Password", secret: true },
  ],
  aliyun_sms: [
    { key: "access_key_id", label: "AccessKey ID" },
    { key: "access_key_secret", label: "AccessKey secret", secret: true },
    { key: "sign_name", label: "Sign name" },
    { key: "template_code", label: "Template code" },
    { key: "test_recipient", label: "Test phone" },
  ],
  tencent_sms: [
    { key: "secret_id", label: "SecretId" },
    { key: "secret_key", label: "SecretKey", secret: true },
    { key: "sdk_app_id", label: "SdkAppId" },
    { key: "sign_name", label: "Sign name" },
    { key: "template_id", label: "Template ID" },
    { key: "test_recipient", label: "Test phone" },
  ],
};

function parseConfig(raw: string): Record<string, unknown> {
  if (!raw) return {};
  try {
    return JSON.parse(raw) as Record<string, unknown>;
  } catch {
    return {};
  }
}

export function NotifyPage() {
  const { t } = useTranslation();
  const [channels, setChannels] = useState<Channel[]>([]);
  const [templates, setTemplates] = useState<Template[]>([]);
  const [logs, setLogs] = useState<LogRow[]>([]);
  const [channelOpen, setChannelOpen] = useState(false);
  const [editingChannel, setEditingChannel] = useState<Channel | null>(null);
  const [channelForm] = Form.useForm();
  const [channelType, setChannelType] = useState<ChannelType>("smtp");
  const [templateOpen, setTemplateOpen] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<Template | null>(null);
  const [templateForm] = Form.useForm();
  const [sendForm] = Form.useForm();
  const [sending, setSending] = useState(false);

  const loadChannels = useCallback(async () => {
    const data = await apiGet<Paged<Channel>>("/notify/channels?page=1&pageSize=100");
    setChannels(data.items);
  }, []);
  const loadTemplates = useCallback(async () => {
    const data = await apiGet<Paged<Template>>("/notify/templates?page=1&pageSize=100");
    setTemplates(data.items);
  }, []);
  const loadLogs = useCallback(async () => {
    const data = await apiGet<Paged<LogRow>>("/notify/logs?page=1&pageSize=50");
    setLogs(data.items);
  }, []);

  const sendNotification = async (values: Record<string, unknown>) => {
    let vars: Record<string, unknown> | undefined;
    const rawVars = typeof values.vars === "string" ? values.vars.trim() : "";
    if (rawVars) {
      try {
        vars = JSON.parse(rawVars) as Record<string, unknown>;
      } catch {
        message.error(t("pages.notify.invalidVars"));
        return;
      }
    }
    const channelId = Number(values.channelId ?? 0);
    const payload = {
      channelType: channelId > 0 ? undefined : String(values.channelType ?? ""),
      channelId: channelId > 0 ? channelId : undefined,
      templateCode: String(values.templateCode ?? "") || undefined,
      recipient: String(values.recipient ?? ""),
      subject: String(values.subject ?? "") || undefined,
      content: String(values.content ?? "") || undefined,
      vars,
    };
    setSending(true);
    try {
      await apiPost("/notify/send", payload);
      message.success(t("pages.notify.sendOk"));
      await loadLogs();
    } catch (error) {
      message.error(error instanceof Error ? error.message : String(error));
    } finally {
      setSending(false);
    }
  };

  useEffect(() => {
    void Promise.all([loadChannels(), loadTemplates(), loadLogs()]);
  }, [loadChannels, loadTemplates, loadLogs]);

  const openChannel = (row?: Channel) => {
    setEditingChannel(row ?? null);
    const type = row?.channelType ?? "smtp";
    setChannelType(type);
    const config = parseConfig(row?.config ?? "");
    channelForm.setFieldsValue({
      channelType: type,
      name: row?.name ?? "",
      enabled: row ? row.enabled === 1 : true,
      ...config,
    });
    setChannelOpen(true);
  };

  const saveChannel = async () => {
    const values = await channelForm.validateFields();
    const config: Record<string, unknown> = {};
    for (const field of CONFIG_FIELDS[values.channelType as ChannelType]) {
      if (values[field.key] !== undefined) config[field.key] = values[field.key];
    }
    const payload = {
      channelType: values.channelType,
      name: values.name,
      enabled: values.enabled ? 1 : 0,
      config,
    };
    if (editingChannel) {
      await apiPut(`/notify/channels/${editingChannel.id}`, payload);
    } else {
      await apiPost("/notify/channels", payload);
    }
    message.success(t("pages.notify.saved"));
    setChannelOpen(false);
    await loadChannels();
  };

  const testChannel = async (row: Channel) => {
    try {
      await apiPost(`/notify/channels/${row.id}/test`);
      message.success(t("pages.notify.testOk"));
    } catch (error) {
      message.error(error instanceof Error ? error.message : String(error));
    }
    await loadLogs();
  };

  const removeChannel = async (row: Channel) => {
    await apiDelete(`/notify/channels/${row.id}`);
    await loadChannels();
  };

  const openTemplate = (row?: Template) => {
    setEditingTemplate(row ?? null);
    templateForm.setFieldsValue({
      channelType: row?.channelType ?? "smtp",
      code: row?.code ?? "",
      name: row?.name ?? "",
      subject: row?.subject ?? "",
      content: row?.content ?? "",
      status: row ? row.status === 1 : true,
    });
    setTemplateOpen(true);
  };

  const saveTemplate = async () => {
    const values = await templateForm.validateFields();
    const payload = { ...values, status: values.status ? 1 : 0 };
    if (editingTemplate) {
      await apiPut(`/notify/templates/${editingTemplate.id}`, payload);
    } else {
      await apiPost("/notify/templates", payload);
    }
    message.success(t("pages.notify.saved"));
    setTemplateOpen(false);
    await loadTemplates();
  };

  const removeTemplate = async (row: Template) => {
    await apiDelete(`/notify/templates/${row.id}`);
    await loadTemplates();
  };

  const channelColumns: TableProps<Channel>["columns"] = [
    { title: t("pages.notify.type"), dataIndex: "channelType", key: "channelType", render: (v: ChannelType) => <Tag color="blue">{v}</Tag> },
    { title: t("pages.notify.name"), dataIndex: "name", key: "name" },
    { title: t("pages.notify.enabled"), dataIndex: "enabled", key: "enabled", render: (v: number) => <Tag color={v === 1 ? "green" : "default"}>{v === 1 ? t("common.enabled") : t("crud.disabled")}</Tag> },
    {
      title: t("pages.notify.actions"), key: "actions",
      render: (_: unknown, row: Channel) => (
        <Space>
          <Button size="small" onClick={() => openChannel(row)}>{t("common.edit")}</Button>
          <Button size="small" onClick={() => void testChannel(row)}>{t("pages.notify.test")}</Button>
          <Button size="small" danger onClick={() => void removeChannel(row)}>{t("common.delete")}</Button>
        </Space>
      ),
    },
  ];

  const templateColumns: TableProps<Template>["columns"] = [
    { title: t("pages.notify.type"), dataIndex: "channelType", key: "channelType", render: (v: string) => <Tag color="blue">{v}</Tag> },
    { title: t("pages.notify.code"), dataIndex: "code", key: "code" },
    { title: t("pages.notify.name"), dataIndex: "name", key: "name" },
    { title: t("pages.notify.subject"), dataIndex: "subject", key: "subject" },
    {
      title: t("pages.notify.actions"), key: "actions",
      render: (_: unknown, row: Template) => (
        <Space>
          <Button size="small" onClick={() => openTemplate(row)}>{t("common.edit")}</Button>
          <Button size="small" danger onClick={() => void removeTemplate(row)}>{t("common.delete")}</Button>
        </Space>
      ),
    },
  ];

  const logColumns: TableProps<LogRow>["columns"] = [
    { title: t("pages.notify.time"), dataIndex: "createdAt", key: "createdAt" },
    { title: t("pages.notify.type"), dataIndex: "channelType", key: "channelType" },
    { title: t("pages.notify.recipient"), dataIndex: "recipient", key: "recipient" },
    { title: t("pages.notify.subject"), dataIndex: "subject", key: "subject" },
    { title: t("pages.notify.status"), dataIndex: "status", key: "status", render: (v: number) => <Tag color={v === 1 ? "green" : "red"}>{v === 1 ? t("pages.notify.sent") : t("pages.notify.failed")}</Tag> },
    {
      title: t("pages.notify.error"), dataIndex: "errorMsg", key: "errorMsg",
      render: (v: string) => <Typography.Text type="danger" ellipsis>{v}</Typography.Text>,
    },
  ];

  return (
    <Page title={t("pages.notify.title")} description={t("pages.notify.desc")}>
      <Tabs
        items={[
          {
            key: "send",
            label: t("pages.notify.send"),
            children: (
              <Card className="table-card" title={t("pages.notify.sendDesc")}>
                <Form
                  form={sendForm}
                  layout="vertical"
                  onFinish={(values: Record<string, unknown>) => void sendNotification(values)}
                >
                  <Form.Item name="channelType" label={t("pages.notify.type")} initialValue="smtp" rules={[{ required: true }]}>
                    <Select options={CHANNEL_TYPES} />
                  </Form.Item>
                  <Form.Item name="channelId" label={t("pages.notify.channel")} initialValue={0}>
                    <Select
                      options={[
                        { value: 0, label: t("pages.notify.autoSelect") },
                        ...channels.map((row) => ({ value: row.id, label: `${row.name} (${row.channelType})` })),
                      ]}
                    />
                  </Form.Item>
                  <Form.Item name="templateCode" label={t("pages.notify.template")}>
                    <Input />
                  </Form.Item>
                  <Form.Item name="recipient" label={t("pages.notify.recipient")} rules={[{ required: true }]}>
                    <Input />
                  </Form.Item>
                  <Form.Item name="subject" label={t("pages.notify.subject")}>
                    <Input />
                  </Form.Item>
                  <Form.Item name="content" label={t("pages.notify.content")}>
                    <Input.TextArea rows={4} />
                  </Form.Item>
                  <Form.Item
                    name="vars"
                    label={t("pages.notify.vars")}
                    extra={t("pages.notify.varsHint")}
                  >
                    <Input.TextArea rows={3} />
                  </Form.Item>
                  <Form.Item>
                    <Button type="primary" htmlType="submit" loading={sending}>
                      {t("pages.notify.send")}
                    </Button>
                  </Form.Item>
                </Form>
              </Card>
            ),
          },
          {
            key: "channels",
            label: t("pages.notify.channels"),
            children: (
              <Card
                className="table-card"
                extra={<Button type="primary" icon={<Plus />} onClick={() => openChannel()}>{t("common.add")}</Button>}
              >
                <Table<Channel> rowKey="id" dataSource={channels} columns={channelColumns} pagination={false} />
              </Card>
            ),
          },
          {
            key: "templates",
            label: t("pages.notify.templates"),
            children: (
              <Card
                className="table-card"
                extra={<Button type="primary" icon={<Plus />} onClick={() => openTemplate()}>{t("common.add")}</Button>}
              >
                <Table<Template> rowKey="id" dataSource={templates} columns={templateColumns} pagination={false} />
              </Card>
            ),
          },
          {
            key: "logs",
            label: t("pages.notify.logs"),
            children: (
              <Card className="table-card">
                <Table<LogRow> rowKey="id" dataSource={logs} columns={logColumns} pagination={false} />
              </Card>
            ),
          },
        ]}
      />

      <Modal
        open={channelOpen}
        title={editingChannel ? t("common.edit") : t("common.add")}
        onCancel={() => setChannelOpen(false)}
        onOk={() => void saveChannel()}
        destroyOnClose
      >
        <Form form={channelForm} layout="vertical">
          <Form.Item name="channelType" label={t("pages.notify.type")} rules={[{ required: true }]}>
            <Select options={CHANNEL_TYPES} onChange={(v: ChannelType) => setChannelType(v)} />
          </Form.Item>
          <Form.Item name="name" label={t("pages.notify.name")} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="enabled" label={t("pages.notify.enabled")} valuePropName="checked">
            <Switch />
          </Form.Item>
          {CONFIG_FIELDS[channelType].map((field) => (
            <Form.Item
              key={field.key}
              name={field.key}
              label={field.label}
              rules={[{ required: !field.secret || !editingChannel }]}
            >
              {field.numeric ? <InputNumber style={{ width: "100%" }} /> : field.secret ? <Input.Password autoComplete="new-password" /> : <Input />}
            </Form.Item>
          ))}
        </Form>
      </Modal>

      <Modal
        open={templateOpen}
        title={editingTemplate ? t("common.edit") : t("common.add")}
        onCancel={() => setTemplateOpen(false)}
        onOk={() => void saveTemplate()}
        destroyOnClose
      >
        <Form form={templateForm} layout="vertical">
          <Form.Item name="channelType" label={t("pages.notify.type")} rules={[{ required: true }]}>
            <Select options={CHANNEL_TYPES} />
          </Form.Item>
          <Form.Item name="code" label={t("pages.notify.code")} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="name" label={t("pages.notify.name")} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="subject" label={t("pages.notify.subject")}><Input /></Form.Item>
          <Form.Item name="content" label={t("pages.notify.content")}><Input.TextArea rows={4} /></Form.Item>
          <Form.Item name="status" label={t("pages.notify.enabled")} valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
    </Page>
  );
}
