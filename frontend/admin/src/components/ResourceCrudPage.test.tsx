import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({ listResourceApi: vi.fn(), updateResourceApi: vi.fn(), updateResourceStatusApi: vi.fn() }));

vi.mock('@/api/system', () => ({
  createResourceApi: vi.fn(),
  deleteResourceApi: vi.fn(),
  listResourceApi: apiMocks.listResourceApi,
  updateResourceApi: apiMocks.updateResourceApi,
  updateResourceStatusApi: apiMocks.updateResourceStatusApi,
}));
vi.mock('@/stores/access', () => ({ hasAccessByCodes: () => true }));
vi.mock('@/stores/preferences', () => ({
  usePreferencesStore: Object.assign(() => ({}), { getState: () => ({ preferences: { custom: { defaultTableSize: 20 } } }) }),
}));
vi.mock('./TablePanel', () => ({
  TablePanel: ({ columns, dataSource }: { columns: Array<{ dataIndex?: string; render?: (value: unknown, row: Record<string, unknown>) => React.ReactNode }>; dataSource: Array<Record<string, unknown>> }) => <div>{dataSource.map((row) => <div key={String(row.id)}>{columns.map((column, index) => <div key={column.dataIndex || index}>{column.render ? column.render(column.dataIndex ? row[column.dataIndex] : undefined, row) : String(column.dataIndex ? row[column.dataIndex] ?? '' : '')}</div>)}</div>)}</div>,
}));
vi.mock('./FormDrawer', () => ({ FormDrawer: ({ children, onOk, open }: { children: React.ReactNode; onOk: () => void | Promise<void>; open: boolean }) => <div aria-hidden={!open} role={open ? 'dialog' : undefined}>{children}<button type="button" onClick={() => void onOk()}>保存</button></div> }));
vi.mock('./Page', () => ({ Page: ({ children }: { children: React.ReactNode }) => <>{children}</> }));

import { ResourceCrudPage } from './ResourceCrudPage';

describe('ResourceCrudPage edit drawer', () => {
  afterEach(cleanup);

  it('wraps plain text cells with width-bound Tooltip for ellipsis tips', async () => {
    apiMocks.listResourceApi.mockResolvedValue([
      { id: 1, name: '超长名称用于触发省略号与 tips', status: 1 },
    ]);

    const { container } = render(
      <ResourceCrudPage
        columns={[{ dataIndex: 'name', title: '名称' }]}
        createCode="create"
        deleteCode="delete"
        endpoint="/system/departments"
        fields={[{ key: 'name', label: '名称', required: true }]}
        title="部门"
        updateCode="update"
      />,
    );

    await waitFor(() => {
      expect(container.querySelector('.table-panel-ellipsis-tip')).toHaveTextContent(
        '超长名称用于触发省略号与 tips',
      );
    });
  });

  it('renders the wrap option as full text instead of an ellipsis tip', async () => {
    apiMocks.listResourceApi.mockResolvedValue([
      { id: 2, value: 'Clean up historical audit logs' },
    ]);

    const { container } = render(
      <ResourceCrudPage
        columns={[{ dataIndex: 'value', title: '英文', wrap: true }]}
        createCode="create"
        deleteCode="delete"
        endpoint="/system/translations"
        fields={[{ key: 'value', label: '英文', required: true }]}
        title="翻译"
        updateCode="update"
      />,
    );

    await waitFor(() => {
      expect(container.querySelector('.table-panel-wrap-cell')).toHaveTextContent(
        'Clean up historical audit logs',
      );
    });
    // Nothing is hidden behind a hover tip on a column that opted into wrapping.
    expect(container.querySelector('.table-panel-ellipsis-tip')).toBeNull();
  });

  it('opens with values from the selected list row', async () => {
    apiMocks.listResourceApi.mockResolvedValue([{ id: 3, code: 'dev', name: '研发部', status: 1 }]);

    render(<ResourceCrudPage columns={[{ dataIndex: 'name', title: '名称' }]} createCode="create" deleteCode="delete" endpoint="/system/departments" fields={[{ key: 'code', label: '编码', required: true }, { key: 'name', label: '名称', required: true }]} title="部门" updateCode="update"/>);

    fireEvent.click(await screen.findByRole('button', { name: '修改' }));

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
      expect(screen.getByLabelText('编码')).toHaveValue('dev');
      expect(screen.getByLabelText('名称')).toHaveValue('研发部');
    });
  });

  it('keeps the just-saved values when reopening an edited row even if reload returns stale data', async () => {
    apiMocks.listResourceApi.mockResolvedValue([{ id: 3, code: 'dev', name: '研发部', status: 1 }]);
    apiMocks.updateResourceApi.mockResolvedValue(undefined);

    render(<ResourceCrudPage columns={[{ dataIndex: 'name', title: '名称' }]} createCode="create" deleteCode="delete" endpoint="/system/departments" fields={[{ key: 'code', label: '编码', required: true }, { key: 'name', label: '名称', required: true }]} title="部门" updateCode="update"/>);

    fireEvent.click(await screen.findByRole('button', { name: '修改' }));
    fireEvent.change(await screen.findByLabelText('名称'), { target: { value: '产品部' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(apiMocks.updateResourceApi).toHaveBeenCalledWith('/system/departments', 3, expect.objectContaining({ name: '产品部' })));
    await waitFor(() => expect(screen.getByText('产品部')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: '修改' }));

    await waitFor(() => expect(screen.getByLabelText('名称')).toHaveValue('产品部'));
  });

  it('does not reset switches while closing after save', async () => {
    apiMocks.listResourceApi.mockResolvedValue([{ id: 3, code: 'dev', name: '研发部', status: 0 }]);
    apiMocks.updateResourceApi.mockResolvedValue(undefined);

    render(<ResourceCrudPage columns={[{ dataIndex: 'name', title: '名称' }]} createCode="create" deleteCode="delete" endpoint="/system/departments" fields={[{ key: 'name', label: '名称', required: true }, { key: 'status', label: '启用', numeric: true, type: 'switch' }]} title="部门" updateCode="update"/>);

    fireEvent.click(await screen.findByRole('button', { name: '修改' }));
    fireEvent.click(screen.getByRole('switch'));
    expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'true');

    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(apiMocks.updateResourceApi).toHaveBeenCalledWith('/system/departments', 3, expect.objectContaining({ status: 1 })));
    expect(screen.getByRole('switch', { hidden: true })).toHaveAttribute('aria-checked', 'true');
  });
});

