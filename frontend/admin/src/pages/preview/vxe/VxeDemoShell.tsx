import { useTranslation } from 'react-i18next';
import { NavLink } from 'react-router-dom';
import type { ReactNode } from 'react';
import './vxe-demo.css';

/** Shared shell for the vxe-table demo pages: nav + titled content. */
export function VxeDemoShell(props: { title: string; description?: string; children: ReactNode }) {
  const { t } = useTranslation();
  const demos: Array<{ path: string; label: string }> = [
    { path: '/__vxe/base', label: t('preview.demoBasic') },
    { path: '/__vxe/group-header', label: t('preview.demoGroupHeader') },
  ];

  return (
    <div className="vxe-demo-page">
      <h2>{props.title}</h2>
      <nav className="vxe-demo-nav">
        {demos.map((demo) => (
          <NavLink key={demo.path} className={({ isActive }) => (isActive ? 'is-active' : '')} to={demo.path}>
            {demo.label}
          </NavLink>
        ))}
      </nav>
      {props.description ? <p style={{ opacity: 0.75 }}>{props.description}</p> : null}
      {props.children}
    </div>
  );
}
