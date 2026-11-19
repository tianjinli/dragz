import { beforeEach, describe, expect, it } from 'vitest';
import { formatDateTime } from './format-time';
import { usePreferencesStore } from '@/stores/preferences';
import { useSystemConfigStore } from '@/stores/system-config';

describe('formatDateTime', () => {
  beforeEach(() => {
    usePreferencesStore.setState({
      preferences: {
        ...usePreferencesStore.getState().preferences,
        app: {
          ...usePreferencesStore.getState().preferences.app,
          timezone: 'Asia/Shanghai',
        },
      },
    });
    useSystemConfigStore.setState({
      loaded: true,
      values: {},
    });
  });

  it('formats ISO timestamps with the default YYYY-MM-dd HH:mm:ss pattern', () => {
    expect(formatDateTime('2026-11-19T18:19:01.2+08:00')).toBe('2026-11-19 18:19:01');
  });

  it('uses system.dateTimeFormat when configured', () => {
    useSystemConfigStore.setState({
      loaded: true,
      values: { 'system.dateTimeFormat': 'YYYY/MM/DD HH:mm' },
    });
    expect(formatDateTime('2026-11-19T18:19:01.2+08:00')).toBe('2026/11/19 18:19');
  });

  it('falls back to the default pattern when the config is blank', () => {
    useSystemConfigStore.setState({
      loaded: true,
      values: { 'system.dateTimeFormat': '   ' },
    });
    expect(formatDateTime('2026-11-19T18:19:01.2+08:00')).toBe('2026-11-19 18:19:01');
  });
});
