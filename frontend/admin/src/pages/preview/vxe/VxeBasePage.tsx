import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Space, Switch } from 'antd';
import type { TFunction } from 'i18next';
import { VxeTable } from '@/components/vxe-table';
import type { VxeColumnProps, VxeTableProps } from '@/components/vxe-table';
import { VxeDemoShell } from './VxeDemoShell';

interface Row {
  id: number;
  name: string;
  role: string;
  age: number;
  email: string;
  status: 'active' | 'disabled';
}

function makeRows(count: number, t: TFunction): Row[] {
  const roles = [
    t('preview.roleAdmin'),
    t('preview.roleOperations'),
    t('preview.roleDeveloper'),
    t('preview.roleGuest'),
  ];
  const names = [
    t('preview.personZhang'),
    t('preview.personLi'),
    t('preview.personWang'),
    t('preview.personZhao'),
    t('preview.personSun'),
    t('preview.personZhou'),
    t('preview.personWu'),
    t('preview.personZheng'),
  ];

  return Array.from({ length: count }, (_, i) => ({
    id: i + 1,
    name: names[i % names.length] + (i >= names.length ? `(${Math.floor(i / names.length) + 1})` : ''),
    role: roles[i % roles.length],
    age: 22 + ((i * 7) % 30),
    email: `user${i + 1}@example.com`,
    status: i % 3 === 0 ? 'disabled' : 'active',
  }));
}

export default function VxeBasePage() {
  const { t } = useTranslation();
  const [rows, setRows] = useState(() => makeRows(30, t));
  const [border, setBorder] = useState(true);
  const [stripe, setStripe] = useState(true);
  const [loading, setLoading] = useState(false);
  const [empty, setEmpty] = useState(false);
  const [lastEvent, setLastEvent] = useState(() => t('preview.waitingEvent'));

  // 语言切换后重新生成演示数据，保证翻译立即生效
  useEffect(() => {
    setRows(makeRows(30, t));
  }, [t]);

  const columns = useMemo<Array<VxeColumnProps<Row>>>(
    () => [
      { field: 'id', title: 'ID', width: 80, fixed: 'left', sortable: true, align: 'center' },
      { field: 'name', title: t('pages.preview.vxeName'), width: 140, fixed: 'left', sortable: true },
      { field: 'role', title: t('preview.colRole'), width: 120 },
      { field: 'age', title: t('pages.preview.vxeAge'), width: 100, sortable: true, align: 'right' },
      { field: 'email', title: t('preview.colEmail'), minWidth: 220 },
      {
        field: 'status',
        title: t('common.status'),
        width: 120,
        align: 'center',
        slots: {
          default: ({ value }) =>
            value === 'active' ? (
              <span className="vxe-demo-tag ok">{t('common.enabled')}</span>
            ) : (
              <span className="vxe-demo-tag off">{t('preview.statusDisabled')}</span>
            ),
        },
      },
      {
        field: 'actions',
        title: t('common.actions'),
        width: 140,
        fixed: 'right',
        align: 'center',
        slots: {
          header: () => <em>{t('preview.actionCustomHeader')}</em>,
          default: ({ row }) => (
            <Button onClick={() => setLastEvent(t('preview.eventActionClick', { id: row.id }))} size="small" type="link">
              {t('common.edit')}
            </Button>
          ),
        },
      },
    ],
    [t],
  );

  const tableProps: VxeTableProps<Row> = {
    data: empty ? [] : rows,
    columns,
    height: 420,
    border,
    stripe,
    loading,
    showOverflow: true,
    sortConfig: { defaultSort: { field: 'id', order: 'asc' } },
    rowConfig: { isCurrent: (row) => row.id === 3 },
    onSortChange: (event) =>
      setLastEvent(
        t('preview.eventSortChange', { field: event.field, order: event.order ?? t('preview.none') }),
      ),
    onCellClick: (event) =>
      setLastEvent(t('preview.eventCellClick', { row: event.rowIndex + 1, field: event.field })),
    onColumnWidthChange: (widths) => setLastEvent(`column-width: id=${widths.id} name=${widths.name}`),
  };

  return (
    <VxeDemoShell description={t('preview.vxeBaseDesc')} title={t('preview.vxeBaseTitle')}>
      <div className="vxe-demo-event">
        {t('preview.lastEventLabel')}
        {lastEvent}
      </div>
      <Space style={{ marginBottom: 16 }} wrap>
        <Switch
          checked={border}
          checkedChildren={t('preview.bordered')}
          onChange={setBorder}
          unCheckedChildren={t('preview.borderless')}
        />
        <Switch
          checked={stripe}
          checkedChildren={t('preview.striped')}
          onChange={setStripe}
          unCheckedChildren={t('preview.plain')}
        />
        <Switch
          checked={loading}
          checkedChildren={t('common.loading')}
          onChange={setLoading}
          unCheckedChildren={t('preview.normal')}
        />
        <Switch
          checked={empty}
          checkedChildren={t('preview.emptyData')}
          onChange={setEmpty}
          unCheckedChildren={t('preview.hasData')}
        />
        <Button onClick={() => setRows(makeRows(30, t))}>{t('preview.resetData')}</Button>
      </Space>
      <VxeTable {...tableProps} />
    </VxeDemoShell>
  );
}
