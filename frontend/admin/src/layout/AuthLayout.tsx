import type { ReactNode } from 'react';
import { AuthCopyright } from '@/components/auth/AuthCopyright';
import { AuthSlogan } from '@/components/auth/AuthSlogan';
import { AuthToolbar } from '@/components/auth/AuthToolbar';
import { usePreferencesStore } from '@/stores/preferences';

function AuthFormPanel({ children }: { children: ReactNode }) {
  const prefs = usePreferencesStore((state) => state.preferences);
  return (
    <div className="auth-form-panel is-center" data-side="bottom">
      <div className="auth-form-content">{children}</div>
      {prefs.copyright.enable ? (
        <div className="auth-form-copyright">
          <AuthCopyright {...prefs.copyright} />
        </div>
      ) : null}
    </div>
  );
}

export function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="auth-shell">
      <AuthToolbar />
      <div className="auth-center-wrap">
        <div className="login-background auth-center-bg" />
        <AuthFormPanel>{children}</AuthFormPanel>
      </div>
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
