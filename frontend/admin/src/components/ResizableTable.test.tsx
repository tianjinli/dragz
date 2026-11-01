import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ColumnDef, ColumnSizingState } from '@tanstack/react-table';

import { ResizableTable } from './ResizableTable';

// antd Table renders a hidden measure row (ResizeObserver) when scroll.x is set.
class ResizeObserverStub {
  disconnect() {}
  observe() {}
  unobserve() {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverStub);

interface DemoRow {
  id: string;
  name: string;
  email: string;
}

const columns: Array<ColumnDef<DemoRow, unknown>> = [
  { accessorKey: 'name', header: '用户名', size: 160, minSize: 100 },
  { accessorKey: 'email', header: '邮箱', size: 240, enableResizing: false },
];

const data: DemoRow[] = [
  { id: '1', name: '张三', email: 'zhangsan@example.com' },
  { id: '2', name: '李四', email: 'lisi@example.com' },
];

const NAME_HANDLE_LABEL = '调整「用户名」列宽';

/** The real header handle — antd also clones the title into a hidden measure row. */
function headerHandle(container: HTMLElement, label: string): HTMLElement {
  const handle = container.querySelector<HTMLElement>(`.ant-table-thead [aria-label="${label}"]`);
  if (!handle) throw new Error(`handle not found in header: ${label}`);
  return handle;
}

function colWidth(container: HTMLElement, index: number): string | null {
  return container.querySelectorAll('colgroup col')[index]?.getAttribute('style') ?? null;
}

describe('ResizableTable (TanStack column sizing)', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(cleanup);

  it('renders headers, cells and one handle per resizable column', () => {
    const { container } = render(<ResizableTable columns={columns} data={data} />);
    expect(container.querySelector('.ant-table-thead')?.textContent).toContain('用户名');
    expect(screen.getByText('张三')).toBeInTheDocument();
    expect(screen.getByText('zhangsan@example.com')).toBeInTheDocument();
    expect(headerHandle(container, NAME_HANDLE_LABEL)).toBeInTheDocument();
    // enableResizing: false → no handle
    expect(container.querySelector('.ant-table-thead [aria-label="调整「邮箱」列宽"]')).toBeNull();
    // declared sizes land on the body colgroup
    expect(colWidth(container, 0)).toContain('160px');
    expect(colWidth(container, 1)).toContain('240px');
  });

  it('commits a new width while dragging the handle (columnResizeMode: onChange)', async () => {
    const onColumnSizingChange = vi.fn();
    const { container } = render(
      <ResizableTable columns={columns} data={data} onColumnSizingChange={onColumnSizingChange} />,
    );
    const handle = headerHandle(container, NAME_HANDLE_LABEL);

    fireEvent.mouseDown(handle, { clientX: 100 });
    // 160px start + (160 - 100) drag delta = 220px
    fireEvent.mouseMove(document, { clientX: 160 });
    fireEvent.mouseUp(document, { clientX: 160 });

    await waitFor(() => {
      expect(colWidth(container, 0)).toContain('220px');
    });
    const lastCall = onColumnSizingChange.mock.calls.at(-1)?.[0] as ColumnSizingState | undefined;
    expect(lastCall).toMatchObject({ name: 220 });
    // untouched column keeps its declared width
    expect(colWidth(container, 1)).toContain('240px');
  });

  it('clamps the dragged width to the column minSize', async () => {
    const { container } = render(<ResizableTable columns={columns} data={data} />);
    fireEvent.mouseDown(headerHandle(container, NAME_HANDLE_LABEL), { clientX: 400 });
    fireEvent.mouseMove(document, { clientX: 0 });
    fireEvent.mouseUp(document, { clientX: 0 });

    await waitFor(() => {
      expect(colWidth(container, 0)).toContain('100px');
    });
  });

  it('marks the active column with data-resizing during the drag', () => {
    const { container } = render(<ResizableTable columns={columns} data={data} />);
    const handle = headerHandle(container, NAME_HANDLE_LABEL);
    expect(handle).not.toHaveAttribute('data-resizing');

    fireEvent.mouseDown(handle, { clientX: 100 });
    expect(handle).toHaveAttribute('data-resizing', 'true');
    expect(document.body).toHaveClass('is-column-resizing');

    fireEvent.mouseUp(document, { clientX: 120 });
    expect(handle).not.toHaveAttribute('data-resizing');
    expect(document.body).not.toHaveClass('is-column-resizing');
  });

  it('resets the column to its declared size on handle double-click', async () => {
    const { container } = render(<ResizableTable columns={columns} data={data} />);
    fireEvent.mouseDown(headerHandle(container, NAME_HANDLE_LABEL), { clientX: 100 });
    fireEvent.mouseMove(document, { clientX: 200 });
    fireEvent.mouseUp(document, { clientX: 200 });
    await waitFor(() => {
      expect(colWidth(container, 0)).toContain('260px');
    });

    fireEvent.doubleClick(headerHandle(container, NAME_HANDLE_LABEL));
    await waitFor(() => {
      expect(colWidth(container, 0)).toContain('160px');
    });
  });

  it('persists widths to localStorage and restores them on remount', async () => {
    const storageKey = 'test:resizable-table';
    const first = render(
      <ResizableTable columns={columns} data={data} storageKey={storageKey} />,
    );
    fireEvent.mouseDown(headerHandle(first.container, NAME_HANDLE_LABEL), { clientX: 100 });
    fireEvent.mouseMove(document, { clientX: 180 });
    fireEvent.mouseUp(document, { clientX: 180 });
    await waitFor(() => {
      expect(colWidth(first.container, 0)).toContain('240px');
    });
    expect(JSON.parse(window.localStorage.getItem(storageKey) ?? '{}')).toMatchObject({ name: 240 });
    first.unmount();

    const second = render(<ResizableTable columns={columns} data={data} storageKey={storageKey} />);
    expect(colWidth(second.container, 0)).toContain('240px');
  });

  it('applies defaultColumnSizing before any drag', () => {
    const { container } = render(
      <ResizableTable columns={columns} data={data} defaultColumnSizing={{ name: 200 }} />,
    );
    expect(colWidth(container, 0)).toContain('200px');
  });
});
