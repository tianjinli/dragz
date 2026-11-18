import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({
  listPluginsApi: vi.fn(),
  installPluginApi: vi.fn(),
  updatePluginStatusApi: vi.fn(),
  uninstallPluginApi: vi.fn(),
}));

vi.mock('@/api/system', () => ({
  listPluginsApi: apiMocks.listPluginsApi,
  installPluginApi: apiMocks.installPluginApi,
  updatePluginStatusApi: apiMocks.updatePluginStatusApi,
  uninstallPluginApi: apiMocks.uninstallPluginApi,
}));

const loadAccess = vi.hoisted(() => vi.fn().mockResolvedValue(undefined));
vi.mock('@/stores/auth', () => ({
  useAuthStore: Object.assign(() => ({ loadAccess }), {
    getState: () => ({ loadAccess }),
  }),
}));

vi.mock('@/components/Page', () => ({
  Page: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
vi.mock('@/components/ListPagePanel', () => ({
  ListPagePanel: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
vi.mock('@/components/TablePanel', () => ({
  TablePanel: ({
    columns,
    dataSource,
  }: {
    columns: Array<{
      dataIndex?: string;
      render?: (value: unknown, row: Record<string, unknown>) => React.ReactNode;
    }>;
    dataSource: Array<Record<string, unknown>>;
  }) => (
    <div>
      {dataSource.map((row) => (
        <div key={String(row.id)}>
          {columns.map((column, index) => (
            <div key={column.dataIndex || index}>
              {column.render
                ? column.render(column.dataIndex ? row[column.dataIndex] : undefined, row)
                : String(column.dataIndex ? row[column.dataIndex] ?? '' : '')}
            </div>
          ))}
        </div>
      ))}
    </div>
  ),
}));

class ResizeObserverMock {
  disconnect() {}
  observe() {}
  unobserve() {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverMock);

import { PluginPage } from './plugins';

const systemRow = {
  id: 'system',
  name: '系统管理',
  version: '1.0.0',
  description: '平台基础能力',
  dependsOn: [],
  builtin: true,
  enabled: true,
  installed: true,
  lastError: '',
  pendingMigrations: 0,
};

const demoRow = {
  id: 'demo',
  name: '示例插件',
  version: '0.1.0',
  description: '演示用',
  dependsOn: ['system'],
  builtin: false,
  enabled: false,
  installed: true,
  lastError: '',
  pendingMigrations: 0,
};

const availableRow = {
  ...demoRow,
  id: 'demo-available',
  name: '可安装示例',
  installed: false,
  pendingMigrations: 2,
};

describe('PluginPage', () => {
  beforeEach(() => {
    apiMocks.listPluginsApi.mockReset();
    apiMocks.installPluginApi.mockReset();
    apiMocks.updatePluginStatusApi.mockReset();
    apiMocks.uninstallPluginApi.mockReset();
    loadAccess.mockClear();
  });
  afterEach(cleanup);

  it('shows a disabled switch for uninstalled plugins and protects the builtin', async () => {
    apiMocks.listPluginsApi.mockResolvedValue([systemRow, demoRow, availableRow]);

    render(<PluginPage />);

    expect(await screen.findByText('示例插件')).toBeTruthy();
    expect(screen.getByText('可安装示例')).toBeTruthy();
    expect(screen.queryByText('未安装')).toBeNull();

    // All rows get a switch: builtin locked on, demo toggleable, available locked off.
    const switches = screen.getAllByRole('switch');
    expect(switches).toHaveLength(3);
    expect((switches[0] as HTMLButtonElement).disabled).toBe(true);
    expect(switches[0].getAttribute('aria-checked')).toBe('true');
    expect((switches[1] as HTMLButtonElement).disabled).toBe(false);
    expect((switches[2] as HTMLButtonElement).disabled).toBe(true);
    expect(switches[2].getAttribute('aria-checked')).toBe('false');
  });

  it('installs an Available plugin without enabling and without refreshing access', async () => {
    apiMocks.listPluginsApi.mockResolvedValue([systemRow, availableRow]);
    apiMocks.installPluginApi.mockResolvedValue(undefined);

    render(<PluginPage />);
    fireEvent.click(await screen.findByRole('button', { name: '安装' }));

    await waitFor(() => {
      expect(apiMocks.installPluginApi).toHaveBeenCalledWith('demo-available');
    });
    await waitFor(() => {
      expect(apiMocks.listPluginsApi).toHaveBeenCalledTimes(2);
    });
    expect(apiMocks.updatePluginStatusApi).not.toHaveBeenCalled();
    expect(loadAccess).not.toHaveBeenCalled();
  });

  it('enables an installed plugin and refreshes the sidebar', async () => {
    apiMocks.listPluginsApi.mockResolvedValue([systemRow, demoRow]);
    apiMocks.updatePluginStatusApi.mockResolvedValue(undefined);

    render(<PluginPage />);
    const switches = await screen.findAllByRole('switch');

    fireEvent.click(switches[1]);

    await waitFor(() => {
      expect(apiMocks.updatePluginStatusApi).toHaveBeenCalledWith('demo', 1);
    });
    await waitFor(() => {
      expect(apiMocks.listPluginsApi).toHaveBeenCalledTimes(2);
    });
    expect(loadAccess).toHaveBeenCalled();
  });

  it('keeps the switch off when enabling fails', async () => {
    apiMocks.listPluginsApi.mockResolvedValue([systemRow, demoRow]);
    apiMocks.updatePluginStatusApi.mockRejectedValue(new Error('migration failed'));

    render(<PluginPage />);
    const switches = await screen.findAllByRole('switch');

    fireEvent.click(switches[1]);

    await waitFor(() => {
      expect(apiMocks.updatePluginStatusApi).toHaveBeenCalledWith('demo', 1);
    });
    await waitFor(() => {
      expect(apiMocks.listPluginsApi).toHaveBeenCalledTimes(2);
    });
    const after = screen.getAllByRole('switch');
    expect(after[1].getAttribute('aria-checked')).toBe('false');
    expect(loadAccess).not.toHaveBeenCalled();
  });

  it('offers uninstall only for an installed disabled non-builtin plugin', async () => {
    apiMocks.listPluginsApi.mockResolvedValue([systemRow, demoRow, availableRow]);

    render(<PluginPage />);
    await screen.findByText('示例插件');

    // Builtin: no uninstall. Available: Install. Installed+disabled: Uninstall.
    expect(screen.queryAllByRole('button', { name: '卸载' })).toHaveLength(1);
    expect(screen.getByRole('button', { name: '安装' })).toBeTruthy();
    expect((screen.getByRole('button', { name: '卸载' }) as HTMLButtonElement).disabled).toBe(
      false,
    );
  });

  it('uninstalls after confirmation and refreshes the sidebar', async () => {
    apiMocks.listPluginsApi
      .mockResolvedValueOnce([systemRow, demoRow])
      .mockResolvedValueOnce([systemRow, { ...demoRow, installed: false, enabled: false }]);
    apiMocks.uninstallPluginApi.mockResolvedValue(undefined);

    render(<PluginPage />);
    await screen.findByText('示例插件');

    fireEvent.click(screen.getByRole('button', { name: '卸载' }));
    fireEvent.click(await screen.findByRole('button', { name: '确认卸载' }));

    await waitFor(() => {
      expect(apiMocks.uninstallPluginApi).toHaveBeenCalledWith('demo');
    });
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '安装' })).toBeTruthy();
    });
    const switches = screen.getAllByRole('switch');
    expect(switches[1].getAttribute('aria-checked')).toBe('false');
    expect((switches[1] as HTMLButtonElement).disabled).toBe(true);
    await waitFor(() => {
      expect(apiMocks.listPluginsApi).toHaveBeenCalledTimes(2);
    });
    expect(loadAccess).toHaveBeenCalled();
  });
});
