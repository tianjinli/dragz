import { describe, expect, it } from 'vitest';
import { safeRedirectPath } from './redirect';
import { antdBorderRadius } from './theme-css';
import { defaultPreferences } from '@/stores/preferences';

describe('safeRedirectPath', () => {
  it('allows same-origin relative paths', () => {
    expect(safeRedirectPath('/analytics', '/home')).toBe('/analytics');
    expect(safeRedirectPath('/system/user?id=1', '/home')).toBe('/system/user?id=1');
  });

  it('rejects open redirects', () => {
    expect(safeRedirectPath('//evil.com', '/home')).toBe('/home');
    expect(safeRedirectPath('https://evil.com', '/home')).toBe('/home');
    expect(safeRedirectPath(null, '/home')).toBe('/home');
  });

  it('sends an empty or root redirect to the workbench fallback', () => {
    expect(safeRedirectPath('', '/analytics')).toBe('/analytics');
    expect(safeRedirectPath('/', '/analytics')).toBe('/analytics');
    expect(safeRedirectPath('/?', '/analytics')).toBe('/analytics');
    expect(safeRedirectPath('/auth/login', '/analytics')).toBe('/analytics');
  });
});

describe('theme radius', () => {
  it('maps vben radius units to antd pixel tokens', () => {
    expect(antdBorderRadius({ ...defaultPreferences, theme: { ...defaultPreferences.theme, radius: '0' } })).toBe(0);
    expect(antdBorderRadius({ ...defaultPreferences, theme: { ...defaultPreferences.theme, radius: '0.5' } })).toBe(8);
    expect(antdBorderRadius({ ...defaultPreferences, theme: { ...defaultPreferences.theme, radius: '1' } })).toBe(16);
  });
});
