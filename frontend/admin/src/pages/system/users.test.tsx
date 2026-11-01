import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { StrictMode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({
  createResourceApi: vi.fn(),
  deleteResourceApi: vi.fn(),
  getResourceApi: vi.fn(),
  getRoleGrantsApi: vi.fn(),
  listDepartmentsApi: vi.fn(),
  listMenusApi: vi.fn(),
  listPositionsApi: vi.fn(),
  listResourceApi: vi.fn(),
  listRolesApi: vi.fn(),
  saveRoleGrantsApi: vi.fn(),
  updateResourceApi: vi.fn(),
  updateResourceStatusApi: vi.fn(),
}));
const {
  createResourceApi,
  getResourceApi,
  getRoleGrantsApi,
  listDepartmentsApi,
  listMenusApi,
  listPositionsApi,
  listResourceApi,
  listRolesApi,
  saveRoleGrantsApi,
  updateResourceApi,
  updateResourceStatusApi,
} = apiMocks;

class ResizeObserverMock {
  disconnect() {}
  observe() {}
  unobserve() {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverMock);

vi.mock('@/api/system', () => ({
  createResourceApi: apiMocks.createResourceApi,
  deleteResourceApi: apiMocks.deleteResourceApi,
  getResourceApi: apiMocks.getResourceApi,
  getRoleGrantsApi: apiMocks.getRoleGrantsApi,
  listDepartmentsApi: apiMocks.listDepartmentsApi,
  listMenusApi: apiMocks.listMenusApi,
  listPositionsApi: apiMocks.listPositionsApi,
  listResourceApi: apiMocks.listResourceApi,
  listRolesApi: apiMocks.listRolesApi,
  saveRoleGrantsApi: apiMocks.saveRoleGrantsApi,
  updateResourceApi: apiMocks.updateResourceApi,
  updateResourceStatusApi: apiMocks.updateResourceStatusApi,
}));

vi.mock('@/stores/access', () => ({
  hasAccessByCodes: () => true,
  useAccessStore: Object.assign(
    (selector?: (state: { userInfo: { passwordMinLength: number; userId: string } | null }) => unknown) => {
      const state = { userInfo: { passwordMinLength: 8, userId: '1' } };
      return selector ? selector(state) : state;
    },
    { getState: () => ({ accessCodes: ['*'], userInfo: { passwordMinLength: 8, userId: '1', isSuperAdmin: true } }) },
  ),
}));

const preferencesState = {
  preferences: {
    custom: { defaultTableSize: 20, enableFormFullscreen: false },
  },
};
vi.mock('@/stores/preferences', () => ({
  usePreferencesStore: Object.assign(
    (selector?: (state: typeof preferencesState) => unknown) => (selector ? selector(preferencesState) : preferencesState),
    { getState: () => preferencesState },
  ),
}));

vi.mock('@/components/TablePanel', () => ({
  TablePanel: ({ columns, dataSource }: { columns: Array<{ dataIndex?: string; render?: (value: unknown, row: Record<string, unknown>) => React.ReactNode }>; dataSource: Array<Record<string, unknown>> }) => (
    <div data-testid="user-table">
      {dataSource.map((row) => (
        <div key={String(row.id)}>
          {columns.map((column, index) => (
            <div key={column.dataIndex || index}>{column.render ? column.render(column.dataIndex ? row[column.dataIndex] : undefined, row) : String(column.dataIndex ? row[column.dataIndex] ?? '' : '')}</div>
          ))}
        </div>
      ))}
    </div>
  ),
}));
vi.mock('@/components/FormDrawer', () => ({ FormDrawer: ({ children, onOk, open }: { children: React.ReactNode; onOk: () => void; open: boolean }) => open ? <><button type="button" onClick={onOk}>保存</button>{children}</> : null }));
vi.mock('@/components/Page', () => ({ Page: ({ children }: { children: React.ReactNode }) => <>{children}</> }));

import { RolePage, UserPage } from './users';

function mockUsersList(result: { items: Record<string, unknown>[]; total: number } | Record<string, unknown>[]) {
  listResourceApi.mockImplementation((path: string) => {
    if (path === '/system/users') return Promise.resolve(result);
    if (path === '/system/roles') return Promise.resolve([]);
    return Promise.resolve([]);
  });
}

describe('UserPage query form', () => {
  afterEach(cleanup);

  beforeEach(() => {
    listDepartmentsApi.mockReset();
    listPositionsApi.mockReset();
    listRolesApi.mockReset();
    listResourceApi.mockReset();
    createResourceApi.mockReset();
    getResourceApi.mockReset();
    updateResourceApi.mockReset();
    updateResourceStatusApi.mockReset();
    listDepartmentsApi.mockResolvedValue([
      { id: 3, name: '研发部', parentId: 0, status: 1 },
      { id: 4, name: '前端组', parentId: 3, status: 1 },
    ]);
    listPositionsApi.mockResolvedValue({ items: [] });
    listRolesApi.mockResolvedValue([]);
    mockUsersList({ items: [], total: 0 });
    createResourceApi.mockResolvedValue({ id: 10 });
    updateResourceApi.mockResolvedValue(undefined);
    updateResourceStatusApi.mockResolvedValue(undefined);
  });

  it('loads the first page without filters initially', async () => {
    render(<UserPage />);

    await waitFor(() => expect(listResourceApi).toHaveBeenCalledWith('/system/users', expect.objectContaining({ page: 1, pageSize: 20 })));
  });

  it('expands every department that has children by default', async () => {
    render(<UserPage />);

    const parent = (await screen.findByText('研发部')).closest('.ant-tree-treenode');
    expect(parent).toHaveAttribute('aria-expanded', 'true');
  });

  it('submits keyword filters and resets them', async () => {
    render(<UserPage />);
    await waitFor(() => expect(listResourceApi).toHaveBeenCalled());

    fireEvent.change(screen.getByPlaceholderText('账号、姓名或手机号'), { target: { value: 'alice' } });
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(listResourceApi).toHaveBeenLastCalledWith('/system/users', expect.objectContaining({ keyword: 'alice', page: 1, pageSize: 20 })));

    fireEvent.click(screen.getByRole('button', { name: /重\s*置/ }));
    await waitFor(() => expect(listResourceApi).toHaveBeenLastCalledWith('/system/users', expect.objectContaining({ page: 1, pageSize: 20 })));
  });

  it('submits the selected department and disabled status', async () => {
    render(<UserPage />);
    await waitFor(() => expect(listDepartmentsApi).toHaveBeenCalled());

    fireEvent.click(await screen.findByText('研发部'));
    await waitFor(() =>
      expect(listResourceApi).toHaveBeenCalledWith(
        '/system/users',
        expect.objectContaining({ departmentId: 3 }),
      ),
    );

    const toolbar = screen.getByRole('search');
    fireEvent.mouseDown(within(toolbar).getByRole('combobox'));
    fireEvent.click(await screen.findByText('禁用'));
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));

    await waitFor(() =>
      expect(listResourceApi).toHaveBeenLastCalledWith(
        '/system/users',
        expect.objectContaining({ departmentId: 3, page: 1, pageSize: 20, status: 0 }),
      ),
    );
  });

  it('prefills the selected department when creating a user', async () => {
    render(<UserPage />);
    await waitFor(() => expect(listDepartmentsApi).toHaveBeenCalled());

    fireEvent.click(await screen.findByText('研发部'));
    fireEvent.click(screen.getByRole('button', { name: /^新增$/ }));
    await screen.findByLabelText('登录账号');
    fireEvent.change(screen.getByLabelText('登录账号'), { target: { value: 'admin1' } });
    fireEvent.change(screen.getByLabelText('初始密码'), { target: { value: 'StrongPwd1!' } });
    fireEvent.change(screen.getByLabelText('用户姓名'), { target: { value: '管理员' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(createResourceApi).toHaveBeenCalledWith('/system/users', expect.objectContaining({ departmentId: 3 })));
  });

  it('keeps department empty when no department is selected before creating', async () => {
    render(<UserPage />);
    await waitFor(() => expect(listDepartmentsApi).toHaveBeenCalled());

    fireEvent.click(screen.getByRole('button', { name: /^新增$/ }));
    await screen.findByLabelText('登录账号');
    fireEvent.change(screen.getByLabelText('登录账号'), { target: { value: 'admin1' } });
    fireEvent.change(screen.getByLabelText('初始密码'), { target: { value: 'StrongPwd1!' } });
    fireEvent.change(screen.getByLabelText('用户姓名'), { target: { value: '管理员' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(createResourceApi).toHaveBeenCalledWith('/system/users', expect.not.objectContaining({ departmentId: expect.anything() })));
  });

  it('opens the edit drawer with the selected row while user details load', async () => {
    mockUsersList({ items: [{ id: 9, username: 'alice', displayName: 'Alice', status: 1 }], total: 1 });
    let resolveUserDetail: (detail: Record<string, unknown>) => void = () => undefined;
    getResourceApi.mockImplementation(() => new Promise((resolve) => { resolveUserDetail = resolve; }));

    render(<UserPage />);
    fireEvent.click(await screen.findByRole('button', { name: '修改' }));

    expect(screen.getByLabelText('登录账号')).toHaveValue('alice');
    expect(screen.getByLabelText('用户姓名')).toHaveValue('Alice');
    expect(getResourceApi).toHaveBeenCalledWith('/system/users/9');
    resolveUserDetail({ id: 9, username: 'alice', displayName: 'Alice', status: 1 });
  });

  it('uses user detail department when editing instead of the selected department', async () => {
    mockUsersList({ items: [{ id: 9, username: 'alice', displayName: 'Alice', status: 1 }], total: 1 });
    getResourceApi.mockResolvedValue({ id: 9, username: 'alice', displayName: 'Alice', departmentId: 4, gender: 0, status: 1, mustChangePassword: true, roleIds: [], positionIds: [] });

    render(<UserPage />);
    await waitFor(() => expect(listDepartmentsApi).toHaveBeenCalled());
    fireEvent.click(await screen.findByText('研发部'));
    fireEvent.click(await screen.findByRole('button', { name: '修改' }));
    await waitFor(() => expect(getResourceApi).toHaveBeenCalledWith('/system/users/9'));
    await waitFor(() => expect(Array.from(document.querySelectorAll('#status')).some((element) => element.getAttribute('aria-checked') === 'true')).toBe(true));
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(updateResourceApi).toHaveBeenCalledWith('/system/users', 9, expect.objectContaining({ departmentId: 4, status: 1 })));
  });

  it('toggles user status through the dedicated endpoint', async () => {
    mockUsersList({ items: [{ id: 9, username: 'alice', displayName: 'Alice', status: 1 }], total: 1 });

    render(<UserPage />);

    const statusSwitch = await screen.findByRole('switch');
    expect(statusSwitch).toHaveAttribute('aria-checked', 'true');
    fireEvent.click(statusSwitch);

    await waitFor(() => expect(updateResourceStatusApi).toHaveBeenCalledWith('/system/users', 9, 0));
    expect(getResourceApi).not.toHaveBeenCalled();
    expect(updateResourceApi).not.toHaveBeenCalled();
  });

  it('marks disabled departments and positions as not selectable in the user form', async () => {
    listDepartmentsApi.mockResolvedValue([
      { id: 3, name: '研发部', parentId: 0, status: 1 },
      { id: 5, name: '已停用部门', parentId: 0, status: 0 },
    ]);
    listPositionsApi.mockResolvedValue({
      items: [
        { id: 11, name: '工程师', status: 1 },
        { id: 12, name: '已停用岗位', status: 0 },
      ],
    });

    render(<UserPage />);
    await waitFor(() => expect(listDepartmentsApi).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('button', { name: /^新增$/ }));
    await screen.findByLabelText('所属部门');

    fireEvent.mouseDown(screen.getByLabelText('所属部门'));
    await waitFor(() => expect(document.querySelector('.department-tree-select-dropdown .ant-select-tree-treenode')).toBeTruthy());
    const departmentNodes = Array.from(document.querySelectorAll('.department-tree-select-dropdown .ant-select-tree-treenode'));
    const disabledDepartment = departmentNodes.find((node) => node.textContent?.includes('已停用部门'));
    const enabledDepartment = departmentNodes.find((node) => node.textContent?.includes('研发部'));
    expect(disabledDepartment).toHaveClass('ant-select-tree-treenode-disabled');
    expect(enabledDepartment).not.toHaveClass('ant-select-tree-treenode-disabled');

    fireEvent.mouseDown(screen.getByLabelText('岗位'));
    await waitFor(() => expect(document.querySelector('.ant-select-item-option')).toBeTruthy());
    const positionOptions = Array.from(document.querySelectorAll('.ant-select-item-option'));
    const disabledPosition = positionOptions.find((node) => node.textContent?.includes('已停用岗位'));
    const enabledPosition = positionOptions.find((node) => node.textContent?.includes('工程师'));
    expect(disabledPosition).toHaveClass('ant-select-item-option-disabled');
    expect(enabledPosition).not.toHaveClass('ant-select-item-option-disabled');
  });

  it('keeps a currently assigned disabled department when editing', async () => {
    listDepartmentsApi.mockResolvedValue([
      { id: 3, name: '研发部', parentId: 0, status: 1 },
      { id: 5, name: '已停用部门', parentId: 0, status: 0 },
    ]);
    mockUsersList({ items: [{ id: 9, username: 'alice', displayName: 'Alice', departmentId: 5, status: 1 }], total: 1 });
    getResourceApi.mockResolvedValue({
      id: 9,
      username: 'alice',
      displayName: 'Alice',
      departmentId: 5,
      gender: 0,
      status: 1,
      mustChangePassword: true,
      roleIds: [],
      positionIds: [],
    });

    render(<UserPage />);
    await waitFor(() => expect(listDepartmentsApi).toHaveBeenCalled());
    fireEvent.click(await screen.findByRole('button', { name: '修改' }));
    await waitFor(() => expect(getResourceApi).toHaveBeenCalledWith('/system/users/9'));
    fireEvent.mouseDown(screen.getByLabelText('所属部门'));
    await waitFor(() => expect(document.querySelector('.department-tree-select-dropdown .ant-select-tree-treenode')).toBeTruthy());
    const currentDepartment = Array.from(document.querySelectorAll('.department-tree-select-dropdown .ant-select-tree-treenode'))
      .find((node) => node.textContent?.includes('已停用部门'));
    expect(currentDepartment).toBeTruthy();
    expect(currentDepartment).not.toHaveClass('ant-select-tree-treenode-disabled');
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(updateResourceApi).toHaveBeenCalledWith('/system/users', 9, expect.objectContaining({ departmentId: 5 })));
  });

  it('does not prefill a disabled department when creating a user', async () => {
    listDepartmentsApi.mockResolvedValue([
      { id: 3, name: '研发部', parentId: 0, status: 1 },
      { id: 5, name: '已停用部门', parentId: 0, status: 0 },
    ]);

    render(<UserPage />);
    await waitFor(() => expect(listDepartmentsApi).toHaveBeenCalled());
    fireEvent.click(await screen.findByText('已停用部门'));
    fireEvent.click(screen.getByRole('button', { name: /^新增$/ }));
    await screen.findByLabelText('登录账号');
    fireEvent.change(screen.getByLabelText('登录账号'), { target: { value: 'admin1' } });
    fireEvent.change(screen.getByLabelText('初始密码'), { target: { value: 'StrongPwd1!' } });
    fireEvent.change(screen.getByLabelText('用户姓名'), { target: { value: '管理员' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(createResourceApi).toHaveBeenCalledWith('/system/users', expect.not.objectContaining({ departmentId: expect.anything() })));
  });
});

describe('RolePage system role operations', () => {
  afterEach(cleanup);

  const rowOf = (container: HTMLElement | null) => container?.closest('div[data-testid="user-table"] > div');

  beforeEach(() => {
    getRoleGrantsApi.mockReset();
    listDepartmentsApi.mockReset();
    listMenusApi.mockReset();
    listResourceApi.mockReset();
    saveRoleGrantsApi.mockReset();
    updateResourceApi.mockReset();
    updateResourceStatusApi.mockReset();
    getRoleGrantsApi.mockResolvedValue({ dataScope: 'self', departmentIds: [], menuIds: [], permissionIds: [] });
    listDepartmentsApi.mockResolvedValue([]);
    listMenusApi.mockResolvedValue([]);
    saveRoleGrantsApi.mockResolvedValue(undefined);
    updateResourceApi.mockResolvedValue(undefined);
    updateResourceStatusApi.mockResolvedValue(undefined);
  });

  it('renders the status switch inside the role drawer', async () => {
    listResourceApi.mockResolvedValue([{ id: 2, name: '运维', code: 'ops', dataScope: 'self', userCount: 0, status: 0, isSystem: false }]);

    render(<StrictMode><RolePage /></StrictMode>);
    fireEvent.click(await screen.findByRole('button', { name: '修改' }));

    await waitFor(() => {
      expect(screen.getByLabelText('角色名称')).toHaveValue('运维');
      expect(screen.getByLabelText('角色编码')).toHaveValue('ops');
      expect(document.getElementById('status')).toBeInTheDocument();
      expect(document.getElementById('status')).toHaveAttribute('aria-checked', 'false');
    });
  });

  it('shows the freshly loaded row when editing a different role after saving', async () => {
    listResourceApi.mockResolvedValue([
      { id: 2, name: '运维', code: 'ops', dataScope: 'self', userCount: 0, status: 1, isSystem: false },
      { id: 3, name: '审计', code: 'audit', dataScope: 'all', userCount: 0, status: 1, isSystem: false },
    ]);

    render(<StrictMode><RolePage /></StrictMode>);

    const editButtons = await screen.findAllByRole('button', { name: '修改' });
    expect(editButtons.length).toBe(2);
    fireEvent.click(editButtons[0]);
    await waitFor(() => expect(screen.getByLabelText('角色名称')).toHaveValue('运维'));

    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => expect(screen.queryByLabelText('角色名称')).not.toBeInTheDocument());

    fireEvent.click(screen.getAllByRole('button', { name: '修改' })[1]);
    await waitFor(() => expect(screen.getByLabelText('角色名称')).toHaveValue('审计'));
    expect(screen.getByLabelText('角色编码')).toHaveValue('audit');
  });

  it('toggles the role status through the dedicated endpoint', async () => {
    listResourceApi.mockResolvedValue([{ id: 2, name: '运维', code: 'ops', dataScope: 'self', userCount: 0, status: 1, isSystem: false }]);

    render(<StrictMode><RolePage /></StrictMode>);

    const statusSwitch = (await screen.findAllByRole('switch'))[0];
    expect(statusSwitch).toHaveAttribute('aria-checked', 'true');
    fireEvent.click(statusSwitch);

    await waitFor(() => expect(updateResourceStatusApi).toHaveBeenCalledWith('/system/roles', 2, 0));
  });

  it('disables every action on the built-in super admin role only', async () => {
    listResourceApi.mockResolvedValue([
      { id: 1, name: '超级管理员', code: 'super_admin', dataScope: 'all', userCount: 1, status: 1, isSystem: true },
      { id: 2, name: '运维', code: 'ops', dataScope: 'self', userCount: 0, status: 1, isSystem: false },
    ]);

    render(<StrictMode><RolePage /></StrictMode>);

    const adminRow = rowOf(await screen.findByText('超级管理员'));
    expect(adminRow).not.toBeNull();
    for (const label of ['修改', '授权', '删除']) {
      expect(within(adminRow!).getByRole('button', { name: label })).toBeDisabled();
    }

    const normalRow = rowOf(await screen.findByText('运维'));
    expect(normalRow).not.toBeNull();
    for (const label of ['修改', '授权', '删除']) {
      expect(within(normalRow!).getByRole('button', { name: label })).toBeEnabled();
    }
  });

  it('filters roles locally by name or code', async () => {
    listResourceApi.mockResolvedValue([
      { id: 2, name: '运维', code: 'ops', dataScope: 'self', userCount: 0, status: 1, isSystem: false },
      { id: 3, name: '审计', code: 'audit', dataScope: 'all', userCount: 0, status: 1, isSystem: false },
    ]);

    render(<StrictMode><RolePage /></StrictMode>);
    await screen.findByText('运维');
    const callsAfterLoad = listResourceApi.mock.calls.length;

    fireEvent.change(screen.getByPlaceholderText('名称或编码'), { target: { value: 'audit' } });

    expect(screen.queryByText('运维')).toBeNull();
    expect(screen.getByText('审计')).toBeInTheDocument();
    expect(listResourceApi.mock.calls.length).toBe(callsAfterLoad);
  });

  it('renders permission buttons inside the menu tree and saves only menu ids', async () => {
    listResourceApi.mockResolvedValue([{ id: 2, name: '运维', code: 'ops', dataScope: 'self', userCount: 0, status: 1, isSystem: false }]);
    // Only the button is granted; saving must add its ancestors on the way up.
    getRoleGrantsApi.mockResolvedValue({ dataScope: 'self', departmentIds: [], menuIds: [12], permissionIds: [] });
    listMenusApi.mockResolvedValue([
      { id: 10, parentId: 0, name: '组织权限', icon: 'lucide:shield-check' },
      { id: 11, parentId: 10, name: '用户管理', icon: 'lucide:users', type: 'page', code: 'system_user' },
      { id: 12, parentId: 11, name: '查询用户', type: 'button', code: 'system:user:query', method: 'GET', path: '/api/system/users' },
    ]);

    render(<StrictMode><RolePage /></StrictMode>);
    fireEvent.click(await screen.findByRole('button', { name: '授权' }));

    await screen.findByText('菜单与按钮');
    expect(screen.getByText('组织权限').closest('.role-grant-tree-title')?.querySelector('svg')).not.toBeNull();
    expect(await screen.findByText('查询用户')).toBeInTheDocument();
    // A button is visually distinct and shows its permission code.
    expect(screen.getByText('按钮')).toBeInTheDocument();
    expect(screen.getByText('system:user:query')).toBeInTheDocument();
    expect(screen.queryByText('接口权限')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(saveRoleGrantsApi).toHaveBeenCalled());
    const payload = saveRoleGrantsApi.mock.calls.at(-1)?.[1] as Record<string, unknown>;
    expect(payload).not.toHaveProperty('permissionIds');
    expect(payload.menuIds).toEqual(expect.arrayContaining([10, 11, 12]));
  });

  it('expands selected parent menus to child menu ids before saving grants', async () => {
    listResourceApi.mockResolvedValue([{ id: 2, name: '运维', code: 'ops', dataScope: 'self', userCount: 0, status: 1, isSystem: false }]);
    getRoleGrantsApi.mockResolvedValue({ dataScope: 'self', departmentIds: [], menuIds: [10], permissionIds: [] });
    listMenusApi.mockResolvedValue([
      { id: 10, parentId: 0, name: '组织权限', icon: 'lucide:shield-check' },
      { id: 11, parentId: 10, name: '用户管理', icon: 'lucide:users' },
      { id: 12, parentId: 10, name: '部门管理', icon: 'lucide:network' },
    ]);

    render(<StrictMode><RolePage /></StrictMode>);
    fireEvent.click(await screen.findByRole('button', { name: '授权' }));
    await screen.findByText('组织权限');

    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(saveRoleGrantsApi).toHaveBeenCalledWith(2, expect.objectContaining({ menuIds: expect.arrayContaining([10, 11, 12]) })));
  });

  it('includes parent menu ids when only child menus are selected', async () => {
    listResourceApi.mockResolvedValue([{ id: 2, name: '运维', code: 'ops', dataScope: 'self', userCount: 0, status: 1, isSystem: false }]);
    getRoleGrantsApi.mockResolvedValue({ dataScope: 'self', departmentIds: [], menuIds: [11], permissionIds: [] });
    listMenusApi.mockResolvedValue([
      { id: 10, parentId: 0, name: '组织权限', icon: 'lucide:shield-check' },
      { id: 11, parentId: 10, name: '用户管理', icon: 'lucide:users' },
      { id: 12, parentId: 10, name: '部门管理', icon: 'lucide:network' },
    ]);

    render(<StrictMode><RolePage /></StrictMode>);
    fireEvent.click(await screen.findByRole('button', { name: '授权' }));
    await screen.findByText('用户管理');

    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(saveRoleGrantsApi).toHaveBeenCalledWith(2, expect.objectContaining({ menuIds: expect.arrayContaining([10, 11]) })));
  });
});
