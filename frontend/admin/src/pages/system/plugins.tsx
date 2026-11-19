import { useCallback, useEffect, useState } from "react";
import { Button, Card, Popconfirm, Switch, Tag, Tooltip, Typography, message } from "antd";
import { Download, PackageMinus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { ListPagePanel } from "@/components/ListPagePanel";
import { Page } from "@/components/Page";
import { actionColumnWidth } from "@/components/TableActionGroup";
import { TablePanel } from "@/components/TablePanel";
import { normalizeListColumns } from "@/utils/table-columns";
import {
  installPluginApi,
  listPluginsApi,
  uninstallPluginApi,
  updatePluginStatusApi,
  type PluginRecord,
} from "@/api/system";
import { useAuthStore } from "@/stores/auth";

/**
 * Plugin center — Available / Installed+Disabled / Installed+Enabled.
 *
 * - Status always shows a Switch. Uninstalled and builtin are disabled (off / on).
 * - Available (!installed): Actions offer Install (migrations, stay disabled).
 * - Installed: Switch toggles Enable/Disable; Uninstall when disabled.
 *
 * The switch and buttons reflect SERVER state, never an optimistic guess:
 * enabling can fail. On failure the interceptor already surfaced the localized
 * backend message; reloading re-reads the truth.
 */
export function PluginPage() {
  const { t } = useTranslation();
  const [items, setItems] = useState<PluginRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [pending, setPending] = useState<string>();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setItems(await listPluginsApi());
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const refreshAccess = () => {
    void useAuthStore.getState().loadAccess().catch(() => undefined);
  };

  const toggle = async (row: PluginRecord, next: boolean) => {
    setPending(row.id);
    try {
      await updatePluginStatusApi(row.id, next ? 1 : 0);
      void message.success(
        next ? t("system.plugins.pluginEnabled") : t("system.plugins.pluginDisabled"),
      );
      await load();
      refreshAccess();
    } catch {
      await load();
    } finally {
      setPending(undefined);
    }
  };

  // Install materialises schema and stamps installed; the switch stays off
  // until the administrator enables explicitly.
  const install = async (row: PluginRecord) => {
    setPending(row.id);
    try {
      await installPluginApi(row.id);
      void message.success(t("system.plugins.installed"));
      await load();
    } catch {
      await load();
    } finally {
      setPending(undefined);
    }
  };

  const uninstall = async (row: PluginRecord) => {
    setPending(row.id);
    try {
      await uninstallPluginApi(row.id);
      // Flip to Available immediately so Status never keeps a disabled Switch
      // while the list refetch is in flight.
      setItems((prev) =>
        prev.map((item) =>
          item.id === row.id
            ? { ...item, installed: false, enabled: false, lastError: "" }
            : item,
        ),
      );
      void message.success(t("system.plugins.uninstalled"));
      await load();
      refreshAccess();
    } catch {
      await load();
    } finally {
      setPending(undefined);
    }
  };

  return (
    <Page title={t("system.plugins.title")}>
      <Card className="table-card">
        <ListPagePanel title={t("system.plugins.listTitle")} toolbar={{ onRefresh: () => void load() }}>
          <TablePanel
            className="list-page-table"
            bordered
            sticky
            scroll={{ x: "max-content" }}
            rowKey="id"
            loading={loading}
            pagination={false}
            dataSource={items}
            columns={normalizeListColumns<PluginRecord>([
              { title: t("system.plugins.id"), dataIndex: "id", width: 120 },
              { title: t("common.name"), dataIndex: "name", width: 160 },
              { title: t("system.plugins.version"), dataIndex: "version", width: 100 },
              { title: t("system.plugins.description"), dataIndex: "description" },
              {
                title: t("system.plugins.dependencies"),
                dataIndex: "dependsOn",
                width: 140,
                render: (value: string[]) =>
                  value && value.length ? value.join(", ") : "-",
              },
              {
                title: t("system.plugins.pendingMigrations"),
                dataIndex: "pendingMigrations",
                width: 120,
                render: (value: number) =>
                  value > 0 ? <Tag color="orange">{value}</Tag> : "-",
              },
              {
                title: t("common.status"),
                dataIndex: "enabled",
                width: 120,
                ellipsis: false,
                render: (_: boolean, row: PluginRecord) => {
                  const notInstalled = !row.builtin && !row.installed;
                  return (
                    <Tooltip
                      title={
                        row.builtin
                          ? t("system.plugins.builtinCannotDisable")
                          : notInstalled
                            ? t("system.plugins.installFirst")
                            : row.lastError || undefined
                      }
                    >
                      <Switch
                        checked={row.enabled}
                        disabled={row.builtin || notInstalled || pending === row.id}
                        loading={pending === row.id}
                        onChange={(next) => void toggle(row, next)}
                      />
                    </Tooltip>
                  );
                },
              },
              {
                title: t("system.plugins.lastError"),
                dataIndex: "lastError",
                render: (value: string) =>
                  value ? (
                    <Typography.Text type="danger">{value}</Typography.Text>
                  ) : (
                    "-"
                  ),
              },
              {
                title: t("common.actions"),
                key: "actions",
                width: actionColumnWidth(2),
                ellipsis: false,
                render: (_: unknown, row: PluginRecord) => {
                  if (row.builtin) {
                    return "-";
                  }
                  if (!row.installed) {
                    return (
                      <Tooltip title={t("system.plugins.install")}>
                        <Button
                          aria-label={t("system.plugins.install")}
                          type="link"
                          size="small"
                          icon={<Download size={16} />}
                          loading={pending === row.id}
                          disabled={pending === row.id}
                          onClick={() => void install(row)}
                        />
                      </Tooltip>
                    );
                  }
                  const blocked = row.enabled
                    ? t("system.plugins.disableFirst")
                    : undefined;
                  return (
                    <Tooltip title={blocked || t("system.plugins.uninstall")}>
                      <Popconfirm
                        title={t("system.plugins.uninstallTitle", { name: row.name })}
                        description={t("system.plugins.uninstallDescription")}
                        okText={t("system.plugins.confirmUninstall")}
                        okButtonProps={{ danger: true }}
                        cancelText={t("common.cancel")}
                        disabled={!!blocked}
                        onConfirm={() => void uninstall(row)}
                      >
                        <Button
                          aria-label={t("system.plugins.uninstall")}
                          danger
                          type="link"
                          size="small"
                          icon={<PackageMinus size={16} />}
                          disabled={!!blocked || pending === row.id}
                          loading={pending === row.id}
                        />
                      </Popconfirm>
                    </Tooltip>
                  );
                },
              },
            ])}
          />
        </ListPagePanel>
      </Card>
    </Page>
  );
}
