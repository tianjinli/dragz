import '@testing-library/jest-dom/vitest';
import { configure } from '@testing-library/react';
// Components translate through a shared i18next singleton; initialise it for
// every test file so t() returns real copy instead of raw keys.
import '@/i18n';
import { afterEach } from 'vitest';

// antd's Form debounce (useDebounce -> @rc-component/util useDelayState)
// schedules state updates on setTimeout(<=10ms) and never cancels the pending
// timer on unmount. When the last test of a file ends, vitest tears the jsdom
// environment down before such a timer fires, and react-dom then crashes with
// "ReferenceError: window is not defined" — an unhandled error that fails the
// run even though every test passed. Flushing 20ms here lets those timers
// resolve while window still exists.
afterEach(async () => {
  await new Promise((resolve) => setTimeout(resolve, 20));
});

// antd drawers and vxe tables render slowly under jsdom; the 1s default
// waitFor/findBy timeout flakes when the whole suite runs on a loaded machine.
configure({ asyncUtilTimeout: 5000 });

Object.defineProperty(window, 'matchMedia', { writable: true, value: (query: string) => ({ matches: false, media: query, onchange: null, addEventListener: () => undefined, removeEventListener: () => undefined, addListener: () => undefined, removeListener: () => undefined, dispatchEvent: () => false }) });
Object.defineProperty(navigator, 'clipboard', { value: { writeText: async () => undefined } });

const getComputedStyle = window.getComputedStyle.bind(window);
Object.defineProperty(window, 'getComputedStyle', {
  configurable: true,
  value: (element: Element, pseudoElement?: string | null) => getComputedStyle(element, pseudoElement ? undefined : pseudoElement),
});
