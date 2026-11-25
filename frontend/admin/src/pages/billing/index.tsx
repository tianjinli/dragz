import { useCallback, useEffect, useState } from "react";
import { Card, Col, Descriptions, Progress, Row, Tag, Typography } from "antd";
import { useTranslation } from "react-i18next";
import { getBillingSummary, getBillingUsage, type BillingQuota, type BillingSubscription, type BillingSummary, type BillingUsageCounter } from "@/api/billing";
import type { PlatformInvoice } from "@/api/platform";
import { ListPagePanel } from "@/components/ListPagePanel";
import { Page } from "@/components/Page";
import { TablePanel } from "@/components/TablePanel";
import { formatDateTime } from "@/utils/format-time";
import { normalizeListColumns } from "@/utils/table-columns";

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

function quotaPercent(used: number, limit: number) {
  if (limit <= 0) return used > 0 ? 100 : 0;
  return Math.min(100, Math.round((used / limit) * 100));
}

function when(value?: string | null) {
  return value ? formatDateTime(value) : "—";
}

export function BillingPage() {
  const { t } = useTranslation();
  const [summary, setSummary] = useState<BillingSummary>();
  const [counters, setCounters] = useState<BillingUsageCounter[]>([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [nextSummary, usage] = await Promise.all([getBillingSummary(), getBillingUsage()]);
      setSummary(nextSummary);
      setCounters(usage.usage ?? []);
    } catch {
      setSummary(undefined);
      setCounters([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const subscription = summary?.subscription ?? null;
  const quotas = summary?.quotas ?? [];
  const invoices = summary?.invoices ?? [];
  // Storage and seats are gauges (one row per tenant), API calls are per-day
  // counters. Showing the period is what makes the number readable.
  const quotaLabel = useCallback(
    (key: string) => {
      const known = t(`billing.quotaNames.${key}`, { defaultValue: '' });
      return known || key;
    },
    [t],
  );

  return (
    <Page title={t("billing.title")}>
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <Card title={t("billing.currentSubscription")} loading={loading}>
            {subscription ? (
              <SubscriptionDetail row={subscription} />
            ) : (
              <Typography.Text type="secondary">{t("billing.noSubscription")}</Typography.Text>
            )}
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card className="table-card fit-list" loading={loading}>
            <ListPagePanel
              title={t("billing.quotas")}
              toolbar={{ onRefresh: () => void load() }}
            >
              <TablePanel
                rowKey="key"
                pagination={false}
                dataSource={quotas}
                locale={{ emptyText: t("billing.empty") }}
                columns={normalizeListColumns<BillingQuota>([
                  {
                    title: t("billing.quotaKey"),
                    dataIndex: "key",
                    render: (value: string) => quotaLabel(value),
                  },
                  { title: t("billing.used"), dataIndex: "used", width: 100 },
                  { title: t("billing.limit"), dataIndex: "limit", width: 100 },
                  {
                    title: t("billing.usage"),
                    key: "usage",
                    width: 180,
                    render: (_: unknown, row: BillingQuota) => (
                      <Progress
                        percent={quotaPercent(row.used, row.limit)}
                        size="small"
                        status={row.limit > 0 && row.used > row.limit ? "exception" : "normal"}
                      />
                    ),
                  },
                ])}
              />
            </ListPagePanel>
          </Card>
        </Col>
        <Col span={24}>
          <Card className="table-card fit-list" loading={loading}>
            <ListPagePanel
              title={t("billing.usageTitle")}
              toolbar={{ onRefresh: () => void load() }}
            >
              <TablePanel
                rowKey={(row) => `${row.quotaKey}:${row.periodKey}`}
                pagination={false}
                dataSource={counters}
                locale={{ emptyText: t("billing.empty") }}
                columns={normalizeListColumns<BillingUsageCounter>([
                  {
                    title: t("billing.quotaKey"),
                    dataIndex: "quotaKey",
                    render: (value: string) => quotaLabel(value),
                  },
                  { title: t("billing.periodKey"), dataIndex: "periodKey", width: 200 },
                  { title: t("billing.used"), dataIndex: "usedValue", width: 160 },
                  {
                    title: t("billing.updatedAt"),
                    dataIndex: "updatedAt",
                    width: 200,
                    render: (value: string) => formatDateTime(value),
                  },
                ])}
              />
            </ListPagePanel>
          </Card>
        </Col>
        <Col span={24}>
          <Card className="table-card fit-list">
            <ListPagePanel
              title={t("billing.invoices")}
              toolbar={{ onRefresh: () => void load() }}
            >
              <TablePanel
                className="list-page-table"
                bordered
                sticky
                scroll={{ x: "max-content" }}
                rowKey="id"
                loading={loading}
                pagination={false}
                dataSource={invoices}
                locale={{ emptyText: t("billing.empty") }}
                columns={normalizeListColumns<PlatformInvoice>([
                  { title: t("platform.id"), dataIndex: "id", width: 80 },
                  { title: t("billing.kind"), dataIndex: "kind", width: 140 },
                  { title: t("common.status"), dataIndex: "status", width: 120 },
                  {
                    title: t("billing.amount"),
                    dataIndex: "amountCents",
                    width: 140,
                    render: (value: number) => formatCents(value),
                  },
                  { title: t("billing.currency"), dataIndex: "currency", width: 100 },
                  {
                    title: t("billing.period"),
                    key: "period",
                    width: 320,
                    render: (_: unknown, row: PlatformInvoice) => formatPeriod(row.periodStart, row.periodEnd),
                  },
                  { title: t("billing.memo"), dataIndex: "memo" },
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
        </Col>
      </Row>
    </Page>
  );
}

function SubscriptionDetail({ row }: { row: BillingSubscription }) {
  const { t } = useTranslation();
  return (
    <Descriptions column={1} size="small">
      <Descriptions.Item label={t("billing.plan")}>
        {row.planName} ({row.planCode})
      </Descriptions.Item>
      <Descriptions.Item label={t("common.status")}>
        <Tag>{subscriptionStatusLabel(t, row.status)}</Tag>
      </Descriptions.Item>
      <Descriptions.Item label={t("billing.startedAt")}>{when(row.startedAt)}</Descriptions.Item>
      <Descriptions.Item label={t("billing.endedAt")}>{when(row.endedAt)}</Descriptions.Item>
      <Descriptions.Item label={t("billing.trialEndsAt")}>{when(row.trialEndsAt)}</Descriptions.Item>
      <Descriptions.Item label={t("billing.graceEndsAt")}>{when(row.graceEndsAt)}</Descriptions.Item>
    </Descriptions>
  );
}

function subscriptionStatusLabel(t: (key: string) => string, status: string) {
  switch (status) {
    case "active":
      return t("platform.statusActive");
    case "trial":
      return t("platform.statusTrial");
    case "overdue":
      return t("platform.statusOverdue");
    case "cancelled":
      return t("platform.statusCancelled");
    default:
      return status;
  }
}
