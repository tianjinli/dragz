import { lazy, Suspense, useEffect, useState } from 'react';
import { App as AntApp, Button, ConfigProvider, Form, Input, Modal, Spin, theme } from 'antd';
import { BrowserRouter, Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { PreferencesDrawer, type PreferenceTab } from '@/components/preferences/PreferencesDrawer';
import { antdLocale } from '@/constants/locales';
import { AdminLayout } from '@/layout/AdminLayout';
import { LoginPage, OAuthCallbackPage } from '@/pages/auth/AuthPages';
import { ComingSoonPage, ForbiddenPage, InternalErrorPage, NotFoundPage, OfflinePage } from '@/pages/fallback/FallbackPages';
import { useAccessStore } from '@/stores/access';
import { LOGOUT_REDIRECT_FLAG, useAuthStore } from '@/stores/auth';
import { useLocaleStore } from '@/stores/locale';
import { cssColorToHex, effectiveDark, resolveThemePrimaryColor, usePreferencesStore } from '@/stores/preferences';
import { antdBorderRadius, applyThemeCssVariables } from '@/utils/theme-css';
import { safeRedirectPath } from '@/utils/redirect';

function ThemeRoot() {
  const prefs = usePreferencesStore((state) => state.preferences);
  const locale = useLocaleStore((state) => state.locale);
  const [, redraw] = useState(0);
  const dark = effectiveDark(prefs);
  const primary = resolveThemePrimaryColor(prefs);
  const primaryHex = cssColorToHex(primary);

  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const fn = () => redraw((value) => value + 1);
    media.addEventListener('change', fn);
    return () => media.removeEventListener('change', fn);
  }, []);

  useEffect(() => {
    applyThemeCssVariables(prefs, primaryHex);
  }, [prefs, primaryHex]);

  return (
    <ConfigProvider
      componentSize={prefs.app.compact ? 'small' : 'middle'}
      form={{ labelWrap: true }}
      locale={antdLocale(locale)}
      theme={{
        algorithm: dark ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: {
          colorPrimary: primary,
          colorError: prefs.theme.colorDestructive,
          colorSuccess: prefs.theme.colorSuccess,
          colorWarning: prefs.theme.colorWarning,
          borderRadius: antdBorderRadius(prefs),
          fontSize: prefs.theme.fontSize,
          motion: true,
          motionDurationFast: '0.15s',
          motionDurationMid: '0.2s',
        },
        components: {
          Segmented: {
            itemSelectedBg: primary,
            itemSelectedColor: '#fff',
          },
          Layout: {
            headerBg: 'var(--surface)',
            siderBg: 'var(--surface)',
          },
          Menu: {
            itemBorderRadius: antdBorderRadius(prefs),
            darkItemBg: 'transparent',
            darkSubMenuItemBg: 'transparent',
            darkItemSelectedBg: 'color-mix(in srgb, var(--primary) 22%, transparent)',
            darkItemHoverBg: 'color-mix(in srgb, var(--primary) 12%, transparent)',
            itemBg: 'transparent',
            subMenuItemBg: 'transparent',
          },
          Table: { headerBg: dark ? '#1f1f1f' : '#fafafa' },
          Tooltip: {
            zIndexPopup: 1100,
          },
        },
      }}
    >
      <AntApp>
        <AppRoutes />
      </AntApp>
    </ConfigProvider>
  );
}

function loginPathForRedirect(pathname: string, search: string) {
  if (sessionStorage.getItem(LOGOUT_REDIRECT_FLAG)) {
    return '/auth/login';
  }
  return `/auth/login?redirect=${encodeURIComponent(pathname + search)}`;
}

function Protected() {
  const { t } = useTranslation();
  const checked = useAccessStore((state) => state.checked);
  const token = useAccessStore((state) => state.accessToken);
  const bootstrap = useAuthStore((state) => state.bootstrap);
  const [ready, setReady] = useState(checked);
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    if (checked) {
      setReady(true);
      return;
    }
    void bootstrap().then((ok) => {
      setReady(true);
      if (!ok) {
        navigate(loginPathForRedirect(location.pathname, location.search), { replace: true });
      }
    });
  }, [bootstrap, checked, location.pathname, location.search, navigate]);

  if (!ready) {
    return (
      <div className="startup-loading">
        <Spin size="large" />
        <span>{t('auth.restoringSession')}</span>
      </div>
    );
  }
  if (!token) {
    return <Navigate replace to={loginPathForRedirect(location.pathname, location.search)} />;
  }
  return <AdminLayout />;
}

function AuthGuest({ children }: { children: React.ReactNode }) {
  const token = useAccessStore((state) => state.accessToken);
  const user = useAccessStore((state) => state.userInfo);
  const prefs = usePreferencesStore((state) => state.preferences);
  const location = useLocation();
  if (token) {
    sessionStorage.removeItem(LOGOUT_REDIRECT_FLAG);
    const redirect = safeRedirectPath(new URLSearchParams(location.search).get('redirect'), user?.homePath || prefs.app.defaultHomePath);
    return <Navigate replace to={redirect} />;
  }
  // Stay on login after intentional logout; drop flag once guest page is shown.
  if (sessionStorage.getItem(LOGOUT_REDIRECT_FLAG)) {
    sessionStorage.removeItem(LOGOUT_REDIRECT_FLAG);
  }
  return children;
}

