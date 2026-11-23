import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { AuthLayout } from '@/layout/AuthLayout';
import { defaultPreferences, usePreferencesStore } from '@/stores/preferences';
import { AuthIntro } from './AuthIntro';

describe('AuthIntro', () => {
  afterEach(() => {
    cleanup();
    usePreferencesStore.setState({ preferences: structuredClone(defaultPreferences) });
  });

  it('shows dragz-specific capabilities as four cards in split layout', () => {
    render(<AuthIntro variant="panel" />);
    expect(screen.getByRole('heading', { level: 2, name: '插件式后台底座，为二次开发而生' })).toBeInTheDocument();
    expect(screen.getByText(/共享库 \+ PostgreSQL RLS/)).toBeInTheDocument();
    expect(screen.getByText(/Casbin 权限码精确到菜单与按钮/)).toBeInTheDocument();
    expect(screen.getByText(/翻译管理里只填中文/)).toBeInTheDocument();
    expect(screen.getAllByRole('heading', { level: 3 }).map((heading) => heading.textContent)).toEqual([
      '编译期插件',
      '租户级隔离',
      '细粒度权限',
      '多语言支持',
    ]);
  });

  it('uses compact capability chips in the centered layout', () => {
    render(<AuthIntro variant="center" />);
    expect(screen.queryByRole('heading', { level: 1 })).not.toBeInTheDocument();
    expect(screen.queryAllByRole('heading', { level: 3 })).toHaveLength(0);
    expect(document.querySelectorAll('.auth-feature-chips li')).toHaveLength(4);
  });

  it('places the slogan title directly under the logo in the centered layout', () => {
    usePreferencesStore.setState({
      preferences: {
        ...defaultPreferences,
        app: { ...defaultPreferences.app, authPageLayout: 'panel-center' },
      },
    });
    const { container } = render(<AuthLayout>form</AuthLayout>);
    const brand = container.querySelector('.auth-center-brand');
    expect(brand?.querySelector('h1')).toHaveTextContent('插件式后台底座，为二次开发而生');
    expect(container.querySelector('.auth-center-intro h1')).toBeNull();
    expect([...container.querySelector('.auth-center-wrap').children].map((child) => child.className.split(' ')[0])).toEqual([
      'login-background',
      'auth-center-brand',
      'auth-form-panel',
      'auth-center-intro',
    ]);
  });
});
