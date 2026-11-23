import { Blocks, KeyRound, Languages, ShieldCheck } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { TypewriterText } from './TypewriterText';

/** Four capabilities that define dragz as an admin foundation. */
const FEATURES = [
  { icon: Blocks, titleKey: 'auth.featurePluginTitle', descKey: 'auth.featurePluginDesc' },
  { icon: ShieldCheck, titleKey: 'auth.featureTenantTitle', descKey: 'auth.featureTenantDesc' },
  { icon: KeyRound, titleKey: 'auth.featurePermTitle', descKey: 'auth.featurePermDesc' },
  { icon: Languages, titleKey: 'auth.featureI18nTitle', descKey: 'auth.featureI18nDesc' },
] as const;

/**
 * Product introduction shown beside the form (split layouts) or above it
 * (centered layout). Keeps both presentations in sync with the same copy.
 */
export function AuthIntro({ variant }: { variant: 'panel' | 'center' }) {
  const { t } = useTranslation();
  const description = t('auth.sloganDesc');

  if (variant === 'center') {
    return (
      <div className="auth-center-intro">
        <p>{description}</p>
        <ul className="auth-feature-chips">
          {FEATURES.map(({ icon: Icon, titleKey }) => (
            <li key={titleKey}>
              <Icon aria-hidden size={15} />
              <span>{t(titleKey)}</span>
            </li>
          ))}
        </ul>
      </div>
    );
  }

  return (
    <div className="auth-intro-detail">
      <h2><TypewriterText text={t('auth.sloganTitle')} /></h2>
      <p>{description}</p>
      <ul className="auth-feature-grid">
        {FEATURES.map(({ icon: Icon, titleKey, descKey }) => (
          <li className="auth-feature-item" key={titleKey}>
            <span className="auth-feature-icon">
              <Icon aria-hidden size={18} />
            </span>
            <div>
              <h3>{t(titleKey)}</h3>
              <p>{t(descKey)}</p>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
