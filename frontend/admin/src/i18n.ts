import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import en from './locales/en.json';
import zh from './locales/zh.json';

const resources = {
  'zh-CN': { translation: zh },
  'en-US': { translation: en },
};

void i18n.use(initReactI18next).init({
  resources,
  lng: 'zh-CN',
  fallbackLng: 'zh-CN',
  interpolation: { escapeValue: false },
  // Resources are bundled, so initialise synchronously: the first render already
  // sees translations instead of raw keys (and tests need no await).
  initAsync: false,
});

/**
 * Keep `<html lang>` in step with the interface language.
 *
 * Screen readers, spell checkers and the browser's own language heuristics read
 * the attribute, not the i18next store, so a page that switched to English while
 * still claiming `lang="zh-CN"` is read out with Chinese phonetics. The value is
 * a language tag, not a locale, so `en-US` narrows to `en`.
 */
const DOCUMENT_LANG: Record<string, string> = {
  'zh-CN': 'zh-CN',
  'en-US': 'en',
};

function syncDocumentLanguage(language: string) {
  if (typeof document === 'undefined') return;
  document.documentElement.lang = DOCUMENT_LANG[language] ?? language;
}

i18n.on('languageChanged', syncDocumentLanguage);
syncDocumentLanguage(i18n.resolvedLanguage ?? i18n.language);

export default i18n;
