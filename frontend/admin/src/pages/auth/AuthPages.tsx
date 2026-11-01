import { useCallback, useEffect, useMemo, useState } from 'react';
import { Alert, App, Button, Checkbox, Divider, Form, Input, Space } from 'antd';
import type { AxiosError } from 'axios';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
import {
  exchangeOAuthApi,
  issueCaptchaApi,
  loginChallengeApi,
  publicAuthProvidersApi,
  startOAuthApi,
} from '@/api/auth';
import { REMEMBER_ME_KEY } from '@/constants/auth';
import { AuthLayout, AuthTitle } from '@/layout/AuthLayout';
import { useAccessStore } from '@/stores/access';
import { useAuthStore } from '@/stores/auth';
import type { ApiEnvelope } from '@/types';
import { safeRedirectPath } from '@/utils/redirect';

type OAuthProvider = { id: number; name: string };

type LoginLockPayload = {
  attemptsRemaining?: number;
  captchaRequired?: boolean;
  failedCount?: number;
  kind?: 'cooldown' | 'lock' | string;
  lockedUntil?: string;
  retryAfterSeconds?: number;
};

const LOGIN_COOLDOWN_MS = 3000;

function resolveLockUntil(payload?: LoginLockPayload): number {
  if (payload?.kind === 'lock') {
    if (payload.lockedUntil) {
      const ms = Date.parse(payload.lockedUntil);
      if (!Number.isNaN(ms) && ms > Date.now()) return ms;
    }
    if (payload.retryAfterSeconds && payload.retryAfterSeconds > 0) {
      return Date.now() + payload.retryAfterSeconds * 1000;
    }
  }
  if (payload?.lockedUntil) {
    const ms = Date.parse(payload.lockedUntil);
    if (!Number.isNaN(ms) && ms > Date.now()) return ms;
  }
  if (payload?.retryAfterSeconds && payload.retryAfterSeconds > 0) {
    return Date.now() + payload.retryAfterSeconds * 1000;
  }
  return Date.now() + LOGIN_COOLDOWN_MS;
}

