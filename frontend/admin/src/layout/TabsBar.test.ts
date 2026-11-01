import { describe, expect, it } from 'vitest';
import { tabCloseAvailability } from './TabsBar';
import type { TabDefinition } from '@/types';

const tab = (key: string, affix = false): TabDefinition => ({ affix, key, path: key, title: key });

describe('tab more-menu availability', () => {
  it('disables close-left on the first tab and close-right on the last tab', () => {
    const tabs = [tab('/a', true), tab('/b'), tab('/c')];
    expect(tabCloseAvailability(tabs[0]!, tabs)).toMatchObject({
      canClose: true,
      canCloseLeft: false,
      canCloseRight: true,
      canCloseOthers: true,
      canCloseAll: true,
    });
    expect(tabCloseAvailability(tabs[2]!, tabs)).toMatchObject({
      canCloseLeft: true,
      canCloseRight: false,
    });
  });

  it('disables close-all when only one tab remains', () => {
    const tabs = [tab('/users', true)];
    expect(tabCloseAvailability(tabs[0]!, tabs)).toEqual({
      canClose: false,
      canCloseAll: false,
      canCloseLeft: false,
      canCloseOthers: false,
      canCloseRight: false,
    });
  });

  it('disables every close action when only the workbench remains', () => {
    const tabs = [tab('/analytics', true)];
    expect(tabCloseAvailability(tabs[0]!, tabs)).toEqual({
      canClose: false,
      canCloseAll: false,
      canCloseLeft: false,
      canCloseOthers: false,
      canCloseRight: false,
    });
  });
});
