const namespace = `${import.meta.env.VITE_APP_NAMESPACE || 'dragz-admin'}-${import.meta.env.VITE_APP_VERSION || '1.0.0'}-${import.meta.env.MODE}`;
export const storageKey = (store: string) => `${namespace}-${store}`;
export function clearAdminStorage() { Object.keys(localStorage).filter((key) => key.startsWith(`${namespace}-`)).forEach((key) => localStorage.removeItem(key)); }

