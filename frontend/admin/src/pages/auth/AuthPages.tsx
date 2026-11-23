import { useCallback, useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type PointerEvent as ReactPointerEvent } from 'react';
import { Alert, App, Button, Checkbox, Divider, Form, Input, Modal } from 'antd';
import type { AxiosError } from 'axios';
import { ArrowRight, CheckCircle2, Loader2, MoveRight, RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import {
  exchangeOAuthApi,
  getTenantContextApi,
  issueCaptchaApi,
  loginChallengeApi,
  publicAuthProvidersApi,
  startOAuthApi,
} from '@/api/auth';
import { getSignupStatus } from '@/api/lifecycle';
import { REMEMBER_ME_KEY } from '@/constants/auth';
import { AuthLayout, AuthTitle } from '@/layout/AuthLayout';
import { useAccessStore } from '@/stores/access';
import { useAuthStore } from '@/stores/auth';
import { usePreferencesStore } from '@/stores/preferences';
import type { ApiEnvelope } from '@/types';
import { safeRedirectPath } from '@/utils/redirect';

type OAuthProvider = { id: number; name: string };

type SlideCaptcha = {
  captchaId: string;
  imageBase64: string;
  pieceBase64: string;
  width: number;
  height: number;
  pieceSize: number;
  pieceTop: number;
};

type SlideFeedback = 'idle' | 'verifying' | 'success' | 'error';

type LoginLockPayload = {
  attemptsRemaining?: number;
  captchaError?: 'required' | 'invalid' | 'expired' | string;
  captchaRequired?: boolean;
  failedCount?: number;
  kind?: 'cooldown' | 'lock' | string;
  lockedUntil?: string;
  retryAfterSeconds?: number;
};

const LOGIN_COOLDOWN_MS = 3000;
const SLIDE_TRACK_WIDTH = 300;
const SLIDE_HANDLE_WIDTH = 40;
const SLIDE_KEYBOARD_STEP = 8;
const SLIDE_KEYBOARD_BIG_STEP = 24;
const VERIFY_PASS_DELAY_MS = 900;

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function resolveLockUntil(payload?: LoginLockPayload): number {
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
  if (hours > 0) return `${hours}:${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
  return `${mins}:${String(secs).padStart(2, '0')}`;
}

export function LoginPage() {
  const { t } = useTranslation();
  const { message: antMessage } = App.useApp();
  const navigate = useNavigate();
  const location = useLocation();
  const { login, loading } = useAuthStore();
  const authPageLayout = usePreferencesStore((state) => state.preferences.app.authPageLayout);
  const panelSide = authPageLayout.replace('panel-', '');
  const [form] = Form.useForm();
  const currentUsername = Form.useWatch<string>('username', form);
  const [remember, setRemember] = useState(false);
  const [providers, setProviders] = useState<OAuthProvider[]>([]);
  const [startingId, setStartingId] = useState<number>();
  const [lockUntil, setLockUntil] = useState<number | null>(null);
  const [lockedUsername, setLockedUsername] = useState<string | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const [captchaRequired, setCaptchaRequired] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [captcha, setCaptcha] = useState<SlideCaptcha | null>(null);
  const [captchaExpiresAt, setCaptchaExpiresAt] = useState(0);
  const [captchaLoading, setCaptchaLoading] = useState(false);
  const [expired, setExpired] = useState(false);
  const [slideX, setSlideX] = useState(0);
  const [draggingSlide, setDraggingSlide] = useState(false);
  const [feedback, setFeedback] = useState<SlideFeedback>('idle');
  const [tenantName, setTenantName] = useState('');
  const [signupEnabled, setSignupEnabled] = useState(false);
  const [, forceScaleRefresh] = useState(0);

  const dragStart = useRef<{ clientX: number; base: number } | null>(null);
  const trackRef = useRef<HTMLDivElement>(null);
  const handleRef = useRef<HTMLButtonElement>(null);
  const focusPendingRef = useRef(false);
  const credentialsRef = useRef<{ password: string; username: string } | null>(null);
  const submittingRef = useRef(false);
  const committingRef = useRef(false);
  const refreshSeqRef = useRef(0);
  const mountedRef = useRef(true);
  const successTimerRef = useRef<number | null>(null);
  const errorRefreshTimerRef = useRef<number | null>(null);

  // Lockout is per-account on the backend (failedCount is tracked by username).
  // Only block the account that was actually locked; switching to another
  // username must not inherit the previous account's lock/cooldown.
  const lockedForCurrentAccount = lockedUsername === null
    || (currentUsername ?? '').trim().toLowerCase() === lockedUsername.trim().toLowerCase();
  const locked = lockUntil !== null && lockUntil > now && lockedForCurrentAccount;
  const lockRemaining = locked && lockUntil ? Math.max(0, lockUntil - now) : 0;
  const verifying = modalOpen && captchaRequired;
  const lockedRef = useRef(locked);
  lockedRef.current = locked;
  const loginLabel = locked && lockRemaining > 0
    ? `${formatCountdown(lockRemaining)}`
    : t('login');

  const getScale = useCallback(() => {
    const renderedWidth = trackRef.current?.clientWidth || SLIDE_TRACK_WIDTH;
    const logicalWidth = captcha?.width || SLIDE_TRACK_WIDTH;
    // The stage is stretched to the field width, so allow up-scaling to keep
    // the piece aligned with the (logically 300px wide) puzzle image.
    return clamp(renderedWidth / logicalWidth, 0.1, 4);
  }, [captcha]);

  const scale = getScale();
  // The captcha handle is a fixed round grip (verify5-style), while the
  // password-only login slide keeps the wider button-shaped handle.
  const handleWidth = captchaRequired && captcha ? 38 : SLIDE_HANDLE_WIDTH;
  const slideMax = Math.max(0, Math.round(((trackRef.current?.clientWidth || SLIDE_TRACK_WIDTH) - handleWidth) / scale * 10) / 10);

  const isExpired = useCallback(() => (
    captchaExpiresAt > 0 && Date.now() >= captchaExpiresAt
  ), [captchaExpiresAt]);

  const refreshCaptcha = useCallback(async (force = false): Promise<boolean> => {
    if (lockedRef.current && !force) return false;
    if (errorRefreshTimerRef.current !== null) {
      window.clearTimeout(errorRefreshTimerRef.current);
      errorRefreshTimerRef.current = null;
    }
    const seq = ++refreshSeqRef.current;
    setCaptchaLoading(true);
    setSlideX(0);
    setDraggingSlide(false);
    setFeedback('idle');
    setExpired(false);
    dragStart.current = null;
    try {
      const result = await issueCaptchaApi();
      if (seq !== refreshSeqRef.current || !mountedRef.current) return false;
      setCaptcha({
        captchaId: result.captchaId,
        imageBase64: result.imageBase64,
        pieceBase64: result.pieceBase64,
        width: result.width,
        height: result.height,
        pieceSize: result.pieceSize,
        pieceTop: result.pieceTop,
      });
      setCaptchaExpiresAt(Date.now() + Math.max(1, result.expiresIn) * 1000);
      return true;
    } catch {
      if (seq !== refreshSeqRef.current || !mountedRef.current) return false;
      setCaptcha(null);
      setCaptchaExpiresAt(0);
      setFeedback('error');
      return false;
    } finally {
      if (seq === refreshSeqRef.current && mountedRef.current) setCaptchaLoading(false);
    }
  }, []);

  // Open the verification modal (the captcha itself is issued lazily here so a
  // normal sign-in never pays for a puzzle the backend did not ask for).
  const openCaptchaModal = useCallback(async (force = true): Promise<boolean> => {
    setCaptchaRequired(true);
    setModalOpen(true);
    return refreshCaptcha(force);
  }, [refreshCaptcha]);

  const closeCaptchaModal = useCallback(() => {
    if (submittingRef.current || feedback === 'verifying' || feedback === 'success') return;
    if (errorRefreshTimerRef.current !== null) {
      window.clearTimeout(errorRefreshTimerRef.current);
      errorRefreshTimerRef.current = null;
    }
    setModalOpen(false);
    setCaptcha(null);
    setCaptchaExpiresAt(0);
    setSlideX(0);
    setFeedback('idle');
    setExpired(false);
    credentialsRef.current = null;
  }, [feedback]);

  const showLoginError = useCallback((message: string) => {
    if (!message) return;
    antMessage.error(message);
  }, [antMessage]);

  const markExpired = useCallback(() => {
    setExpired(true);
    setSlideX(0);
    setFeedback('idle');
  }, []);

  const submit = async (values: { password: string; username: string }, slideOverride?: number) => {
    if (submittingRef.current || lockedRef.current) return;
    const effectiveX = slideOverride ?? slideX;
    const slideReady = captchaRequired ? !!captcha && effectiveX > 0 : effectiveX >= slideMax - 2;
    if (!slideReady) {
      showLoginError(t(captchaRequired ? 'auth.captchaRequired' : 'auth.slideToLogin'));
      setSlideX(0);
      setFeedback('idle');
      return;
    }
    if (captchaRequired && (expired || isExpired())) {
      markExpired();
      return;
    }

    submittingRef.current = true;
    setFeedback('verifying');
    try {
      const home = await login({
        username: values.username,
        password: values.password,
        deviceName: navigator.userAgent,
        captchaId: captchaRequired ? captcha?.captchaId : undefined,
        captchaCode: captchaRequired ? String(Math.round(effectiveX)) : undefined,
      });
      localStorage.setItem(REMEMBER_ME_KEY, remember ? values.username : '');
      const user = useAccessStore.getState().userInfo;
      if (user?.realName) {
        antMessage.success(t('auth.loginSuccessDesc', { name: user.realName }));
      } else {
        antMessage.success(t('auth.loginSuccess'));
      }

      const target = safeRedirectPath(new URLSearchParams(location.search).get('redirect'), home);
      if (captchaRequired) {
        // Keep the modal up so the user actually sees "Verification passed",
        // then hand off to the app. Non-captcha sign-in keeps the instant jump.
        setFeedback('success');
        successTimerRef.current = window.setTimeout(() => {
          if (mountedRef.current) navigate(target, { replace: true });
        }, VERIFY_PASS_DELAY_MS);
      } else {
        setFeedback('success');
        navigate(target, { replace: true });
      }
    } catch (error) {
      const ax = error as AxiosError<ApiEnvelope<LoginLockPayload>>;
      const envelope = ax.response?.data;
      const payload = envelope?.data;
      const isCaptchaFailure = envelope?.code === 40002;
      const captchaReason = isCaptchaFailure ? payload?.captchaError : undefined;
      const until = resolveLockUntil(payload);
      // 40002 covers three cases. A wrong answer shows inline under the image;
      // an expired puzzle gets the refresh mask; neither raises a toast.
      if (captchaReason === 'required') {
        showLoginError(t('auth.captchaRequired'));
      } else if (captchaReason !== 'expired' && captchaReason !== 'invalid') {
        showLoginError(envelope?.message || t('auth.loginFailed'));
      }
      setSlideX(0);
      setLockUntil(until);
      setLockedUsername(values.username);
      setNow(Date.now());
      if (captchaRequired && !isCaptchaFailure) {
        // Password/account errors belong to the form view; close the modal and
        // keep what was typed so the user can correct it.
        setModalOpen(false);
        setCaptcha(null);
        setCaptchaExpiresAt(0);
        setFeedback('idle');
        credentialsRef.current = null;
        return;
      }
      if (captchaReason === 'expired') {
        setExpired(true);
        setFeedback('idle');
      } else {
        setFeedback('error');
        if (payload?.captchaRequired || captchaRequired) {
          if (modalOpen) {
            if (captchaReason === 'invalid') {
              // Let the inline failure text read before swapping the puzzle.
              errorRefreshTimerRef.current = window.setTimeout(() => { void refreshCaptcha(true); }, 1200);
            } else {
              await refreshCaptcha();
            }
          } else {
            await openCaptchaModal(true);
          }
        }
      }
    } finally {
      submittingRef.current = false;
    }
  };

  /**
   * The modal only shows up once the user actually asks to verify, so a normal
   * sign-in never pays for a verification they did not ask for. The backend
   * still challenges every attempt when security.captcha.enabled is on.
   */
  const handleFormSubmit = (values: { password: string; username: string }) => {
    if (submittingRef.current || lockedRef.current) return;
    credentialsRef.current = values;
    if (!captchaRequired) {
      void submit(values);
      return;
    }
    focusPendingRef.current = true;
    void openCaptchaModal();
  };

  const showFieldErrorsAtTop = (errorFields?: { errors: string[] }[]) => {
    const first = errorFields?.find((field) => field.errors.length > 0)?.errors[0];
    showLoginError(first || t('auth.loginFailed'));
  };

  const handleSubmitClick = () => {
    if (submittingRef.current || lockedRef.current || captchaLoading || modalOpen) return;
    void form.validateFields()
      .then((values) => handleFormSubmit(values as { password: string; username: string }))
      .catch((error: { errorFields?: { errors: string[] }[] }) => {
        showFieldErrorsAtTop(error.errorFields);
      });
  };

  const commitSlide = (finalX: number) => {
    if (committingRef.current || submittingRef.current || captchaLoading || locked || loading || feedback === 'success') {
      setSlideX(0);
      return;
    }
    if (captchaRequired) {
      if (!captcha) return;
      if (expired || isExpired()) {
        markExpired();
        return;
      }
      if (finalX <= 0) {
        setSlideX(0);
        return;
      }
    } else if (finalX < slideMax - 2) {
      setSlideX(0);
      return;
    }

    committingRef.current = true;
    setSlideX(finalX);
    setFeedback('verifying');
    const finish = () => {
      committingRef.current = false;
    };
    // Inside the modal the form is behind an overlay, so submit the credentials
    // kept from the moment "Start verification" was pressed.
    const pending = credentialsRef.current;
    if (pending) {
      void submit(pending, finalX).finally(finish);
      return;
    }
    void form.validateFields()
      .then((values) => submit(values as { password: string; username: string }, finalX))
      .catch((error: { errorFields?: { errors: string[] }[] }) => {
        showFieldErrorsAtTop(error.errorFields);
        setFeedback('idle');
        setSlideX(0);
      })
      .finally(finish);
  };

  const beginSlide = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (captchaLoading || locked || loading || submittingRef.current || feedback === 'success' || (captchaRequired && !captcha)) return;
    if (captchaRequired && (expired || isExpired())) {
      markExpired();
      return;
    }
    event.currentTarget.setPointerCapture(event.pointerId);
    dragStart.current = { clientX: event.clientX, base: slideX };
    setDraggingSlide(true);
    setFeedback('idle');
  };

  const moveSlide = (event: ReactPointerEvent<HTMLButtonElement>) => {
    const start = dragStart.current;
    if (!start || (captchaRequired && !captcha)) return;
    const delta = (event.clientX - start.clientX) / getScale();
    setSlideX(clamp(start.base + delta, 0, slideMax));
  };

  const endSlide = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    const start = dragStart.current;
    if (!start) {
      setSlideX(0);
      return;
    }
    const delta = (event.clientX - start.clientX) / getScale();
    const finalX = clamp(start.base + delta, 0, slideMax);
    dragStart.current = null;
    setDraggingSlide(false);
    commitSlide(finalX);
  };

  const cancelSlide = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    dragStart.current = null;
    setDraggingSlide(false);
    setSlideX(0);
    setFeedback('idle');
  };

  const handleSlideKeyDown = (event: ReactKeyboardEvent<HTMLButtonElement>) => {
    if (captchaLoading || locked || loading || submittingRef.current || feedback === 'success' || (captchaRequired && !captcha)) return;
    if (captchaRequired && (expired || isExpired())) {
      markExpired();
      return;
    }
    const step = event.shiftKey ? SLIDE_KEYBOARD_BIG_STEP : SLIDE_KEYBOARD_STEP;
    let next = slideX;
    if (event.key === 'ArrowRight') next += step;
    else if (event.key === 'ArrowLeft') next -= step;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = slideMax;
    else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      commitSlide(slideX);
      return;
    } else return;
    event.preventDefault();
    setFeedback('idle');
    setSlideX(clamp(next, 0, slideMax));
  };

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      if (successTimerRef.current !== null) window.clearTimeout(successTimerRef.current);
      if (errorRefreshTimerRef.current !== null) window.clearTimeout(errorRefreshTimerRef.current);
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    void getTenantContextApi()
      .then((result) => {
        if (!cancelled && result.resolved && result.name) setTenantName(result.name);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    const element = trackRef.current;
    if (!element || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(() => forceScaleRefresh((value) => value + 1));
    observer.observe(element);
    return () => observer.disconnect();
  }, [modalOpen]);

  // Hand focus to the freshly revealed slider so keyboard users can finish the
  // challenge without tabbing through the form again.
  useEffect(() => {
    if (!focusPendingRef.current || !modalOpen || !captcha) return;
    focusPendingRef.current = false;
    // The modal portal mounts after this render, so focus after the next frame.
    const frame = window.requestAnimationFrame(() => handleRef.current?.focus());
    return () => window.cancelAnimationFrame(frame);
  }, [captcha, modalOpen]);

  useEffect(() => {
    const saved = localStorage.getItem(REMEMBER_ME_KEY) || '';
    if (saved) {
      form.setFieldValue('username', saved);
      setRemember(true);
    }
    // Only learn whether this deployment requires a captcha. The puzzle itself is
    // fetched lazily when the user presses "Start verification", so the page opens clean.
    void loginChallengeApi(saved)
      .then((result) => setCaptchaRequired(result.captchaRequired))
      .catch(() => undefined);
  }, [form]);

  useEffect(() => {
    let cancelled = false;
    void getSignupStatus()
      .then((status) => {
        if (!cancelled) setSignupEnabled(status.enabled);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    void publicAuthProvidersApi()
      .then((items) => setProviders(items.filter((item): item is OAuthProvider => typeof item.id === 'number' && typeof item.name === 'string')))
      .catch(() => setProviders([]));
  }, []);

  useEffect(() => {
    if (lockUntil === null || lockUntil <= now) return;
    const timer = window.setInterval(() => setNow(Date.now()), 250);
    return () => window.clearInterval(timer);
  }, [lockUntil, now]);

  useEffect(() => {
    if (lockUntil === null || lockUntil > now) return;
    setLockUntil(null);
    setLockedUsername(null);
    setFeedback('idle');
    if (verifying && !captcha) void refreshCaptcha();
  }, [captcha, verifying, lockUntil, now, refreshCaptcha]);

  // Flip to the expired mask the moment the puzzle's TTL elapses, without
  // waiting for the user to touch the slider.
  useEffect(() => {
    if (!captcha || captchaExpiresAt <= 0) return;
    const delay = captchaExpiresAt - Date.now();
    if (delay <= 0) {
      setExpired(true);
      return;
    }
    const timer = window.setTimeout(() => setExpired(true), delay);
    return () => window.clearTimeout(timer);
  }, [captcha, captchaExpiresAt]);

  useEffect(() => {
    if (feedback !== 'error') return;
    const timer = window.setTimeout(() => setFeedback('idle'), 1200);
    return () => window.clearTimeout(timer);
  }, [feedback]);

  const startOAuth = async (provider: OAuthProvider) => {
    setStartingId(provider.id);
    try {
      const result = await startOAuthApi(provider.id);
      window.location.assign(result.authorizationUrl);
    } finally {
      setStartingId(undefined);
    }
  };

  const feedbackMessage = feedback === 'verifying'
    ? t('auth.captchaVerifying')
    : feedback === 'error' ? t('auth.captchaFailed') : '';
  // The track itself is the login button when captcha is off. Dimming only the
  // handle leaves the rest of the button looking live, so mark the whole track.
  const slideDisabled = (captchaRequired && !captcha) || captchaLoading || locked
    || loading || submittingRef.current || feedback === 'verifying' || feedback === 'success'
    || feedback === 'error' || expired;

  const slideCaptcha = (
    <div className={`auth-slide-captcha is-${feedback}${captchaRequired ? ' is-captcha' : ''}`}>
      <div className="auth-slide-captcha-board">
        {captchaRequired && (
          captcha ? (
            <div
              className="auth-slide-captcha-stage"
              style={{ aspectRatio: `${captcha.width} / ${captcha.height}` }}
            >
              <img alt={t('auth.captcha')} className="auth-slide-captcha-bg" draggable={false} src={captcha.imageBase64} />
              <img
                alt=""
                className="auth-slide-captcha-piece"
                draggable={false}
                src={captcha.pieceBase64}
                style={{
                  left: slideX * scale,
                  top: captcha.pieceTop * scale,
                  width: captcha.pieceSize * scale,
                  height: captcha.pieceSize * scale,
                }}
              />
              {feedback === 'verifying' || feedback === 'success' ? (
                <div className={`auth-slide-captcha-status is-${feedback}`} role="status">
                  {feedback === 'success'
                    ? <CheckCircle2 size={30} strokeWidth={2} />
                    : <Loader2 className="auth-slide-captcha-status-spin" size={30} strokeWidth={2} />}
                  <span>{feedback === 'success' ? t('auth.verifyPassed') : t('auth.captchaVerifying')}</span>
                </div>
              ) : null}
              <div className="auth-slide-captcha-actions">
                <button
                  aria-label={t('auth.captchaRefreshLabel')}
                  className="auth-slide-captcha-action"
                  disabled={captchaLoading || locked || feedback === 'verifying' || feedback === 'success'}
                  title={t('auth.captchaRefreshLabel')}
                  type="button"
                  onClick={() => void refreshCaptcha()}
                >
                  <RefreshCw size={14} />
                </button>
              </div>
            </div>
          ) : (
            <div className="auth-slide-captcha-loading">{t('auth.captchaLoading')}</div>
          )
        )}
        <div className={`auth-slide-captcha-track${slideDisabled ? ' is-disabled' : ''}`} ref={trackRef}>
          <div className="auth-slide-captcha-fill" style={{ width: slideX * scale + handleWidth / 2 }} />
          <span className={`auth-slide-captcha-hint${captchaRequired && feedback === 'error' ? ' is-error' : ''}${captchaRequired && feedback === 'idle' && !expired ? ' is-shimmer' : ''}`}>
            {captchaRequired
              ? (feedback === 'error' ? t('auth.captchaFailed') : t('auth.captchaPlaceholder'))
              : (locked && lockRemaining > 0 ? loginLabel : t('auth.slideToLogin'))}
          </span>
          <button
            ref={handleRef}
            aria-label={captchaRequired ? t('auth.captcha') : loginLabel}
            aria-valuemax={slideMax}
            aria-valuemin={0}
            aria-valuenow={Math.round(slideX)}
            className={`auth-slide-captcha-handle${draggingSlide ? ' is-dragging' : ''}${loading ? ' is-busy' : ''}`}
            disabled={slideDisabled}
            role="slider"
            style={{ left: slideX * scale, width: handleWidth }}
            tabIndex={0}
            type="button"
            onKeyDown={handleSlideKeyDown}
            onPointerCancel={cancelSlide}
            onPointerDown={beginSlide}
            onPointerMove={moveSlide}
            onPointerUp={endSlide}
          >
            {loading
              ? <Loader2 className="auth-slide-captcha-status-spin" size={16} />
              : captchaRequired
                ? <ArrowRight size={18} strokeWidth={2.6} />
                : <MoveRight size={18} />}
          </button>
        </div>
        {captchaRequired && captcha && expired ? (
          <button
            aria-label={t('auth.captchaExpired')}
            className="auth-slide-captcha-expired"
            onClick={() => void refreshCaptcha(true)}
            title={t('auth.captchaExpired')}
            type="button"
          >
            <RefreshCw size={30} strokeWidth={1.8} />
            <span className="auth-slide-captcha-expired-title">{t('auth.captchaExpiredTitle')}</span>
            <span className="auth-slide-captcha-expired-hint">{t('auth.captchaExpiredHint')}</span>
          </button>
        ) : null}
      </div>
      <span aria-live="polite" className="auth-slide-captcha-sr">{feedbackMessage}</span>
    </div>
  );

  return (
    <AuthLayout>
      <AuthTitle />
      {tenantName ? (
        <Alert showIcon style={{ marginBottom: 16 }} title={t('auth.tenantBanner', { name: tenantName })} type="info" />
      ) : null}
      <Form
        className="auth-login-form"
        form={form}
        layout="vertical"
        onFinish={(values) => handleFormSubmit(values as { password: string; username: string })}
        onFinishFailed={({ errorFields }) => showFieldErrorsAtTop(errorFields)}
        requiredMark={false}
      >
        <Form.Item name="username" rules={[{ required: true, message: t('auth.usernameRequired') }]}>
          <Input autoComplete="username" placeholder={t('auth.usernamePlaceholder')} size="large" />
        </Form.Item>
        <Form.Item name="password" rules={[{ required: true, message: t('auth.passwordRequired') }]}>
          <Input.Password autoComplete="current-password" placeholder={t('auth.passwordPlaceholder')} size="large" />
        </Form.Item>
        <div className="auth-remember-row">
          <Checkbox checked={remember} onChange={(event) => setRemember(event.target.checked)}>
            {t('auth.rememberMe')}
          </Checkbox>
        </div>
        {!captchaRequired ? slideCaptcha : null}
        {captchaRequired ? (
          <Button
            block
            className="auth-login-submit"
            data-testid="login-submit"
            disabled={locked}
            htmlType="button"
            loading={captchaLoading || loading}
            onClick={handleSubmitClick}
            size="large"
            type="primary"
          >
            {loginLabel}
          </Button>
        ) : null}
      </Form>
      {providers.length > 0 && !verifying ? (
        <div className="auth-oauth-block">
          <Divider plain>{t('auth.oauthDivider')}</Divider>
          <div className="auth-oauth-list">
            {providers.map((provider) => (
              <Button block key={provider.id} loading={startingId === provider.id} onClick={() => void startOAuth(provider)}>
                {provider.name}
              </Button>
            ))}
          </div>
        </div>
      ) : null}
      {signupEnabled && !verifying ? (
        <div className="auth-links">
          <span>{t('signup.noAccount')}</span>
          <Link to="/auth/register">{t('signup.submit')}</Link>
        </div>
      ) : null}
      {modalOpen ? (
        <Modal
          className="auth-captcha-modal"
          rootClassName={`auth-captcha-root auth-captcha-root--${panelSide}`}
          open
          title={t('auth.captchaTitle')}
          centered
          footer={null}
          width={360}
          closable={feedback !== 'verifying' && feedback !== 'success'}
          mask={{ closable: feedback !== 'verifying' && feedback !== 'success' }}
          onCancel={closeCaptchaModal}
        >
          {slideCaptcha}
        </Modal>
      ) : null}
    </AuthLayout>
  );
}

export function OAuthCallbackPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [error, setError] = useState('');
  useEffect(() => {
    const code = new URLSearchParams(location.search).get('code');
    if (!code) {
      setError(t('auth.missingCode'));
      return;
    }
    void exchangeOAuthApi(code)
      .then(async (token) => {
        useAccessStore.getState().setAccessToken(token.accessToken);
        await useAuthStore.getState().loadAccess();
        navigate(useAccessStore.getState().userInfo?.homePath || '/analytics', { replace: true });
      })
      .catch(() => setError(t('auth.oauthFailed')));
  }, [location.search, navigate, t]);
  return (
    <AuthLayout>
      <AuthTitle subtitle={t('auth.oauthSubtitle')} title={t('auth.oauthTitle')} />
      {error ? <Alert type="error" title={error} /> : <Alert showIcon type="info" title={t('auth.oauthProcessing')} />}
    </AuthLayout>
  );
}
