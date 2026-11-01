import { describe, expect, it } from 'vitest';
import { defaultPreferences } from '@/stores/preferences';
import { getPreferenceDifference, hasPreferenceDifference, serializePreferenceDifference } from './preferences-diff';

describe('preference difference', () => {
  it('returns no configuration for Golden Master defaults', () => {
    expect(getPreferenceDifference(defaultPreferences, defaultPreferences)).toEqual({});
    expect(hasPreferenceDifference(defaultPreferences, defaultPreferences)).toBe(false);
  });

  it('copies only nested values which differ from defaults', () => {
    const current = {
      ...defaultPreferences,
      app: { ...defaultPreferences.app, dynamicTitle: false },
      sidebar: { ...defaultPreferences.sidebar, width: 268 },
    };
    expect(getPreferenceDifference(current, defaultPreferences)).toEqual({
      app: { dynamicTitle: false },
      sidebar: { width: 268 },
    });
    const serialized = serializePreferenceDifference(current, defaultPreferences);
    expect(serialized).toContain('export const overrides = {');
    expect(serialized).toContain('dynamicTitle: false');
    expect(serialized).toContain('width: 268');
  });

  it('treats widget order as one intentional preference', () => {
    const current = { ...defaultPreferences, widget: { ...defaultPreferences.widget, order: [...defaultPreferences.widget.order].reverse() } };
    expect(getPreferenceDifference(current, defaultPreferences)).toEqual({ widget: { order: current.widget.order } });
  });
});
