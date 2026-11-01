import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { useState } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { VxeTable } from './VxeTable';
import type { SortChangeEvent, VxeColumnProps } from '../types';

afterEach(cleanup);

interface Row {
  id: number;
  name: string;
  age: number;
}

const rows: Row[] = [
  { id: 3, name: '王五', age: 32 },
  { id: 1, name: '张三', age: 21 },
  { id: 2, name: '李四', age: 25 },
];

const baseColumns: Array<VxeColumnProps<Row>> = [
  { field: 'id', title: 'ID', width: 80, sortable: true },
  { field: 'name', title: '姓名', width: 120 },
  { field: 'age', title: '年龄', width: 100, sortable: true },
];

function firstBodyRow() {
  const body = document.querySelector('.vxe-table--body') as HTMLElement;
  return within(body).getAllByRole('row')[0];
}

describe('VxeTable', () => {
  it('renders rows and column titles', () => {
    render(<VxeTable columns={baseColumns} data={rows} />);
    expect(screen.getByText('ID')).toBeTruthy();
    expect(screen.getByText('张三')).toBeTruthy();
    expect(screen.getByText('李四')).toBeTruthy();
  });

  it('shows empty state when data is empty', () => {
    render(<VxeTable columns={baseColumns} data={[]} emptyText="没有记录" />);
    expect(screen.getByText('没有记录')).toBeTruthy();
  });

  it('sorts asc -> desc -> none on header click and emits sort-change', () => {
    const onSortChange = vi.fn<(event: SortChangeEvent) => void>();
    render(<VxeTable columns={baseColumns} data={rows} onSortChange={onSortChange} />);

    fireEvent.click(screen.getByText('ID'));
    expect(firstBodyRow().textContent).toContain('张三'); // id 1 asc
    expect(onSortChange).toHaveBeenLastCalledWith({ field: 'id', order: 'asc', sortList: [{ field: 'id', order: 'asc' }] });

    fireEvent.click(screen.getByText('ID'));
    expect(firstBodyRow().textContent).toContain('王五'); // id 3 desc
    expect(onSortChange).toHaveBeenLastCalledWith({ field: 'id', order: 'desc', sortList: [{ field: 'id', order: 'desc' }] });

    fireEvent.click(screen.getByText('ID'));
    expect(firstBodyRow().textContent).toContain('王五'); // back to source order
    expect(onSortChange).toHaveBeenLastCalledWith({ field: 'id', order: null, sortList: [] });
  });

  it('uses a custom sorter when provided', () => {
    render(
      <VxeTable
        columns={[{ field: 'age', title: '年龄', width: 80, sortable: true, sorter: (a, b) => b.age - a.age }]}
        data={rows}
      />,
    );
    fireEvent.click(screen.getByText('年龄'));
    expect(firstBodyRow().textContent).toContain('32');
  });

  it('supports formatter and slot renderers', () => {
    const columns: Array<VxeColumnProps<Row>> = [
      { field: 'name', title: '姓名', width: 100, formatter: (value) => `《${value}》` },
      {
        field: 'age',
        title: '年龄',
        width: 100,
        slots: { default: ({ value }) => <strong data-testid="age-slot">{String(value)}岁</strong> },
      },
    ];
    render(<VxeTable columns={columns} data={rows} />);
    expect(screen.getByText('《张三》')).toBeTruthy();
    expect(screen.getAllByTestId('age-slot')[0].textContent).toBe('32岁');
  });

  it('applies default sort from sortConfig', () => {
    render(<VxeTable columns={baseColumns} data={rows} sortConfig={{ defaultSort: { field: 'age', order: 'desc' } }} />);
    expect(firstBodyRow().textContent).toContain('王五'); // 32 desc
  });

  it('supports controlled sort prop', () => {
    function Controlled() {
      const [sort, setSort] = useState<{ field: string; order: 'asc' | 'desc' | null }>({ field: 'age', order: 'asc' });
      return (
        <VxeTable
          columns={baseColumns}
          data={rows}
          sort={sort}
          onSortChange={(event) => setSort({ field: event.field, order: event.order })}
        />
      );
    }
    render(<Controlled />);
    expect(firstBodyRow().textContent).toContain('张三'); // 21 asc
    fireEvent.click(screen.getByText('年龄'));
    expect(firstBodyRow().textContent).toContain('王五'); // 32 desc
  });

  it('emits cell-click, row-click and header-cell-click events', () => {
    const onCellClick = vi.fn();
    const onRowClick = vi.fn();
    const onHeaderCellClick = vi.fn();
    render(
      <VxeTable
        columns={baseColumns}
        data={rows}
        onCellClick={onCellClick}
        onHeaderCellClick={onHeaderCellClick}
        onRowClick={onRowClick}
      />,
    );
    fireEvent.click(screen.getByText('李四'));
    expect(onCellClick).toHaveBeenCalledWith(expect.objectContaining({ field: 'name', rowIndex: 2 }));
    expect(onRowClick).toHaveBeenCalledWith({ row: rows[2], rowIndex: 2 });
    fireEvent.click(screen.getByText('姓名'));
    expect(onHeaderCellClick).toHaveBeenCalledWith(expect.objectContaining({ column: expect.objectContaining({ field: 'name' }) }));
  });

  it('pins fixed columns with sticky offsets', () => {
    render(
      <VxeTable
        columns={[
          { field: 'id', title: 'ID', width: 80, fixed: 'left' },
          { field: 'name', title: '姓名', width: 120, fixed: 'left' },
          { field: 'age', title: '年龄', width: 80, fixed: 'right' },
        ]}
        data={rows}
      />,
    );
    const header = document.querySelector('.vxe-table--header') as HTMLElement;
    const cells = within(header).getAllByRole('columnheader');
    expect((cells[0] as HTMLElement).style.position).toBe('sticky');
    expect((cells[0] as HTMLElement).style.left).toBe('0px');
    expect((cells[1] as HTMLElement).style.left).toBe('80px');
    expect((cells[2] as HTMLElement).style.right).toBe('0px');
  });

  it('resizes a column by dragging the header handle', () => {
    const onColumnWidthChange = vi.fn();
    render(<VxeTable columns={baseColumns} data={rows} onColumnWidthChange={onColumnWidthChange} />);
    const handle = document.querySelector('.vxe-header--resizable') as HTMLElement;
    expect(handle).toBeTruthy();
    fireEvent.mouseDown(handle, { clientX: 200 });
    fireEvent.mouseMove(window, { clientX: 260 });
    fireEvent.mouseUp(window);
    expect(onColumnWidthChange).toHaveBeenCalled();
    const widths = onColumnWidthChange.mock.lastCall?.[0] as Record<string, number>;
    expect(widths.id).toBe(140); // 80 + (260 - 200)
  });

  it('shows loading overlay', () => {
    const { rerender } = render(<VxeTable columns={baseColumns} data={rows} loading={false} />);
    expect(document.querySelector('.vxe-table--loading')).toBeNull();
    rerender(<VxeTable columns={baseColumns} data={rows} loading />);
    expect(document.querySelector('.vxe-table--loading')).toBeTruthy();
  });

  it('renders group headers with colSpan/rowSpan and keeps leaf sorting working', () => {
    const columns: Array<VxeColumnProps<Row>> = [
      {
        title: '身份',
        children: [
          { field: 'id', title: 'ID', width: 80, sortable: true },
          { field: 'name', title: '姓名', width: 120 },
        ],
      },
      { field: 'age', title: '年龄', width: 100, sortable: true },
    ];
    render(<VxeTable columns={columns} data={rows} />);

    const header = document.querySelector('.vxe-table--header') as HTMLElement;
    const [topRow, secondRow] = within(header).getAllByRole('row');
    const groupCell = within(topRow).getByText('身份').closest('th') as HTMLElement;
    expect(groupCell.colSpan).toBe(2);
    expect(groupCell.rowSpan).toBe(1);
    const ageCell = within(topRow).getByText('年龄').closest('th') as HTMLElement;
    expect(ageCell.rowSpan).toBe(2);
    expect(within(secondRow).getByText('ID')).toBeTruthy();

    fireEvent.click(within(secondRow).getByText('ID'));
    expect(firstBodyRow().textContent).toContain('张三');
  });

  it('does not render columns with visible: false', () => {
    const columns: Array<VxeColumnProps<Row>> = [
      { field: 'id', title: 'ID', width: 80 },
      { field: 'name', title: '姓名', width: 120, visible: false },
      { field: 'age', title: '年龄', width: 100 },
    ];
    render(<VxeTable columns={columns} data={rows} />);
    expect(screen.queryByText('姓名')).toBeNull();
    expect(screen.queryByText('张三')).toBeNull();
    expect(screen.getByText('年龄')).toBeTruthy();
  });
});
