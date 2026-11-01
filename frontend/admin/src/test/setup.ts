import '@testing-library/jest-dom/vitest';
import { configure } from '@testing-library/react';
// Components translate through a shared i18next singleton; initialise it for
// every test file so t() returns real copy instead of raw keys.
import '@/i18n';

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
