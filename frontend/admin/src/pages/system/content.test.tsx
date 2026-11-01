import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { StrictMode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({
  downloadFileApi: vi.fn(),
  getFileDetailApi: vi.fn(),
  listFilesApi: vi.fn(),
  purgeFileApi: vi.fn(),
  recycleFileApi: vi.fn(),
  reload: vi.fn(),
  restoreFileApi: vi.fn(),
  uploadFileApi: vi.fn(),
}));

vi.mock('@/api/system', () => ({
  downloadFileApi: apiMocks.downloadFileApi,
  getFileDetailApi: apiMocks.getFileDetailApi,
  getMyNoticeApi: vi.fn(),
  listFilesApi: apiMocks.listFilesApi,
  listMyNoticesApi: vi.fn(),
  markNoticeReadApi: vi.fn(),
  publishNoticeApi: vi.fn(),
  purgeFileApi: apiMocks.purgeFileApi,
  recycleFileApi: apiMocks.recycleFileApi,
  restoreFileApi: apiMocks.restoreFileApi,
  revokeNoticeApi: vi.fn(),
  uploadFileApi: apiMocks.uploadFileApi,
}));

vi.mock('@/components/ResourceCrudPage', () => ({ ResourceCrudPage: () => null }));

vi.mock('@/components/DataTablePage', () => ({
  DataTablePage: ({
    title,
    loadData,
    actions = [],
    extra,
  }: {
    title: string;
    loadData: (query: { keyword: string; page: number; pageSize: number }) => unknown;
    actions?: Array<{ label: string; onClick: (row: { id: number; originalName: string }, reload: () => Promise<void>) => unknown }>;
    extra?: React.ReactNode;
  }) => {
    void loadData({ keyword: '', page: 1, pageSize: 20 });
    const row = { id: 9, originalName: 'photo.png' };
    return (
      <div>
        <h1>{title}</h1>
        <div>{extra}</div>
        {actions.map((action) => (
          <button key={action.label} onClick={() => void action.onClick(row, apiMocks.reload)}>
            {action.label}
          </button>
        ))}
      </div>
    );
  },
}));

import { FilePage, FileRecyclePage, FileReferencesPage } from './content';

describe('FilePage', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('lists non-recycled files, offers upload and wires row actions', async () => {
    apiMocks.listFilesApi.mockResolvedValue({ items: [], total: 0 });
    apiMocks.downloadFileApi.mockResolvedValue(undefined);
    apiMocks.recycleFileApi.mockResolvedValue(undefined);

    render(
      <StrictMode>
        <FilePage />
      </StrictMode>,
    );

    expect(screen.getByText('文件管理')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /上传文件/ })).toBeInTheDocument();
    expect(apiMocks.listFilesApi).toHaveBeenCalledWith(
      expect.objectContaining({ recycled: false, page: 1, pageSize: 20 }),
    );

    fireEvent.click(screen.getByRole('button', { name: '下载' }));
    await waitFor(() =>
      expect(apiMocks.downloadFileApi).toHaveBeenCalledWith(9, 'photo.png'),
    );

    fireEvent.click(screen.getByRole('button', { name: '删除' }));
    await waitFor(() => expect(apiMocks.recycleFileApi).toHaveBeenCalledWith(9));
    await waitFor(() => expect(apiMocks.reload).toHaveBeenCalled());
  });
});

describe('FileRecyclePage', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('lists only recycled files and restores or purges them', async () => {
    apiMocks.listFilesApi.mockResolvedValue({ items: [], total: 0 });
    apiMocks.restoreFileApi.mockResolvedValue(undefined);
    apiMocks.purgeFileApi.mockResolvedValue(undefined);

    render(
      <StrictMode>
        <FileRecyclePage />
      </StrictMode>,
    );

    expect(screen.getByText('文件回收站')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /上传文件/ })).not.toBeInTheDocument();
    expect(apiMocks.listFilesApi).toHaveBeenCalledWith(
      expect.objectContaining({ recycled: true }),
    );

    fireEvent.click(screen.getByRole('button', { name: '恢复' }));
    await waitFor(() => expect(apiMocks.restoreFileApi).toHaveBeenCalledWith(9));
    await waitFor(() => expect(apiMocks.reload).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole('button', { name: '彻底删除' }));
    await waitFor(() => expect(apiMocks.purgeFileApi).toHaveBeenCalledWith(9));
    await waitFor(() => expect(apiMocks.reload).toHaveBeenCalledTimes(2));
  });
});

describe('FileReferencesPage', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('lists ready files and shows references from the detail endpoint', async () => {
    apiMocks.listFilesApi.mockResolvedValue({ items: [], total: 0 });
    apiMocks.getFileDetailApi.mockResolvedValue({
      file: { id: 9, originalName: 'photo.png' },
      references: [
        { id: 1, businessType: 'order', businessId: '1001', fieldName: 'attachments' },
      ],
    });

    render(
      <StrictMode>
        <FileReferencesPage />
      </StrictMode>,
    );

    expect(apiMocks.listFilesApi).toHaveBeenCalledWith(
      expect.objectContaining({ status: 'ready' }),
    );

    fireEvent.click(screen.getByRole('button', { name: '查看引用' }));
    await waitFor(() => expect(apiMocks.getFileDetailApi).toHaveBeenCalledWith(9));
    expect(await screen.findByText('order')).toBeInTheDocument();
  });
});
