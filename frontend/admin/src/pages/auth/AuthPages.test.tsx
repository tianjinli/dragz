import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { ACCOUNT_LOCK_DURATION_MS, REMEMBER_ME_KEY } from '@/constants/auth';
import { LoginPage, loginLockAccentFromRatio, loginLockProgressScale, resolveLockTotalMs } from './AuthPages';

const mocks = vi.hoisted(() => ({
  loginChallengeApi: vi.fn(),
  issueCaptchaApi: vi.fn(),
  publicAuthProvidersApi: vi.fn(),
  getTenantContextApi: vi.fn(),
  login: vi.fn(),
  navigate: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
  clear: vi.fn(),
}));

vi.mock('@/api/auth', () => ({
  exchangeOAuthApi: vi.fn(),
  getTenantContextApi: mocks.getTenantContextApi,
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

vi.mock('@/components/auth/AuthToast', () => ({
  AUTH_TOAST_HOLD_MS: 3000,
  authToast: { error: mocks.error, success: mocks.success, clear: mocks.clear },
  AuthLoginActionSurface: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  AuthToastHost: () => null,
  AuthToastView: ({ text }: { text: string }) => <div data-testid="auth-toast-view">{text}</div>,
}));

vi.mock('@/layout/AuthLayout', () => ({
  AuthLayout: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AuthTitle: () => <div />,
}));

vi.mock('react-router-dom', () => ({
  useLocation: () => ({ search: '' }),
  useNavigate: () => mocks.navigate,
}));

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
    window.sessionStorage.clear();
    mocks.loginChallengeApi.mockResolvedValue({ captchaRequired: true, lockMinutes: 30 });
    mocks.issueCaptchaApi.mockImplementation(() => Promise.resolve(captchaResult()));
    mocks.publicAuthProvidersApi.mockResolvedValue([]);
    mocks.getTenantContextApi.mockResolvedValue({ resolved: false, tenantId: 0, slug: '', name: '' });
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
    expect(screen.queryByText('Dragz 安全验证')).not.toBeInTheDocument();
    expect(document.querySelector('.auth-slide-captcha-handle svg')).toBeInTheDocument();
    expect(document.querySelector('.auth-slide-captcha-hint')).toHaveClass('is-shimmer');
    expect(mocks.login).not.toHaveBeenCalled();
  });

  it('密码错误时关闭模态框退回表单并 toast 提示', async () => {
    mocks.login.mockRejectedValueOnce({
      response: { data: { code: 40101, message: '用户名或密码错误' } },
    });
    await renderLogin();
    drag(screen.getByRole('slider'), 0, 60);
    await waitFor(() => expect(mocks.error).toHaveBeenCalledWith('用户名或密码错误'));
    await waitFor(() => expect(screen.queryByRole('slider')).not.toBeInTheDocument());
    expect(screen.getByPlaceholderText('请输入密码')).toHaveValue('pass1234');
  });

  it('网络错误不伪装成锁定倒计时，也不写入会话锁定状态', async () => {
    // No HTTP envelope at all: offline, gateway timeout, or a 5xx page.
    mocks.login.mockRejectedValueOnce(new Error('Network Error'));
    await renderLogin();
    drag(screen.getByRole('slider'), 0, 60);
    await waitFor(() => expect(mocks.error).toHaveBeenCalled());
    expect(screen.queryByTestId('login-lock-countdown')).not.toBeInTheDocument();
    expect(document.querySelector('.auth-login-lock-progress')).toBeNull();
    expect(await screen.findByTestId('login-submit')).not.toBeDisabled();
    expect(window.sessionStorage.getItem(`LOGIN_LOCK_STATE_${location.hostname}`)).toBeNull();
  });

  it('验证码验证失败时图片下方内联显示失败,延时换新图', async () => {
    mocks.login.mockRejectedValueOnce({
      response: {
        data: {
          code: 40002,
          message: '滑块验证失败，请刷新验证码后重试',
          data: { captchaRequired: true, captchaError: 'invalid' },
        },
      },
    });
    await renderLogin();
    drag(screen.getByRole('slider'), 0, 60);
    await waitFor(() => expect(document.querySelector('.auth-slide-captcha-hint.is-error')).toHaveTextContent('验证未通过，请重试'));
    expect(mocks.error).not.toHaveBeenCalled();
    expect(mocks.issueCaptchaApi).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(mocks.issueCaptchaApi).toHaveBeenCalledTimes(2), { timeout: 3000 });
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByRole('slider')).toBeInTheDocument();
    expect(window.localStorage.getItem(REMEMBER_ME_KEY)).toBeNull();
  });

  it('服务端判定失效时遮住图片与滑块,不弹提示,点击遮罩刷新', async () => {
    mocks.login.mockRejectedValueOnce({
      response: {
        data: {
          code: 40002,
          message: '验证码已失效，请刷新验证码后重试',
          data: { captchaRequired: true, captchaError: 'expired' },
        },
      },
    });
    await renderLogin();
    drag(screen.getByRole('slider'), 0, 60);
    const mask = await screen.findByRole('button', { name: '验证码已失效，点击刷新' });
    expect(mocks.error).not.toHaveBeenCalled();
    expect(mocks.issueCaptchaApi).toHaveBeenCalledTimes(1);
    fireEvent.click(mask);
    await waitFor(() => expect(mocks.issueCaptchaApi).toHaveBeenCalledTimes(2));
    expect(screen.queryByRole('button', { name: '验证码已失效，点击刷新' })).not.toBeInTheDocument();
  });

  it('未提交验证码时提示完成验证', async () => {
    mocks.login.mockRejectedValueOnce({
      response: {
        data: {
          code: 40002,
          message: '请完成滑块验证',
          data: { captchaRequired: true, captchaError: 'required' },
        },
      },
    });
    await renderLogin();
    drag(screen.getByRole('slider'), 0, 60);
    await waitFor(() => expect(mocks.issueCaptchaApi).toHaveBeenCalledTimes(2));
    expect(mocks.error).toHaveBeenCalledWith('请完成滑块验证');
  });

  it('用户名与密码输入会过滤中文等非 ASCII 字符', async () => {
    render(<LoginPage />);
    const username = screen.getByPlaceholderText('请输入用户名');
    const password = screen.getByPlaceholderText('请输入密码');
    fireEvent.change(username, { target: { value: 'admin用户' } });
    fireEvent.change(password, { target: { value: 'pass密码123' } });
    await waitFor(() => expect(username).toHaveValue('admin'));
    expect(password).toHaveValue('pass123');
  });

  it('验证码模式下点「登录」且用户名为空则聚焦用户名且不弹 toast', async () => {
    render(<LoginPage />);
    fireEvent.click(await screen.findByRole('button', { name: /登\s*录/ }));
    const username = screen.getByPlaceholderText('请输入用户名');
    await waitFor(() => expect(document.activeElement).toBe(username));
    expect(username.closest('.ant-form-item')).toHaveClass('is-shake-error');
    expect(mocks.error).not.toHaveBeenCalled();
    expect(mocks.issueCaptchaApi).not.toHaveBeenCalled();
  });

  it('验证码模式下点「登录」且密码为空则聚焦密码且不弹 toast', async () => {
    render(<LoginPage />);
    fireEvent.change(screen.getByPlaceholderText('请输入用户名'), { target: { value: 'admin' } });
    fireEvent.click(await screen.findByRole('button', { name: /登\s*录/ }));
    const password = screen.getByPlaceholderText('请输入密码');
    await waitFor(() => expect(document.activeElement).toBe(password));
    expect(password.closest('.ant-form-item')).toHaveClass('is-shake-error');
    expect(mocks.error).not.toHaveBeenCalled();
    expect(mocks.issueCaptchaApi).not.toHaveBeenCalled();
  });

  it('验证码模式下点「登录」且用户名与密码均为空则两项均抖动', async () => {
    render(<LoginPage />);
    fireEvent.click(await screen.findByRole('button', { name: /登\s*录/ }));
    const username = screen.getByPlaceholderText('请输入用户名');
    const password = screen.getByPlaceholderText('请输入密码');
    await waitFor(() => expect(document.activeElement).toBe(username));
    expect(username.closest('.ant-form-item')).toHaveClass('is-shake-error');
    expect(password.closest('.ant-form-item')).toHaveClass('is-shake-error');
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

  it('无验证码时滑到尽头且用户名为空则聚焦用户名且不弹出错误 toast', async () => {
    mocks.loginChallengeApi.mockResolvedValue({ captchaRequired: false });
    render(<LoginPage />);
    const slider = await screen.findByRole('slider');
    drag(slider, 0, 260);
    const username = screen.getByPlaceholderText('请输入用户名');
    await waitFor(() => expect(document.activeElement).toBe(username));
    expect(username.closest('.ant-form-item')).toHaveClass('is-shake-error');
    expect(mocks.error).not.toHaveBeenCalled();
    expect(mocks.login).not.toHaveBeenCalled();
  });

  it('无验证码时滑到尽头且密码为空则聚焦密码且不弹出错误 toast', async () => {
    mocks.loginChallengeApi.mockResolvedValue({ captchaRequired: false });
    render(<LoginPage />);
    await screen.findByRole('slider');
    fireEvent.change(screen.getByPlaceholderText('请输入用户名'), { target: { value: 'admin' } });
    drag(screen.getByRole('slider'), 0, 260);
    const password = screen.getByPlaceholderText('请输入密码');
    await waitFor(() => expect(document.activeElement).toBe(password));
    expect(mocks.error).not.toHaveBeenCalled();
    expect(mocks.login).not.toHaveBeenCalled();
  });

  it('滑块可拖到最右端,右缘与轨道齐平', async () => {
    await renderLogin();
    const handle = screen.getByRole('slider');
    fireEvent.pointerDown(handle, { clientX: 0, pointerId: 1 });
    fireEvent.pointerMove(handle, { clientX: 262, pointerId: 1 });
    expect(handle).toHaveAttribute('aria-valuenow', '262');
    expect(parseFloat(handle.style.left) + parseFloat(handle.style.width)).toBe(300);
    fireEvent.pointerUp(handle, { clientX: 262, pointerId: 1 });
    await waitFor(() => expect(mocks.login).toHaveBeenCalledWith(expect.objectContaining({ captchaCode: '262' })));
  });

  it('验证中与验证成功状态叠在拼图图片上', async () => {
    let resolveLogin: (value: string) => void = () => {};
    mocks.login.mockImplementationOnce(() => new Promise<string>((resolve) => { resolveLogin = resolve; }));
    await renderLogin();
    drag(screen.getByRole('slider'), 0, 60);
    const verifying = await screen.findByRole('status');
    expect(verifying).toHaveClass('is-verifying');
    expect(verifying.textContent).toContain('正在验证，请稍候');
    expect(verifying.closest('.auth-slide-captcha-stage')).not.toBeNull();
    resolveLogin('/analytics');
    await waitFor(() => expect(screen.getByRole('status')).toHaveClass('is-success'));
    expect(screen.getByText('验证通过').closest('.auth-slide-captcha-stage')).not.toBeNull();
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

  it('本地过期时遮住图片与滑块,点击遮罩才刷新', async () => {
    let currentTime = 1000;
    const nowSpy = vi.spyOn(Date, 'now').mockImplementation(() => currentTime);
    await renderLogin();
    currentTime += 500_000;
    expect(nowSpy).toHaveBeenCalled();
    drag(screen.getByRole('slider'), 0, 60);
    const mask = await screen.findByRole('button', { name: '验证码已失效，点击刷新' });
    expect(mocks.issueCaptchaApi).toHaveBeenCalledTimes(1);
    expect(mocks.login).not.toHaveBeenCalled();
    expect(mocks.error).not.toHaveBeenCalled();
    fireEvent.click(mask);
    await waitFor(() => expect(mocks.issueCaptchaApi).toHaveBeenCalledTimes(2));
    expect(screen.queryByRole('button', { name: '验证码已失效，点击刷新' })).not.toBeInTheDocument();
  });

  it('锁定进度条以固定锁定时长为满格，而非首次见到的剩余时间', async () => {
    const lockMinutes = 5;
    const totalMs = lockMinutes * 60 * 1000;
    const remainingMs = 30_000;
    mocks.loginChallengeApi.mockResolvedValue({ captchaRequired: true, lockMinutes });
    mocks.login.mockRejectedValueOnce({
      response: {
        data: {
          code: 42301,
          message: '密码错误次数过多',
          data: {
            captchaRequired: true,
            kind: 'lock',
            retryAfterSeconds: remainingMs / 1000,
            lockedUntil: new Date(Date.now() + remainingMs).toISOString(),
          },
        },
      },
    });
    await renderLogin();
    drag(screen.getByRole('slider'), 0, 60);
    await waitFor(() => expect(document.querySelector('.auth-login-lock-progress')).not.toBeNull());
    const bar = document.querySelector('.auth-login-lock-progress') as HTMLElement;
    expect(loginLockProgressScale(remainingMs, totalMs)).toBeCloseTo(0.1, 5);
    const scale = parseFloat(bar.style.transform.replace('scaleX(', '').replace(')', ''));
    expect(scale).toBeGreaterThan(0.08);
    expect(scale).toBeLessThanOrEqual(0.1);
    expect(bar.style.background).toContain(loginLockAccentFromRatio(scale));
  });

  it('resolveLockTotalMs 忽略 retryAfterSeconds，仅用配置的锁定时长', () => {
    const configuredMs = 5 * 60 * 1000;
    expect(resolveLockTotalMs({ kind: 'lock', retryAfterSeconds: 30 }, configuredMs)).toBe(configuredMs);
    expect(resolveLockTotalMs({ kind: 'cooldown', retryAfterSeconds: 3 }, configuredMs)).toBe(3000);
  });

  it('刷新后会话内恢复锁定进度比例', async () => {
    const totalMs = ACCOUNT_LOCK_DURATION_MS;
    const remainingMs = totalMs / 2;
    const lockUntil = Date.now() + remainingMs;
    window.localStorage.setItem(REMEMBER_ME_KEY, 'admin');
    window.sessionStorage.setItem(
      `LOGIN_LOCK_STATE_${location.hostname}`,
      JSON.stringify({ lockUntil, lockTotalMs: totalMs, lockedUsername: 'admin' }),
    );
    render(<LoginPage />);
    await waitFor(() => expect(screen.getByPlaceholderText('请输入用户名')).toHaveValue('admin'));
    await waitFor(() => expect(document.querySelector('.auth-login-lock-progress')).not.toBeNull());
    const bar = document.querySelector('.auth-login-lock-progress') as HTMLElement;
    expect(parseFloat(bar.style.transform.replace('scaleX(', '').replace(')', ''))).toBeCloseTo(0.5, 1);
  });

  it('锁定状态下退回表单且「登录」按钮切换为倒计时面板', async () => {
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
    // 锁定态由倒计时面板承载，不再叠加会遮挡它的红色 toast
    expect(mocks.error).not.toHaveBeenCalled();
    expect(screen.getByTestId('login-lock-countdown')).toHaveTextContent(/\d{2}:\d{2}/);
  });

  it('423 锁定错误改用倒计时面板且不再弹 toast', async () => {
    mocks.login.mockRejectedValueOnce({
      response: {
        data: {
          code: 42301,
          message: '账号已锁定，请稍后再试',
          data: { captchaRequired: true, kind: 'lock', lockedUntil: new Date(Date.now() + 60_000).toISOString() },
        },
      },
    });
    await renderLogin();
    drag(screen.getByRole('slider'), 0, 60);
    await waitFor(() => expect(screen.getByTestId('login-submit')).toBeDisabled());
    expect(mocks.error).not.toHaveBeenCalled();
    expect(screen.queryByTestId('login-lock-toast')).not.toBeInTheDocument();
    // 锁定后按钮整体切换为倒计时面板：不再显示「登录」，而是锁图标 + 文案 + 倒计时
    expect(screen.getByTestId('login-submit')).not.toHaveTextContent(/^登\s*录$/);
    expect(screen.getByTestId('login-submit')).toHaveTextContent('尝试次数过多');
    expect(screen.getByTestId('login-lock-countdown')).toHaveTextContent(/\d{2}:\d{2}/);
    expect(document.querySelector('.auth-login-lock-progress')).not.toBeNull();
  });

  it('无验证码模式锁定后整条滑块切换为倒计时面板', async () => {
    mocks.loginChallengeApi.mockResolvedValue({ captchaRequired: false });
    mocks.login.mockRejectedValueOnce({
      response: {
        data: {
          code: 42301,
          message: '密码错误次数过多',
          data: { kind: 'lock', lockedUntil: new Date(Date.now() + 60_000).toISOString() },
        },
      },
    });
    render(<LoginPage />);
    await fillCredentials();
    const handle = await screen.findByRole('slider');
    drag(handle, 0, 260);
    await waitFor(() => expect(handle.parentElement).toHaveClass('is-disabled'));
    expect(handle).toBeDisabled();
    expect(mocks.error).not.toHaveBeenCalled();
    expect(screen.queryByTestId('login-lock-toast')).not.toBeInTheDocument();
    expect(screen.getByTestId('login-lock-countdown')).toHaveTextContent(/尝试次数过多/);
    expect(screen.getByTestId('login-lock-countdown')).toHaveTextContent(/\d{2}:\d{2}/);
    expect(document.querySelector('.auth-login-lock-progress')).not.toBeNull();
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
