import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { REMEMBER_ME_KEY } from '@/constants/auth';
import { LoginPage } from './AuthPages';

const mocks = vi.hoisted(() => ({
  loginChallengeApi: vi.fn(),
  issueCaptchaApi: vi.fn(),
  publicAuthProvidersApi: vi.fn(),
  login: vi.fn(),
  navigate: vi.fn(),
  success: vi.fn(),
  info: vi.fn(),
  error: vi.fn(),
}));

vi.mock('@/api/auth', () => ({
  exchangeOAuthApi: vi.fn(),
  issueCaptchaApi: mocks.issueCaptchaApi,
  loginChallengeApi: mocks.loginChallengeApi,
  publicAuthProvidersApi: mocks.publicAuthProvidersApi,
  startOAuthApi: vi.fn(),
}));

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ login: mocks.login, loading: false }),
}));

vi.mock('@/stores/access', () => ({
  useAccessStore: { getState: () => ({ userInfo: { homePath: '/analytics', realName: '管理员' } }) },
}));

vi.mock('@/layout/AuthLayout', () => ({
  AuthLayout: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AuthTitle: () => <div />,
}));

vi.mock('react-router-dom', () => ({
  useLocation: () => ({ search: '' }),
  useNavigate: () => mocks.navigate,
}));

vi.mock('antd', async () => {
  const actual = await vi.importActual<typeof import('antd')>('antd');
  return {
    ...actual,
    App: { ...actual.App, useApp: () => ({ message: { info: mocks.info, success: mocks.success, error: mocks.error } }) },
  };
});

function captchaResult(expiresIn = 300) {
  return {
    captchaId: 'captcha-1',
    captchaType: 'slide' as const,
    imageBase64: 'data:image/svg+xml;base64,AAAA',
    pieceBase64: 'data:image/svg+xml;base64,BBBB',
    width: 300,
    height: 150,
    pieceSize: 60,
    pieceTop: 46,
    expiresIn,
  };
}

async function fillCredentials() {
  fireEvent.change(screen.getByPlaceholderText('请输入用户名'), { target: { value: 'admin' } });
  fireEvent.change(screen.getByPlaceholderText('请输入密码'), { target: { value: 'pass1234' } });
}

// When captcha is enabled the page shows a Login button; pressing it validates
// the form, issues the puzzle and opens the modal.
async function renderLogin() {
  render(<LoginPage />);
  await fillCredentials();
  fireEvent.click(await screen.findByRole('button', { name: /登\s*录/ }));
  await waitFor(() => expect(mocks.issueCaptchaApi).toHaveBeenCalled());
}

function drag(handle: HTMLElement, from: number, to: number) {
  fireEvent.pointerDown(handle, { clientX: from, pointerId: 1 });
  fireEvent.pointerMove(handle, { clientX: to, pointerId: 1 });
  fireEvent.pointerUp(handle, { clientX: to, pointerId: 1 });
}

