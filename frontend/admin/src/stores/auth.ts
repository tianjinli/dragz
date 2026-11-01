import { create } from 'zustand';
import { getAccessCodesApi, getMenusApi, getUserInfoApi, loginApi, logoutApi, refreshTokenApi, type LoginInput } from '@/api/auth';
import { useAccessStore } from './access';
import { useLockScreenStore } from './lock-screen';
import { useTabsStore } from './tabs';

export const LOGOUT_REDIRECT_FLAG = 'admin:intentional-logout';

let loadAccessPromise: Promise<void> | null = null;
let bootstrapPromise: Promise<boolean> | null = null;

interface AuthStore {
  loading: boolean;
  bootstrap: () => Promise<boolean>;
  login: (input: LoginInput) => Promise<string>;
  logout: () => Promise<void>;
  loadAccess: () => Promise<void>;
}

export const useAuthStore = create<AuthStore>((set, get) => ({
  loading: false,
  loadAccess: async () => {
    if (loadAccessPromise) return loadAccessPromise;
    loadAccessPromise = Promise.all([getUserInfoApi(), getAccessCodesApi(), getMenusApi()])
      .then(([userInfo, accessCodes, menus]) => {
        const access = useAccessStore.getState();
        access.setUserInfo(userInfo);
        access.setAccessCodes(accessCodes);
        access.setMenus(menus);
        access.setChecked(true);
      })
      .finally(() => {
        loadAccessPromise = null;
      });
    return loadAccessPromise;
  },
  bootstrap: async () => {
    if (bootstrapPromise) return bootstrapPromise;
    if (sessionStorage.getItem(LOGOUT_REDIRECT_FLAG)) {
      useAccessStore.setState({
        accessCodes: [],
        accessToken: null,
        checked: true,
        menus: [],
        userInfo: null,
      });
      return false;
    }
    set({ loading: true });
    bootstrapPromise = (async () => {
      try {
        if (!useAccessStore.getState().accessToken) {
          const token = await refreshTokenApi();
          useAccessStore.getState().setAccessToken(token.accessToken);
        }
        await get().loadAccess();
        return true;
      } catch {
        useAccessStore.getState().clear();
        return false;
      } finally {
        set({ loading: false });
        bootstrapPromise = null;
      }
    })();
    return bootstrapPromise;
  },
  login: async (input) => {
    set({ loading: true });
    try {
      sessionStorage.removeItem(LOGOUT_REDIRECT_FLAG);
      const token = await loginApi(input);
      useAccessStore.getState().setAccessToken(token.accessToken);
      await get().loadAccess();
      return useAccessStore.getState().userInfo?.homePath || '/analytics';
    } finally {
      set({ loading: false });
    }
  },
  logout: async () => {
    try {
      await logoutApi();
    } finally {
      sessionStorage.setItem(LOGOUT_REDIRECT_FLAG, '1');
      // Keep checked=true so Protected does not re-bootstrap via refresh cookie.
      useAccessStore.setState({
        accessCodes: [],
        accessToken: null,
        checked: true,
        menus: [],
        userInfo: null,
      });
      useTabsStore.setState({ activeKey: '', tabs: [], visitHistory: [], refreshKeys: {}, maximized: false });
      useLockScreenStore.getState().logoutUnlock();
    }
  },
}));
