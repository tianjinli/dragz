import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Switch,
  Tabs,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import { Copy, Mail, Plus, ShieldCheck, Trash2, UserRoundPen } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  createInvite,
  inviteLink,
  listInvites,
  listMembers,
  removeMember,
  revokeInvite,
  updateMember,
  type CreateInviteResult,
  type Invite,
  type Member,
} from '@/api/lifecycle';
import { listRolesApi, type ResourceRecord } from '@/api/system';
import { FormDrawer } from '@/components/FormDrawer';
import { ListPagePanel } from '@/components/ListPagePanel';
import { Page } from '@/components/Page';
import { TableActionGroup, actionColumnWidth } from '@/components/TableActionGroup';
import { TablePanel } from '@/components/TablePanel';
import { useAccessStore } from '@/stores/access';
import { formatDateTime } from '@/utils/format-time';
import { normalizeListColumns } from '@/utils/table-columns';

interface MemberFormValues {
  isTenantAdmin: boolean;
  roleIds: number[];
}

interface InviteFormValues {
  email: string;
  ttlHours: number;
  isTenantAdmin: boolean;
}

const DEFAULT_INVITE_TTL_HOURS = 72;

/**
 * Tenant membership: who is in this tenant, and who has been invited.
 *
 * Members and invitations are one screen because they answer one question —
 * "who can use this workspace". A member is a global account plus a
 * tenant-scoped row; an invitation is the pending version of that row.
 */
