import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { StrictMode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({
  listResourceApi: vi.fn(),
  listDepartmentsApi: vi.fn(),
  listMenusApi: vi.fn(),
  listUsersApi: vi.fn(),
}));

vi.mock('@/api/system', () => ({
  createResourceApi: vi.fn(),
  deleteResourceApi: vi.fn(),
  listResourceApi: apiMocks.listResourceApi,
  updateResourceApi: vi.fn(),
  listDepartmentsApi: apiMocks.listDepartmentsApi,
  listMenusApi: apiMocks.listMenusApi,
  listUsersApi: apiMocks.listUsersApi,
}));
vi.mock('@/stores/access', () => ({ hasAccessByCodes: () => true }));

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

class ResizeObserverMock {
  disconnect() {}
  observe() {}
  unobserve() {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverMock);
vi.mock('@/components/Page', () => ({ Page: ({ children }: { children: React.ReactNode }) => <>{children}</> }));

import { MenuPage } from './organization';

const menus = [
  { id: 1, parentId: 0, type: 'directory', name: '组织权限', code: 'organization', icon: 'lucide:shield-check', routePath: '/organization', sort: 1, status: 1 },
  { id: 2, parentId: 1, type: 'page', name: '用户管理', code: 'system_user', icon: 'lucide:users', routePath: '/organization/users', sort: 1, status: 1 },
  // managed: 1 = the route table owns it (declared permission code).
  { id: 3, parentId: 2, type: 'button', name: '新增用户', code: 'system:user:create', icon: '', routePath: '', method: 'POST', path: '/api/system/users', sort: 1, status: 1, managed: 1 },
  { id: 4, parentId: 0, type: 'page', name: '工作台', code: 'dashboard', icon: 'lucide:layout-dashboard', routePath: '/analytics', sort: 0, status: 1 },
  // managed: 0 = created by an administrator, fully editable.
  { id: 5, parentId: 2, type: 'button', name: '导出用户', code: 'system:user:export', icon: '', routePath: '', method: 'GET', path: '/api/system/users/export', sort: 2, status: 1, managed: 0 },
];

function mockMenus() {
  apiMocks.listResourceApi.mockResolvedValue(menus);
  apiMocks.listMenusApi.mockResolvedValue(menus);
  apiMocks.listDepartmentsApi.mockResolvedValue([]);
  apiMocks.listUsersApi.mockResolvedValue({ items: [], total: 0 });
}

const rowOf = (element: HTMLElement) => element.closest('.ant-table-row');

async function expand(container: HTMLElement, name: string) {
  const row = rowOf(await screen.findByText(name));
  const button = Array.from(container.querySelectorAll('.ant-table-row-expand-icon:not(.ant-table-row-expand-icon-spaced)')).find((b) => row?.contains(b));
  if (button) fireEvent.click(button);
}

async function openTreeOptions(id: string): Promise<string[]> {
  fireEvent.mouseDown(document.getElementById(id)!);
  await waitFor(() => expect(document.querySelector('.department-tree-select-dropdown .department-tree-title')).not.toBeNull());
  return Array.from(document.querySelectorAll('.department-tree-select-dropdown .department-tree-title')).map((node) => node.textContent ?? '');
}

function readSegmented(): string[] {
  return Array.from(document.querySelectorAll('.ant-segmented-item-label')).map((node) => node.textContent ?? '');
}

describe('MenuPage', () => {
  afterEach(cleanup);

  it('renders the icon before the menu name in the name column', async () => {
    mockMenus();
    render(<StrictMode><MenuPage /></StrictMode>);

    const nameCell = rowOf(await screen.findByText('组织权限'))?.querySelector('.menu-name-cell');
    expect(nameCell?.closest('td')).toHaveClass('ant-table-cell-ellipsis');
    expect(nameCell?.querySelector('svg')).not.toBeNull();
    expect(nameCell?.textContent).toContain('组织权限');
    const tip = nameCell?.querySelector('.table-panel-ellipsis-tip');
    expect(tip?.previousElementSibling?.tagName.toLowerCase()).toBe('svg');
  });

  it('left-aligns the menu name column', async () => {
    mockMenus();
    render(<StrictMode><MenuPage /></StrictMode>);

    const nameCell = rowOf(await screen.findByText('组织权限'))?.querySelector('.menu-name-cell');
    expect(nameCell?.closest('td')).toHaveStyle({ textAlign: 'left' });
  });

  it('enables 新增下级 on directories and pages but not on buttons', async () => {
    mockMenus();
    const { container } = render(<StrictMode><MenuPage /></StrictMode>);

    const directoryRow = rowOf(await screen.findByText('组织权限'));
    expect(within(directoryRow!).getByRole('button', { name: '新增下级' })).toBeEnabled();

    await expand(container, '组织权限');
    const pageRow = rowOf(await screen.findByText('用户管理'));
    // A page's children are permission buttons, so it can add sub-nodes.
    expect(within(pageRow!).getByRole('button', { name: '新增下级' })).toBeEnabled();

    await expand(container, '用户管理');
    const buttonRow = rowOf(await screen.findByText('新增用户'));
    expect(within(buttonRow!).getByRole('button', { name: '新增下级' })).toBeDisabled();
  });

  it('offers only 按钮 when adding under a page', async () => {
    mockMenus();
    const { container } = render(<StrictMode><MenuPage /></StrictMode>);

    await expand(container, '组织权限');
    const pageRow = rowOf(await screen.findByText('用户管理'));
    fireEvent.click(within(pageRow!).getByRole('button', { name: '新增下级' }));

    await waitFor(() => expect(screen.getByText('新增菜单')).toBeInTheDocument());
    await waitFor(() => expect(readSegmented().length).toBeGreaterThan(0));
    expect(readSegmented()).toEqual(['按钮']);
  });

  it('limits type options to directory/page when adding under a directory', async () => {
    mockMenus();
    render(<StrictMode><MenuPage /></StrictMode>);

    const directoryRow = rowOf(await screen.findByText('组织权限'));
    fireEvent.click(within(directoryRow!).getByRole('button', { name: '新增下级' }));

    await waitFor(() => expect(screen.getByText('新增菜单')).toBeInTheDocument());
    await waitFor(() => expect(readSegmented().length).toBeGreaterThan(0));
    const options = readSegmented();
    expect(options).toEqual(expect.arrayContaining(['目录', '页面']));
    expect(options).not.toContain('按钮');
  });

  it('keeps only the current type when editing a node that has children', async () => {
    mockMenus();
    const { container } = render(<StrictMode><MenuPage /></StrictMode>);

    await expand(container, '组织权限');
    const pageRow = rowOf(await screen.findByText('用户管理'));
    fireEvent.click(within(pageRow!).getByRole('button', { name: '修改' }));

    await waitFor(() => expect(screen.getByText('修改菜单')).toBeInTheDocument());
    await waitFor(() => expect(readSegmented().length).toBeGreaterThan(0));
    expect(readSegmented()).toEqual(['页面']);
  });

  it('offers only directory/page types without button when editing a page without children', async () => {
    mockMenus();
    render(<StrictMode><MenuPage /></StrictMode>);

    const pageRow = rowOf(await screen.findByText('工作台'));
    fireEvent.click(within(pageRow!).getByRole('button', { name: '修改' }));

    await waitFor(() => expect(screen.getByText('修改菜单')).toBeInTheDocument());
    await waitFor(() => expect(readSegmented().length).toBeGreaterThan(0));
    expect(readSegmented()).toEqual(['目录', '页面']);
  });

  it('offers directories and pages as parent options, excluding self and descendants', async () => {
    mockMenus();
    const { container } = render(<StrictMode><MenuPage /></StrictMode>);

    await expand(container, '组织权限');
    const pageRow = rowOf(await screen.findByText('用户管理'));
    fireEvent.click(within(pageRow!).getByRole('button', { name: '修改' }));

    await waitFor(() => expect(screen.getByText('修改菜单')).toBeInTheDocument());
    // 上级菜单选中框只显示文字、不显示图标（图标仅在下拉中）。
    await waitFor(() => {
      const box = document.getElementById('parentId')?.closest('.ant-select-content');
      expect(box?.textContent).toContain('组织权限');
      expect(box?.querySelector('.department-tree-title')).toBeNull();
    });
    const options = await openTreeOptions('parentId');
    // 按钮不能挂子级，所以不出现在选项里；页面可以，因为按钮要挂在页面下。
    expect(options).toContain('组织权限');
    expect(options).toContain('工作台');
    expect(options).not.toContain('用户管理');
    expect(options).not.toContain('新增用户');
  });

  it('opens the icon picker popover and selects an icon', async () => {
    mockMenus();
    render(<StrictMode><MenuPage /></StrictMode>);
    fireEvent.click(await screen.findByRole('button', { name: '新增' }));
    await waitFor(() => expect(screen.getByText('新增菜单')).toBeInTheDocument());

    const box = await screen.findByRole('button', { name: '选择图标' });
    expect(document.getElementById('icon')?.closest('.icon-picker')?.textContent).toContain('请选择');
    fireEvent.click(box);

    await waitFor(() => expect(document.querySelector('.icon-picker-item')).not.toBeNull());
    // 分页控件：第一页 / 前5页 / 页码 / 后5页 / 最后一页
    expect(document.querySelector('[aria-label="第一页"]')).not.toBeNull();
    expect(document.querySelector('[aria-label="向前翻 5 页"]')).not.toBeNull();
    expect(document.querySelector('[aria-label="向后翻 5 页"]')).not.toBeNull();
    expect(document.querySelector('[aria-label="最后一页"]')).not.toBeNull();
    expect(Array.from(document.querySelectorAll('.icon-picker-pagination-pages button')).some((node) => node.textContent === '1')).toBe(true);

    const firstIcon = document.querySelector('.icon-picker-item')!;
    fireEvent.click(firstIcon);

    await waitFor(() => {
      const box2 = document.getElementById('icon')?.closest('.icon-picker');
      expect(box2?.querySelector('.icon-picker-name')?.textContent).not.toContain('请选择');
      expect(box2?.querySelector('.icon-picker-icon svg')).not.toBeNull();
    });
  }, 120000);

  it('submits the type the segmented control already shows', async () => {
    const createResourceApi = (await import('@/api/system')).createResourceApi as ReturnType<typeof vi.fn>;
    mockMenus();
    render(<StrictMode><MenuPage /></StrictMode>);
    fireEvent.click(await screen.findByRole('button', { name: '新增' }));
    await waitFor(() => expect(screen.getByText('新增菜单')).toBeInTheDocument());

    // The segmented highlights 目录 from the first render, so that is what the
    // operator believes is selected and saving must not report it as unset.
    await waitFor(() => expect(document.querySelector('.ant-segmented-item-selected')?.textContent).toBe('目录'));
    fireEvent.change(document.getElementById('name')!, { target: { value: '新菜单' } });
    fireEvent.change(document.getElementById('code')!, { target: { value: 'system:new-menu' } });
    fireEvent.click(screen.getByRole('button', { name: '保 存' }));

    await waitFor(() => expect(createResourceApi).toHaveBeenCalled());
    expect(createResourceApi.mock.calls[0]?.[1]).toMatchObject({ type: 'directory', name: '新菜单' });
    expect(document.querySelector('.ant-form-item-explain-error')?.textContent).toBeUndefined();
  }, 120000);

  it('follows the offered types when the parent narrows them', async () => {
    const createResourceApi = (await import('@/api/system')).createResourceApi as ReturnType<typeof vi.fn>;
    mockMenus();
    createResourceApi.mockClear();
    render(<StrictMode><MenuPage /></StrictMode>);
    fireEvent.click(await screen.findByRole('button', { name: '新增' }));
    await waitFor(() => expect(document.querySelector('.ant-segmented-item-selected')?.textContent).toBe('目录'));

    // Picking a page as the parent leaves 按钮 as the only legal type, so the
    // adopted 目录 must not linger behind an unselected control.
    fireEvent.mouseDown(document.getElementById('parentId')!);
    await waitFor(() => expect(document.querySelector('.department-tree-select-dropdown .department-tree-title')).not.toBeNull());
    const pageNode = Array.from(document.querySelectorAll('.department-tree-select-dropdown .department-tree-title')).find((node) => node.textContent?.includes('用户管理'))!;
    fireEvent.click(pageNode);
    fireEvent.mouseDown(document.body);

    await waitFor(() => expect(readSegmented()).toEqual(['按钮']), { timeout: 15000 });
    await waitFor(() => expect(document.querySelector('.ant-segmented-item-selected')?.textContent).toBe('按钮'), { timeout: 15000 });

    fireEvent.change(document.getElementById('name')!, { target: { value: '导出日志' } });
    fireEvent.change(document.getElementById('code')!, { target: { value: 'system:user:export-log' } });
    fireEvent.click(screen.getByRole('button', { name: '保 存' }));

    await waitFor(() => expect(createResourceApi).toHaveBeenCalled());
    expect(createResourceApi.mock.calls[0]?.[1]).toMatchObject({ parentId: 2, type: 'button', name: '导出日志' });
  }, 120000);
});


describe('MenuPage permission buttons', () => {
  afterEach(cleanup);

  it('locks endpoint fields on a route-synced button', async () => {
    mockMenus();
    const { container } = render(<StrictMode><MenuPage /></StrictMode>);
    await expand(container, '组织权限');
    await expand(container, '用户管理');

    const row = rowOf(await screen.findByText('新增用户'));
    expect(row).not.toBeNull();
    fireEvent.click(within(row!).getByRole('button', { name: '修改' }));

    await waitFor(() => expect(screen.getByLabelText('接口路径')).toHaveValue('/api/system/users'));
    // method/path belong to the route table: visible, never editable.
    expect(screen.getByLabelText('请求方法')).toHaveValue('POST');
    expect(screen.getByLabelText('请求方法')).toBeDisabled();
    expect(screen.getByLabelText('接口路径')).toBeDisabled();
    expect(screen.getByLabelText('权限码')).toBeDisabled();
    // Route-shaped fields make no sense for a button.
    expect(screen.queryByLabelText('路由路径')).toBeNull();
    expect(screen.queryByLabelText('组件路径')).toBeNull();
  });

  it('keeps a hand-made button fully editable', async () => {
    mockMenus();
    const { container } = render(<StrictMode><MenuPage /></StrictMode>);
    await expand(container, '组织权限');
    await expand(container, '用户管理');

    const row = rowOf(await screen.findByText('导出用户'));
    expect(row).not.toBeNull();
    fireEvent.click(within(row!).getByRole('button', { name: '修改' }));

    await waitFor(() => expect(screen.getByLabelText('接口路径')).toHaveValue('/api/system/users/export'));
    expect(screen.getByLabelText('请求方法')).toBeEnabled();
    expect(screen.getByLabelText('接口路径')).toBeEnabled();
    expect(screen.getByLabelText('权限码')).toBeEnabled();
    expect(screen.getByLabelText('权限码')).toHaveValue('system:user:export');
  });

  it('labels a button row with its source in the type column', async () => {
    mockMenus();
    const { container } = render(<StrictMode><MenuPage /></StrictMode>);
    await expand(container, '组织权限');
    await expand(container, '用户管理');

    const syncedRow = rowOf(await screen.findByText('新增用户'));
    expect(within(syncedRow!).getByText('按钮')).toBeInTheDocument();
    expect(within(syncedRow!).getByText('路由同步')).toBeInTheDocument();

    const manualRow = rowOf(await screen.findByText('导出用户'));
    expect(within(manualRow!).getByText('手工')).toBeInTheDocument();
  });
});
