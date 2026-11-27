import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { StrictMode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({
  listResourceApi: vi.fn(),
  listDepartmentsApi: vi.fn(),
  listUsersApi: vi.fn(),
}));

vi.mock('@/api/system', () => ({
  createResourceApi: vi.fn(),
  deleteResourceApi: vi.fn(),
  listResourceApi: apiMocks.listResourceApi,
  updateResourceApi: vi.fn(),
  listDepartmentsApi: apiMocks.listDepartmentsApi,
  listUsersApi: apiMocks.listUsersApi,
}));
vi.mock('@/stores/access', () => ({ hasAccessByCodes: () => true }));

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

class ResizeObserverMock {
  disconnect() {}
  observe() {}
  unobserve() {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverMock);

vi.mock('@/components/Page', () => ({ Page: ({ children }: { children: React.ReactNode }) => <>{children}</> }));

import { useSystemConfigStore } from '@/stores/system-config';
import { DepartmentPage, PositionPage } from './organization';

const departments = [
  { id: 1, parentId: 0, name: '总公司', code: 'HQ', ancestors: '', remark: '集团总部', leaderUserId: 11, leaderName: '张三', sort: 0, status: 1, createdAt: '2026-11-19T18:19:01.2+08:00' },
  { id: 2, parentId: 1, name: '研发部', code: 'RD', ancestors: '1', remark: '负责产品研发', leaderUserId: 12, leaderName: '李四', sort: 1, status: 0, createdAt: '2026-11-20T09:05:00+08:00' },
];

describe('DepartmentPage edit drawer', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    useSystemConfigStore.setState({ loaded: false, values: {} });
  });

  it('brings the current record into the edit form', async () => {
    apiMocks.listResourceApi.mockResolvedValue(departments);
    apiMocks.listDepartmentsApi.mockResolvedValue(departments);
    apiMocks.listUsersApi.mockImplementation(({ departmentId }: { departmentId: number }) => Promise.resolve({
      items: departmentId === 1
        ? [{ id: 11, username: 'zhangsan', displayName: '张三', departmentId: 1 }]
        : [{ id: 12, username: 'lisi', displayName: '李四', departmentId: 2 }],
      total: 1,
    }));

    render(<StrictMode><DepartmentPage /></StrictMode>);

    fireEvent.click(await screen.findByRole('button', { name: '修改' }));

    await waitFor(() => {
      expect(screen.getByLabelText('部门编码')).toHaveValue('HQ');
      expect(screen.getByLabelText('部门名称')).toHaveValue('总公司');
    });
    await waitFor(() => {
      expect(document.getElementById('leaderUserId')?.closest('.ant-select')?.textContent).toContain('张三');
    });
    expect(document.getElementById('sort')).toHaveValue(0);
    expect(document.getElementById('status')).toHaveAttribute('aria-checked', 'true');
  });

  it('places the expand toggle left of the department name inside the name cell', async () => {
    apiMocks.listResourceApi.mockResolvedValue(departments);
    apiMocks.listDepartmentsApi.mockResolvedValue(departments);
    apiMocks.listUsersApi.mockResolvedValue({ items: [], total: 0 });

    const { container } = render(<StrictMode><DepartmentPage /></StrictMode>);

    await waitFor(() => expect(container.querySelector('.ant-table-tbody tr.ant-table-row')).toBeInTheDocument());
    const firstCell = container.querySelector('.ant-table-tbody tr.ant-table-row > td:first-child');
    const expandButton = firstCell?.querySelector('.ant-table-row-expand-icon');
    expect(firstCell).toHaveTextContent('总公司');
    expect(expandButton).not.toBeNull();
    expect(firstCell).toHaveClass('ant-table-cell-ellipsis');
    expect(firstCell?.querySelector('.table-panel-ellipsis-tip')).toHaveTextContent('总公司');
    expect(firstCell).not.toHaveAttribute('title');
    const nameNode = Array.from(firstCell?.childNodes || []).find(
      (node) => node.textContent?.includes('总公司') && node !== expandButton,
    );
    expect(nameNode).toBeDefined();
    expect(expandButton!.compareDocumentPosition(nameNode!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('shows remark followed by creation time as the last data columns, instead of the hierarchy path', async () => {
    useSystemConfigStore.setState({
      loaded: true,
      values: { 'system.dateTimeFormat': 'YYYY/MM/DD HH:mm' },
    });
    apiMocks.listResourceApi.mockResolvedValue(departments);
    apiMocks.listDepartmentsApi.mockResolvedValue(departments);
    apiMocks.listUsersApi.mockResolvedValue({ items: [], total: 0 });

    const { container } = render(<StrictMode><DepartmentPage /></StrictMode>);

    await waitFor(() => expect(container.querySelector('.ant-table-tbody tr.ant-table-row')).toBeInTheDocument());
    const header = container.querySelector('.ant-table-thead')!;
    expect(header.textContent).toContain('部门名称');
    expect(header.textContent).toContain('备注');
    expect(header.textContent).toContain('创建时间');
    expect(header.textContent).not.toContain('层级路径');
    const nameHeader = Array.from(header.querySelectorAll('th')).find((th) => th.textContent?.includes('部门名称'));
    // Name cells stay left-aligned so the tree indent shows; the header is
    // centred by CSS, which jsdom does not load.
    expect(nameHeader).toHaveStyle({ textAlign: 'left' });
    const { readFileSync } = await import('node:fs');
    const { dirname, resolve } = await import('node:path');
    const { fileURLToPath } = await import('node:url');
    const baseCss = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../../styles/base.css'), 'utf8');
    expect(baseCss).toMatch(/\.department-resource-table \.ant-table-thead > tr > th:first-child\s*\{[^}]*text-align:\s*center/s);
    const headers = Array.from(header.querySelectorAll('th')).map((th) => th.textContent || '');
    const remarkIndex = headers.findIndex((text) => text.includes('备注'));
    const createdAtIndex = headers.findIndex((text) => text.includes('创建时间'));
    const statusIndex = headers.findIndex((text) => text.includes('状态'));
    expect(remarkIndex).toBeGreaterThan(statusIndex);
    expect(createdAtIndex).toBe(remarkIndex + 1);
    expect(createdAtIndex).toBe(headers.length - 2); // last data column, before the actions column
    const firstRow = container.querySelector('.ant-table-tbody tr.ant-table-row')!;
    expect(firstRow.textContent).toContain('集团总部');
    expect(firstRow.textContent).toContain('2026/11/19 18:19');
    expect(firstRow.querySelector('.menu-name-cell svg')).toBeNull();
  });

  it('saves the edited record back to the update endpoint', async () => {
    const updateResourceApi = (await import('@/api/system')).updateResourceApi as ReturnType<typeof vi.fn>;
    apiMocks.listResourceApi.mockResolvedValue(departments);
    apiMocks.listDepartmentsApi.mockResolvedValue(departments);
    apiMocks.listUsersApi.mockResolvedValue({ items: [], total: 0 });

    render(<StrictMode><DepartmentPage /></StrictMode>);

    fireEvent.click(await screen.findByRole('button', { name: '修改' }));
    const nameInput = await screen.findByLabelText('部门名称');
    await waitFor(() => expect(nameInput).toHaveValue('总公司'));
    fireEvent.change(nameInput, { target: { value: '集团总部' } });
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));

    await waitFor(() => expect(updateResourceApi).toHaveBeenCalledWith('/system/departments', 1, expect.objectContaining({ name: '集团总部', status: 1 })));
  });

  it('treats a root department parentId 0 as no parent in the edit dialog', async () => {
    apiMocks.listResourceApi.mockResolvedValue([departments[0]]);
    apiMocks.listDepartmentsApi.mockResolvedValue([departments[0]]);
    apiMocks.listUsersApi.mockResolvedValue({ items: [], total: 0 });

    render(<StrictMode><DepartmentPage /></StrictMode>);
    fireEvent.click(await screen.findByRole('button', { name: '修改' }));

    await waitFor(() => {
      const parentSelect = document.getElementById('parentId')?.closest('.ant-select-content');
      expect(parentSelect).not.toHaveClass('ant-select-content-has-value');
      expect(parentSelect?.querySelector('.ant-select-selection-item')).toBeNull();
    });
  });

  it('still shows the leader name even when the leader is outside the loaded department options', async () => {
    apiMocks.listResourceApi.mockResolvedValue([departments[1]]);
    apiMocks.listDepartmentsApi.mockResolvedValue(departments);
    apiMocks.listUsersApi.mockResolvedValue({ items: [{ id: 99, username: 'other', displayName: '其他人', departmentId: 1 }], total: 1 });

    render(<StrictMode><DepartmentPage /></StrictMode>);
    fireEvent.click(await screen.findByRole('button', { name: '修改' }));

    await waitFor(() => {
      const leaderSelect = document.getElementById('leaderUserId')?.closest('.ant-select');
      expect(leaderSelect?.textContent).toContain('李四');
    });
  });

  it('opens the create dialog prefilled with the row as parent when clicking 新增下级', async () => {
    apiMocks.listResourceApi.mockResolvedValue(departments);
    apiMocks.listDepartmentsApi.mockResolvedValue(departments);
    apiMocks.listUsersApi.mockResolvedValue({ items: [], total: 0 });

    render(<StrictMode><DepartmentPage /></StrictMode>);
    fireEvent.click(await screen.findByRole('button', { name: '新增下级' }));

    await waitFor(() => {
      expect(screen.getByText('新增部门')).toBeInTheDocument();
      const parentSelect = document.getElementById('parentId')?.closest('.ant-select');
      expect(parentSelect?.textContent).toContain('总公司');
    });
  });

  it('applies the department search only on query click and restores it on reset', async () => {
    // Flat rows keep this test focused on search timing, not tree expansion.
    const flatDepartments = departments.map((item) => ({ ...item, parentId: 0, ancestors: '0' }));
    apiMocks.listResourceApi.mockResolvedValue(flatDepartments);
    apiMocks.listDepartmentsApi.mockResolvedValue(departments);
    apiMocks.listUsersApi.mockResolvedValue({ items: [], total: 0 });

    const { container } = render(<StrictMode><DepartmentPage /></StrictMode>);
    const rows = () => container.querySelectorAll('.ant-table-tbody tr.ant-table-row');
    await waitFor(() => expect(rows().length).toBe(2));
    const callsAfterLoad = apiMocks.listResourceApi.mock.calls.length;

    const queryButton = screen.getByRole('button', { name: /查\s*询/ });
    const resetButton = screen.getByRole('button', { name: /重\s*置/ });
    fireEvent.change(screen.getByPlaceholderText('名称或编码'), { target: { value: 'HQ' } });

    // Typing alone must not filter: both departments are still shown.
    expect(rows().length).toBe(2);
    fireEvent.click(queryButton);

    await waitFor(() => expect(rows().length).toBe(1));
    expect(rows()[0].textContent).toContain('总公司');
    // Local filtering never refetches from the server.
    expect(apiMocks.listResourceApi.mock.calls.length).toBe(callsAfterLoad);

    fireEvent.click(resetButton);
    await waitFor(() => expect(rows().length).toBe(2));
    expect(apiMocks.listResourceApi.mock.calls.length).toBe(callsAfterLoad);
  });
});

