import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Form, Input } from 'antd';
import '@/i18n';
import { FormDrawer } from './FormDrawer';

afterEach(() => cleanup());

function DrawerWithForm({ onClose }: { onClose: () => void }) {
  const [form] = Form.useForm();
  return (
    <FormDrawer form={form} onClose={onClose} onOk={() => undefined} open title="编辑">
      <Form form={form} initialValues={{ name: '' }}>
        <Form.Item label="名称" name="name">
          <Input />
        </Form.Item>
      </Form>
    </FormDrawer>
  );
}

describe('FormDrawer', () => {
  it('hides the close icon and shows sticky cancel/save plus maximize', () => {
    const onClose = vi.fn();
    const onOk = vi.fn();
    render(
      <FormDrawer onClose={onClose} onOk={onOk} open title="新增用户">
        <div>form body</div>
      </FormDrawer>,
    );

    expect(screen.getByText('新增用户')).toBeInTheDocument();
    expect(screen.getByText('form body')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /取\s*消/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /保\s*存/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '最大化' })).toBeInTheDocument();
    expect(document.querySelector('.ant-drawer-close')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: /取\s*消/ }));
    expect(onClose).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    expect(onOk).toHaveBeenCalledTimes(1);
  });

  it('toggles maximize and restore labels', () => {
    render(
      <FormDrawer onClose={() => undefined} onOk={() => undefined} open title="编辑">
        body
      </FormDrawer>,
    );

    fireEvent.click(screen.getByRole('button', { name: '最大化' }));
    expect(screen.getByRole('button', { name: '还原' })).toBeInTheDocument();
    expect(document.querySelector('.form-drawer-maximized')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: '还原' }));
    expect(screen.getByRole('button', { name: '最大化' })).toBeInTheDocument();
    expect(document.querySelector('.form-drawer-maximized')).toBeNull();
  });

  it('closes an untouched form without asking', () => {
    const onClose = vi.fn();
    render(<DrawerWithForm onClose={onClose} />);

    fireEvent.click(screen.getByRole('button', { name: /取\s*消/ }));

    expect(screen.queryByText('放弃未保存的修改？')).toBeNull();
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('asks before discarding edits, and stays open when the operator declines', async () => {
    const onClose = vi.fn();
    render(<DrawerWithForm onClose={onClose} />);

    fireEvent.change(screen.getByLabelText('名称'), { target: { value: '改了一半' } });
    fireEvent.click(screen.getByRole('button', { name: /取\s*消/ }));

    expect((await screen.findAllByText('放弃未保存的修改？')).length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole('button', { name: /继\s*续\s*编\s*辑/ }));

    await waitFor(() => expect(onClose).not.toHaveBeenCalled());
  });

  it('closes an edited form once the operator confirms the discard', async () => {
    const onClose = vi.fn();
    render(<DrawerWithForm onClose={onClose} />);

    fireEvent.change(screen.getByLabelText('名称'), { target: { value: '改了一半' } });
    fireEvent.click(screen.getByRole('button', { name: /取\s*消/ }));

    fireEvent.click(await screen.findByRole('button', { name: /放\s*弃\s*修\s*改/ }));

    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
  });
});
