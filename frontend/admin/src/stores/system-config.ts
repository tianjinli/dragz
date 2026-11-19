import { create } from 'zustand';
import { listConfigsApi, type ResourceRecord } from '@/api/system';

/** dayjs-compatible tokens: YYYY MM DD HH mm ss */
export const DEFAULT_DATE_TIME_FORMAT = 'YYYY-MM-DD HH:mm:ss';

function configKey(row: ResourceRecord) {
  return String(row.configKey ?? row.key ?? '');
}

function configValue(row: ResourceRecord) {
  return String(row.value ?? row.configValue ?? '');
}

interface SystemConfigState {
  loaded: boolean;
  values: Record<string, string>;
  refresh: () => Promise<void>;
  get: (key: string, fallback?: string) => string;
}

export const useSystemConfigStore = create<SystemConfigState>((set, get) => ({
  loaded: false,
  values: {},
  refresh: async () => {
    try {
      const rows = await listConfigsApi();
      const values: Record<string, string> = {};
      for (const row of rows || []) {
        const key = configKey(row);
        if (key) values[key] = configValue(row);
      }
      set({ loaded: true, values });
    } catch {
      set({ loaded: true });
    }
  },
  get: (key, fallback = '') => {
    const value = get().values[key];
    if (value == null || value === '') return fallback;
    return value;
  },
}));

export function getDateTimeFormat() {
  const raw = useSystemConfigStore
    .getState()
    .get('system.dateTimeFormat', DEFAULT_DATE_TIME_FORMAT)
    .trim();
  return raw || DEFAULT_DATE_TIME_FORMAT;
}
