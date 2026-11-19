import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { storageKey } from './storage';

interface LockStore {
  locked: boolean;
  lock: () => void;
  logoutUnlock: () => void;
  unlock: () => void;
}

export const useLockScreenStore = create<LockStore>()(
  persist(
    (set) => ({
      locked: false,
      lock: () => set({ locked: true }),
      logoutUnlock: () => set({ locked: false }),
      unlock: () => set({ locked: false }),
    }),
    {
      name: storageKey('lock-screen'),
      partialize: (state) => ({ locked: state.locked }),
      migrate: (persisted) => {
        const value = (persisted || {}) as { locked?: boolean };
        return { locked: !!value.locked };
      },
      version: 3,
    },
  ),
);