describe('ResourceCrudPage status switch', () => {
  afterEach(cleanup);

  it('renders required fixed-option selects as segmented regardless of option count', async () => {
    apiMocks.listResourceApi.mockResolvedValue([{ id: 3, code: 'dev', name: '研发部', status: 1 }]);

    render(<ResourceCrudPage columns={[{ dataIndex: 'name', title: '名称' }]} createCode="create" deleteCode="delete" endpoint="/system/departments" fields={[{ key: 'method', label: '方法', required: true, type: 'select', options: ['GET','POST','PUT','PATCH','DELETE'].map((item) => ({ label: item, value: item })) }]} title="部门" updateCode="update"/>);

    fireEvent.click(await screen.findByRole('button', { name: '修改' }));

    await waitFor(() => expect(document.querySelector('.ant-segmented')).not.toBeNull());
    expect(within(screen.getByRole('dialog')).queryByRole('combobox')).toBeNull();
    const labels = Array.from(document.querySelectorAll('.ant-segmented-item-label')).map((node) => node.textContent);
    expect(labels).toEqual(['GET', 'POST', 'PUT', 'PATCH', 'DELETE']);
  });

  it('keeps non-required option selects as a dropdown', async () => {
    apiMocks.listResourceApi.mockResolvedValue([{ id: 3, code: 'dev', name: '研发部', status: 1 }]);

    render(<ResourceCrudPage columns={[{ dataIndex: 'name', title: '名称' }]} createCode="create" deleteCode="delete" endpoint="/system/departments" fields={[{ key: 'method', label: '方法', type: 'select', options: ['GET','POST'].map((item) => ({ label: item, value: item })) }]} title="部门" updateCode="update"/>);

    fireEvent.click(await screen.findByRole('button', { name: '修改' }));

    await waitFor(() => expect(within(screen.getByRole('dialog')).queryByRole('combobox')).not.toBeNull());
  });

  it('renders an interactive switch and updates status on toggle', async () => {
    apiMocks.listResourceApi.mockReset();
    apiMocks.updateResourceApi.mockReset();
    apiMocks.updateResourceStatusApi.mockReset();
    apiMocks.listResourceApi.mockResolvedValue([{ id: 3, code: 'dev', name: '研发部', status: 1 }]);
    apiMocks.updateResourceApi.mockResolvedValue(undefined);
    apiMocks.updateResourceStatusApi.mockResolvedValue(undefined);

    render(<ResourceCrudPage columns={[{ dataIndex: 'name', title: '名称' }, { dataIndex: 'status', title: '状态' }]} createCode="create" deleteCode="delete" endpoint="/system/departments" fields={[{ key: 'name', label: '名称', required: true }, { key: 'status', label: '启用', numeric: true, type: 'switch' }]} title="部门" updateCode="update"/>);

    const statusSwitches = await screen.findAllByRole('switch');
    expect(statusSwitches[0]).toHaveAttribute('aria-checked', 'true');
    const callsAfterInitial = apiMocks.listResourceApi.mock.calls.length;
    fireEvent.click(statusSwitches[0]);

    await waitFor(() => expect(apiMocks.updateResourceStatusApi).toHaveBeenCalledWith('/system/departments', 3, 0));
    expect(apiMocks.updateResourceApi).not.toHaveBeenCalled();
    expect(apiMocks.listResourceApi.mock.calls.length).toBe(callsAfterInitial);
    await waitFor(() => expect(screen.getAllByRole('switch')[0]).toHaveAttribute('aria-checked', 'false'));
  });

  it('keeps the plain tag render when status is not a numeric switch field', async () => {
    apiMocks.listResourceApi.mockReset();
    apiMocks.listResourceApi.mockResolvedValue([{ id: 3, name: '公告', status: 'published' }]);

    render(<ResourceCrudPage columns={[{ dataIndex: 'name', title: '标题' }, { dataIndex: 'status', title: '状态' }]} createCode="create" deleteCode="delete" endpoint="/system/notices" fields={[{ key: 'name', label: '标题', required: true }, { key: 'status', label: '状态', type: 'select', options: [{ label: '草稿', value: 'draft' }, { label: '已发布', value: 'published' }] }]} title="通知公告" updateCode="update"/>);

    expect(await screen.findByText('停用')).toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
  });
});

