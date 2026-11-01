import i18n from '@/i18n';

const mobilePattern = /^1[3-9]\d{9}$/;
const emailPattern = /^[^\s@<>()[\],:;\\"]+@[^\s@<>()[\],:;\\"]+\.[^\s@<>()[\],:;\\"]+$/;
const usernamePattern = /^[A-Za-z][A-Za-z0-9_]{3,63}$/;

export function usernameError(value?: string): string | undefined {
  return !value || usernamePattern.test(value) ? undefined : i18n.t('validation.username');
}

export function passwordError(value: string, minLength: number, _username?: string): string | undefined {
  const characterLength = Array.from(value).length;
  const byteLength = new TextEncoder().encode(value).length;
  if (characterLength < minLength || characterLength > 64) return i18n.t('validation.passwordLength', { min: minLength });
  if (byteLength > 72) return i18n.t('validation.passwordBytes');
  if (hasSequentialOrRepeatedRun(value, 6)) return i18n.t('validation.passwordSequence');
  const classes = [/[A-Z]/, /[a-z]/, /\d/, /[!-/:-@[-`{-~]/].filter((pattern) => pattern.test(value)).length;
  return classes >= 3 ? undefined : i18n.t('validation.passwordClasses');
}

function hasSequentialOrRepeatedRun(value: string, minimum: number): boolean {
  const chars = Array.from(value.toLowerCase());
  for (let start = 0; start + minimum <= chars.length; start += 1) {
    let ascending = true; let descending = true; let repeated = true;
    for (let index = start + 1; index < start + minimum; index += 1) {
      const previous = chars[index - 1].codePointAt(0)!;
      const current = chars[index].codePointAt(0)!;
      ascending &&= current === previous + 1;
      descending &&= current === previous - 1;
      repeated &&= current === previous;
    }
    if (ascending || descending || repeated) return true;
  }
  return false;
}

export function emailError(value?: string): string | undefined {
  return !value || (value.length <= 255 && emailPattern.test(value)) ? undefined : i18n.t('validation.email');
}

export function mobileError(value?: string): string | undefined {
  return !value || mobilePattern.test(value) ? undefined : i18n.t('validation.mobile');
}

export function formValidator(check: (value?: string) => string | undefined) {
  return (_: unknown, value?: string) => {
    const message = check(value);
    return message ? Promise.reject(new Error(message)) : Promise.resolve();
  };
}
