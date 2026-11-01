import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Card, Space, Tag } from 'antd';
import { RotateCcw } from 'lucide-react';
import type { TFunction } from 'i18next';
import { Page } from '@/components/Page';
import { ResizableTable } from '@/components/ResizableTable';
import type { ColumnDef, ColumnSizingState } from '@tanstack/react-table';

interface DemoRow {
  id: string;
  username: string;
  email: string;
  department: string;
  role: string;
  status: 'active' | 'disabled';
  amount: number;
  remark: string;
  createdAt: string;
}

const STORAGE_KEY = 'demo:resizable-table:column-sizing';

/** Deterministic mock rows — no randomness so the demo is stable across reloads. */
function buildRows(count: number, t: TFunction): DemoRow[] {
  const departments = [
    t('preview.deptTech'),
    t('preview.deptProduct'),
    t('preview.deptOperations'),
    t('preview.deptMarketing'),
    t('preview.deptSupport'),
  ];
  const roles = [
    t('preview.roleAdmin'),
    t('preview.roleOperations'),
    t('preview.roleDeveloper'),
    t('preview.roleTester'),
    t('preview.roleGuest'),
  ];
  const remarks = [
    t('preview.remarkLong'),
    t('preview.remarkShort'),
    t('preview.remarkPermission'),
  ];

  return Array.from({ length: count }, (_, index) => {
    const n = index + 1;
    return {
      id: `U-${String(1000 + n)}`,
      username: t('preview.demoUsername', { n: String(n).padStart(2, '0') }),
      email: `user${n}@example.com`,
      department: departments[n % departments.length],
      role: roles[n % roles.length],
      status: n % 4 === 0 ? 'disabled' : 'active',
      amount: Math.round((199 + n * 37.5) * 100) / 100,
      remark: remarks[n % remarks.length],
      createdAt: `2026-10-${String((n % 28) + 1).padStart(2, '0')} 09:${String(n % 60).padStart(2, '0')}:00`,
    };
  });
}

export function ResizableTablePreviewPage() {
  const { t } = useTranslation();
  const [sizing, setSizing] = useState<ColumnSizingState>({});
  const [resetKey, setResetKey] = useState(0);

  const columns = useMemo<Array<ColumnDef<DemoRow, unknown>>>(
    () => [
      {
        accessorKey: 'id',
        header: 'ID',
        enableResizing: false,
        size: 110,
        meta: { antd: { align: 'center' } },
      },
      {
        accessorKey: 'username',
        header: t('preview.colUsername'),
        size: 160,
        minSize: 100,
        meta: { antd: { ellipsis: true } },
      },
      {
        accessorKey: 'email',
        header: t('preview.colEmail'),
        size: 220,
        meta: { antd: { ellipsis: true } },
      },
      {
        accessorKey: 'department',
        header: t('preview.colDepartment'),
        size: 140,
        meta: { antd: { align: 'center' } },
      },
      {
        accessorKey: 'role',
        header: t('preview.colRole'),
        size: 120,
        meta: { antd: { align: 'center' } },
      },
      {
        accessorKey: 'status',
        header: t('common.status'),
        size: 110,
        cell: ({ row }) => (
          <Tag color={row.original.status === 'active' ? 'green' : 'default'}>
            {row.original.status === 'active' ? t('common.enabled') : t('preview.statusDisabled')}
          </Tag>
        ),
        meta: { antd: { align: 'center' } },
      },
      {
        accessorKey: 'amount',
        header: t('preview.colAmount'),
        size: 130,
        cell: ({ getValue }) => `¥${(getValue() as number).toFixed(2)}`,
        meta: { antd: { align: 'right' } },
      },
      {
        accessorKey: 'remark',
        header: t('common.remark'),
        size: 260,
        minSize: 160,
        meta: { antd: { ellipsis: true } },
      },
      {
        accessorKey: 'createdAt',
        header: t('common.createdAt'),
        size: 180,
        meta: { antd: { align: 'center' } },
      },
    ],
    [t],
  );

  const rows = useMemo(() => buildRows(24, t), [t]);

  const handleSizingChange = useCallback((next: ColumnSizingState) => setSizing(next), []);

  const resetAll = () => {
    try {
      window.localStorage.removeItem(STORAGE_KEY);
    } catch {
      /* ignore */
    }
    setResetKey((key) => key + 1);
  };

  return (
    <Page
      description={t('preview.resizableDesc')}
      extra={
        <Space>
          <Button icon={<RotateCcw size={16} />} onClick={resetAll}>
            {t('preview.resetAllWidth')}
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Alert showIcon type="info" message={t('preview.dragHint')} />
        <Card title={t('preview.resizeCardTitle')}>
          <ResizableTable<DemoRow>
            key={resetKey}
            columns={columns}
            data={rows}
            defaultColumnSizing={{ username: 160, email: 220 }}
            onColumnSizingChange={handleSizingChange}
            storageKey={STORAGE_KEY}
            tableProps={{ bordered: true, size: 'middle' }}
          />
        </Card>
        <Card size="small" title={t('preview.sizingCardTitle')}>
          <pre style={{ margin: 0, fontSize: 12 }} data-testid="sizing-json">
            {JSON.stringify(sizing, null, 2)}
          </pre>
        </Card>
      </Space>
    </Page>
  );
}

export default ResizableTablePreviewPage;
