import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { StrictMode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({
  clearCacheApi: vi.fn(),
  deleteCacheKeyApi: vi.fn(),
  listCacheKeysApi: vi.fn(),
  listJobLogsApi: vi.fn(),
  listJobsApi: vi.fn(),
  reload: vi.fn(),
  runJobApi: vi.fn(),
  testWebhookApi: vi.fn(),
  updateJobApi: vi.fn(),
  renderedRows: [] as Array<Record<string, unknown>>,
}));

vi.mock('@/api/system', () => ({
  clearCacheApi: apiMocks.clearCacheApi,
  deleteCacheKeyApi: apiMocks.deleteCacheKeyApi,
  listCacheKeysApi: apiMocks.listCacheKeysApi,
  listJobLogsApi: apiMocks.listJobLogsApi,
  listJobsApi: apiMocks.listJobsApi,
  runJobApi: apiMocks.runJobApi,
  testWebhookApi: apiMocks.testWebhookApi,
  updateJobApi: apiMocks.updateJobApi,
}));

vi.mock('@/components/ResourceCrudPage', () => ({ ResourceCrudPage: () => null }));

vi.mock('@/components/DataTablePage', () => ({
  DataTablePage: ({
    title,
    loadData,
    columns = [],
    actions = [],
    extra,
  }: {
    title: string;
    loadData: (query: { keyword: string; page: number; pageSize: number }) => unknown;
    columns?: Array<{ dataIndex?: string; render?: (value: unknown, row: Record<string, unknown>) => unknown }>;
    actions?: Array<{ label: string; onClick: (row: Record<string, unknown>, reload: () => Promise<void>) => unknown }>;
    extra?: React.ReactNode;
  }) => {
    void loadData({ keyword: '', page: 1, pageSize: 20 });
    const rows = apiMocks.renderedRows as Array<Record<string, unknown>>;
    return (
      <div>
        <h2>{title}</h2>
        <div>{extra}</div>
        {actions.map((action) => (
          <button key={action.label} onClick={() => void action.onClick(rows[0], apiMocks.reload)}>
            {action.label}
          </button>
        ))}
        {rows.map((row, rowIndex) => (
          <div key={rowIndex} data-testid={`row-${rowIndex}`}>
            {columns.map((column, index) => (
              <span key={index}>
                {column.render
                  ? column.render(column.dataIndex ? row[column.dataIndex] : undefined, row)
                  : String(column.dataIndex ? row[column.dataIndex] ?? '' : '')}
              </span>
            ))}
          </div>
        ))}
      </div>
    );
  },
}));

import { JobPage } from './tools';

const baseJobRow = {
  id: 9,
  name: '清理历史审计日志',
  handler: 'maintenance.cleanup_audit_logs',
  cronExpression: '30 3 * * 0',
  status: 1,
  nextRunAt: '2026-09-20T03:30:00+08:00',
  triggerType: 'manual',
  success: true,
  attempt: 1,
  startedAt: '2026-09-14T10:00:00+08:00',
  finishedAt: '2026-09-14T10:00:01.500+08:00',
  output: 'affected 3 records',
  errorMessage: '',
};

describe('JobPage', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    apiMocks.renderedRows = [];
  });

  it('runs a job immediately, refreshes the list and shows the log result', async () => {
    apiMocks.renderedRows = [baseJobRow];
    apiMocks.listJobsApi.mockResolvedValue({ items: [baseJobRow], total: 1, page: 1, pageSize: 20 });
    apiMocks.listJobLogsApi.mockResolvedValue({ items: [baseJobRow], total: 1, page: 1, pageSize: 20 });
    apiMocks.runJobApi.mockResolvedValue(undefined);

    render(
      <StrictMode>
        <JobPage />
      </StrictMode>,
    );

    fireEvent.click(screen.getByRole('button', { name: '立即执行' }));

    await waitFor(() => expect(apiMocks.runJobApi).toHaveBeenCalledWith(9));
    await waitFor(() => expect(apiMocks.reload).toHaveBeenCalled());

    expect(await screen.findByText('成功')).toBeInTheDocument();
    expect(screen.getByText('手动')).toBeInTheDocument();
    expect(screen.getByText('1500')).toBeInTheDocument();
    expect(screen.getByText('affected 3 records')).toBeInTheDocument();
  });

  it('renders failure, running and scheduled log states', async () => {
    apiMocks.renderedRows = [
      { ...baseJobRow, triggerType: 'scheduled', success: false, errorMessage: 'boom' },
      { ...baseJobRow, success: null, finishedAt: null },
    ];
    apiMocks.listJobsApi.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 20 });
    apiMocks.listJobLogsApi.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 20 });

    render(
      <StrictMode>
        <JobPage />
      </StrictMode>,
    );

    fireEvent.click(screen.getByRole('button', { name: '执行日志' }));

    expect(await screen.findByText('失败')).toBeInTheDocument();
    expect(screen.getByText('运行中')).toBeInTheDocument();
    expect(screen.getByText('定时')).toBeInTheDocument();
    expect(screen.getByText('boom')).toBeInTheDocument();
    expect(screen.getByText('—')).toBeInTheDocument();
  });
});
