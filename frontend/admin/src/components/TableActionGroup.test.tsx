import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TableActionGroup, type TableAction } from './TableActionGroup';

class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverMock);

afterEach(() => cleanup());

describe('TableActionGroup more menu', () => {
  it('closes the overflow menu after clicking a more-menu item', async () => {
    const onHistory = vi.fn();
    const actions: TableAction[] = [
      { label: '编辑', onClick: vi.fn() },
      { label: '删除', danger: true, onClick: vi.fn() },
      { label: '改价历史', inMore: true, onClick: onHistory },
      { label: '库存流水', inMore: true, onClick: vi.fn() },
    ];

    render(<TableActionGroup actions={actions} />);

    fireEvent.click(screen.getByRole('button', { name: '更多操作' }));
    expect(await screen.findByText('改价历史')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '改价历史' }));
    expect(onHistory).toHaveBeenCalledTimes(1);

    await waitFor(() => {
      expect(document.querySelector('.ant-dropdown:not(.ant-dropdown-hidden)')).toBeNull();
      expect(screen.queryByRole('button', { name: '改价历史' })).not.toBeInTheDocument();
    });
  });
});
