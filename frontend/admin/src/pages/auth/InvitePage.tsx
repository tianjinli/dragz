import { useCallback, useEffect, useState } from 'react';
import { Alert, Button, Descriptions, Spin, Tag } from 'antd';
import type { AxiosError } from 'axios';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { acceptInvite, previewInvite, type InvitePreview } from '@/api/lifecycle';
import { AuthLayout, AuthTitle } from '@/layout/AuthLayout';
import { useAccessStore } from '@/stores/access';
import { useAuthStore } from '@/stores/auth';
import type { ApiEnvelope } from '@/types';
import { formatDateTime } from '@/utils/format-time';

/**
 * Invitation landing page.
 *
 * The link is public: the visitor sees which tenant invited them and which
 * mailbox it was sent to before signing in. Accepting is the step that needs a
 * session, because the membership attaches to an account — so a guest is sent
 * to the login page and comes straight back here.
 */
export function InvitePage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { token = '' } = useParams();
  const accessToken = useAccessStore((state) => state.accessToken);
  const switchTenant = useAuthStore((state) => state.switchTenant);
  const [preview, setPreview] = useState<InvitePreview>();
  const [loading, setLoading] = useState(true);
  const [accepting, setAccepting] = useState(false);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    if (!token) {
      setLoading(false);
      setError(t('invite.invalid'));
      return;
    }
    setLoading(true);
    try {
      setPreview(await previewInvite(token));
      setError('');
    } catch (err) {
      const envelope = (err as AxiosError<ApiEnvelope<unknown>>).response?.data;
      setError(envelope?.message || t('invite.invalid'));
    } finally {
      setLoading(false);
    }
  }, [t, token]);

  useEffect(() => {
    void load();
  }, [load]);

  const accept = async () => {
    setAccepting(true);
    setError('');
    try {
      const result = await acceptInvite(token);
      // The invitation only grants membership. Opening the new tenant is a
      // separate step, and doing it here is what makes the link end somewhere
      // useful instead of on a page the account cannot see yet.
      await switchTenant(result.tenantId);
      navigate(useAccessStore.getState().userInfo?.homePath || '/analytics', { replace: true });
    } catch (err) {
      const envelope = (err as AxiosError<ApiEnvelope<unknown>>).response?.data;
      if (envelope?.code === 401) {
        navigate(`/auth/login?redirect=${encodeURIComponent(`/auth/invite/${token}`)}`, { replace: true });
        return;
      }
      setError(envelope?.message || t('invite.acceptFailed'));
    } finally {
      setAccepting(false);
    }
  };

  const status = preview?.status;
  const pending = status === 'pending';

  return (
    <AuthLayout>
      <AuthTitle subtitle={t('invite.subtitle')} title={t('invite.title')} />
      {loading ? (
        <div className="startup-loading">
          <Spin size="large" />
        </div>
      ) : error && !preview ? (
        <>
          <Alert showIcon title={error} type="error" />
          <div className="auth-links">
            <Link to="/auth/login">{t('signup.backToLogin')}</Link>
          </div>
        </>
      ) : preview ? (
        <>
          {error ? <Alert showIcon style={{ marginBottom: 16 }} title={error} type="error" /> : null}
          <Descriptions column={1} size="small">
            <Descriptions.Item label={t('invite.tenant')}>{preview.tenantName}</Descriptions.Item>
            <Descriptions.Item label={t('invite.email')}>{preview.email}</Descriptions.Item>
            <Descriptions.Item label={t('common.status')}>
              <Tag color={pending ? 'processing' : status === 'accepted' ? 'success' : 'default'}>
                {t(`invite.status.${status ?? 'pending'}`)}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label={t('invite.expiresAt')}>{formatDateTime(preview.expiresAt)}</Descriptions.Item>
          </Descriptions>
          {status === 'accepted' ? (
            <Alert
              showIcon
              description={t('invite.alreadyAcceptedDesc')}
              style={{ marginTop: 16 }}
              title={t('invite.alreadyAccepted')}
              type="success"
            />
          ) : pending ? (
            accessToken ? (
              <Button
                block
                className="auth-login-submit"
                loading={accepting}
                size="large"
                style={{ marginTop: 16 }}
                type="primary"
                onClick={() => void accept()}
              >
                {t('invite.accept')}
              </Button>
            ) : (
              <Button
                block
                className="auth-login-submit"
                size="large"
                style={{ marginTop: 16 }}
                type="primary"
                onClick={() => navigate(`/auth/login?redirect=${encodeURIComponent(`/auth/invite/${token}`)}`)}
              >
                {t('invite.signInToAccept')}
              </Button>
            )
          ) : (
            <Alert
              showIcon
              description={t('invite.notPendingDesc')}
              style={{ marginTop: 16 }}
              title={t('invite.notPending')}
              type="warning"
            />
          )}
        </>
      ) : null}
    </AuthLayout>
  );
}