function formatCountdown(ms: number) {
  const total = Math.max(0, Math.ceil(ms / 1000));
  const hours = Math.floor(total / 3600);
  const mins = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  if (hours > 0) return `${hours}:${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
  return `${mins}:${String(secs).padStart(2, '0')}`;
}

export function LoginPage() {
  const { t } = useTranslation();
  const { message: antMessage } = App.useApp();
  const navigate = useNavigate();
  const location = useLocation();
  const { login, loading } = useAuthStore();
  const [form] = Form.useForm();
  const [remember, setRemember] = useState(false);
  const [providers, setProviders] = useState<OAuthProvider[]>([]);
  const [startingId, setStartingId] = useState<number>();
  const [formError, setFormError] = useState('');
  const [lockUntil, setLockUntil] = useState<number | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const [captchaRequired, setCaptchaRequired] = useState(false);
  const [captchaId, setCaptchaId] = useState('');
  const [captchaImage, setCaptchaImage] = useState('');
  const [captchaLoading, setCaptchaLoading] = useState(false);

  const locked = lockUntil !== null && lockUntil > now;
  const lockRemaining = useMemo(() => (lockUntil ? Math.max(0, lockUntil - now) : 0), [lockUntil, now]);

  const refreshCaptcha = useCallback(async () => {
    setCaptchaLoading(true);
    try {
      const result = await issueCaptchaApi();
      setCaptchaId(result.captchaId);
      setCaptchaImage(result.imageBase64);
      form.setFieldValue('captchaCode', '');
    } catch {
      setCaptchaId('');
      setCaptchaImage('');
    } finally {
      setCaptchaLoading(false);
    }
  }, [form]);

  const ensureCaptcha = useCallback(async () => {
    setCaptchaRequired(true);
    await refreshCaptcha();
  }, [refreshCaptcha]);

  useEffect(() => {
    const saved = localStorage.getItem(REMEMBER_ME_KEY) || '';
    if (saved) {
      form.setFieldValue('username', saved);
      setRemember(true);
      void loginChallengeApi(saved)
        .then(async (result) => {
          if (result.captchaRequired) await ensureCaptcha();
        })
        .catch(() => undefined);
    }
  }, [ensureCaptcha, form]);

  useEffect(() => {
    void publicAuthProvidersApi()
      .then((items) => setProviders(items.filter((item): item is OAuthProvider => typeof item.id === 'number' && typeof item.name === 'string')))
      .catch(() => setProviders([]));
  }, []);

  useEffect(() => {
    if (!locked) return;
    const timer = window.setInterval(() => setNow(Date.now()), 250);
    return () => window.clearInterval(timer);
  }, [locked]);

  useEffect(() => {
    if (lockUntil !== null && lockUntil <= now) {
      setLockUntil(null);
      setFormError('');
    }
  }, [lockUntil, now]);

  const submit = async (values: { captchaCode?: string; password: string; username: string }) => {
    if (locked) return;
    setFormError('');
    try {
      localStorage.setItem(REMEMBER_ME_KEY, remember ? values.username : '');
      const home = await login({
        username: values.username,
        password: values.password,
        deviceName: navigator.userAgent,
        captchaId: captchaRequired ? captchaId : undefined,
        captchaCode: captchaRequired ? values.captchaCode : undefined,
      });
      const user = useAccessStore.getState().userInfo;
      if (user?.realName) {
        antMessage.success(`${t('auth.loginSuccessDesc')}:${user.realName}`);
      } else {
        antMessage.success(t('auth.loginSuccess'));
      }
      setCaptchaRequired(false);
      setCaptchaId('');
      setCaptchaImage('');
      navigate(safeRedirectPath(new URLSearchParams(location.search).get('redirect'), home), { replace: true });
    } catch (error) {
      const ax = error as AxiosError<ApiEnvelope<LoginLockPayload>>;
      const envelope = ax.response?.data;
      const payload = envelope?.data;
      setFormError(envelope?.message || t('auth.loginFailed'));
      setLockUntil(resolveLockUntil(payload));
      setNow(Date.now());
      if (payload?.captchaRequired || captchaRequired) {
        await ensureCaptcha();
      }
    }
  };

  const startOAuth = async (provider: OAuthProvider) => {
    setStartingId(provider.id);
    try {
      const result = await startOAuthApi(provider.id);
      window.location.assign(result.authorizationUrl);
    } finally {
      setStartingId(undefined);
    }
  };

  const alertMessage = (() => {
    if (!formError) return '';
    if (locked && lockRemaining > 0) {
      return t('auth.lockCountdown', { error: formError, countdown: formatCountdown(lockRemaining) });
    }
    return formError;
  })();

  return (
    <AuthLayout>
      <AuthTitle />
      {alertMessage ? <Alert className="auth-login-alert" showIcon type="error" message={alertMessage} /> : null}
      <Form
        className="auth-login-form"
        form={form}
        layout="vertical"
        onFinish={(values) => void submit(values)}
        requiredMark={false}
      >
        <Form.Item name="username" rules={[{ required: true, message: t('auth.usernameRequired') }]}>
          <Input placeholder={t('auth.usernamePlaceholder')} size="large" />
        </Form.Item>
        <Form.Item name="password" rules={[{ required: true, message: t('auth.passwordRequired') }]}>
          <Input.Password placeholder={t('auth.passwordPlaceholder')} size="large" />
        </Form.Item>
        {captchaRequired ? (
          <Form.Item name="captchaCode" rules={[{ required: true, message: t('auth.captchaRequired') }]}>
            <Space.Compact className="auth-captcha-row">
              <Input placeholder={t('auth.captchaPlaceholder')} size="large" />
              <button
                aria-label={t('auth.captchaRefresh')}
                className="auth-captcha-image"
                disabled={captchaLoading || locked}
                type="button"
                onClick={() => void refreshCaptcha()}
              >
                {captchaImage ? <img alt={t('auth.captcha')} src={captchaImage} /> : <span>{t('auth.captchaRefresh')}</span>}
              </button>
            </Space.Compact>
          </Form.Item>
        ) : null}
        <div className="auth-remember-row">
          <Checkbox checked={remember} onChange={(event) => setRemember(event.target.checked)}>
            {t('auth.rememberMe')}
          </Checkbox>
        </div>
        <Button
          block
          className={`auth-login-button${loading ? ' is-loading' : ''}`}
          disabled={locked}
          htmlType="submit"
          loading={loading}
          size="large"
          type="primary"
        >
          {t('login')}
        </Button>
      </Form>
      {providers.length > 0 ? (
        <div className="auth-oauth-block">
          <Divider plain>{t('auth.oauthDivider')}</Divider>
          <div className="auth-oauth-list">
            {providers.map((provider) => (
              <Button block key={provider.id} loading={startingId === provider.id} onClick={() => void startOAuth(provider)}>
                {provider.name}
              </Button>
            ))}
          </div>
        </div>
      ) : null}
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
        navigate('/analytics', { replace: true });
      })
      .catch(() => setError(t('auth.oauthFailed')));
  }, [location.search, navigate, t]);
  return (
    <AuthLayout>
      <AuthTitle subtitle={t('auth.oauthSubtitle')} title={t('auth.oauthTitle')} />
      {error ? <Alert type="error" message={error} /> : <Alert showIcon type="info" message={t('auth.oauthProcessing')} />}
    </AuthLayout>
  );
}
