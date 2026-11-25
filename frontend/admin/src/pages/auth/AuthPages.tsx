import { useEffect, useState } from 'react';
import { Alert } from 'antd';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
import { exchangeOAuthApi } from '@/api/auth';
import { LoginFormPanel } from '@/components/auth/LoginFormPanel';
import { AuthLayout, AuthTitle } from '@/layout/AuthLayout';
import {
  loginLockAccentFromRatio,
  loginLockProgressScale,
  loginLockTintFromRatio,
  resolveLockTotalMs,
} from '@/pages/auth/login-lock';
import { useAccessStore } from '@/stores/access';
import { useAuthStore } from '@/stores/auth';

export { loginLockAccentFromRatio, loginLockProgressScale, loginLockTintFromRatio, resolveLockTotalMs };

export function LoginPage() {
  return (
    <AuthLayout>
      <LoginFormPanel />
    </AuthLayout>
  );
}

export function OAuthCallbackPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [error, setError] = useState('');
  useEffect(() => {
    const code = new URLSearchParams(location.search).get('code');
    if (!code) {
      setError(t('auth.missingCode'));
      return;
    }
    void exchangeOAuthApi(code)
      .then(async (token) => {
        useAccessStore.getState().setAccessToken(token.accessToken);
        await useAuthStore.getState().loadAccess();
        navigate(useAccessStore.getState().userInfo?.homePath || '/analytics', { replace: true });
      })
      .catch(() => setError(t('auth.oauthFailed')));
  }, [location.search, navigate, t]);
  return (
    <AuthLayout>
      <AuthTitle subtitle={t('auth.oauthSubtitle')} title={t('auth.oauthTitle')} />
      {error ? <Alert type="error" title={error} /> : <Alert showIcon type="info" title={t('auth.oauthProcessing')} />}
    </AuthLayout>
  );
}