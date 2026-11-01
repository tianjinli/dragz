import { useCallback, useEffect, useState } from "react";
import { Button, Card, Popconfirm, Switch, Tag, Tooltip, Typography, message } from "antd";
import { useTranslation } from "react-i18next";
import { ListPagePanel } from "@/components/ListPagePanel";
import { Page } from "@/components/Page";
import { TablePanel } from "@/components/TablePanel";
import { normalizeListColumns } from "@/utils/table-columns";
import {
  listPluginsApi,
  uninstallPluginApi,
  updatePluginStatusApi,
  type PluginRecord,
} from "@/api/system";
import { useAuthStore } from "@/stores/auth";

/**
 * Plugin center.
 *
 * The switch reflects SERVER state, never an optimistic guess: enabling runs
 * migrations, so the request can fail and leave the plugin off. On failure the
 * request interceptor has already surfaced the localized backend message, and
 * reloading re-reads the truth.
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

  const toggle = async (row: PluginRecord, next: boolean) => {
    setPending(row.id);
    try {
      await updatePluginStatusApi(row.id, next ? 1 : 0);
      void message.success(
        next ? t("system.plugins.pluginEnabled") : t("system.plugins.pluginDisabled"),
      );
      await load();
      // Enabling/disabling changes which menus the backend sends back.
      void useAuthStore.getState().loadAccess().catch(() => undefined);
    } catch {
      // The interceptor already showed the backend message; re-read so the
      // switch shows what actually happened.
      await load();
    } finally {
      setPending(undefined);
    }
  };

  const uninstall = async (row: PluginRecord) => {
    setPending(row.id);
    try {
      await uninstallPluginApi(row.id);
      void message.success(t("system.plugins.uninstalled"));
      await load();
      // The plugin's menus and permission buttons are gone; the sidebar has to
      // be rebuilt or the pages would still be listed.
      void useAuthStore.getState().loadAccess().catch(() => undefined);
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
                width: 100,
                ellipsis: false,
                render: (_: boolean, row: PluginRecord) => (
                  <Tooltip
                    title={
                      row.builtin
                        ? t("system.plugins.builtinCannotDisable")
                        : row.lastError || undefined
                    }
                  >
                    <Switch
                      checked={row.enabled}
                      disabled={row.builtin || pending === row.id}
                      loading={pending === row.id}
                      onChange={(next) => void toggle(row, next)}
                    />
                  </Tooltip>
                ),
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
                width: 110,
                ellipsis: false,
                render: (_: unknown, row: PluginRecord) => {
                  // Uninstall drops tables. The backend refuses an enabled
                  // plugin, so the button is disabled rather than failing on
                  // click; a builtin has no data to throw away at all.
                  const blocked = row.builtin
                    ? t("system.plugins.builtinCannotUninstall")
                    : row.enabled
                      ? t("system.plugins.disableFirst")
                      : undefined;
                  return (
                    <Tooltip title={blocked}>
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
                          danger
                          type="link"
                          size="small"
                          disabled={!!blocked || pending === row.id}
                          loading={pending === row.id}
                        >
                          {t("system.plugins.uninstall")}
                        </Button>
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
