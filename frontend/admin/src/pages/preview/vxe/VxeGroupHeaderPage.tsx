import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Space, Switch } from 'antd';
import { VxeTable } from '@/components/vxe-table';
import type { VxeColumnProps } from '@/components/vxe-table';
import { VxeDemoShell } from './VxeDemoShell';

interface Row {
  id: number;
  name: string;
  age: number;
  province: string;
  city: string;
  address: string;
  zip: string;
}

export default function VxeGroupHeaderPage() {
  const { t } = useTranslation();
  const [border, setBorder] = useState(true);
  const [lastEvent, setLastEvent] = useState(() => t('preview.waitingEvent'));

  const rows = useMemo<Row[]>(
    () => [
      {
        id: 1,
        name: t('preview.personZhang'),
        age: 32,
        province: t('preview.provinceGuangdong'),
        city: t('preview.cityShenzhen'),
        address: t('preview.addressNanshan'),
        zip: '518000',
      },
      {
        id: 2,
        name: t('preview.personLi'),
        age: 25,
        province: t('preview.provinceZhejiang'),
        city: t('preview.cityHangzhou'),
        address: t('preview.addressXihu'),
        zip: '310000',
      },
      {
        id: 3,
        name: t('preview.personWang'),
        age: 41,
        province: t('preview.provinceJiangsu'),
        city: t('preview.cityNanjing'),
        address: t('preview.addressGulou'),
        zip: '210000',
      },
      {
        id: 4,
        name: t('preview.personZhao'),
        age: 28,
        province: t('preview.provinceGuangdong'),
        city: t('preview.cityGuangzhou'),
        address: t('preview.addressTianhe'),
        zip: '510000',
      },
      {
        id: 5,
        name: t('preview.personSun'),
        age: 36,
        province: t('preview.provinceSichuan'),
        city: t('preview.cityChengdu'),
        address: t('preview.addressJinjiang'),
        zip: '610000',
      },
      {
        id: 6,
        name: t('preview.personZhou'),
        age: 23,
        province: t('preview.provinceZhejiang'),
        city: t('preview.cityNingbo'),
        address: t('preview.addressHaishu'),
        zip: '315000',
      },
    ],
    [t],
  );

  const columns = useMemo<Array<VxeColumnProps<Row>>>(
    () => [
      { field: 'id', title: 'ID', width: 70, fixed: 'left', align: 'center' },
      {
        title: t('pages.preview.vxeGroup'),
        children: [
          { field: 'name', title: t('pages.preview.vxeName'), width: 110, fixed: 'left', sortable: true },
          { field: 'age', title: t('pages.preview.vxeAge'), width: 90, sortable: true, align: 'right' },
        ],
      },
      {
        title: t('preview.groupAddress'),
        children: [
          {
            title: t('preview.groupRegion'),
            children: [
              { field: 'province', title: t('preview.colProvince'), width: 100 },
              { field: 'city', title: t('pages.preview.vxeCity'), width: 100 },
            ],
          },
          { field: 'address', title: t('preview.colAddress'), minWidth: 240 },
          { field: 'zip', title: t('preview.colZip'), width: 100, align: 'center' },
        ],
      },
      { field: 'extra', title: t('preview.colHiddenRemark'), width: 120, visible: false },
    ],
    [t],
  );

  return (
    <VxeDemoShell description={t('preview.vxeGroupHeaderDesc')} title={t('preview.vxeGroupHeaderTitle')}>
      <div className="vxe-demo-event">
        {t('preview.lastEventLabel')}
        {lastEvent}
      </div>
      <Space style={{ marginBottom: 16 }}>
        <Switch
          checked={border}
          checkedChildren={t('preview.bordered')}
          onChange={setBorder}
          unCheckedChildren={t('preview.borderless')}
        />
      </Space>
      <VxeTable
        border={border}
        columns={columns}
        data={rows}
        maxHeight={360}
        showOverflow
        stripe
        onCellClick={(event) =>
          setLastEvent(t('preview.eventCellClick', { row: event.rowIndex + 1, field: event.field }))
        }
        onHeaderCellClick={(event) =>
          setLastEvent(`header-cell-click: ${event.column.field ?? event.column.title}`)
        }
        onSortChange={(event) =>
          setLastEvent(
            t('preview.eventSortChange', { field: event.field, order: event.order ?? t('preview.none') }),
          )
        }
      />
    </VxeDemoShell>
  );
}
