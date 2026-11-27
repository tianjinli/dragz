import { useState } from 'react';
import { App as AntApp, Button, Form, Input, Modal } from 'antd';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { changePasswordApi } from '@/api/auth';
import { useAccessStore } from '@/stores/access';
import { useAuthStore } from '@/stores/auth';
import { formValidator, passwordError } from '@/utils/user-validation';

interface PasswordValues {
  confirm: string;
  newPassword: string;
  oldPassword: string;
}

/**
 * Blocks the app until an account created with an initial password replaces
 * it. There is deliberately no close button: the backend rejects every other
 * protected route while sys_user.must_change_password is set.
 */
export function ForcePasswordChangeModal() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { message } = AntApp.useApp();
  const required = useAccessStore((state) => Boolean(state.userInfo?.mustChangePassword));
  const minLength = useAccessStore((state) => state.userInfo?.passwordMinLength ?? 8);
  const username = useAccessStore((state) => state.userInfo?.username ?? '');
  const [form] = Form.useForm<PasswordValues>();
  const [loading, setLoading] = useState(false);

  const submit = async (values: PasswordValues) => {
    setLoading(true);
    try {
      const result = await changePasswordApi({
        newPassword: values.newPassword,
        oldPassword: values.oldPassword,
      });
      form.resetFields();
      // ChangeSelfPassword bumps token_version, so the current access token
      // is already invalid. Drop the session instead of letting requests fail
      // one by one with 401.
      if (result.reloginRequired !== false) {
        await useAuthStore.getState().logout();
        navigate('/auth/login', { replace: true });
      } else {
        await useAuthStore.getState().loadAccess();
      }
      void message.success(t('auth.passwordChangedRelogin'));
    } catch {
      // The request layer already surfaces the envelope message.
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      closable={false}
      footer={null}
      keyboard={false}
      mask={{ closable: false }}
      open={required}
      title={t('auth.firstPasswordTitle')}
      width={440}
    >
      <p style={{ marginBottom: 16 }}>{t('auth.firstPasswordDesc')}</p>
      <Form
        form={form}
        layout="vertical"
        preserve={false}
        onFinish={(values) => void submit(values)}
      >
        <Form.Item
          label={t('pages.profile.currentPassword')}
          name="oldPassword"
          rules={[{ required: true }]}
        >
          <Input.Password autoComplete="current-password" />
        </Form.Item>
        <Form.Item
          extra={t('pages.profile.passwordHint', { min: minLength })}
          label={t('pages.profile.newPassword')}
          name="newPassword"
          rules={[
            { required: true },
            { validator: formValidator((value) => passwordError(value || '', minLength, username)) },
          ]}
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Form.Item
          dependencies={['newPassword']}
          label={t('pages.profile.confirmPassword')}
          name="confirm"
          rules={[
            { required: true },
            ({ getFieldValue }) => ({
              validator: (_, value: string) =>
                value === getFieldValue('newPassword')
                  ? Promise.resolve()
                  : Promise.reject(new Error(t('validation.passwordMismatch'))),
            }),
          ]}
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Form.Item style={{ marginBottom: 0 }}>
          <Button block htmlType="submit" loading={loading} size="large" type="primary">
            {t('auth.confirmChangePassword')}
          </Button>
        </Form.Item>
      </Form>
    </Modal>
  );
}
