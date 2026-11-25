import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@/styles/auth.css';
import { AuthLoginActionSurface, AuthToastView, authToast } from './AuthToast';

describe('AuthLoginActionSurface', () => {
  afterEach(() => {
    authToast.clear();
    cleanup();
  });

  it('renders error toast over the login action surface', () => {
    render(
      <AuthLoginActionSurface>
        <button style={{ width: '100%', height: 38 }} type="button">
          Login
        </button>
      </AuthLoginActionSurface>,
    );
    act(() => authToast.error('请输入用户名'));
    const bar = screen.getByRole('status').querySelector('.auth-toast.is-error.is-enter');
    expect(bar).toHaveTextContent('请输入用户名');
    expect(bar?.querySelector('.auth-toast-progress')).toBeInTheDocument();
  });

  it('keeps the toast visible over the bottom login control band', () => {
    render(
      <AuthLoginActionSurface>
        <div style={{ height: 160 }} data-testid="tall-captcha-stub" />
        <button style={{ width: '100%', height: 38 }} type="button">
          Login
        </button>
      </AuthLoginActionSurface>,
    );
    act(() => authToast.error('请输入用户名'));
    const layer = screen.getByRole('status');
    const surface = layer.parentElement;
    const bar = layer.querySelector('.auth-toast.is-error');
    expect(surface).toHaveClass('auth-login-action-surface');
    expect(layer).toHaveClass('auth-toast-layer');
    expect(bar).toBeTruthy();
    expect(bar).toHaveTextContent('请输入用户名');
    expect(layer.querySelector('.auth-toast-progress')).toBeInTheDocument();
  });

  it('exposes the toast layer above the control with full opacity', () => {
    render(
      <AuthLoginActionSurface>
        <div style={{ width: '100%', height: 38, background: '#dce6f8' }} data-testid="track-stub" />
      </AuthLoginActionSurface>,
    );
    act(() => authToast.error('登录失败'));
    const layer = screen.getByRole('status');
    const bar = layer.querySelector('.auth-toast.is-error.is-enter');
    expect(layer).toHaveClass('auth-toast-layer');
    expect(bar).toHaveClass('is-enter');
    expect(bar).not.toHaveClass('is-leave');
    const layerStyle = getComputedStyle(layer);
  const zIndex = Number.parseInt(layerStyle.zIndex, 10);
    if (!Number.isNaN(zIndex)) {
      expect(zIndex).toBeGreaterThanOrEqual(20);
    }
  });
});

describe('AuthToastView', () => {
  it('renders the full message visibly without a title tooltip', () => {
    const long =
      '这是一段非常长的错误提示信息，应该在窄栏里自动换行并完整显示，而不是省略号截断';
    const { container } = render(
      <div style={{ width: 200 }}>
        <AuthToastView text={long} persistent />
      </div>,
    );
    const text = container.querySelector('.auth-toast-text');
    const bar = container.querySelector('.auth-toast');
    expect(text).not.toHaveAttribute('title');
    expect(text).toHaveTextContent(long);
    const textStyle = text ? getComputedStyle(text) : null;
    expect(textStyle?.whiteSpace).not.toBe('nowrap');
    expect(textStyle?.textOverflow).not.toBe('ellipsis');
    const barStyle = bar ? getComputedStyle(bar) : null;
    expect(barStyle?.overflow).not.toBe('hidden');
    expect(textStyle?.overflow).not.toBe('hidden');
  });
});