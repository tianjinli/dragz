import type { Preferences } from '@/types';
import { effectiveDark, resolveThemePrimaryColor } from '@/stores/preferences';

function relativeLuminance(hex: string) {
  const normalized = hex.replace('#', '');
  if (normalized.length < 6) return 0.5;
  const r = Number.parseInt(normalized.slice(0, 2), 16) / 255;
  const g = Number.parseInt(normalized.slice(2, 4), 16) / 255;
  const b = Number.parseInt(normalized.slice(4, 6), 16) / 255;
  const toLinear = (channel: number) => (channel <= 0.03928 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4);
  return 0.2126 * toLinear(r) + 0.7152 * toLinear(g) + 0.0722 * toLinear(b);
}

export function contrastForeground(hexOrCss: string) {
  const hex = hexOrCss.startsWith('#') ? hexOrCss : '#0066e6';
  return relativeLuminance(hex) > 0.45 ? '#09090b' : '#fafafa';
}

const NEUTRAL_PRESETS = new Set(['zinc', 'neutral', 'slate', 'gray']);

/** Theme radius is stored as rem units (`"0"` … `"1"`). Keep `0` (do not `|| 0.5`). */
export function parseThemeRadiusRem(value: string | number | undefined): number {
  const parsed = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(parsed) ? parsed : 0.5;
}

export function applyThemeCssVariables(preferences: Preferences, primaryHex: string) {
  const root = document.documentElement;
  const dark = effectiveDark(preferences);
  const primary = resolveThemePrimaryColor(preferences);
  const radius = parseThemeRadiusRem(preferences.theme.radius);

  root.dataset.theme = dark ? 'dark' : 'light';
  root.dataset.builtinTheme = preferences.theme.builtinType;
  root.dataset.radius = String(preferences.theme.radius);
  root.style.setProperty('--primary', primary);
  root.style.setProperty('--primary-foreground', contrastForeground(primaryHex));
  root.style.setProperty('--radius', `${radius}rem`);
  root.style.setProperty('--font-size-base', `${preferences.theme.fontSize}px`);
  root.style.setProperty('--menu-font-size', `calc(${preferences.theme.fontSize}px * 0.875)`);
  root.style.fontSize = `${preferences.theme.fontSize}px`;

  root.classList.toggle('invert-mode', preferences.app.colorWeakMode);
  root.classList.toggle('grayscale-mode', preferences.app.colorGrayMode);

  if (NEUTRAL_PRESETS.has(preferences.theme.builtinType)) {
    root.style.setProperty('--text', dark ? '#fafafa' : '#18181b');
    root.style.setProperty('--muted', dark ? '#a1a1aa' : '#71717a');
    root.style.setProperty('--border', dark ? '#27272a' : '#e4e4e7');
  } else {
    root.style.removeProperty('--text');
    root.style.removeProperty('--muted');
    root.style.removeProperty('--border');
  }
}

export function antdBorderRadius(preferences: Preferences) {
  return parseThemeRadiusRem(preferences.theme.radius) * 16;
}
