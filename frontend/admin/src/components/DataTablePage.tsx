import { ListPagePanel } from "@/components/ListPagePanel";
import { useCallback, useEffect, useState } from "react";
import { Button, Card, Input, Space, type TableColumnsType } from "antd";
import { RotateCcw, Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { ResourceRecord } from "@/api/system";
import { normalizeListColumns } from "@/utils/table-columns";
import { TablePanel } from "./TablePanel";
import { Page } from "./Page";
import { actionColumnWidth, TableActionGroup } from "./TableActionGroup";

export interface RowAction {
  danger?: boolean;
  hidden?: (row: ResourceRecord) => boolean;
  label: string;
  onClick: (
    row: ResourceRecord,
    reload: () => Promise<void>,
  ) => void | Promise<void>;
}
export function DataTablePage({
  actions = [],
  columns,
  loadData,
  paged = true,
  title,
  extra,
}: {
  actions?: RowAction[];
  columns: TableColumnsType<ResourceRecord>;
  extra?: React.ReactNode;
  loadData: (query: {
    keyword: string;
    page: number;
    pageSize: number;
  }) => Promise<ResourceRecord[] | { items: ResourceRecord[]; total?: number }>;
  paged?: boolean;
  title: string;
}) {
  const { t } = useTranslation();
  const [keyword, setKeyword] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [items, setItems] = useState<ResourceRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await loadData({ keyword, page, pageSize });
      const rows = Array.isArray(data) ? data : data.items;
      setItems(rows);
      setTotal(Array.isArray(data) ? data.length : (data.total ?? rows.length));
    } finally {
      setLoading(false);
    }
  }, [keyword, loadData, page, pageSize]);
  useEffect(() => {
    void load();
  }, [load]);
  const allColumns = normalizeListColumns<ResourceRecord>(
    actions.length
      ? [
          ...columns,
          {
            title: t("common.actions"),
            key: "actions",
            width: actionColumnWidth(actions),
            render: (_, row) => (
              <TableActionGroup
                actions={actions
                  .filter((action) => !action.hidden?.(row))
                  .map((action) => ({
                    ...action,
                    onClick: () => action.onClick(row, load),
                  }))}
              />
            ),
          },
        ]
      : columns,
  );
  return (
    <Page title={title}>
      <Card className="table-card">
        <ListPagePanel
          title={t("common.listTitle", { title })}
          toolbar={{
            search: (
              <Space wrap>
                <Input
                  allowClear
                  prefix={<Search size={16} />}
                  placeholder={t("common.keyword")}
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  onPressEnter={() => void load()}
                />
                <Button
                  aria-label={t("common.search")}
                  title={t("common.search")}
                  type="primary"
                  icon={<Search size={16} />}
                  onClick={() => void load()}
                />
                <Button
                  aria-label={t("common.reset")}
                  title={t("common.reset")}
                  icon={<RotateCcw size={16} />}
                  onClick={() => {
                    setKeyword("");
                    setPage(1);
                  }}
                />
              </Space>
            ),
            actions: extra,
            onRefresh: () => void load(),
          }}
        >
          <TablePanel
            className="list-page-table"
            bordered
            sticky
            scroll={{ x: "max-content" }}
            columns={allColumns}
            dataSource={items}
            loading={loading}
            rowKey="id"
            pagination={
              paged
                ? {
                    current: page,
                    pageSize,
                    total,
                    showSizeChanger: true,
                    showTotal: (n) => t("common.total", { count: n }),
                    onChange: (p, s) => {
                      setPage(p);
                      setPageSize(s);
                    },
                  }
                : false
            }
          />
        </ListPagePanel>
      </Card>
    </Page>
  );
}
