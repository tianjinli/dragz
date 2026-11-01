/** Restrict post-login redirect to same-origin relative paths. */
export function safeRedirectPath(value: string | null | undefined, fallback: string): string {
  if (!value) return fallback;
  if (!value.startsWith('/') || value.startsWith('//')) return fallback;
  if (value.includes('://')) return fallback;
  return value;
}
