import { App as AntApp } from 'antd';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ForcePasswordChangeModal } from './ForcePasswordChangeModal';

const mocks = vi.hoisted(() => ({
  changePasswordApi: vi.fn(),
  logout: vi.fn(),
  loadAccess: vi.fn(),
  navigate: vi.fn(),
  accessState: { userInfo: null as null | Record<string, unknown> },
}));

vi.mock('@/api/auth', () => ({ changePasswordApi: mocks.changePasswordApi }));
vi.mock('@/stores/auth', () => ({
  useAuthStore: {
    getState: () => ({ logout: mocks.logout, loadAccess: mocks.loadAccess }),
  },
}));
vi.mock('@/stores/access', () => ({
  useAccessStore: (selector: (state: { userInfo: unknown }) => unknown) =>
    selector(mocks.accessState),
}));
vi.mock('react-router-dom', () => ({ useNavigate: () => mocks.navigate }));

function renderModal() {
  return render(
    <AntApp>
      <ForcePasswordChangeModal />
    </AntApp>,
  );
}

describe('ForcePasswordChangeModal', () => {
  beforeEach(() => {
    mocks.accessState.userInfo = {
      mustChangePassword: true,
      passwordMinLength: 8,
      username: 'admin',
    };
    mocks.changePasswordApi.mockReset().mockResolvedValue({ reloginRequired: true, success: true });
    mocks.logout.mockReset().mockResolvedValue(undefined);
    mocks.loadAccess.mockReset().mockResolvedValue(undefined);
    mocks.navigate.mockReset();
  });

  afterEach(cleanup);

  it('cannot be dismissed while the initial password is pending', () => {
    renderModal();
    expect(screen.getByText('首次登录需要修改密码')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '确认修改' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '取消' })).not.toBeInTheDocument();
    expect(document.querySelector('.ant-modal-close')).toBeNull();
  });

  it('is hidden once the flag is cleared', () => {
    mocks.accessState.userInfo = { mustChangePassword: false, passwordMinLength: 8, username: 'admin' };
    renderModal();
    expect(screen.queryByText('首次登录需要修改密码')).not.toBeInTheDocument();
  });

  it('submits the new password and returns to login with the new credentials', async () => {
    renderModal();
    // Modal renders through a portal, outside the render container.
    fireEvent.change(document.querySelector<HTMLInputElement>('#oldPassword')!, { target: { value: 'OldPass1!' } });
    fireEvent.change(document.querySelector<HTMLInputElement>('#newPassword')!, { target: { value: 'NewPass9!x' } });
    fireEvent.change(document.querySelector<HTMLInputElement>('#confirm')!, { target: { value: 'NewPass9!x' } });
    fireEvent.click(screen.getByRole('button', { name: '确认修改' }));

    await waitFor(() =>
      expect(mocks.changePasswordApi).toHaveBeenCalledWith({
        newPassword: 'NewPass9!x',
        oldPassword: 'OldPass1!',
      }),
    );
    await waitFor(() => expect(mocks.logout).toHaveBeenCalledTimes(1));
    expect(mocks.navigate).toHaveBeenCalledWith('/auth/login', { replace: true });
    expect(mocks.loadAccess).not.toHaveBeenCalled();
  });
});
