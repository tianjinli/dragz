/** Paths that are not a page to resume after login. Root renders no workbench. */
function isEntryPath(value: string) {
  const path = value.split(/[?#]/, 1)[0] || '/';
  return path === '/' || path === '/auth/login' || path === '/auth/oauth-callback';
}

/** Restrict post-login redirect to same-origin relative paths. */
export function safeRedirectPath(value: string | null | undefined, fallback: string): string {
  if (!value) return fallback;
  if (!value.startsWith('/') || value.startsWith('//')) return fallback;
  if (value.includes('://') || isEntryPath(value)) return fallback;
  return value;
}
