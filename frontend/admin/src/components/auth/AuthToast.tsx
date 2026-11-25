import { useEffect, useState, type CSSProperties, type ReactNode } from 'react';
import { CheckCircle2, CircleAlert } from 'lucide-react';

type ToastKind = 'error' | 'success';
type ToastPhase = 'enter' | 'leave';

type ToastState = {
  id: number;
  kind: ToastKind;
  text: string;
  phase: ToastPhase;
} | null;

/** Keep in sync with `--auth-toast-hold` / progress animation in auth.css */
export const AUTH_TOAST_HOLD_MS = 3000;
const LEAVE_MS = 260;

let seq = 0;
let current: ToastState = null;
let hideTimer: number | null = null;
const listeners = new Set<(state: ToastState) => void>();

function publish(next: ToastState) {
  current = next;
  listeners.forEach((listener) => listener(next));
}

function clearTimer() {
  if (hideTimer === null) return;
  window.clearTimeout(hideTimer);
  hideTimer = null;
}

function scheduleLeave(id: number) {
  clearTimer();
  hideTimer = window.setTimeout(() => {
    if (!current || current.id !== id) return;
    publish({ ...current, phase: 'leave' });
    hideTimer = window.setTimeout(() => {
      hideTimer = null;
      if (current?.id === id) publish(null);
    }, LEAVE_MS);
  }, AUTH_TOAST_HOLD_MS);
}

function show(kind: ToastKind, text: string) {
  const trimmed = text.trim();
  if (!trimmed) return;
  clearTimer();
  const id = ++seq;
  publish({ id, kind, text: trimmed, phase: 'enter' });
  scheduleLeave(id);
}

/** Imperative auth-page toast: one bar at a time, no antd message. */
export const authToast = {
  error(text: string) {
    show('error', text);
  },
  success(text: string) {
    show('success', text);
  },
  clear() {
    clearTimer();
    publish(null);
  },
};

type AuthToastViewProps = {
  kind?: ToastKind;
  phase?: ToastPhase;
  text: string;
  holdMs?: number;
  /** Skip enter/leave animation (e.g. static preview). */
  persistent?: boolean;
};

/** Shared opaque bar used by ephemeral toast and the lock overlay. */
export function AuthToastView({
  kind = 'error',
  phase = 'enter',
  text,
  holdMs = AUTH_TOAST_HOLD_MS,
  persistent = false,
}: AuthToastViewProps) {
  const Icon = kind === 'success' ? CheckCircle2 : CircleAlert;
  const className = persistent
    ? `auth-toast is-${kind} is-persistent`
    : `auth-toast is-${kind} is-${phase}`;

  return (
    <div
      className={className}
      style={{ '--auth-toast-hold': `${holdMs}ms` } as CSSProperties}
    >
      <span aria-hidden className="auth-toast-accent" />
      <span aria-hidden className="auth-toast-icon-wrap">
        <Icon className="auth-toast-icon" size={15} strokeWidth={2.4} />
      </span>
      <span className="auth-toast-text">{text}</span>
      {!persistent ? (
        <span aria-hidden className="auth-toast-progress" />
      ) : null}
    </div>
  );
}

/** Login slider / submit anchor: toast overlays this wrapper, not the whole form row. */
export function AuthLoginActionSurface({ children }: { children: ReactNode }) {
  return (
    <div className="auth-login-action-surface">
      {children}
      <AuthToastHost />
    </div>
  );
}

/** Renders over the login action (slider / submit). Mount inside `.auth-login-action-surface`. */
export function AuthToastHost() {
  const [toast, setToast] = useState<ToastState>(current);

  useEffect(() => {
    listeners.add(setToast);
    setToast(current);
    return () => {
      // Only detach — never clear the shared toast here. Clearing on unmount
      // wiped 423/lock messages when React remounted the host (e.g. Strict Mode
      // or swapping slider ↔ submit).
      listeners.delete(setToast);
    };
  }, []);

  if (!toast) return null;

  return (
    <div className="auth-toast-layer" aria-live="polite" role="status">
      <AuthToastView
        key={toast.id}
        kind={toast.kind}
        phase={toast.phase}
        text={toast.text}
      />
    </div>
  );
}
