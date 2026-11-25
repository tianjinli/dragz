import type { FormProps } from 'antd';

/**
 * Shared `scrollToFirstError` behaviour for every validated form.
 *
 * Without it a failed submit on a longer form looks like nothing happened: the
 * message appears under a field that is scrolled out of view, so the operator
 * presses Save again instead of fixing the field.
 *
 * `block: 'center'` keeps the offending row away from the sticky footer and the
 * drawer header, where it would be half-hidden behind an overlay.
 */
export const SCROLL_TO_FIRST_ERROR: FormProps['scrollToFirstError'] = {
  behavior: 'smooth',
  block: 'center',
};
