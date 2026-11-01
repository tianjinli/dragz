import axios, { AxiosError, type AxiosRequestConfig, type InternalAxiosRequestConfig } from 'axios';
import { message } from 'antd';
import i18n from '@/i18n';
import { useAccessStore } from '@/stores/access';
import { useLocaleStore } from '@/stores/locale';
import { usePreferencesStore } from '@/stores/preferences';
import type { ApiEnvelope } from '@/types';

const baseURL = import.meta.env.VITE_GLOB_API_URL || '/api';

export const rawClient = axios.create({ baseURL, timeout: 30_000, withCredentials: true });
export const requestClient = axios.create({ baseURL, timeout: 30_000, withCredentials: true });

type RetriableConfig = InternalAxiosRequestConfig & { _retried?: boolean };

let isRefreshing = false;
let refreshTokenQueue: Array<(token: string | null) => void> = [];
let loginExpiredDispatched = false;
const recentErrorKeys = new Map<string, number>();
const pendingGetRequests = new Map<string, Promise<unknown>>();

function isSuccess(code: number) {
  return code === 0 || code === 200;
}

function isPublicAuthEndpoint(url?: string) {
  if (!url) return false;
  return (
    url.startsWith('/auth/login') ||
    url.startsWith('/auth/captcha') ||
    url.startsWith('/auth/login-challenge') ||
    url.startsWith('/auth/refresh') ||
    url.startsWith('/auth/providers') ||
    url.startsWith('/auth/oauth/')
  );
}

/** Refresh/bootstrap 401s stay silent; login credential failures must surface. */
function shouldSilentPublicAuthError(status?: number, url?: string) {
  return status === 401 && isPublicAuthEndpoint(url) && !url?.startsWith('/auth/login');
}

function showErrorOnce(text: string) {
  const now = Date.now();
  const last = recentErrorKeys.get(text) || 0;
  if (now - last < 1500) return;
  recentErrorKeys.set(text, now);
  void message.error({ content: text, key: `req-err:${text}` });
}

function stableStringify(value: unknown): string {
  if (value === null || typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(stableStringify).join(',')}]`;
  const object = value as Record<string, unknown>;
  return `{${Object.keys(object).sort().map((key) => `${JSON.stringify(key)}:${stableStringify(object[key])}`).join(',')}}`;
}

function getRequestKey(url: string, config?: AxiosRequestConfig) {
  const params = config?.params === undefined ? '' : stableStringify(config.params);
  return `${url}?${params}`;
}

function flushRefreshQueue(token: string | null) {
  const queue = refreshTokenQueue;
  refreshTokenQueue = [];
  queue.forEach((resolve) => resolve(token));
}

function dispatchLoginExpiredOnce() {
  if (loginExpiredDispatched) return;
  loginExpiredDispatched = true;
  const { preferences } = usePreferencesStore.getState();
  window.dispatchEvent(new CustomEvent('admin:login-expired', { detail: preferences.app.loginExpiredMode }));
  window.setTimeout(() => {
    loginExpiredDispatched = false;
  }, 2000);
}

async function refreshAccessToken(): Promise<string | null> {
  try {
    const response = await rawClient.post<ApiEnvelope<{ accessToken: string }>>('/auth/refresh');
    const token = response.data.data.accessToken;
    useAccessStore.getState().setAccessToken(token);
    return token;
  } catch {
    const mode = usePreferencesStore.getState().preferences.app.loginExpiredMode;
    // Modal mode keeps the session so the inline re-login can succeed; page mode clears immediately.
    if (mode !== 'modal') useAccessStore.getState().clear();
    dispatchLoginExpiredOnce();
    return null;
  }
}

requestClient.interceptors.request.use((config) => {
  const token = useAccessStore.getState().accessToken;
  if (token) config.headers.Authorization = `Bearer ${token}`;
  config.headers['Accept-Language'] = useLocaleStore.getState().locale;
  return config;
});

requestClient.interceptors.response.use(
  (response) => {
    if (response.config.responseType === 'blob') return response;
    const envelope = response.data as ApiEnvelope<unknown>;
    if (envelope && typeof envelope.code === 'number') {
      if (isSuccess(envelope.code)) return envelope.data as never;
      const text = envelope.message || i18n.t('requestFailed');
      showErrorOnce(text);
      return Promise.reject(new Error(text));
    }
    return response.data;
  },
  async (error: AxiosError<ApiEnvelope<unknown>>) => {
    const original = error.config as RetriableConfig | undefined;
    const status = error.response?.status;
    const enableRefresh = usePreferencesStore.getState().preferences.app.enableRefreshToken;

    if (status === 401 && original && !original._retried && !isPublicAuthEndpoint(original.url) && enableRefresh) {
      original._retried = true;

      if (isRefreshing) {
        const token = await new Promise<string | null>((resolve) => {
          refreshTokenQueue.push(resolve);
        });
        if (!token) return Promise.reject(error);
        original.headers.Authorization = `Bearer ${token}`;
        return requestClient.request(original);
      }

      isRefreshing = true;
      const token = await refreshAccessToken();
      isRefreshing = false;
      flushRefreshQueue(token);

      if (!token) {
        showErrorOnce(error.response?.data?.message || error.message || i18n.t('requestFailed'));
        return Promise.reject(error);
      }

      original.headers.Authorization = `Bearer ${token}`;
      return requestClient.request(original);
    }

    const text = error.response?.data?.message || error.message || i18n.t('requestFailed');
    if (!shouldSilentPublicAuthError(status, original?.url)) {
      showErrorOnce(text);
    }
    return Promise.reject(error);
  },
);

export async function apiGet<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  if (config?.responseType === 'blob') return requestClient.get(url, config) as Promise<T>;
  const key = getRequestKey(url, config);
  const pending = pendingGetRequests.get(key);
  if (pending) return pending as Promise<T>;
  const request = (requestClient.get(url, config) as Promise<T>).finally(() => {
    pendingGetRequests.delete(key);
  });
  pendingGetRequests.set(key, request);
  return request;
}

/**
 * Deduped GETs carry no language in their key, so an in-flight one would be
 * reused for the post-switch refetch and hand back text in the old language.
 */
export function dropPendingGetDedupes() {
  pendingGetRequests.clear();
}
export async function apiPost<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
  return requestClient.post(url, data, config) as Promise<T>;
}
export async function apiPut<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
  return requestClient.put(url, data, config) as Promise<T>;
}
export async function apiDelete<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  return requestClient.delete(url, config) as Promise<T>;
}
