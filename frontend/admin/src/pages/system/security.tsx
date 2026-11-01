import { useCallback, useState } from "react";
import { Button, Form, Input, Modal, Tag, Typography, message } from "antd";
import { Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  createTokenApi,
  listErrorLogsApi,
  listLoginLogsApi,
  listOperationLogsApi,
  listSessionsApi,
  listTokensApi,
  resolveErrorLogApi,
  revokeSessionApi,
  revokeTokenApi,
  type ResourceRecord,
} from "@/api/system";
import { DataTablePage } from "@/components/DataTablePage";
import i18n from "@/i18n";

export function OnlineUserPage() {
  const { t } = useTranslation();
  const load = useCallback(
    (q: { keyword: string; page: number; pageSize: number }) =>
      listSessionsApi(q),
    [],
  );
  return (
    <DataTablePage
      title={t("system.security.onlineUser")}
      loadData={load}
      columns={[
        { title: t("system.security.user"), dataIndex: "username" },
        { title: t("system.security.ipAddress"), dataIndex: "ip" },
        { title: t("system.security.device"), dataIndex: "deviceName" },
        { title: t("system.security.browser"), dataIndex: "userAgent" },
        { title: t("system.security.lastActive"), dataIndex: "lastActiveAt" },
      ]}
      actions={[
        {
          label: t("system.security.forceLogout"),
          danger: true,
          onClick: async (row) => {
            await revokeSessionApi(row.id);
            void message.success(t("system.security.sessionRevoked"));
          },
        },
      ]}
    />
  );
}
export function TokenPage() {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [shown, setShown] = useState("");
  const [version, setVersion] = useState(0);
  const [form] = Form.useForm();
  const load = useCallback(
    (q: { keyword: string; page: number; pageSize: number }) =>
      listTokensApi({ ...q, version }),
    [version],
  );
  const create = async () => {
    const value = await form.validateFields();
    const result = await createTokenApi(value);
    setShown(result.token);
    setOpen(false);
    setVersion((v) => v + 1);
  };
  return (
    <>
      <DataTablePage
        title={t("system.security.token")}
        loadData={load}
        columns={[
          { title: t("common.name"), dataIndex: "name" },
          { title: t("system.security.prefix"), dataIndex: "tokenPrefix" },
          { title: t("system.security.scopes"), dataIndex: "scopes" },
          { title: t("system.security.expiresAt"), dataIndex: "expiresAt" },
          { title: t("common.status"), dataIndex: "status" },
        ]}
        actions={[
          {
            label: t("system.security.revoke"),
            danger: true,
            onClick: async (row) => {
              await revokeTokenApi(row.id);
              void message.success(t("system.security.revoked"));
            },
          },
        ]}
        extra={
          <Button
            type="primary"
            icon={<Plus size={16} />}
            onClick={() => setOpen(true)}
          >
            {t("common.add")}
          </Button>
        }
      />
      <Modal
        open={open}
        title={t("system.security.createToken")}
        onCancel={() => setOpen(false)}
        onOk={() => void create()}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label={t("common.name")}
            rules={[{ required: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="expiresAt" label={t("system.security.expiresAt")}>
            <Input placeholder="YYYY-MM-DD HH:mm:ss" />
          </Form.Item>
          <Form.Item name="scopes" label={t("system.security.scopesLabel")}>
            <Input placeholder={t("system.security.scopesPlaceholder")} />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        open={!!shown}
        title={t("system.security.tokenShownOnce")}
        footer={
          <Button type="primary" onClick={() => setShown("")}>
            {t("system.security.savedToken")}
          </Button>
        }
        closable={false}
      >
        <Typography.Paragraph copyable code>
          {shown}
        </Typography.Paragraph>
      </Modal>
    </>
  );
}
const logColumns = () => [
  { title: i18n.t("system.security.user"), dataIndex: "username" },
  { title: i18n.t("system.security.ip"), dataIndex: "ip" },
  { title: i18n.t("system.security.method"), dataIndex: "method" },
  { title: i18n.t("system.security.path"), dataIndex: "path" },
  { title: i18n.t("system.security.statusCode"), dataIndex: "statusCode" },
  { title: i18n.t("system.security.time"), dataIndex: "createdAt" },
];
export function LoginLogPage() {
  const { t } = useTranslation();
  const load = useCallback(
    (q: { keyword: string; page: number; pageSize: number }) =>
      listLoginLogsApi(q),
    [],
  );
  return (
    <DataTablePage
      title={t("system.security.loginLog")}
      loadData={load}
      columns={[
        { title: t("system.security.account"), dataIndex: "username" },
        { title: t("system.security.ip"), dataIndex: "ip" },
        {
          title: t("system.security.result"),
          dataIndex: "success",
          render: (v) => (
            <Tag color={v ? "success" : "error"}>
              {v ? t("common.success") : t("common.failed")}
            </Tag>
          ),
        },
        { title: t("system.security.device"), dataIndex: "userAgent" },
        { title: t("system.security.time"), dataIndex: "createdAt" },
      ]}
    />
  );
}
export function OperationLogPage() {
  const { t } = useTranslation();
  const load = useCallback(
    (q: { keyword: string; page: number; pageSize: number }) =>
      listOperationLogsApi(q),
    [],
  );
  return (
    <DataTablePage
      title={t("system.security.operationLog")}
      loadData={load}
      columns={logColumns()}
    />
  );
}
export function ErrorLogPage() {
  const { t } = useTranslation();
  const load = useCallback(
    (q: { keyword: string; page: number; pageSize: number }) =>
      listErrorLogsApi(q),
    [],
  );
  return (
    <DataTablePage
      title={t("system.security.errorLog")}
      loadData={load}
      columns={[
        ...logColumns(),
        { title: t("system.security.error"), dataIndex: "message" },
      ]}
      actions={[
        {
          label: t("system.security.markResolved"),
          hidden: (r: ResourceRecord) => !!r.resolvedAt,
          onClick: async (row) => {
            await resolveErrorLogApi(row.id);
            void message.success(t("system.security.markedResolved"));
          },
        },
      ]}
    />
  );
}