describe('LoginPage 滑块验证', () => {
  beforeAll(() => {
    Object.defineProperty(Element.prototype, 'setPointerCapture', { configurable: true, value: vi.fn() });
    Object.defineProperty(Element.prototype, 'releasePointerCapture', { configurable: true, value: vi.fn() });
    Object.defineProperty(Element.prototype, 'hasPointerCapture', { configurable: true, value: () => false });
  });

  beforeEach(() => {
    window.localStorage.clear();
    mocks.loginChallengeApi.mockResolvedValue({ captchaRequired: true });
    mocks.issueCaptchaApi.mockImplementation(() => Promise.resolve(captchaResult()));
    mocks.publicAuthProvidersApi.mockResolvedValue([]);
    mocks.login.mockResolvedValue('/analytics');
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  it('挂载时只查询挑战；点「登录」后才签发滑块拼图', async () => {
    render(<LoginPage />);
    await waitFor(() => expect(mocks.loginChallengeApi).toHaveBeenCalledTimes(1));
    expect(mocks.issueCaptchaApi).not.toHaveBeenCalled();
    expect(await screen.findByRole('button', { name: /登\s*录/ })).toBeInTheDocument();
    expect(screen.queryByAltText('滑块验证')).not.toBeInTheDocument();

    await fillCredentials();
    fireEvent.click(screen.getByRole('button', { name: /登\s*录/ }));

    await waitFor(() => expect(mocks.issueCaptchaApi).toHaveBeenCalledTimes(1));
    expect(screen.getByAltText('滑块验证')).toBeInTheDocument();
    expect(screen.getByRole('slider')).toHaveAttribute('aria-valuemin', '0');
    expect(mocks.login).not.toHaveBeenCalled();
  });

  it('点「登录」弹出模态框,不提交登录,表单仍在', async () => {
    await renderLogin();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByRole('slider')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('请输入密码')).toBeInTheDocument();
    expect(screen.getByText('安全验证')).toBeInTheDocument();
    expect(mocks.login).not.toHaveBeenCalled();
  });

  it('密码错误时关闭模态框退回表单并保留已填内容', async () => {
    mocks.login.mockRejectedValueOnce({
      response: { data: { code: 40101, message: '用户名或密码错误' } },
    });
    await renderLogin();
    drag(screen.getByRole('slider'), 0, 60);
    expect(await screen.findByPlaceholderText('请输入密码')).toHaveValue('pass1234');
    await waitFor(() => expect(screen.queryByRole('slider')).not.toBeInTheDocument());
    expect(mocks.error).toHaveBeenCalledWith('用户名或密码错误');
  });

  it('验证码错误时停留在模态框并换新图', async () => {
    mocks.login.mockRejectedValueOnce({
      response: { data: { code: 40002, message: '验证错误', data: { captchaRequired: true } } },
    });
    await renderLogin();
    drag(screen.getByRole('slider'), 0, 60);
    await waitFor(() => expect(mocks.issueCaptchaApi).toHaveBeenCalledTimes(2));
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByRole('slider')).toBeInTheDocument();
    expect(mocks.error).toHaveBeenCalledWith('验证错误');
    expect(mocks.info).not.toHaveBeenCalled();
    expect(window.localStorage.getItem(REMEMBER_ME_KEY)).toBeNull();
  });

  it('账号密码为空时点「登录」不签发验证码', async () => {
    render(<LoginPage />);
    fireEvent.click(await screen.findByRole('button', { name: /登\s*录/ }));
    expect(await screen.findByText('请输入用户名')).toBeInTheDocument();
    expect(mocks.issueCaptchaApi).not.toHaveBeenCalled();
  });

  it('未启用验证码时保留滑块直接登录', async () => {
    mocks.loginChallengeApi.mockResolvedValue({ captchaRequired: false });
    render(<LoginPage />);
    await fillCredentials();
    expect(screen.queryByRole('button', { name: /登\s*录/ })).not.toBeInTheDocument();
    expect(mocks.issueCaptchaApi).not.toHaveBeenCalled();

    drag(screen.getByRole('slider'), 0, 260);
    await waitFor(() => expect(mocks.login).toHaveBeenCalledTimes(1));
    expect(mocks.login).toHaveBeenCalledWith(expect.objectContaining({
      captchaId: undefined,
      captchaCode: undefined,
      username: 'admin',
    }));
  });

  it('未拖动松手不提交登录', async () => {
    await renderLogin();
    drag(screen.getByRole('slider'), 10, 10);
    expect(mocks.login).not.toHaveBeenCalled();
    expect(screen.getByRole('slider')).toHaveAttribute('aria-valuenow', '0');
  });

  it('拖动松手提交位移与验证码 id,通过后在模态框显示「验证通过」再跳转', async () => {
    await renderLogin();
    drag(screen.getByRole('slider'), 0, 60);
    await waitFor(() => expect(mocks.login).toHaveBeenCalledTimes(1));
    expect(mocks.login).toHaveBeenCalledWith(expect.objectContaining({
      captchaId: 'captcha-1',
      captchaCode: '60',
      username: 'admin',
    }));
    expect(await screen.findByText('验证通过')).toBeInTheDocument();
    expect(mocks.navigate).not.toHaveBeenCalled();
    await waitFor(() => expect(mocks.navigate).toHaveBeenCalledWith('/analytics', { replace: true }), { timeout: 3000 });
  });

  it('验证码过期时阻止提交并自动刷新', async () => {
    let currentTime = 1000;
    const nowSpy = vi.spyOn(Date, 'now').mockImplementation(() => currentTime);
    await renderLogin();
    currentTime += 500_000;
    expect(nowSpy).toHaveBeenCalled();
    drag(screen.getByRole('slider'), 0, 60);
    await waitFor(() => expect(mocks.issueCaptchaApi).toHaveBeenCalledTimes(2));
    expect(mocks.login).not.toHaveBeenCalled();
    expect(mocks.info).not.toHaveBeenCalled();
  });

  it('锁定状态下退回表单且「登录」按钮禁用', async () => {
    mocks.login.mockRejectedValueOnce({
      response: {
        data: {
          code: 42301,
          message: '密码错误次数过多',
          data: { captchaRequired: true, kind: 'lock', lockedUntil: new Date(Date.now() + 60_000).toISOString() },
        },
      },
    });
    await renderLogin();
    drag(screen.getByRole('slider'), 0, 60);
    await waitFor(() => expect(screen.getByTestId('login-submit')).toBeDisabled());
    expect(screen.getByPlaceholderText('请输入密码')).toHaveValue('pass1234');
  });

  it('账号被锁定后换成其他账号，登录按钮恢复可用', async () => {
    mocks.login.mockRejectedValueOnce({
      response: {
        data: {
          code: 42301,
          message: '密码错误次数过多',
          data: { captchaRequired: true, kind: 'lock', lockedUntil: new Date(Date.now() + 600_000).toISOString() },
        },
      },
    });
    await renderLogin();
    drag(screen.getByRole('slider'), 0, 60);
    await waitFor(() => expect(screen.getByTestId('login-submit')).toBeDisabled());

    // Switching to a different account must lift the per-account lock.
    fireEvent.change(screen.getByPlaceholderText('请输入用户名'), { target: { value: 'another_user' } });
    await waitFor(() => expect(screen.getByTestId('login-submit')).not.toBeDisabled());

    // Switching back to the locked account restores the lock.
    fireEvent.change(screen.getByPlaceholderText('请输入用户名'), { target: { value: 'admin' } });
    await waitFor(() => expect(screen.getByTestId('login-submit')).toBeDisabled());
  });

  it('重复的松手与取消事件不会重复登录', async () => {
    await renderLogin();
    const handle = screen.getByRole('slider');
    fireEvent.pointerDown(handle, { clientX: 0, pointerId: 1 });
    fireEvent.pointerUp(handle, { clientX: 60, pointerId: 1 });
    fireEvent.pointerUp(handle, { clientX: 60, pointerId: 1 });
    fireEvent.pointerCancel(handle, { pointerId: 1 });
    await waitFor(() => expect(mocks.login).toHaveBeenCalledTimes(1));
  });

  it('键盘方向键和回车也能完成验证', async () => {
    await renderLogin();
    const handle = screen.getByRole('slider');
    fireEvent.keyDown(handle, { key: 'ArrowRight' });
    expect(handle).toHaveAttribute('aria-valuenow', '8');
    fireEvent.keyDown(handle, { key: 'Enter' });
    await waitFor(() => expect(mocks.login).toHaveBeenCalledTimes(1));
    expect(mocks.login).toHaveBeenCalledWith(expect.objectContaining({ captchaCode: '8' }));
  });
});
