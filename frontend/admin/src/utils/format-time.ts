import dayjs from 'dayjs';
import { usePreferencesStore } from '@/stores/preferences';
import {
  DEFAULT_DATE_TIME_FORMAT,
  getDateTimeFormat,
} from '@/stores/system-config';

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

/**
 * Format a timestamp with the system config `system.dateTimeFormat`.
 * Empty config falls back to `YYYY-MM-dd HH:mm:ss`, so ISO values like
 * `2026-11-19T18:19:01.2+08:00` render as `2026-11-19 18:19:01`.
 */
export function formatDateTime(value: string | number | Date | null | undefined) {
  if (value == null || value === '') return '—';
  const timezone = usePreferencesStore.getState().preferences.app.timezone || 'Asia/Shanghai';
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  const pattern = getDateTimeFormat() || DEFAULT_DATE_TIME_FORMAT;
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
    const tokens: Record<string, string> = {
      YYYY: get('year'),
      MM: get('month'),
      DD: get('day'),
      dd: get('day'),
      HH: get('hour'),
      mm: get('minute'),
      ss: get('second'),
    };
    // Prefer explicit tokens so ISO input always becomes the configured display.
    // Accept both DD and dd for day-of-month (admin config convenience).
    if (/YYYY|MM|DD|dd|HH|mm|ss/.test(pattern)) {
      return pattern.replace(/YYYY|MM|DD|dd|HH|mm|ss/g, (token) => tokens[token] || token);
    }
    return dayjs(date).format(pattern);
  } catch {
    return date.toLocaleString();
  }
}