function LoginExpiredModal() {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const login = useAuthStore((state) => state.login);
  const loading = useAuthStore((state) => state.loading);
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const { message } = AntApp.useApp();

  useEffect(() => {
    const handler = (event: Event) => {
      const mode = (event as CustomEvent<string>).detail;
      if (mode === 'modal') {
        setOpen(true);
        return;
      }
      useAccessStore.getState().clear();
      useAccessStore.getState().setChecked(true);
      sessionStorage.setItem(LOGOUT_REDIRECT_FLAG, '1');
      navigate('/auth/login', { replace: true });
    };
    window.addEventListener('admin:login-expired', handler);
    return () => window.removeEventListener('admin:login-expired', handler);
  }, [navigate]);

  const submit = async (values: { password: string; username: string }) => {
    try {
      const home = await login(values);
      setOpen(false);
      form.resetFields();
      message.success(t('auth.loginSuccess'));
      navigate(home, { replace: true });
    } catch {
      /* request layer shows details */
    }
  };

  return (
    <Modal
      closable={false}
      confirmLoading={loading}
      footer={null}
      keyboard={false}
      mask={{ closable: false }}
      open={open}
      title={t('auth.loginExpiredTitle')}
    >
      <p style={{ marginBottom: 16 }}>{t('auth.loginExpiredDesc')}</p>
      <Form form={form} layout="vertical" onFinish={(values) => void submit(values)}>
        <Form.Item name="username" rules={[{ required: true, message: t('auth.usernameRequired') }]}>
          <Input placeholder={t('auth.usernamePlaceholder')} size="large" />
        </Form.Item>
        <Form.Item name="password" rules={[{ required: true, message: t('auth.passwordRequired') }]}>
          <Input.Password placeholder={t('auth.passwordPlaceholder')} size="large" />
        </Form.Item>
        <Form.Item>
          <Button block htmlType="submit" loading={loading} size="large" type="primary">
            {t('login')}
          </Button>
        </Form.Item>
      </Form>
    </Modal>
  );
}

function PreferencesVisualPreview() {
  const location = useLocation();
  const update = usePreferencesStore((state) => state.update);
  const query = new URLSearchParams(location.search);
  const requested = query.get('tab');
  const tab: PreferenceTab = requested === 'layout' || requested === 'general' ? requested : 'appearance';
  const mode = query.get('theme') === 'light' ? 'light' : 'dark';
  const mobile = query.get('viewport') === 'mobile';

  useEffect(() => {
    update({ theme: { ...usePreferencesStore.getState().preferences.theme, mode } });
  }, [mode, update]);

  useEffect(() => {
    if (mobile) document.documentElement.dataset.prefPreviewMobile = 'true';
    return () => {
      delete document.documentElement.dataset.prefPreviewMobile;
    };
  }, [mobile]);

  return (
    <div className="preferences-preview-stage">
      <div className="brand">
        <img alt="Dragz Admin" className="brand-mark" height={31} src="/favicon.svg" width={31} />
        <strong>Dragz Admin</strong>
      </div>
      <PreferencesDrawer initialTab={tab} open onClose={() => undefined} />
    </div>
  );
}

const ResizableTablePreviewPage = lazy(() => import('@/pages/preview/ResizableTablePreviewPage'));
const VxeBasePage = lazy(() => import('@/pages/preview/vxe/VxeBasePage'));
const VxeGroupHeaderPage = lazy(() => import('@/pages/preview/vxe/VxeGroupHeaderPage'));

function AppRoutes() {
  return (
    <>
      <Routes>
        <Route
          element={
            <AuthGuest>
              <LoginPage />
            </AuthGuest>
          }
          path="/auth/login"
        />
        <Route element={<OAuthCallbackPage />} path="/auth/oauth-callback" />
        <Route element={<ForbiddenPage />} path="/403" />
        <Route element={<NotFoundPage />} path="/404" />
        <Route element={<InternalErrorPage />} path="/500" />
        <Route element={<OfflinePage />} path="/offline" />
        <Route element={<ComingSoonPage />} path="/coming-soon" />
        {import.meta.env.DEV ? <Route element={<PreferencesVisualPreview />} path="/__preferences-preview" /> : null}
        {import.meta.env.DEV ? (
          <Route
            element={
              <Suspense
                fallback={
                  <div className="startup-loading">
                    <Spin size="large" />
                  </div>
                }
              >
                <ResizableTablePreviewPage />
              </Suspense>
            }
            path="/__resizable-table"
          />
        ) : null}
        {import.meta.env.DEV ? (
          <>
            <Route element={<Navigate replace to="/__vxe/base" />} path="/__vxe" />
            {[
              { path: '/__vxe/base', element: <VxeBasePage /> },
              { path: '/__vxe/group-header', element: <VxeGroupHeaderPage /> },
            ].map(({ path, element }) => (
              <Route
                element={
                  <Suspense
                    fallback={
                      <div className="startup-loading">
                        <Spin size="large" />
                      </div>
                    }
                  >
                    {element}
                  </Suspense>
                }
                key={path}
                path={path}
              />
            ))}
          </>
        ) : null}
        <Route element={<Protected />} path="/*" />
      </Routes>
      <LoginExpiredModal />
    </>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <ThemeRoot />
    </BrowserRouter>
  );
}