describe('PositionPage search', () => {
  afterEach(cleanup);

  it('renders positions as table rows', async () => {
    apiMocks.listResourceApi.mockResolvedValue({ items: [{ id: 1, name: '工程师', code: 'ENG', userCount: 3, sort: 1, status: 1, remark: '研发' }], total: 1 });

    const { container } = render(<StrictMode><PositionPage /></StrictMode>);

    await waitFor(() => expect(container.querySelector('.ant-table-tbody tr.ant-table-row')).not.toBeNull());
    expect(container.querySelector('.ant-table-tbody tr.ant-table-row')!.textContent).toContain('工程师');
    expect(container.querySelector('.ant-table-thead')!.textContent).toContain('岗位名称');
    expect(apiMocks.listResourceApi).toHaveBeenCalledWith('/system/positions', expect.objectContaining({ keyword: '', page: 1, pageSize: 20 }));
  });

  it('submits keyword only when clicking query', async () => {
    apiMocks.listResourceApi.mockResolvedValue({ items: [], total: 0 });

    render(<StrictMode><PositionPage /></StrictMode>);
    await waitFor(() => expect(apiMocks.listResourceApi).toHaveBeenCalledWith('/system/positions', expect.objectContaining({ keyword: '', page: 1, pageSize: 20 })));
    const callsAfterLoad = apiMocks.listResourceApi.mock.calls.length;

    fireEvent.change(screen.getByPlaceholderText('名称或编码'), { target: { value: 'dev' } });
    expect(apiMocks.listResourceApi.mock.calls.length).toBe(callsAfterLoad);

    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(apiMocks.listResourceApi).toHaveBeenLastCalledWith('/system/positions', expect.objectContaining({ keyword: 'dev', page: 1, pageSize: 20 })));
  });
});


