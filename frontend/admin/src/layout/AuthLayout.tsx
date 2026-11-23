import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { AuthCopyright } from '@/components/auth/AuthCopyright';
import { AuthIntro } from '@/components/auth/AuthIntro';
import { AuthSlogan } from '@/components/auth/AuthSlogan';
import { AuthToolbar } from '@/components/auth/AuthToolbar';
import { TypewriterText } from '@/components/auth/TypewriterText';
import { usePreferencesStore } from '@/stores/preferences';

function AuthFormPanel({ children, side }: { children: ReactNode; side?: 'left' | 'right' }) {
  return (
    <div className={`auth-form-panel${side ? '' : ' is-center'}`} data-side={side ?? 'bottom'}>
      <div className="auth-form-content">{children}</div>
    </div>
  );
}

export function AuthLayout({ children }: { children: ReactNode }) {
  const { t } = useTranslation();
  const prefs = usePreferencesStore((state) => state.preferences);
  const layout = prefs.app.authPageLayout.replace('panel-', '');
  const isCenter = layout === 'center';

  const introPanel = (
    <div className={`auth-intro-panel is-${layout}`}>
      <div className="auth-intro-bg" />
      <div className="login-background auth-center-bg" />
      <div className={`auth-intro-content ${layout === 'left' ? 'enter-x' : 'enter-x-reverse'}`}>
        <AuthSlogan />
        <AuthIntro variant="panel" />
      </div>
    </div>
  );

  return (
    <div className={`auth-shell ${isCenter ? 'is-center-layout' : 'is-split-layout'}`}>
      <AuthToolbar />
      {layout === 'left' ? <AuthFormPanel side="left">{children}</AuthFormPanel> : null}
      {!isCenter ? introPanel : null}
      {layout === 'right' ? <AuthFormPanel side="right">{children}</AuthFormPanel> : null}
      {isCenter ? (
        <div className="auth-center-wrap">
          <div className="login-background auth-center-bg" />
          <div className="auth-center-brand">
            <AuthSlogan />
            <h1><TypewriterText text={t('auth.sloganTitle')} /></h1>
          </div>
          <AuthFormPanel>{children}</AuthFormPanel>
          <AuthIntro variant="center" />
        </div>
      ) : null}
      {prefs.copyright.enable ? (
        <div className="auth-page-copyright">
          <AuthCopyright {...prefs.copyright} />
        </div>
      ) : null}
    </div>
  );
}

export function AuthTitle({ subtitle, title }: { subtitle?: string; title?: ReactNode }) {
  return (
    <div className="auth-title">
      <AuthSlogan />
      {title ? <h2>{title}</h2> : null}
      {subtitle ? <p>{subtitle}</p> : null}
    </div>
  );
}
