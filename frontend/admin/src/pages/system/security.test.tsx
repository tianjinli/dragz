import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { StrictMode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({
  listSessionsApi: vi.fn(),
  reload: vi.fn(),
  revokeSessionApi: vi.fn(),
}));

vi.mock('@/api/system', () => ({
  createTokenApi: vi.fn(),
  listErrorLogsApi: vi.fn(),
  listLoginLogsApi: vi.fn(),
  listOperationLogsApi: vi.fn(),
  listSessionsApi: apiMocks.listSessionsApi,
  listTokensApi: vi.fn(),
  resolveErrorLogApi: vi.fn(),
  revokeSessionApi: apiMocks.revokeSessionApi,
  revokeTokenApi: vi.fn(),
}));

vi.mock('@/components/DataTablePage', () => ({
  DataTablePage: ({
    title,
    loadData,
    actions = [],
  }: {
    title: string;
    loadData: (query: { keyword: string; page: number; pageSize: number }) => unknown;
    actions?: Array<{ label: string; onClick: (row: { id: number }, reload: () => Promise<void>) => unknown }>;
  }) => {
    void loadData({ keyword: '', page: 1, pageSize: 20 });
    return (
      <div>
        <h1>{title}</h1>
        {actions.map((action) => (
          <button key={action.label} type="button" onClick={() => void action.onClick({ id: 9 }, apiMocks.reload)}>
            {action.label}
          </button>
        ))}
      </div>
    );
  },
}));

import { OnlineUserPage } from './security';

describe('OnlineUserPage', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('reloads the session list after forcing a user offline', async () => {
    apiMocks.listSessionsApi.mockResolvedValue({ items: [], total: 0 });
    apiMocks.revokeSessionApi.mockResolvedValue(undefined);
    apiMocks.reload.mockResolvedValue(undefined);

    render(
      <StrictMode>
        <OnlineUserPage />
      </StrictMode>,
    );

    fireEvent.click(screen.getByRole('button', { name: '强制下线' }));

    await waitFor(() => expect(apiMocks.revokeSessionApi).toHaveBeenCalledWith(9));
    await waitFor(() => expect(apiMocks.reload).toHaveBeenCalledTimes(1));
  });
});
