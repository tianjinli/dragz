import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { StrictMode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({
  deleteDictionaryItemApi: vi.fn(),
  deleteDictionaryTypeApi: vi.fn(),
  listConfigsApi: vi.fn(),
  listDictionaryTypesApi: vi.fn(),
  listDictionaryItemsApi: vi.fn(),
  updateConfigApi: vi.fn(),
  updateDictionaryItemApi: vi.fn(),
}));

vi.mock('@/api/system', () => ({
  createDictionaryItemApi: vi.fn(),
  createDictionaryTypeApi: vi.fn(),
  createResourceApi: vi.fn(),
  deleteDictionaryItemApi: apiMocks.deleteDictionaryItemApi,
  deleteDictionaryTypeApi: apiMocks.deleteDictionaryTypeApi,
  deleteResourceApi: vi.fn(),
  listConfigsApi: apiMocks.listConfigsApi,
  listDictionaryItemsApi: apiMocks.listDictionaryItemsApi,
  listDictionaryTypesApi: apiMocks.listDictionaryTypesApi,
  listResourceApi: vi.fn(),
  updateConfigApi: apiMocks.updateConfigApi,
  updateDictionaryItemApi: apiMocks.updateDictionaryItemApi,
  updateDictionaryTypeApi: vi.fn(),
  updateResourceApi: vi.fn(),
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
vi.mock('@/components/TablePanel', () => ({
  TablePanel: ({ columns, dataSource }: { columns: Array<{ dataIndex?: string; render?: (value: unknown, row: Record<string, unknown>) => React.ReactNode }>; dataSource: Array<Record<string, unknown>> }) => <div>{dataSource.map((row) => <div key={String(row.id)}>{columns.map((column, index) => <div key={column.dataIndex || index}>{column.render ? column.render(column.dataIndex ? row[column.dataIndex] : undefined, row) : String(column.dataIndex ? row[column.dataIndex] ?? '' : '')}</div>)}</div>)}</div>,
}));
vi.mock('@/components/Page', () => ({ Page: ({ children }: { children: React.ReactNode }) => <>{children}</> }));

import { ConfigPage, DictionaryPage } from './settings';

describe('DictionaryPage edit drawer echo', () => {
  afterEach(cleanup);

  it('echoes the list item into the edit dialog under StrictMode', async () => {
    apiMocks.listDictionaryTypesApi.mockResolvedValue([{ id: 1, name: '性别', code: 'gender', isSystem: false, itemCount: 1 }]);
    apiMocks.listDictionaryItemsApi.mockResolvedValue([{ id: 10, label: '男', value: '1', color: 'blue', sort: 0, status: 1 }]);
    apiMocks.updateDictionaryItemApi.mockResolvedValue(undefined);

    render(<StrictMode><DictionaryPage /></StrictMode>);

    fireEvent.click(await screen.findByText('性别'));
    const editButtons = await waitFor(() => {
      // 条目表格是异步加载的：只等到第一个「编辑」就取最后一个会点到字典类型
      // 那一行，抽屉里自然没有「标签」。等类型行和条目行都在再取。
      const buttons = screen.getAllByRole('button', { name: '编辑' });
      expect(buttons.length).toBeGreaterThanOrEqual(2);
      return buttons;
    });
    fireEvent.click(editButtons[editButtons.length - 1]);

    await waitFor(() => {
      expect(screen.getByLabelText('标签')).toHaveValue('男');
      expect(screen.getByLabelText('数值')).toHaveValue('1');
      expect(document.getElementById('status')).toHaveAttribute('aria-checked', 'true');
    });

    fireEvent.change(screen.getByLabelText('标签'), { target: { value: '男士' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => expect(apiMocks.updateDictionaryItemApi).toHaveBeenCalledWith(10, expect.objectContaining({ label: '男士', status: 1 })));
  });

  it('shows the freshly loaded row when editing a different dictionary item after saving', async () => {
    apiMocks.listDictionaryTypesApi.mockResolvedValue([{ id: 1, name: '性别', code: 'gender', isSystem: false, itemCount: 2 }]);
    apiMocks.listDictionaryItemsApi.mockResolvedValue([
      { id: 10, label: '男', value: '1', color: 'blue', sort: 0, status: 1 },
      { id: 11, label: '女', value: '2', color: 'pink', sort: 1, status: 1 },
    ]);
    apiMocks.updateDictionaryItemApi.mockResolvedValue(undefined);

    render(<StrictMode><DictionaryPage /></StrictMode>);
    fireEvent.click(await screen.findByText('性别'));

    const editButtons = await waitFor(() => {
      // 条目列表是异步加载的：等两行条目都渲染出来再取按钮，否则只会拿到
      // 字典类型表那一行的「编辑」，下面的倒序索引就会错位。
      const buttons = screen.getAllByRole('button', { name: '编辑' });
      expect(buttons.length).toBeGreaterThanOrEqual(3);
      return buttons;
    });
    fireEvent.click(editButtons[editButtons.length - 2]);
    await waitFor(() => expect(screen.getByLabelText('标签')).toHaveValue('男'));

    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => expect(apiMocks.updateDictionaryItemApi).toHaveBeenCalledWith(10, expect.objectContaining({ label: '男' })));

    const refreshedEditButtons = screen.getAllByRole('button', { name: '编辑' });
    fireEvent.click(refreshedEditButtons[refreshedEditButtons.length - 1]);
    await waitFor(() => expect(screen.getByLabelText('标签')).toHaveValue('女'));
  });
});

describe('DictionaryPage delete', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  const types = [{ id: 1, name: '性别', code: 'gender', isSystem: false, itemCount: 1 }];
  const items = [{ id: 10, label: '男', value: '1', color: 'blue', sort: 0, status: 1 }];

  async function confirmDelete(button: HTMLElement) {
    fireEvent.click(button);
    // antd 会在两个汉字的按钮文案里插一个空格（"确 定"），所以用正则匹配。
    const ok = await screen.findByRole('button', { name: /确\s*定/ });
    fireEvent.click(ok);
  }

  it('deletes a dictionary type after confirmation', async () => {
    apiMocks.listDictionaryTypesApi.mockResolvedValue(types);
    apiMocks.listDictionaryItemsApi.mockResolvedValue(items);
    apiMocks.deleteDictionaryTypeApi.mockResolvedValue(undefined);

    render(<StrictMode><DictionaryPage /></StrictMode>);
    await screen.findByText('性别');

    // The types table renders first, so its 删除 button comes first.
    const deleteButtons = await screen.findAllByRole('button', { name: '删除' });
    await confirmDelete(deleteButtons[0]);

    await waitFor(() => expect(apiMocks.deleteDictionaryTypeApi).toHaveBeenCalledWith(1));
  });

  it('deletes a dictionary item after confirmation', async () => {
    apiMocks.listDictionaryTypesApi.mockResolvedValue(types);
    apiMocks.listDictionaryItemsApi.mockResolvedValue(items);
    apiMocks.deleteDictionaryItemApi.mockResolvedValue(undefined);

    render(<StrictMode><DictionaryPage /></StrictMode>);
    fireEvent.click(await screen.findByText('性别'));
    await screen.findByText('男');

    const deleteButtons = await screen.findAllByRole('button', { name: '删除' });
    await confirmDelete(deleteButtons[deleteButtons.length - 1]);

    await waitFor(() => expect(apiMocks.deleteDictionaryItemApi).toHaveBeenCalledWith(10));
  });

  it('does not delete a system dictionary', async () => {
    apiMocks.listDictionaryTypesApi.mockResolvedValue([
      { id: 2, name: '系统字典', code: 'sys_kind', isSystem: true, itemCount: 0 },
    ]);
    apiMocks.listDictionaryItemsApi.mockResolvedValue([]);

    render(<StrictMode><DictionaryPage /></StrictMode>);
    await screen.findByText('系统字典');

    const deleteButtons = await screen.findAllByRole('button', { name: '删除' });
    expect(deleteButtons[0]).toBeDisabled();
  });
});

describe('ConfigPage inline save', () => {
  afterEach(cleanup);

  const configRows = [
    { id: 1, group: 'site', name: '站点名称', key: 'site.name', value: 'old-name', valueType: 'string' },
    { id: 2, group: 'site', name: '保留数量', key: 'site.keep', value: '5', valueType: 'number' },
    { id: 3, group: 'site', name: '启用开关', key: 'site.enabled', value: 'false', valueType: 'boolean' },
    { id: 4, group: 'site', name: '访问密钥', key: 'site.secret', value: '********', valueType: 'string', isSecret: true },
  ];

  it('saves text and number values on blur, saves on enter, saves switch on change', async () => {
    apiMocks.listConfigsApi.mockResolvedValue(configRows);
    apiMocks.updateConfigApi.mockResolvedValue(undefined);

    render(<StrictMode><ConfigPage /></StrictMode>);

    const text = await screen.findByLabelText('站点名称');
    fireEvent.change(text, { target: { value: 'new-name' } });
    fireEvent.blur(text);
    await waitFor(() => expect(apiMocks.updateConfigApi).toHaveBeenCalledWith('site.name', 'new-name'));

    fireEvent.change(text, { target: { value: 'new-name' } });
    fireEvent.blur(text);
    await waitFor(() => expect(apiMocks.updateConfigApi.mock.calls.filter((call) => call[0] === 'site.name')).toHaveLength(1));

    const number = screen.getByLabelText('保留数量');
    fireEvent.change(number, { target: { value: '8' } });
    fireEvent.blur(number);
    await waitFor(() => expect(apiMocks.updateConfigApi).toHaveBeenCalledWith('site.keep', '8'));

    fireEvent.change(number, { target: { value: '9' } });
    fireEvent.keyDown(number, { key: 'Enter' });
    await waitFor(() => expect(apiMocks.updateConfigApi).toHaveBeenCalledWith('site.keep', '9'));

    fireEvent.click(screen.getByRole('switch'));
    await waitFor(() => expect(apiMocks.updateConfigApi).toHaveBeenCalledWith('site.enabled', 'true'));
  });

  it('skips saving untouched or cleared secret fields on blur', async () => {
    apiMocks.listConfigsApi.mockResolvedValue(configRows);
    apiMocks.updateConfigApi.mockResolvedValue(undefined);

    render(<StrictMode><ConfigPage /></StrictMode>);

    const secret = await screen.findByLabelText('访问密钥');
    fireEvent.blur(secret);
    fireEvent.change(secret, { target: { value: 'brand-new' } });
    fireEvent.blur(secret);
    await waitFor(() => expect(apiMocks.updateConfigApi).toHaveBeenCalledWith('site.secret', 'brand-new'));
    expect(apiMocks.updateConfigApi.mock.calls.filter((call) => call[0] === 'site.secret')).toHaveLength(1);
  });
});



