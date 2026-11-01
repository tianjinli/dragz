import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * rc-overflow (antd horizontal Menu) sums item offsetWidth vs container clientWidth.
 * Horizontal margins are excluded from offsetWidth, so margin gaps cause clipped items
 * without showing `.ant-menu-overflow-item-rest`. Spacing must use padding/border instead.
 */
describe('header horizontal menu overflow CSS', () => {
  const cssPath = resolve(dirname(fileURLToPath(import.meta.url)), '../styles/layout.css');
  const css = readFileSync(cssPath, 'utf8');

  it('does not space header overflow items with horizontal margin', () => {
    const block = css.match(
      /Horizontal Menu Overflow[\s\S]*?\.ant-menu-horizontal > \.ant-menu-overflow-item[\s\S]*?\{([^}]+)\}/,
    );
    expect(block?.[1]).toBeTruthy();
    const body = block![1]!;
    expect(body).toMatch(/margin-inline:\s*0/);
    expect(body).toMatch(/border-inline:\s*5px\s+solid\s+transparent/);
    expect(/margin-inline:\s*[1-9]/.test(body)).toBe(false);
  });

  it('keeps header-menu as a bounded overflow box for Overflow measurement', () => {
    expect(css).toMatch(/\.header-menu\s*\{[^}]*min-width:\s*0/s);
    expect(css).toMatch(/\.header-menu\s*\{[^}]*overflow:\s*hidden/s);
  });

  it('restores header menu selected pill backgrounds (not only Ant underline)', () => {
    expect(css).toMatch(/--header-menu-item-bg-active:/);
    expect(css).toMatch(
      /\.layout-header-nav[\s\S]*\.ant-menu-item-selected[\s\S]*background:\s*var\(--header-menu-item-bg-active\)/,
    );
    expect(css).toMatch(
      /\.layout-mixed-nav[\s\S]*\.ant-menu-item-selected[\s\S]*background:\s*var\(--header-menu-item-bg-active\)/,
    );
    expect(css).toMatch(
      /\.layout-header-mixed-nav[\s\S]*\.ant-menu-item-selected[\s\S]*background:\s*var\(--header-menu-item-bg-active\)/,
    );
  });
});
