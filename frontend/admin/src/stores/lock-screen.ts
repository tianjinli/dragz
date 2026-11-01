import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { storageKey } from './storage';

async function sha256(text: string) {
  const data = new TextEncoder().encode(text);
  const digest = await crypto.subtle.digest('SHA-256', data);
  return Array.from(new Uint8Array(digest))
    .map((byte) => byte.toString(16).padStart(2, '0'))
    .join('');
}

interface LockStore {
  locked: boolean;
  passwordHash: string;
  lock: (password: string) => Promise<void>;
  logoutUnlock: () => void;
  unlock: (password: string) => Promise<boolean>;
}

export const useLockScreenStore = create<LockStore>()(
  persist(
    (set, get) => ({
      locked: false,
      passwordHash: '',
      lock: async (password) => {
        const passwordHash = await sha256(password);
        set({ locked: true, passwordHash });
      },
      logoutUnlock: () => set({ locked: false, passwordHash: '' }),
      unlock: async (password) => {
        const hash = await sha256(password);
        const valid = hash === get().passwordHash;
        if (valid) set({ locked: false, passwordHash: '' });
        return valid;
      },
    }),
    {
      name: storageKey('lock-screen'),
      partialize: (state) => ({ locked: state.locked, passwordHash: state.passwordHash }),
      migrate: (persisted) => {
        const value = (persisted || {}) as { locked?: boolean; password?: string; passwordHash?: string };
        // Drop legacy plaintext password if present.
        return {
          locked: !!value.locked,
          passwordHash: value.passwordHash || '',
        };
      },
      version: 2,
    },
  ),
);
