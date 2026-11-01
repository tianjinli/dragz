import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({
  listPluginsApi: vi.fn(),
  updatePluginStatusApi: vi.fn(),
  uninstallPluginApi: vi.fn(),
}));

vi.mock('@/api/system', () => ({
  listPluginsApi: apiMocks.listPluginsApi,
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
  lastError: '',
  pendingMigrations: 2,
};

describe('PluginPage', () => {
  beforeEach(() => {
    apiMocks.listPluginsApi.mockReset();
    apiMocks.updatePluginStatusApi.mockReset();
    apiMocks.uninstallPluginApi.mockReset();
    loadAccess.mockClear();
  });
  afterEach(cleanup);

  it('renders plugins and protects the builtin switch', async () => {
    apiMocks.listPluginsApi.mockResolvedValue([systemRow, demoRow]);

    render(<PluginPage />);

    expect(await screen.findByText('示例插件')).toBeTruthy();
    expect(screen.getByText('2')).toBeTruthy();

    const switches = screen.getAllByRole('switch');
    expect(switches).toHaveLength(2);
    // system is builtin: its switch must be disabled.
    expect((switches[0] as HTMLButtonElement).disabled).toBe(true);
    expect((switches[1] as HTMLButtonElement).disabled).toBe(false);
  });

  it('enables a plugin and refreshes the sidebar', async () => {
    apiMocks.listPluginsApi.mockResolvedValue([systemRow, demoRow]);
    apiMocks.updatePluginStatusApi.mockResolvedValue(undefined);

    render(<PluginPage />);
    const switches = await screen.findAllByRole('switch');

    fireEvent.click(switches[1]);

    await waitFor(() => {
      expect(apiMocks.updatePluginStatusApi).toHaveBeenCalledWith('demo', 1);
    });
    // The list is re-read after the toggle so the switch shows server truth.
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
    // A failed enable must not pretend the sidebar changed.
    expect(loadAccess).not.toHaveBeenCalled();
  });

  // Uninstall drops tables, so the row action must be unreachable unless the
  // plugin is already disabled — the backend would answer 409 anyway, but the
  // UI should never offer it.
  it('offers uninstall only for a disabled non-builtin plugin', async () => {
    apiMocks.listPluginsApi.mockResolvedValue([systemRow, demoRow]);

    render(<PluginPage />);
    await screen.findByText('示例插件');

    const buttons = screen.getAllByRole('button', { name: '卸载' });
    expect(buttons).toHaveLength(2);
    // system: builtin. demo: disabled, so it may be uninstalled.
    expect((buttons[0] as HTMLButtonElement).disabled).toBe(true);
    expect((buttons[1] as HTMLButtonElement).disabled).toBe(false);
  });

  it('uninstalls after confirmation and refreshes the sidebar', async () => {
    apiMocks.listPluginsApi.mockResolvedValue([systemRow, demoRow]);
    apiMocks.uninstallPluginApi.mockResolvedValue(undefined);

    render(<PluginPage />);
    await screen.findByText('示例插件');

    fireEvent.click(screen.getAllByRole('button', { name: '卸载' })[1]);
    fireEvent.click(await screen.findByRole('button', { name: '确认卸载' }));

    await waitFor(() => {
      expect(apiMocks.uninstallPluginApi).toHaveBeenCalledWith('demo');
    });
    await waitFor(() => {
      expect(apiMocks.listPluginsApi).toHaveBeenCalledTimes(2);
    });
    expect(loadAccess).toHaveBeenCalled();
  });
});