export function MemberPage() {
  const { t } = useTranslation();
  // userId is serialized as a string on the profile. Comparing it as a number
  // keeps "you cannot disable or remove yourself" working across the two types.
  const currentUserId = Number(useAccessStore((state) => state.userInfo?.userId) ?? 0);
  const [members, setMembers] = useState<Member[]>([]);
  const [invites, setInvites] = useState<Invite[]>([]);
  const [roles, setRoles] = useState<ResourceRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [pending, setPending] = useState<string>();
  const [tab, setTab] = useState('members');
  const [memberTarget, setMemberTarget] = useState<Member>();
  const [inviteOpen, setInviteOpen] = useState(false);
  const [issued, setIssued] = useState<CreateInviteResult>();
  const [memberForm] = Form.useForm<MemberFormValues>();
  const [inviteForm] = Form.useForm<InviteFormValues>();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [memberRows, inviteRows] = await Promise.all([listMembers(), listInvites()]);
      setMembers(memberRows);
      setInvites(inviteRows);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
    void listRolesApi()
      .then(setRoles)
      .catch(() => setRoles([]));
  }, [load]);

  // A member only knows its role codes (the row is a join with the account), so
  // the edit drawer maps codes back to ids to preselect the role picker.
  const roleIdByCode = useMemo(() => {
    const index = new Map<string, number>();
    roles.forEach((role) => {
      if (typeof role.code === 'string') index.set(role.code, Number(role.id));
    });
    return index;
  }, [roles]);

  const roleOptions = useMemo(
    () => roles.map((role) => ({ label: String(role.name ?? role.code ?? role.id), value: Number(role.id) })),
    [roles],
  );

  const openMember = (row: Member) => {
    setMemberTarget(row);
    memberForm.setFieldsValue({
      isTenantAdmin: row.isTenantAdmin,
      roleIds: row.roleCodes.map((code) => roleIdByCode.get(code)).filter((id): id is number => typeof id === 'number'),
    });
  };

  const toggleStatus = async (row: Member, enabled: boolean) => {
    const key = `${row.userId}:status`;
    setPending(key);
    try {
      await updateMember(row.userId, { status: enabled ? 1 : 0 });
      void message.success(t('system.members.statusUpdated'));
      await load();
    } catch {
      await load();
    } finally {
      setPending(undefined);
    }
  };

  const submitMember = async () => {
    if (!memberTarget) return;
    let values: MemberFormValues;
    try {
      values = await memberForm.validateFields();
    } catch {
      return;
    }
    setSaving(true);
    try {
      await updateMember(memberTarget.userId, {
        isTenantAdmin: values.isTenantAdmin,
        roleIds: values.roleIds ?? [],
      });
      void message.success(t('system.members.updated'));
      setMemberTarget(undefined);
      await load();
    } catch {
      /* the request layer surfaces the message */
    } finally {
      setSaving(false);
    }
  };

  const confirmRemove = (row: Member) => {
    Modal.confirm({
      title: t('system.members.removeConfirmTitle', { name: row.displayName || row.username }),
      content: t('system.members.removeConfirmDesc'),
      okText: t('common.confirm'),
      okButtonProps: { danger: true },
      cancelText: t('common.cancel'),
      onOk: async () => {
        await removeMember(row.userId);
        void message.success(t('system.members.removed'));
        await load();
      },
    });
  };

  const submitInvite = async () => {
    let values: InviteFormValues;
    try {
      values = await inviteForm.validateFields();
    } catch {
      return;
    }
    setSaving(true);
    try {
      const result = await createInvite({
        email: values.email.trim(),
        ttlHours: values.ttlHours,
        isTenantAdmin: values.isTenantAdmin,
      });
      setInviteOpen(false);
      setIssued(result);
      await load();
    } catch {
      /* the request layer surfaces the message */
    } finally {
      setSaving(false);
    }
  };

  const confirmRevoke = (row: Invite) => {
    Modal.confirm({
      title: t('system.members.revokeConfirmTitle', { email: row.email }),
      okText: t('common.confirm'),
      okButtonProps: { danger: true },
      cancelText: t('common.cancel'),
      onOk: async () => {
        await revokeInvite(row.id);
        void message.success(t('system.members.revoked'));
        await load();
      },
    });
  };

  const copyToken = async (token: string) => {
    try {
      await navigator.clipboard.writeText(inviteLink(token));
      void message.success(t('system.members.linkCopied'));
    } catch {
      void message.warning(t('system.members.copyFailed'));
    }
  };

  return (
    <Page>
      <Card className="table-card">
        <Tabs
          activeKey={tab}
          items={[
            { key: 'members', label: `${t('system.members.membersTab')} (${members.length})` },
            { key: 'invites', label: `${t('system.members.invitesTab')} (${invites.length})` },
          ]}
          onChange={setTab}
        />
        {tab === 'members' ? (
          <ListPagePanel
            title={t('system.members.membersTitle')}
            toolbar={{ onRefresh: () => void load() }}
          >
            <TablePanel
              bordered
              className="list-page-table"
              columns={normalizeListColumns<Member>([
                { title: t('system.members.userId'), dataIndex: 'userId', width: 100 },
                {
                  title: t('system.members.account'),
                  key: 'account',
                  width: 220,
                  render: (_: unknown, row: Member) => (
                    <Space size={6} wrap>
                      <span>{row.displayName || row.username}</span>
                      <Typography.Text type="secondary">{row.username}</Typography.Text>
                    </Space>
                  ),
                },
                { title: t('system.members.email'), dataIndex: 'email', width: 220 },
                { title: t('system.members.mobile'), dataIndex: 'mobile', width: 150 },
                {
                  title: t('system.members.roles'),
                  key: 'roles',
                  width: 220,
                  render: (_: unknown, row: Member) =>
                    row.roleCodes.length > 0 ? (
                      <Space size={4} wrap>
                        {row.roleCodes.map((code) => (
                          <Tag key={code}>{code}</Tag>
                        ))}
                      </Space>
                    ) : (
                      '—'
                    ),
                },
                {
                  title: t('system.members.tenantAdmin'),
                  dataIndex: 'isTenantAdmin',
                  width: 130,
                  render: (value: boolean, row: Member) =>
                    value ? (
                      <Tag color="gold" icon={<ShieldCheck size={12} />}>
                        {row.isPlatformOperator ? t('system.members.operatorAdmin') : t('common.yes')}
                      </Tag>
                    ) : (
                      '—'
                    ),
                },
                {
                  title: t('common.status'),
                  dataIndex: 'status',
                  width: 110,
                  render: (value: number, row: Member) => (
                    <Switch
                      checked={value === 1}
                      disabled={pending === `${row.userId}:status` || row.userId === currentUserId}
                      loading={pending === `${row.userId}:status`}
                      onChange={(next) => void toggleStatus(row, next)}
                    />
                  ),
                },
                { title: t('system.members.joinedAt'), dataIndex: 'createdAt', width: 180, render: (value: string) => formatDateTime(value) },
                {
                  title: t('common.actions'),
                  key: 'actions',
                  width: actionColumnWidth(2),
                  render: (_: unknown, row: Member) => (
                    <TableActionGroup
                      actions={[
                        {
                          icon: <UserRoundPen size={16} />,
                          label: t('common.edit'),
                          onClick: () => openMember(row),
                        },
                        {
                          icon: <Trash2 size={16} />,
                          label: t('system.members.remove'),
                          danger: true,
                          disabled: row.userId === currentUserId,
                          onClick: () => confirmRemove(row),
                        },
                      ]}
                    />
                  ),
                },
              ])}
              dataSource={members}
              loading={loading}
              pagination={false}
              rowKey="userId"
              scroll={{ x: 'max-content' }}
              sticky
            />
          </ListPagePanel>
        ) : (
          <ListPagePanel
            title={t('system.members.invitesTitle')}
            toolbar={{
              onRefresh: () => void load(),
              actions: (
                <Button icon={<Plus size={16} />} type="primary" onClick={() => setInviteOpen(true)}>
                  {t('system.members.invite')}
                </Button>
              ),
            }}
          >
            <TablePanel
              bordered
              className="list-page-table"
              columns={normalizeListColumns<Invite>([
                { title: t('system.members.email'), dataIndex: 'email', width: 240 },
                {
                  title: t('common.status'),
                  dataIndex: 'status',
                  width: 120,
                  render: (value: Invite['status']) => (
                    <Tag color={value === 'pending' ? 'processing' : value === 'accepted' ? 'success' : 'default'}>
                      {t(`invite.status.${value}`)}
                    </Tag>
                  ),
                },
                {
                  title: t('system.members.tenantAdmin'),
                  dataIndex: 'isTenantAdmin',
                  width: 110,
                  render: (value: boolean) => (value ? t('common.yes') : t('common.no')),
                },
                { title: t('invite.expiresAt'), dataIndex: 'expiresAt', width: 180, render: (value: string) => formatDateTime(value) },
                {
                  title: t('system.members.acceptedAt'),
                  dataIndex: 'acceptedAt',
                  width: 180,
                  render: (value?: string | null) => (value ? formatDateTime(value) : '—'),
                },
                { title: t('common.createdAt'), dataIndex: 'createdAt', width: 180, render: (value: string) => formatDateTime(value) },
                {
                  title: t('common.actions'),
                  key: 'actions',
                  width: actionColumnWidth(1),
                  render: (_: unknown, row: Invite) => (
                    <TableActionGroup
                      actions={[
                        {
                          icon: <Trash2 size={16} />,
                          label: t('system.members.revoke'),
                          danger: true,
                          disabled: row.status !== 'pending',
                          onClick: () => confirmRevoke(row),
                        },
                      ]}
                    />
                  ),
                },
              ])}
              dataSource={invites}
              loading={loading}
              pagination={false}
              rowKey="id"
              scroll={{ x: 'max-content' }}
              sticky
            />
          </ListPagePanel>
        )}
      </Card>

      <FormDrawer
        confirmLoading={saving}
        open={!!memberTarget}
        title={t('system.members.editTitle', { name: memberTarget?.displayName || memberTarget?.username || '' })}
        onClose={() => setMemberTarget(undefined)}
        onOk={() => void submitMember()}
      >
        <Form form={memberForm} layout="vertical" preserve={false}>
          <Form.Item
            label={t('system.members.tenantAdmin')}
            name="isTenantAdmin"
            valuePropName="checked"
            extra={t('system.members.tenantAdminHint')}
          >
            <Switch />
          </Form.Item>
          <Form.Item label={t('system.members.roles')} name="roleIds">
            <Select mode="multiple" options={roleOptions} placeholder={t('common.pleaseSelect')} />
          </Form.Item>
        </Form>
      </FormDrawer>

      <FormDrawer
        confirmLoading={saving}
        open={inviteOpen}
        title={t('system.members.invite')}
        onClose={() => setInviteOpen(false)}
        onOk={() => void submitInvite()}
      >
        <Form
          form={inviteForm}
          initialValues={{ ttlHours: DEFAULT_INVITE_TTL_HOURS, isTenantAdmin: false }}
          layout="vertical"
          preserve={false}
        >
          <Form.Item
            label={t('system.members.email')}
            name="email"
            rules={[
              { required: true, whitespace: true, message: t('system.members.emailRequired') },
              { type: 'email', message: t('validation.email') },
            ]}
          >
            <Input maxLength={255} placeholder={t('system.members.emailPlaceholder')} prefix={<Mail size={14} />} />
          </Form.Item>
          <Form.Item
            extra={t('system.members.ttlHint')}
            label={t('system.members.ttlHours')}
            name="ttlHours"
            rules={[{ type: 'number', min: 1, max: 24 * 30, message: t('system.members.ttlRange') }]}
          >
            <InputNumber className="full-width" max={24 * 30} min={1} precision={0} />
          </Form.Item>
          <Form.Item
            extra={t('system.members.tenantAdminHint')}
            label={t('system.members.tenantAdmin')}
            name="isTenantAdmin"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
        </Form>
      </FormDrawer>

      <Modal
        footer={
          <Button type="primary" onClick={() => setIssued(undefined)}>
            {t('common.close')}
          </Button>
        }
        open={!!issued}
        title={t('system.members.inviteCreated')}
        onCancel={() => setIssued(undefined)}
      >
        <Alert
          showIcon
          description={t('system.members.inviteTokenWarning')}
          style={{ marginBottom: 16 }}
          title={t('system.members.inviteTokenTitle')}
          type="warning"
        />
        <Typography.Paragraph copyable={{ text: issued ? inviteLink(issued.token) : '' }} code>
          {issued ? inviteLink(issued.token) : ''}
        </Typography.Paragraph>
        <Tooltip title={t('system.members.copyLink')}>
          <Button icon={<Copy size={16} />} onClick={() => issued && void copyToken(issued.token)}>
            {t('system.members.copyLink')}
          </Button>
        </Tooltip>
      </Modal>
    </Page>
  );
}
