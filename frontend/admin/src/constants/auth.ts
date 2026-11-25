export const AUTH_LOGO = '/favicon.svg';

export const REMEMBER_ME_KEY = `REMEMBER_ME_USERNAME_${typeof location !== 'undefined' ? location.hostname : 'localhost'}`;

/**
 * Fallback when GET /auth/login-challenge cannot be loaded.
 * Matches backend default for `security.login.lockMinutes` (账号锁定分钟数).
 *
 * Do not use login error `retryAfterSeconds` as the progress-bar denominator: on a
 * fresh lock it equals lockMinutes×60, but on later attempts while still locked it
 * is only the remaining seconds.
 */
export const ACCOUNT_LOCK_DURATION_MS = 30 * 60 * 1000;

export function accountLockDurationMs(lockMinutes: number) {
  const mins = lockMinutes > 0 ? lockMinutes : 30;
  return mins * 60 * 1000;
}

export const LOGIN_LOCK_STATE_KEY = `LOGIN_LOCK_STATE_${typeof location !== 'undefined' ? location.hostname : 'localhost'}`;
