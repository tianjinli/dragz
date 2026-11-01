import { applyThemeCssVariables } from '@/utils/theme-css';
import { cssColorToHex, resolveThemePrimaryColor } from '@/stores/preferences';
import type { Preferences } from '@/types';

type ViewTransition = { ready: Promise<void>; skipTransition: () => void };

/** Circular view-transition when switching light/dark (Vben-style). */
export function runThemeViewTransition(event: { clientX: number; clientY: number }, apply: () => void) {
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const doc = document as Document & {
    startViewTransition?: (callback: () => void | Promise<void>) => ViewTransition;
  };

  if (!doc.startViewTransition || reducedMotion) {
    apply();
    return;
  }

  const x = event.clientX;
  const y = event.clientY;
  const goingDark = document.documentElement.dataset.theme !== 'dark';
  const endRadius = Math.hypot(Math.max(x, innerWidth - x), Math.max(y, innerHeight - y));
  const transition = doc.startViewTransition(() => {
    apply();
  });

  void transition.ready.then(() => {
    const clipPath = [`circle(0px at ${x}px ${y}px)`, `circle(${endRadius}px at ${x}px ${y}px)`];
    /* 切深色：新层白色圆扩散；切浅色：旧层黑色圆收缩（反向） */
    const animation = document.documentElement.animate(
      { clipPath: goingDark ? clipPath : [...clipPath].reverse() },
      {
        duration: 450,
        easing: 'ease-in',
        pseudoElement: goingDark ? '::view-transition-new(root)' : '::view-transition-old(root)',
      },
    );
    animation.onfinish = () => transition.skipTransition();
  });
}

/** Apply CSS theme tokens inside the view-transition callback so the snapshot actually changes. */
export function applyPreferencesThemeNow(preferences: Preferences) {
  const primary = resolveThemePrimaryColor(preferences);
  applyThemeCssVariables(preferences, cssColorToHex(primary));
}
