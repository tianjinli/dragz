import { usePreferencesStore } from '@/stores/preferences';

export function formatInTimezone(
  value: string | number | Date,
  options: Intl.DateTimeFormatOptions = {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  },
) {
  const timezone = usePreferencesStore.getState().preferences.app.timezone || 'Asia/Shanghai';
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  try {
    return new Intl.DateTimeFormat(undefined, { ...options, timeZone: timezone }).format(date);
  } catch {
    return date.toLocaleString();
  }
}

/** True when a column dataIndex looks like a timestamp field (…At / …Time / …Date). */
export function isDateTimeField(dataIndex: unknown): boolean {
  if (dataIndex == null) return false;
  const key = Array.isArray(dataIndex) ? dataIndex[dataIndex.length - 1] : dataIndex;
  return typeof key === 'string' && /(?:At|Time|Date)$/.test(key);
}

export function formatDateTime(value: string | number | Date | null | undefined) {
  if (value == null || value === '') return '—';
  const timezone = usePreferencesStore.getState().preferences.app.timezone || 'Asia/Shanghai';
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  try {
    const parts = new Intl.DateTimeFormat('en-CA', {
      timeZone: timezone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    }).formatToParts(date);
    const get = (type: Intl.DateTimeFormatPartTypes) =>
      parts.find((part) => part.type === type)?.value || '';
    return `${get('year')}-${get('month')}-${get('day')} ${get('hour')}:${get('minute')}:${get('second')}`;
  } catch {
    return date.toLocaleString();
  }
}
