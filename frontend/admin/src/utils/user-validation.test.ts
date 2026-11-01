import { describe, expect, it } from 'vitest';
import { emailError, mobileError, passwordError, usernameError } from './user-validation';

describe('user validation', () => {
  it('enforces the username format', () => {
    expect(usernameError('admin1')).toBeUndefined();
    expect(usernameError('user_name')).toBeUndefined();
    expect(usernameError('1admin')).toBeDefined();
    expect(usernameError('ab')).toBeDefined();
    expect(usernameError('admin-name')).toBeDefined();
    expect(usernameError('admin.name')).toBeDefined();
    expect(usernameError('管理员')).toBeDefined();
    expect(usernameError('admin 1')).toBeDefined();
  });

  it('enforces the password length and character classes', () => {
    expect(passwordError('Aa1!abcd', 8)).toBeUndefined();
    expect(passwordError('abcdefg1', 8)).toBeDefined();
    expect(passwordError('Aa1!abc', 8)).toBeDefined();
    expect(passwordError('Aa1!abcdef', 8)).toBeDefined();
    expect(passwordError('Aa1!111111', 8)).toBeDefined();
    expect(passwordError('Example1!x', 8, 'example')).toBeUndefined();
    expect(passwordError(`Aa1!${'x'.repeat(61)}`, 8)).toBeDefined();
  });

  it('accepts optional contact fields and rejects invalid values', () => {
    expect(emailError()).toBeUndefined();
    expect(emailError('admin@example.com')).toBeUndefined();
    expect(emailError('Admin <admin@example.com>')).toBeDefined();
    expect(mobileError()).toBeUndefined();
    expect(mobileError('13800138000')).toBeUndefined();
    expect(mobileError('12800138000')).toBeDefined();
  });
});
