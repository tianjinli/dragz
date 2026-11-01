import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { AdminUserInfo, MenuRoute } from '@/types';
import { storageKey } from './storage';

interface AccessStore {
  accessCodes: string[]; accessToken: string | null; checked: boolean;
  menus: MenuRoute[]; userInfo: AdminUserInfo | null;
  clear: () => void; setAccessCodes: (value: string[]) => void; setAccessToken: (value: string | null) => void;
  setChecked: (value: boolean) => void; setMenus: (value: MenuRoute[]) => void; setUserInfo: (value: AdminUserInfo | null) => void;
}

export const useAccessStore = create<AccessStore>()(persist((set) => ({
  accessCodes: [], accessToken: null, checked: false, menus: [], userInfo: null,
  clear: () => set({ accessCodes: [], accessToken: null, checked: false, menus: [], userInfo: null }),
  setAccessCodes: (accessCodes) => set({ accessCodes }), setAccessToken: (accessToken) => set({ accessToken }),
  setChecked: (checked) => set({ checked }), setMenus: (menus) => set({ menus }), setUserInfo: (userInfo) => set({ userInfo }),
}), { name: storageKey('access'), partialize: (state) => ({ accessToken: state.accessToken, accessCodes: state.accessCodes }) }));

export function hasAccessByCodes(codes: string[]) {
  const { accessCodes, userInfo } = useAccessStore.getState();
  return accessCodes.includes('*') || !!userInfo?.isSuperAdmin || codes.some((code) => accessCodes.includes(code));
}
