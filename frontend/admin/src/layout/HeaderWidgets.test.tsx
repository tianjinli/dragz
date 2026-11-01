import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TippedDropdown } from './HeaderWidgets';

// antd v6 Trigger measures its popup through rc-resize-observer, which jsdom
// does not implement; without the stub the trigger throws before rendering.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
Object.defineProperty(globalThis, 'ResizeObserver', { writable: true, value: ResizeObserverStub });

const items = [
  { key: 'zh-CN', label: '简体中文' },
  { key: 'en-US', label: 'English' },
];

function renderDropdown() {
  const onSelect = vi.fn();
  render(
    <TippedDropdown items={items} onSelect={onSelect} selectedKeys={['zh-CN']} title="语言切换">
      <button type="button">lang</button>
    </TippedDropdown>,
  );
  return { onSelect };
}

describe('TippedDropdown', () => {
  afterEach(cleanup);

  it('shows the tip on hover and hides it once the menu opens', async () => {
    renderDropdown();
    const button = screen.getByRole('button', { name: 'lang' });

    fireEvent.mouseEnter(button);
    await waitFor(() => {
      const tip = document.querySelector('.ant-tooltip');
      expect(tip).not.toBeNull();
      expect(tip?.className).not.toContain('ant-tooltip-hidden');
    });

    fireEvent.click(button);
    await waitFor(() => {
      expect(screen.getByText('English')).toBeTruthy();
      expect(document.querySelector('.ant-tooltip')?.className).toContain('ant-tooltip-hidden');
    });
  }, 120000);

  it('still reports the picked item', async () => {
    const { onSelect } = renderDropdown();
    fireEvent.click(screen.getByRole('button', { name: 'lang' }));

    fireEvent.click(await screen.findByText('English'));

    await waitFor(() => expect(onSelect).toHaveBeenCalled());
    expect(onSelect.mock.calls[0][0].key).toBe('en-US');
  }, 120000);
});
