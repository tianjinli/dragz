import type { Locale } from 'antd/es/locale';
import enUS from 'antd/locale/en_US';
import zhCN from 'antd/locale/zh_CN';
import type { LocaleCode } from '@/types';

/**
 * Languages the admin runs in.
 *
 * The product ships exactly two interfaces, Chinese and English, so this list
 * is the same pair the backend renders preset data in
 * (backend/internal/pkg/i18n/languages.go). Each tag's primary subtag is what
 * goes out as `Accept-Language`, so switching language here also switches the
 * language of menus, roles and departments coming back from the API.
 */
export const LOCALE_CODES: LocaleCode[] = ['zh-CN', 'en-US'];

const ANTD_LOCALES: Record<LocaleCode, Locale> = {
  'zh-CN': zhCN,
  'en-US': enUS,
};

/** Ant Design's own locale bundle for the given code (component strings). */
export function antdLocale(code: LocaleCode): Locale {
  return ANTD_LOCALES[code] ?? enUS;
}
