const SCROLLING_CLASS = 'is-scrolling';
const HIDE_DELAY_MS = 900;
const timers = new WeakMap<Element, number>();

function markScrolling(target: Element) {
  target.classList.add(SCROLLING_CLASS);
  const previous = timers.get(target);
  if (previous) window.clearTimeout(previous);
  timers.set(
    target,
    window.setTimeout(() => {
      target.classList.remove(SCROLLING_CLASS);
      timers.delete(target);
    }, HIDE_DELAY_MS),
  );
}

function onScroll(event: Event) {
  const target = event.target;
  if (!(target instanceof Element)) return;
  if (target === document.documentElement || target === document.body) {
    markScrolling(document.documentElement);
    return;
  }
  markScrolling(target);
}

/** Show custom vertical scrollbar only while scrolling; hide when idle. */
export function initAutohideScrollbar() {
  document.addEventListener('scroll', onScroll, { capture: true, passive: true });
  return () => document.removeEventListener('scroll', onScroll, { capture: true });
}
