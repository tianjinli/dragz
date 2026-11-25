import { useCallback, useEffect, useState } from "react";
import { Card, Select } from "antd";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";
import {
  listInvoices,
  listTenantInvoices,
  listTenants,
  type PlatformInvoice,
  type PlatformTenant,
} from "@/api/platform";
import { ListPagePanel } from "@/components/ListPagePanel";
import { Page } from "@/components/Page";
import { TablePanel } from "@/components/TablePanel";
import { useAccessStore } from "@/stores/access";
import { formatDateTime } from "@/utils/format-time";
import { normalizeListColumns } from "@/utils/table-columns";
import { SuperAdminDenied } from "./super-admin";

function parseTenantId(value: string | null): number | undefined {
  if (!value) return undefined;
  const id = Number(value);
  if (!Number.isInteger(id) || id <= 0) return undefined;
  return id;
}

function formatCents(cents: number) {
  const negative = cents < 0;
  const abs = Math.abs(Math.trunc(cents));
  const text = `${Math.trunc(abs / 100)}.${String(abs % 100).padStart(2, "0")}`;
  return negative ? `-${text}` : text;
}

function formatPeriod(start?: string | null, end?: string | null) {
  if (!start && !end) return "—";
  const left = start ? formatDateTime(start) : "—";
  const right = end ? formatDateTime(end) : "—";
  return `${left} – ${right}`;
}

export function PlatformInvoicesPage() {
  const isSuperAdmin = useAccessStore((state) => !!state.userInfo?.isSuperAdmin);
  if (!isSuperAdmin) return <SuperAdminDenied />;
  return <InvoiceCatalog />;
}

function InvoiceCatalog() {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const tenantId = parseTenantId(params.get("tenantId"));
  const [tenants, setTenants] = useState<PlatformTenant[]>([]);
  const [items, setItems] = useState<PlatformInvoice[]>([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [tenantRows, invoiceRows] = await Promise.all([
        listTenants(),
        tenantId ? listTenantInvoices(tenantId) : listInvoices(),
      ]);
      setTenants(tenantRows);
      setItems(invoiceRows);
    } catch {
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [tenantId]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <Page title={t("platform.invoicesTitle")}>
      <Card className="table-card fit-list">
        <ListPagePanel
          title={t("platform.invoicesList")}
          toolbar={{
            onRefresh: () => void load(),
            search: (
              <Select
                allowClear
                showSearch
                optionFilterProp="label"
                placeholder={t("platform.tenantFilter")}
                style={{ minWidth: 280 }}
                value={tenantId}
                options={tenants.map((tenant) => ({
                  label: `${tenant.name} (${tenant.code})`,
                  value: tenant.id,
                }))}
                onChange={(value) => {
                  if (value) setParams({ tenantId: String(value) });
                  else setParams({});
                }}
              />
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
            columns={normalizeListColumns<PlatformInvoice>([
              { title: t("platform.id"), dataIndex: "id", width: 80 },
              { title: t("platform.tenantId"), dataIndex: "tenantId", width: 110 },
              { title: t("platform.kind"), dataIndex: "kind", width: 140 },
              { title: t("common.status"), dataIndex: "status", width: 120 },
              {
                title: t("platform.amount"),
                dataIndex: "amountCents",
                width: 140,
                render: (value: number) => formatCents(value),
              },
              { title: t("platform.currency"), dataIndex: "currency", width: 100 },
              {
                title: t("platform.period"),
                key: "period",
                width: 320,
                render: (_: unknown, row: PlatformInvoice) => formatPeriod(row.periodStart, row.periodEnd),
              },
              { title: t("platform.memo"), dataIndex: "memo" },
              {
                title: t("common.createdAt"),
                dataIndex: "createdAt",
                width: 180,
                render: (value: string) => formatDateTime(value),
              },
            ])}
          />
        </ListPagePanel>
      </Card>
    </Page>
  );
}
