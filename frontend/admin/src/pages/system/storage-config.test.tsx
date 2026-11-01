import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { StrictMode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({
  createResourceApi: vi.fn(),
  listResourceApi: vi.fn(),
  testStorageConfigApi: vi.fn(),
  updateResourceApi: vi.fn(),
}));

vi.mock('@/api/system', () => ({
  createResourceApi: apiMocks.createResourceApi,
  deleteResourceApi: vi.fn(),
  getResourceApi: vi.fn(),
  listResourceApi: apiMocks.listResourceApi,
  testStorageConfigApi: apiMocks.testStorageConfigApi,
  updateResourceApi: apiMocks.updateResourceApi,
  updateResourceStatusApi: vi.fn(),
}));
vi.mock('@/stores/access', () => ({ hasAccessByCodes: () => true }));

const preferencesState = {
  preferences: {
    custom: { defaultTableSize: 20, enableFormFullscreen: false },
  },
};
vi.mock('@/stores/preferences', () => ({
  usePreferencesStore: Object.assign(
    (selector?: (state: typeof preferencesState) => unknown) =>
      selector ? selector(preferencesState) : preferencesState,
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
  TablePanel: ({
    columns,
    dataSource,
  }: {
    columns: Array<{
      dataIndex?: string;
      key?: string;
      render?: (value: unknown, row: Record<string, unknown>) => React.ReactNode;
      title?: React.ReactNode;
    }>;
    dataSource: Array<Record<string, unknown>>;
  }) => (
    <div>
      {dataSource.map((row) => (
        <div key={String(row.id)} data-testid={`row-${row.id}`}>
          {columns.map((column, index) => (
            <div key={String(column.key || column.dataIndex || index)}>
              {column.render
                ? column.render(column.dataIndex ? row[column.dataIndex] : undefined, row)
                : String(column.dataIndex ? (row[column.dataIndex] ?? '') : '')}
            </div>
          ))}
        </div>
      ))}
    </div>
  ),
}));
vi.mock('@/components/Page', () => ({
  Page: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

import { StorageConfigPage } from './settings';

describe('StorageConfigPage', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('lists storage fields and tests connection', async () => {
    apiMocks.listResourceApi.mockResolvedValue([
      {
        id: 1,
        name: '本地存储',
        storageType: 'local',
        basePath: './data/uploads',
        maxFileSize: 104857600,
        isDefault: true,
        status: 1,
        useHttps: true,
        privateAccess: true,
      },
    ]);
    apiMocks.testStorageConfigApi.mockResolvedValue(undefined);

    render(
      <StrictMode>
        <StorageConfigPage />
      </StrictMode>,
    );

    const row = await screen.findByTestId('row-1');
    expect(row).toHaveTextContent('本地存储');
    expect(row).toHaveTextContent('100 MB');
    expect(row).toHaveTextContent('默认');
    expect(row).toHaveTextContent('./data/uploads');

    fireEvent.click(screen.getByRole('button', { name: '测试连接' }));
    await waitFor(() => expect(apiMocks.testStorageConfigApi).toHaveBeenCalledWith(1));
  });

  it('creates a local storage config with schema fields', async () => {
    apiMocks.listResourceApi.mockResolvedValue([]);
    apiMocks.createResourceApi.mockResolvedValue({ id: 2 });

    render(
      <StrictMode>
        <StorageConfigPage />
      </StrictMode>,
    );

    fireEvent.click(await screen.findByRole('button', { name: '新增' }));
    await screen.findByRole('dialog');
    const nameInput = document.getElementById('name') as HTMLInputElement;
    const pathInput = document.getElementById('basePath') as HTMLInputElement;
    fireEvent.change(nameInput, { target: { value: '备份盘' } });
    fireEvent.change(pathInput, { target: { value: './data/backup' } });
    fireEvent.click(document.querySelector('.form-drawer-footer .ant-btn-primary')!);

    await waitFor(() =>
      expect(apiMocks.createResourceApi).toHaveBeenCalledWith(
        '/system/storage-configs',
        expect.objectContaining({
          name: '备份盘',
          storageType: 'local',
          basePath: './data/backup',
          maxFileSize: 104857600,
          useHttps: true,
          privateAccess: true,
          isDefault: false,
          status: 1,
        }),
      ),
    );
  });
});
