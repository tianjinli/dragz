import { AUTH_LOGO } from '@/constants/auth';
import { usePreferencesStore } from '@/stores/preferences';

export function AuthSlogan() {
  const prefs = usePreferencesStore((state) => state.preferences);
  return (
    <div className="auth-logo-mark">
      <img alt={prefs.app.name} height={42} src={AUTH_LOGO} width={42} />
      <span>{prefs.app.name}</span>
    </div>
  );
}
