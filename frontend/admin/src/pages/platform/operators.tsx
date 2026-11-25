import { useCallback, useEffect, useState } from 'react';
import { Button, Card, Form, Input, Modal, Select, Space, Tag, Typography, message } from 'antd';
import { Plus, ShieldOff, ShieldUser } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  grantPlatformOperator,
  listPlatformOperators,
  revokePlatformOperator,
  type PlatformOperator,
} from '@/api/lifecycle';
import { listUsersApi } from '@/api/system';
import { FormDrawer } from '@/components/FormDrawer';
import { ListPagePanel } from '@/components/ListPagePanel';
import { Page } from '@/components/Page';
import { TableActionGroup, actionColumnWidth } from '@/components/TableActionGroup';
import { TablePanel } from '@/components/TablePanel';
import { formatDateTime } from '@/utils/format-time';
import { normalizeListColumns } from '@/utils/table-columns';

interface GrantValues {
  note: string;
  userId: number;
}

interface UserOption {
  label: string;
  value: number;
}

/**
 * The platform staff directory.
 *
 * Platform staff can act across tenants (impersonate, edit plans, read every
 * invoice), so the roster is explicit data rather than a flag on a membership
 * row: removing someone from a tenant must not silently remove their platform
 * access, and granting platform access must not require a tenant in the first
 * place.
 */
export function PlatformOperatorsPage() {
  const { t } = useTranslation();
  const [items, setItems] = useState<PlatformOperator[]>([]);
  const [options, setOptions] = useState<UserOption[]>([]);
  const [searching, setSearching] = useState(false);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm<GrantValues>();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setItems(await listPlatformOperators());
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const searchUsers = useCallback(async (keyword: string) => {
    setSearching(true);
    try {
      const result = await listUsersApi({ keyword: keyword.trim(), page: 1, pageSize: 20 });
      setOptions(
        result.items.map((item) => ({
          label: `${String(item.realName ?? item.username ?? item.id)} · ${String(item.username ?? item.id)}`,
          value: Number(item.id),
        })),
      );
    } catch {
      setOptions([]);
    } finally {
      setSearching(false);
    }
  }, []);

  const submit = async () => {
    let values: GrantValues;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    setSaving(true);
    try {
      await grantPlatformOperator(values.userId, values.note?.trim() ?? '');
      void message.success(t('platform.operators.granted'));
      setOpen(false);
      form.resetFields();
      await load();
    } catch {
      /* the request layer surfaces the message */
    } finally {
      setSaving(false);
    }
  };

  const confirmRevoke = (row: PlatformOperator) => {
    Modal.confirm({
      title: t('platform.operators.revokeConfirmTitle', { name: row.displayName || row.username }),
      content: t('platform.operators.revokeConfirmDesc'),
      okText: t('common.confirm'),
      okButtonProps: { danger: true },
      cancelText: t('common.cancel'),
      onOk: async () => {
        await revokePlatformOperator(row.userId);
        void message.success(t('platform.operators.revoked'));
        await load();
      },
    });
  };

  return (
    <Page>
      <Card className="table-card fit-list">
        <ListPagePanel
          title={t('platform.operators.title')}
          toolbar={{
            onRefresh: () => void load(),
            actions: (
              <Button
                icon={<Plus size={16} />}
                type="primary"
                onClick={() => {
                  void searchUsers('');
                  setOpen(true);
                }}
              >
                {t('platform.operators.grant')}
              </Button>
            ),
          }}
        >
          <TablePanel
            bordered
            className="list-page-table"
            columns={normalizeListColumns<PlatformOperator>([
              { title: t('system.members.userId'), dataIndex: 'userId', width: 100 },
              {
                title: t('system.members.account'),
                key: 'account',
                width: 240,
                render: (_: unknown, row: PlatformOperator) => (
                  <Space size={6} wrap>
                    <Tag color="gold" icon={<ShieldUser size={12} />}>
                      {row.displayName || row.username}
                    </Tag>
                    <Typography.Text type="secondary">{row.username}</Typography.Text>
                  </Space>
                ),
              },
              { title: t('system.members.email'), dataIndex: 'email', width: 220 },
              { title: t('platform.operators.note'), dataIndex: 'note', render: (value: string) => value || '—' },
              {
                title: t('platform.operators.grantedBy'),
                dataIndex: 'grantedBy',
                width: 140,
                render: (value?: number | null) => (value ? value : '—'),
              },
              {
                title: t('platform.operators.grantedAt'),
                dataIndex: 'createdAt',
                width: 180,
                render: (value: string) => formatDateTime(value),
              },
              {
                title: t('common.actions'),
                key: 'actions',
                width: actionColumnWidth(1),
                render: (_: unknown, row: PlatformOperator) => (
                  <TableActionGroup
                    actions={[
                      {
                        icon: <ShieldOff size={16} />,
                        label: t('platform.operators.revoke'),
                        danger: true,
                        onClick: () => confirmRevoke(row),
                      },
                    ]}
                  />
                ),
              },
            ])}
            dataSource={items}
            loading={loading}
            pagination={false}
            rowKey="userId"
            scroll={{ x: 'max-content' }}
            sticky
          />
        </ListPagePanel>
      </Card>

      <FormDrawer
        confirmLoading={saving}
        form={form}
        okText={t('platform.operators.grant')}
        open={open}
        title={t('platform.operators.grant')}
        onClose={() => setOpen(false)}
        onOk={() => void submit()}
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item
            extra={t('platform.operators.userHint')}
            label={t('platform.operators.user')}
            name="userId"
            rules={[{ required: true, message: t('platform.operators.userRequired') }]}
          >
            <Select
              filterOption={false}
              loading={searching}
              options={options}
              placeholder={t('platform.operators.userPlaceholder')}
              showSearch
              onSearch={(value) => void searchUsers(value)}
            />
          </Form.Item>
          <Form.Item label={t('platform.operators.note')} name="note">
            <Input.TextArea maxLength={200} placeholder={t('platform.operators.notePlaceholder')} rows={3} />
          </Form.Item>
        </Form>
      </FormDrawer>
    </Page>
  );
}
