import type { ReactNode } from 'react';
import type { Preferences } from '@/types';

function ExternalLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a href={href} rel="noopener noreferrer" target="_blank">
      {children}
    </a>
  );
}

export function AuthCopyright({ companyName, companySiteLink, date, icp, icpLink }: Preferences['copyright']) {
  const site = (companySiteLink || '').trim();
  const company = (companyName || '').trim();
  const year = (date || '').trim() || String(new Date().getFullYear());
  return (
    <div className="auth-copyright">
      {icp ? (
        <ExternalLink href={icpLink.trim() || 'https://beian.miit.gov.cn/'}>{icp}</ExternalLink>
      ) : null}
      <span>Copyright © {year}</span>
      {company ? (site ? <ExternalLink href={site}>{company}</ExternalLink> : <span>{company}</span>) : null}
    </div>
  );
}
