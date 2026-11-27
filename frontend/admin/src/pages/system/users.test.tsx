import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
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

const resizeObserverCallbacks: ResizeObserverCallback[] = [];
class ResizeObserverMock {
  constructor(callback: ResizeObserverCallback) {
    resizeObserverCallbacks.push(callback);
  }
  disconnect() {}
  observe() {}
  unobserve() {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverMock);

// Static antd message mounts its own React root on a timer. Under jsdom that
// render can land after this file's environment is torn down and surfaces as
// an unhandled "window is not defined" from react-dom's scheduler.
vi.mock('antd', async (importOriginal) => {
  const actual = await importOriginal<typeof import('antd')>();
  return {
    ...actual,
    message: { ...actual.message, error: vi.fn(), success: vi.fn(), info: vi.fn(), warning: vi.fn() },
  };
});

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
    app: { timezone: 'Asia/Shanghai' },
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

import { useSystemConfigStore } from '@/stores/system-config';
import { RolePage, UserPage } from './users';

function mockUsersList(result: { items: Record<string, unknown>[]; total: number } | Record<string, unknown>[]) {
  listResourceApi.mockImplementation((path: string) => {
    if (path === '/system/users') return Promise.resolve(result);
    if (path === '/system/roles') return Promise.resolve([]);
    return Promise.resolve([]);
  });
}

async function flushPendingUi() {
  // Let antd Form debounce / Tree measure timers settle while jsdom still exists.
  await new Promise((resolve) => setTimeout(resolve, 50));
}

describe('UserPage query form', () => {
  afterEach(async () => {
    cleanup();
    await flushPendingUi();
    useSystemConfigStore.setState({ loaded: false, values: {} });
  });

  beforeEach(() => {
    resizeObserverCallbacks.length = 0;
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

  it('resizes, hides the divider when collapsed, and restores the department tree', async () => {
    const { container } = render(<UserPage />);
    await screen.findByText('研发部');

    expect(screen.queryByRole('button', { name: '钉住' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '取消钉住' })).not.toBeInTheDocument();
    const resizer = screen.getByRole('separator', { name: '拖动调整部门树宽度' });
    expect(container.querySelector('.department-side-panel-resize-grip')).not.toBeNull();
    fireEvent.pointerDown(resizer, { button: 0, clientX: 300, pointerId: 1 });
    fireEvent.pointerMove(resizer, { clientX: 380, pointerId: 1 });
    fireEvent.pointerUp(resizer, { clientX: 380, pointerId: 1 });

    const treeCard = container.querySelector('.department-tree-card');
    expect(container.querySelector('.user-list-layout')).toHaveStyle({ '--side-panel-width': '322px' });
    expect(document.body.style.cursor).toBe('');

    fireEvent.click(screen.getByRole('button', { name: '收起部门树' }));
    expect(container.querySelector('.user-list-layout')).toHaveClass('is-side-panel-collapsed');
    expect(treeCard).toHaveAttribute('aria-hidden', 'true');
    expect(screen.queryByRole('separator')).not.toBeInTheDocument();
    // 收起态的横线靠这个 class 在 CSS 里隐藏
    expect(container.querySelector('.department-side-panel-resizer')).toHaveClass('is-side-panel-collapsed');

    fireEvent.click(screen.getByRole('button', { name: '展开部门树' }));
    expect(container.querySelector('.user-list-layout')).not.toHaveClass('is-side-panel-collapsed');
    expect(container.querySelector('.user-list-layout')).toHaveStyle({ '--side-panel-width': '322px' });
    expect(screen.getByText('研发部')).toBeInTheDocument();
    expect(screen.getByRole('separator', { name: '拖动调整部门树宽度' })).toBeInTheDocument();
    expect(listDepartmentsApi).toHaveBeenCalledTimes(1);
  });

  it('shows the hidden role and position tag count and restores all tags when wide', async () => {
    const names = ['角色一', '角色二', '角色三', '角色四', '角色五'];
    mockUsersList({
      items: [{ id: 9, username: 'alice', roleNames: names.join(','), positionNames: names.join(',') }],
      total: 1,
    });
    render(<UserPage />);
    await screen.findByText('alice');

    const cells = Array.from(document.querySelectorAll<HTMLElement>('.list-tag-cell'));
    expect(cells).toHaveLength(2);
    for (const cell of cells) {
      Object.defineProperty(cell, 'clientWidth', { configurable: true, value: 140 });
      const measured = Array.from(cell.querySelector('.list-tag-measure')!.children) as HTMLElement[];
      measured.forEach((element) => {
        const width = element.textContent?.startsWith('+') ? 34 : 60;
        element.getBoundingClientRect = () => ({
          width, height: 22, x: 0, y: 0, top: 0, left: 0, right: width, bottom: 22,
          toJSON: () => ({}),
        });
      });
    }
    await act(async () => {
      resizeObserverCallbacks.forEach((callback) => callback([], {} as ResizeObserver));
    });

    for (const cell of cells) {
      const visibleTags = Array.from(cell.querySelectorAll(':scope > .ant-tag'));
      expect(visibleTags.map((tag) => tag.textContent)).toEqual(['角色一', '+4']);
      // 悬浮提示里以同样的彩色标签展示被隐藏的名字，而不是纯文本 title。
      const overflow = cell.querySelector('.list-tag-overflow')!;
      expect(overflow).not.toHaveAttribute('title');
      fireEvent.mouseEnter(overflow);
      await waitFor(() => {
        const tooltipTags = Array.from(document.querySelectorAll('.ant-tooltip .ant-tag'));
        expect(tooltipTags.map((tag) => tag.textContent)).toEqual(['角色二', '角色三', '角色四', '角色五']);
        expect(tooltipTags.every((tag) => /(^|\s)ant-tag-[a-z]+/.test(tag.className))).toBe(true);
      });
      fireEvent.mouseLeave(overflow);
    }

    cells.forEach((cell) => Object.defineProperty(cell, 'clientWidth', { configurable: true, value: 1000 }));
    await act(async () => {
      resizeObserverCallbacks.forEach((callback) => callback([], {} as ResizeObserver));
    });
    for (const cell of cells) {
      const visibleTags = Array.from(cell.querySelectorAll(':scope > .ant-tag'));
      expect(visibleTags.map((tag) => tag.textContent)).toEqual(names);
    }
  });

  it('leaves cells with missing list data blank', async () => {
    mockUsersList({
      items: [{ id: 9, username: 'alice', displayName: null, mobile: '', roleNames: '', positionNames: null, departmentName: '', status: null, createdAt: null }],
      total: 1,
    });
    const { container } = render(<UserPage />);
    await screen.findByRole('button', { name: '修改' });

    expect(container).not.toHaveTextContent('—');
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
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

  it('disables password reset for the built-in super admin only', async () => {
    mockUsersList({
      items: [
        { id: 1, username: 'admin', displayName: 'Super Admin', status: 1, isSuperAdmin: true },
        { id: 2, username: 'alice', displayName: 'Alice', status: 1, isSuperAdmin: false },
      ],
      total: 2,
    });

    render(<UserPage />);
    await screen.findByText('Alice');

    const rowOf = (name: string) => screen.getByText(name).closest('div[data-testid="user-table"] > div');
    expect(within(rowOf('Super Admin')!).getByRole('button', { name: '重置密码' })).toBeDisabled();
    expect(within(rowOf('Alice')!).getByRole('button', { name: '重置密码' })).toBeEnabled();
  });
});

describe('RolePage system role operations', () => {
  afterEach(async () => {
    cleanup();
    await flushPendingUi();
    useSystemConfigStore.setState({ loaded: false, values: {} });
  });

  const rowOf = (container: HTMLElement | null) => container?.closest('div[data-testid="user-table"] > div');

  beforeEach(() => {
    getRoleGrantsApi.mockReset();
    listDepartmentsApi.mockReset();
    listMenusApi.mockReset();
    listResourceApi.mockReset();
    saveRoleGrantsApi.mockReset();
    updateResourceApi.mockReset();
    updateResourceStatusApi.mockReset();
    getRoleGrantsApi.mockResolvedValue({ dataScope: 1, departmentIds: [], menuIds: [], permissionIds: [] });
    listDepartmentsApi.mockResolvedValue([]);
    listMenusApi.mockResolvedValue([]);
    saveRoleGrantsApi.mockResolvedValue(undefined);
    updateResourceApi.mockResolvedValue(undefined);
    updateResourceStatusApi.mockResolvedValue(undefined);
  });

  it('renders the status switch inside the role drawer', async () => {
    listResourceApi.mockResolvedValue([{ id: 2, name: '运维', code: 'ops', dataScope: 1, userCount: 0, status: 0, isSystem: false }]);

    render(<StrictMode><RolePage /></StrictMode>);
    fireEvent.click(await screen.findByRole('button', { name: '修改' }));

    await waitFor(() => {
      expect(screen.getByLabelText('角色名称')).toHaveValue('运维');
      expect(screen.getByLabelText('角色编码')).toHaveValue('ops');
      expect(document.getElementById('status')).toBeInTheDocument();
      expect(document.getElementById('status')).toHaveAttribute('aria-checked', 'false');
    });
  });

  it('orders data scope options by their enum values', async () => {
    listResourceApi.mockResolvedValue([]);

    render(<StrictMode><RolePage /></StrictMode>);
    fireEvent.click(await screen.findByRole('button', { name: '新增' }));

    await waitFor(() => expect(document.getElementById('dataScope')).toBeInTheDocument());
    fireEvent.mouseDown(document.getElementById('dataScope')!.closest('.ant-select')!);

    await waitFor(() => {
      const options = Array.from(document.querySelectorAll('.ant-select-item-option'));
      expect(options.length).toBe(5);
      expect(options.map((option) => option.textContent)).toEqual([
        '全部数据', '仅本人', '本部门', '本部门及下级', '自定义部门',
      ]);
    });
  });

  it('shows the freshly loaded row when editing a different role after saving', async () => {
    listResourceApi.mockResolvedValue([
      { id: 2, name: '运维', code: 'ops', dataScope: 1, userCount: 0, status: 1, isSystem: false },
      { id: 3, name: '审计', code: 'audit', dataScope: 0, userCount: 0, status: 1, isSystem: false },
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
    listResourceApi.mockResolvedValue([{ id: 2, name: '运维', code: 'ops', dataScope: 1, userCount: 0, status: 1, isSystem: false }]);

    render(<StrictMode><RolePage /></StrictMode>);

    const statusSwitch = (await screen.findAllByRole('switch'))[0];
    expect(statusSwitch).toHaveAttribute('aria-checked', 'true');
    fireEvent.click(statusSwitch);

    await waitFor(() => expect(updateResourceStatusApi).toHaveBeenCalledWith('/system/roles', 2, 0));
  });

  it('disables every action on the built-in super admin role only', async () => {
    listResourceApi.mockResolvedValue([
      { id: 1, name: '超级管理员', code: 'super_admin', dataScope: 0, userCount: 1, status: 1, isSystem: true },
      { id: 2, name: '运维', code: 'ops', dataScope: 1, userCount: 0, status: 1, isSystem: false },
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

  it('filters roles locally on query click and resets with the reset button', async () => {
    useSystemConfigStore.setState({
      loaded: true,
      values: { 'system.dateTimeFormat': 'YYYY/MM/DD HH:mm' },
    });
    listResourceApi.mockResolvedValue([
      { id: 2, name: '运维', code: 'ops', dataScope: 1, userCount: 0, status: 1, isSystem: false, createdAt: '2026-11-19T18:19:01.2+08:00' },
      { id: 3, name: '审计', code: 'audit', dataScope: 0, userCount: 0, status: 1, isSystem: false, createdAt: '2026-11-20T09:05:00+08:00' },
    ]);

    render(<StrictMode><RolePage /></StrictMode>);
    await screen.findByText('运维');
    expect(screen.getByText('2026/11/19 18:19')).toBeInTheDocument();
    const callsAfterLoad = listResourceApi.mock.calls.length;

    const queryButton = screen.getByRole('button', { name: '查询' });
    const resetButton = screen.getByRole('button', { name: '重置' });
    const searchInput = screen.getByPlaceholderText('名称或编码');
    fireEvent.change(searchInput, { target: { value: 'audit' } });
    // Staged local search: typing alone does not filter.
    expect(screen.getByText('运维')).toBeInTheDocument();

    fireEvent.click(queryButton);

    expect(screen.queryByText('运维')).toBeNull();
    expect(screen.getByText('审计')).toBeInTheDocument();
    expect(listResourceApi.mock.calls.length).toBe(callsAfterLoad);

    fireEvent.click(resetButton);

    expect(searchInput).toHaveValue('');
    expect(screen.getByText('运维')).toBeInTheDocument();
    expect(screen.getByText('审计')).toBeInTheDocument();
  });

  it('renders permission buttons inside the menu tree and saves only menu ids', async () => {
    listResourceApi.mockResolvedValue([{ id: 2, name: '运维', code: 'ops', dataScope: 1, userCount: 0, status: 1, isSystem: false }]);
    // Only the button is granted; saving must add its ancestors on the way up.
    getRoleGrantsApi.mockResolvedValue({ dataScope: 1, departmentIds: [], menuIds: [12], permissionIds: [] });
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
    listResourceApi.mockResolvedValue([{ id: 2, name: '运维', code: 'ops', dataScope: 1, userCount: 0, status: 1, isSystem: false }]);
    getRoleGrantsApi.mockResolvedValue({ dataScope: 1, departmentIds: [], menuIds: [10], permissionIds: [] });
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
    listResourceApi.mockResolvedValue([{ id: 2, name: '运维', code: 'ops', dataScope: 1, userCount: 0, status: 1, isSystem: false }]);
    getRoleGrantsApi.mockResolvedValue({ dataScope: 1, departmentIds: [], menuIds: [11], permissionIds: [] });
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
