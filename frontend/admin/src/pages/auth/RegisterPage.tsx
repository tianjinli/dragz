import { useEffect, useState } from 'react';
import { Alert, Button, Form, Input, Spin } from 'antd';
import type { AxiosError } from 'axios';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from 'react-router-dom';
import { getSignupStatus, registerTenant, type SignupInput, type SignupResult } from '@/api/lifecycle';
import { AuthLayout, AuthTitle } from '@/layout/AuthLayout';
import type { ApiEnvelope } from '@/types';
import {
  asciiCredentialFromInput,
  emailError,
  formValidator,
  mobileError,
  passwordError,
  usernameError,
} from '@/utils/user-validation';

type SignupValues = SignupInput & { confirmPassword: string };

const TENANT_CODE_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_-]{1,63}$/;
/** Fallback while /auth/signup-status is in flight; the server value wins. */
const DEFAULT_MIN_LENGTH = 8;

/**
 * Public self-service signup. One submission creates the tenant, its first
 * administrator and a trial subscription, so the page is deliberately blunt
 * about what it is doing — the visitor is about to become a tenant owner.
 *
 * Registration can be switched off per deployment (feature.signup.enabled), in
 * which case the page says so instead of offering a form that always fails.
 */
export function RegisterPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [form] = Form.useForm<SignupValues>();
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [minLength, setMinLength] = useState(DEFAULT_MIN_LENGTH);
  const [created, setCreated] = useState<SignupResult>();
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    void getSignupStatus()
      .then((status) => {
        if (cancelled) return;
        setEnabled(status.enabled);
        if (status.passwordMinLength > 0) setMinLength(status.passwordMinLength);
      })
      .catch(() => {
        if (!cancelled) setEnabled(false);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const submit = async (values: SignupValues) => {
    setSubmitting(true);
    setError('');
    try {
      const result = await registerTenant({
        tenantName: values.tenantName.trim(),
        tenantCode: values.tenantCode?.trim() || undefined,
        username: values.username.trim(),
        password: values.password,
        email: values.email?.trim() || undefined,
        mobile: values.mobile?.trim() || undefined,
        displayName: values.displayName?.trim() || undefined,
      });
      setCreated(result);
    } catch (err) {
      const envelope = (err as AxiosError<ApiEnvelope<unknown>>).response?.data;
      setError(envelope?.message || t('signup.failed'));
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <AuthLayout>
        <div className="startup-loading">
          <Spin size="large" />
        </div>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout>
      <AuthTitle subtitle={t('signup.subtitle')} title={t('signup.title')} />
      {!enabled ? (
        <>
          <Alert showIcon title={t('signup.closedTitle')} description={t('signup.closedDesc')} type="warning" />
          <div className="auth-links">
            <Link to="/auth/login">{t('signup.backToLogin')}</Link>
          </div>
        </>
      ) : created ? (
        <>
          <Alert
            showIcon
            description={t('signup.successDesc', { tenant: created.tenantName, account: created.tenantCode })}
            title={t('signup.successTitle')}
            type="success"
          />
          <Button block className="auth-login-submit" size="large" type="primary" onClick={() => navigate('/auth/login')}>
            {t('signup.goToLogin')}
          </Button>
        </>
      ) : (
        <>
          {error ? <Alert showIcon style={{ marginBottom: 16 }} title={error} type="error" /> : null}
          <Form form={form} layout="vertical" requiredMark={false} onFinish={(values) => void submit(values)}>
            <Form.Item
              label={t('signup.tenantName')}
              name="tenantName"
              rules={[
                { required: true, whitespace: true, message: t('signup.tenantNameRequired') },
                { max: 128, message: t('signup.tenantNameRequired') },
              ]}
            >
              <Input maxLength={128} placeholder={t('signup.tenantNamePlaceholder')} size="large" />
            </Form.Item>
            <Form.Item
              extra={t('signup.tenantCodeHint')}
              label={t('signup.tenantCode')}
              name="tenantCode"
              rules={[
                {
                  validator: async (_rule, value?: string) => {
                    const code = (value ?? '').trim();
                    if (!code || TENANT_CODE_PATTERN.test(code)) return;
                    throw new Error(t('signup.tenantCodeInvalid'));
                  },
                },
              ]}
            >
              <Input maxLength={64} placeholder={t('signup.tenantCodePlaceholder')} size="large" />
            </Form.Item>
            <Form.Item
              getValueFromEvent={asciiCredentialFromInput}
              label={t('auth.username')}
              name="username"
              rules={[{ required: true, message: t('auth.usernameRequired') }, { validator: formValidator(usernameError) }]}
            >
              <Input autoComplete="username" maxLength={64} placeholder={t('auth.usernamePlaceholder')} size="large" />
            </Form.Item>
            <Form.Item label={t('signup.displayName')} name="displayName">
              <Input maxLength={64} placeholder={t('signup.displayNamePlaceholder')} size="large" />
            </Form.Item>
            <Form.Item
              extra={t('signup.passwordHint', { min: minLength })}
              getValueFromEvent={asciiCredentialFromInput}
              label={t('auth.password')}
              name="password"
              rules={[
                { required: true, message: t('auth.passwordRequired') },
                { validator: formValidator((value) => passwordError(value ?? '', minLength)) },
              ]}
            >
              <Input.Password autoComplete="new-password" maxLength={64} placeholder={t('auth.passwordPlaceholder')} size="large" />
            </Form.Item>
            <Form.Item
              dependencies={['password']}
              getValueFromEvent={asciiCredentialFromInput}
              label={t('signup.confirmPassword')}
              name="confirmPassword"
              rules={[
                { required: true, message: t('signup.confirmPasswordRequired') },
                ({ getFieldValue }) => ({
                  validator: async (_rule, value?: string) => {
                    if (value === getFieldValue('password')) return;
                    throw new Error(t('validation.passwordMismatch'));
                  },
                }),
              ]}
            >
              <Input.Password autoComplete="new-password" maxLength={64} size="large" />
            </Form.Item>
            <Form.Item label={t('signup.email')} name="email" rules={[{ validator: formValidator(emailError) }]}>
              <Input autoComplete="email" maxLength={255} size="large" />
            </Form.Item>
            <Form.Item label={t('signup.mobile')} name="mobile" rules={[{ validator: formValidator(mobileError) }]}>
              <Input autoComplete="tel" maxLength={32} size="large" />
            </Form.Item>
            <Button block className="auth-login-submit" htmlType="submit" loading={submitting} size="large" type="primary">
              {t('signup.submit')}
            </Button>
          </Form>
          <div className="auth-links">
            <span>{t('signup.haveAccount')}</span>
            <Link to="/auth/login">{t('signup.backToLogin')}</Link>
          </div>
        </>
      )}
    </AuthLayout>
  );
}
