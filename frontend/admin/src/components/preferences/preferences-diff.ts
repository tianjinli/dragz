import type { Preferences } from '@/types';

type JsonRecord = Record<string, unknown>;

function isRecord(value: unknown): value is JsonRecord {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function equal(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true;
  if (Array.isArray(left) && Array.isArray(right)) {
    return left.length === right.length && left.every((value, index) => equal(value, right[index]));
  }
  if (isRecord(left) && isRecord(right)) {
    const keys = Object.keys(left);
    return keys.length === Object.keys(right).length && keys.every((key) => equal(left[key], right[key]));
  }
  return false;
}

function difference(current: unknown, baseline: unknown): unknown {
  if (equal(current, baseline)) return undefined;
  if (!isRecord(current) || !isRecord(baseline)) return current;

  const result: JsonRecord = {};
  for (const [key, value] of Object.entries(current)) {
    const child = difference(value, baseline[key]);
    if (child !== undefined) result[key] = child;
  }
  return Object.keys(result).length ? result : undefined;
}

/** Returns only values which differ from the Golden Master defaults. */
export function getPreferenceDifference(current: Preferences, baseline: Preferences): Partial<Preferences> {
  return (difference(current, baseline) ?? {}) as Partial<Preferences>;
}

export function hasPreferenceDifference(current: Preferences, baseline: Preferences): boolean {
  return Object.keys(getPreferenceDifference(current, baseline)).length > 0;
}

function serializeValue(value: unknown, indent: number): string {
  const pad = '  '.repeat(indent);
  const next = '  '.repeat(indent + 1);
  if (typeof value === 'string') return JSON.stringify(value);
  if (typeof value === 'number' || typeof value === 'boolean' || value === null) return String(value);
  if (Array.isArray(value)) {
    if (!value.length) return '[]';
    return `[\n${value.map((item) => `${next}${serializeValue(item, indent + 1)},`).join('\n')}\n${pad}]`;
  }
  if (isRecord(value)) {
    const entries = Object.entries(value);
    if (!entries.length) return '{}';
    return `{\n${entries.map(([key, child]) => `${next}${/^[A-Za-z_$][\w$]*$/.test(key) ? key : JSON.stringify(key)}: ${serializeValue(child, indent + 1)},`).join('\n')}\n${pad}}`;
  }
  return 'undefined';
}

/** Emits a TypeScript object literal suitable for pasting into preferences overrides. */
export function serializePreferenceDifference(current: Preferences, baseline: Preferences): string {
  const diff = getPreferenceDifference(current, baseline);
  return `export const overrides = ${serializeValue(diff, 0)} as const;\n`;
}
