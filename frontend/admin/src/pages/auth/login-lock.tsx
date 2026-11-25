import type { CSSProperties } from 'react';
import { useTranslation } from 'react-i18next';
import { ACCOUNT_LOCK_DURATION_MS, LOGIN_LOCK_STATE_KEY } from '@/constants/auth';

export type LoginLockPayload = {
  attemptsRemaining?: number;
  captchaError?: 'required' | 'invalid' | 'expired' | string;
  captchaRequired?: boolean;
  failedCount?: number;
  kind?: 'cooldown' | 'lock' | string;
  lockedUntil?: string;
  retryAfterSeconds?: number;
};

const LOGIN_COOLDOWN_MS = 3000;

type PersistedLoginLock = {
  lockTotalMs: number;
  lockUntil: number;
  lockedUsername: string;
};

export function readPersistedLoginLock(): PersistedLoginLock | null {
  try {
    const raw = sessionStorage.getItem(LOGIN_LOCK_STATE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as PersistedLoginLock;
    if (
      typeof parsed.lockUntil !== 'number'
      || typeof parsed.lockTotalMs !== 'number'
      || typeof parsed.lockedUsername !== 'string'
      || parsed.lockTotalMs <= 0
      || parsed.lockUntil <= Date.now()
    ) {
      sessionStorage.removeItem(LOGIN_LOCK_STATE_KEY);
      return null;
    }
    return parsed;
  } catch {
    sessionStorage.removeItem(LOGIN_LOCK_STATE_KEY);
    return null;
  }
}

export function writePersistedLoginLock(state: PersistedLoginLock | null) {
  if (!state || state.lockUntil <= Date.now()) {
    sessionStorage.removeItem(LOGIN_LOCK_STATE_KEY);
    return;
  }
  sessionStorage.setItem(LOGIN_LOCK_STATE_KEY, JSON.stringify(state));
}

/** Account lock bar spans the configured lock window; cooldown uses the flat 3s penalty. */
export function resolveLockTotalMs(
  payload: LoginLockPayload | undefined,
  configuredAccountLockMs: number,
): number {
  if (payload?.kind === 'lock') {
    return configuredAccountLockMs > 0 ? configuredAccountLockMs : ACCOUNT_LOCK_DURATION_MS;
  }
  return LOGIN_COOLDOWN_MS;
}

const LOCK_ACCENT_ROSE = '#e11d48';
const LOCK_ACCENT_AMBER = '#f59e0b';
const LOCK_ACCENT_GREEN = '#16a34a';
const LOCK_TINT_ROSE = '#fb7185';
const LOCK_TINT_AMBER = '#fcd34d';
const LOCK_TINT_GREEN = '#4ade80';

function hexToRgb(hex: string): [number, number, number] {
  const value = parseInt(hex.slice(1), 16);
  return [(value >> 16) & 255, (value >> 8) & 255, value & 255];
}

function mixHex(from: string, to: string, amount: number) {
  const t = clamp(amount, 0, 1);
  const [r1, g1, b1] = hexToRgb(from);
  const [r2, g2, b2] = hexToRgb(to);
  const r = Math.round(r1 + (r2 - r1) * t);
  const g = Math.round(g1 + (g2 - g1) * t);
  const b = Math.round(b1 + (b2 - b1) * t);
  return `rgb(${r}, ${g}, ${b})`;
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

export function loginLockAccentFromRatio(ratio: number) {
  const remaining = clamp(ratio, 0, 1);
  if (remaining >= 0.5) {
    return mixHex(LOCK_ACCENT_AMBER, LOCK_ACCENT_ROSE, (remaining - 0.5) / 0.5);
  }
  return mixHex(LOCK_ACCENT_GREEN, LOCK_ACCENT_AMBER, remaining / 0.5);
}

export function loginLockTintFromRatio(ratio: number) {
  const remaining = clamp(ratio, 0, 1);
  if (remaining >= 0.5) {
    return mixHex(LOCK_TINT_AMBER, LOCK_TINT_ROSE, (remaining - 0.5) / 0.5);
  }
  return mixHex(LOCK_TINT_GREEN, LOCK_TINT_AMBER, remaining / 0.5);
}

export function loginLockProgressScale(remaining: number, totalMs: number | null) {
  if (!totalMs || totalMs <= 0) return null;
  return clamp(remaining / totalMs, 0, 1);
}

export function resolveLockUntil(payload?: LoginLockPayload): number {
  if (payload?.kind === 'lock') {
    if (payload.lockedUntil) {
      const ms = Date.parse(payload.lockedUntil);
      if (!Number.isNaN(ms) && ms > Date.now()) return ms;
    }
    if (payload.retryAfterSeconds && payload.retryAfterSeconds > 0) {
      return Date.now() + payload.retryAfterSeconds * 1000;
    }
  }
  if (payload?.lockedUntil) {
    const ms = Date.parse(payload.lockedUntil);
    if (!Number.isNaN(ms) && ms > Date.now()) return ms;
  }
  if (payload?.retryAfterSeconds && payload.retryAfterSeconds > 0) {
    return Date.now() + payload.retryAfterSeconds * 1000;
  }
  return Date.now() + LOGIN_COOLDOWN_MS;
}

function formatCountdown(ms: number) {
  const total = Math.max(0, Math.ceil(ms / 1000));
  const hours = Math.floor(total / 3600);
  const mins = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  if (hours > 0) {
    return `${hours}:${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
  }
  return `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
}

export function LockCountdownText({ remaining }: { remaining: number }) {
  const { t } = useTranslation();
  if (remaining <= 0) return null;
  const time = formatCountdown(remaining);
  const [before, after] = t('auth.lockRetryIn', { time: '[[time]]' }).split('[[time]]');
  return (
    <>
      {before}
      <strong className="auth-lock-time">{time}</strong>
      {after}
    </>
  );
}

export function LoginLockProgressBar({ remaining, totalMs }: { remaining: number; totalMs: number | null }) {
  const progress = loginLockProgressScale(remaining, totalMs);
  if (progress === null) return null;
  const accent = loginLockAccentFromRatio(progress);
  const tint = loginLockTintFromRatio(progress);
  return (
    <span
      aria-hidden
      className="auth-login-lock-progress"
      style={{
        transform: `scaleX(${progress})`,
        background: `linear-gradient(90deg, color-mix(in srgb, ${tint} 55%, transparent), ${accent})`,
      }}
    />
  );
}

export function loginLockSurfaceStyle(ratio: number | null): CSSProperties | undefined {
  if (ratio === null) return undefined;
  return {
    '--auth-lock-accent': loginLockAccentFromRatio(ratio),
    '--auth-lock-tint': loginLockTintFromRatio(ratio),
  } as CSSProperties;
}

export { clamp };