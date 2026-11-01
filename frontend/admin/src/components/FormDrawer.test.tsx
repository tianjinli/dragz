import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@/i18n';
import { FormDrawer } from './FormDrawer';

afterEach(() => cleanup());

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
});
