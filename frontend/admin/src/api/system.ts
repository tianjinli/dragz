import { apiDelete, apiGet, apiPost, apiPut } from "./request";
import { requestClient } from "./request";
import type { ListQuery, PagedResult } from "@/types";

export type ResourceRecord = Record<string, unknown> & { id: number };
export const listResourceApi = <T>(path: string, params?: ListQuery) =>
  apiGet<T>(path, { params });
export const getResourceApi = <T>(path: string) => apiGet<T>(path);
export const createResourceApi = <T = { id: number }>(
  path: string,
  data: unknown,
) => apiPost<T>(path, data);
export const updateResourceApi = (path: string, id: number, data: unknown) =>
  apiPut<void>(`${path}/${id}`, data);
export const updateResourceStatusApi = (
  path: string,
  id: number,
  status: number,
) => apiPut<void>(`${path}/${id}/status`, { status });
export const deleteResourceApi = (path: string, id: number) =>
  apiDelete<void>(`${path}/${id}`);

export const listDepartmentsApi = () =>
  apiGet<ResourceRecord[]>("/system/departments");
export const listPositionsApi = (params?: ListQuery) =>
  apiGet<PagedResult<ResourceRecord>>("/system/positions", { params });
export const listRolesApi = (params?: ListQuery) =>
  apiGet<ResourceRecord[]>("/system/roles", { params });
export const getRoleGrantsApi = (id: number) =>
  apiGet<Record<string, unknown>>(`/system/roles/${id}/grants`);
export const saveRoleGrantsApi = (id: number, data: unknown) =>
  apiPut<void>(`/system/roles/${id}/grants`, data);
export const listMenusApi = () => apiGet<ResourceRecord[]>("/system/menus");
export const listDictionaryTypesApi = (params?: ListQuery) =>
  apiGet<ResourceRecord[]>("/system/dictionaries", { params });
export const createDictionaryTypeApi = (data: unknown) =>
  apiPost<{ id: number }>("/system/dictionaries", data);
export const updateDictionaryTypeApi = (id: number, data: unknown) =>
  apiPut<void>(`/system/dictionaries/${id}`, data);
export const deleteDictionaryTypeApi = (id: number) =>
  apiDelete<void>(`/system/dictionaries/${id}`);
export const listDictionaryItemsApi = (id: number) =>
  apiGet<ResourceRecord[]>(`/system/dictionaries/${id}/items`);
export const createDictionaryItemApi = (id: number, data: unknown) =>
  apiPost<void>(`/system/dictionaries/${id}/items`, data);
export const updateDictionaryItemApi = (id: number, data: unknown) =>
  apiPut<void>(`/system/dictionary-items/${id}`, data);
export const deleteDictionaryItemApi = (id: number) =>
  apiDelete<void>(`/system/dictionary-items/${id}`);
export const listConfigsApi = (group?: string) =>
  apiGet<ResourceRecord[]>("/system/configs", { params: { group } });
export const updateConfigApi = (key: string, value: string) =>
  apiPut<void>(`/system/configs/${encodeURIComponent(key)}`, { value });

export interface PluginRecord {
  id: string;
  name: string;
  version: string;
  description: string;
  dependsOn: string[];
  builtin: boolean;
  enabled: boolean;
  lastError: string;
  pendingMigrations: number;
}
export const listPluginsApi = () => apiGet<PluginRecord[]>("/system/plugins");
export const updatePluginStatusApi = (id: string, status: number) =>
  apiPut<void>(`/system/plugins/${encodeURIComponent(id)}/status`, { status });
/**
 * Uninstall drops the plugin's tables and menus. It is only allowed while the
 * plugin is disabled, so the UI must never offer it on an enabled row.
 */
export const uninstallPluginApi = (id: string) =>
  apiDelete<void>(`/system/plugins/${encodeURIComponent(id)}`);
export const listAuthProvidersApi = () =>
  apiGet<ResourceRecord[]>("/system/auth-providers");
export const listStorageConfigsApi = () =>
  apiGet<ResourceRecord[]>("/system/storage-configs");
export const testStorageConfigApi = (id: number) =>
  apiPost<void>(`/system/storage-configs/${id}/test`);
export const listNoticesApi = (params?: ListQuery) =>
  apiGet<PagedResult<ResourceRecord>>("/system/notices", { params });
export const publishNoticeApi = (id: number) =>
  apiPost<void>(`/system/notices/${id}/publish`);
export const revokeNoticeApi = (id: number) =>
  apiPost<void>(`/system/notices/${id}/revoke`);
export const listMyNoticesApi = (params?: ListQuery) =>
  apiGet<PagedResult<ResourceRecord>>("/system/my-notices", { params });
export const getMyNoticeApi = (id: number) =>
  apiGet<ResourceRecord>(`/system/my-notices/${id}`);
export const markNoticeReadApi = (id: number) =>
  apiPost<void>(`/system/my-notices/${id}/read`);
export const listFilesApi = (params?: ListQuery) =>
  apiGet<PagedResult<ResourceRecord>>("/system/files", { params });
export const uploadFileApi = (data: FormData) =>
  apiPost<ResourceRecord & { url?: string }>("/system/files/upload", data);
/** Upload via system storage; defaults to public so the returned `url` can be used in <img src>. */
export async function uploadStorageFileApi(
  file: Blob | File,
  options?: { accessType?: "private" | "public"; storageId?: number },
) {
  const data = new FormData();
  data.append("file", file);
  data.append("accessType", options?.accessType ?? "public");
  if (options?.storageId) data.append("storageId", String(options.storageId));
  return uploadFileApi(data);
}
export const recycleFileApi = (id: number) =>
  apiDelete<void>(`/system/files/${id}`);
export const restoreFileApi = (id: number) =>
  apiPost<void>(`/system/files/${id}/restore`);
export const purgeFileApi = (id: number) =>
  apiDelete<void>(`/system/files/${id}/purge`);
export const getFileDetailApi = (id: number) =>
  apiGet<{ file: ResourceRecord; references: ResourceRecord[] }>(
    `/system/files/${id}`,
  );
/** Origin used to absolutize public file URLs (API host when configured, else page origin). */
export function publicApiOrigin(): string {
  const base = String(import.meta.env.VITE_GLOB_API_URL || "/api").trim();
  if (/^https?:\/\//i.test(base)) {
    try {
      return new URL(base).origin;
    } catch {
      /* fall through */
    }
  }
  if (typeof window !== "undefined" && window.location?.origin) {
    return window.location.origin;
  }
  return "";
}

/** Rewrite relative/public paths to absolute http(s); leave absolute http(s) unchanged. */
export function toAbsoluteHttpUrl(pathOrUrl?: string | null): string {
  const raw = String(pathOrUrl ?? "").trim();
  if (!raw) return "";
  if (/^https?:\/\//i.test(raw)) return raw;
  if (raw.startsWith("//")) {
    const proto =
      typeof window !== "undefined" && window.location?.protocol === "https:"
        ? "https:"
        : "http:";
    return `${proto}${raw}`;
  }
  const path = raw.startsWith("/") ? raw : `/${raw}`;
  const origin = publicApiOrigin();
  return origin ? `${origin}${path}` : path;
}

/** Public content URL for files uploaded with accessType=public (always absolute http(s) when origin is known). */
export function storageFileContentUrl(fileId: number) {
  return toAbsoluteHttpUrl(`/api/system/files/${fileId}/content`);
}
export function parseStorageFileId(urlOrId?: string | number | null) {
  if (urlOrId == null || urlOrId === "") return undefined;
  if (typeof urlOrId === "number" && Number.isFinite(urlOrId) && urlOrId > 0) {
    return urlOrId;
  }
  const raw = String(urlOrId).trim();
  if (/^\d+$/.test(raw)) return Number(raw);
  const matched = raw.match(/\/(?:api\/)?system\/files\/(\d+)\/(?:content|download)\b/);
  if (matched) return Number(matched[1]);
  const wxMatched = raw.match(/\/(?:api\/)?wx\/files\/(\d+)\/content\b/);
  if (wxMatched) return Number(wxMatched[1]);
  return undefined;
}
export async function downloadFileApi(id: number, filename: string) {
  const response = await requestClient.get<Blob>(
    `/system/files/${id}/download`,
    { responseType: "blob" },
  );
  const url = URL.createObjectURL(response.data);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}
export const listSessionsApi = (params?: ListQuery) =>
  apiGet<PagedResult<ResourceRecord>>("/system/sessions", { params });
export const revokeSessionApi = (id: number) =>
  apiPost<void>(`/system/sessions/${id}/revoke`);
export const listTokensApi = (params?: ListQuery) =>
  apiGet<{ items: ResourceRecord[] }>("/system/tokens", { params });
export const createTokenApi = (data: unknown) =>
  apiPost<{ id: number; shownOnce: boolean; token: string }>(
    "/system/tokens",
    data,
  );
export const revokeTokenApi = (id: number) =>
  apiPost<void>(`/system/tokens/${id}/revoke`);
export const listLoginLogsApi = (params?: ListQuery) =>
  apiGet<PagedResult<ResourceRecord>>("/system/logs/login", { params });
export const listOperationLogsApi = (params?: ListQuery) =>
  apiGet<PagedResult<ResourceRecord>>("/system/logs/operation", { params });
export const listErrorLogsApi = (params?: ListQuery) =>
  apiGet<PagedResult<ResourceRecord>>("/system/logs/error", { params });
export const resolveErrorLogApi = (id: number) =>
  apiPost<void>(`/system/logs/error/${id}/resolve`);
export const getDashboardApi = () =>
  apiGet<Record<string, unknown>>("/dashboard");

export const listJobsApi = () =>
  apiGet<{ handlers: string[]; items: ResourceRecord[] }>("/system/jobs");
export const runJobApi = (id: number) =>
  apiPost<void>(`/system/jobs/${id}/run`);
export const updateJobApi = (id: number, data: unknown) =>
  apiPut<void>(`/system/jobs/${id}`, data);
export const listJobLogsApi = (id: number, params?: ListQuery) =>
  apiGet<PagedResult<ResourceRecord>>(`/system/jobs/${id}/logs`, { params });
export const listWebhooksApi = () =>
  apiGet<ResourceRecord[]>("/system/webhooks");
export const testWebhookApi = (id: number) =>
  apiPost<void>(`/system/webhooks/${id}/test`);
export const listCacheKeysApi = (params?: ListQuery) =>
  apiGet<{ items: ResourceRecord[]; nextCursor: number }>("/system/cache", {
    params,
  });
export const deleteCacheKeyApi = (key: string) =>
  apiDelete<void>(`/system/cache/${encodeURIComponent(key)}`);
export const clearCacheApi = (data: {
  confirmation?: string;
  pattern: string;
}) => apiPost<void>("/system/cache/clear", data);

export interface UserInput {
  departmentId?: number;
  displayName: string;
  email?: string;
  gender: number;
  mobile?: string;
  mustChangePassword: boolean;
  password?: string;
  positionIds: number[];
  remark?: string;
  roleIds: number[];
  status: number;
  username?: string;
}
export const listUsersApi = (params: ListQuery) =>
  apiGet<PagedResult<ResourceRecord>>("/system/users", { params });
export const getUserApi = (id: number) =>
  apiGet<ResourceRecord>(`/system/users/${id}`);
export const createUserApi = (data: UserInput) =>
  apiPost<{ id: number }>("/system/users", data);
export const updateUserApi = (id: number, data: UserInput) =>
  apiPut<void>(`/system/users/${id}`, data);
export const updateUserStatusApi = (id: number, status: number) =>
  apiPut<void>(`/system/users/${id}/status`, { status });
export const deleteUserApi = (id: number) =>
  apiDelete<void>(`/system/users/${id}`);
export const resetUserPasswordApi = (id: number, password: string) =>
  apiPost<void>(`/system/users/${id}/reset-password`, { password });
export const forceLogoutUserApi = (id: number) =>
  apiPost<void>(`/system/users/${id}/logout`);

/* ---- translations (preset-data localization) ---- */
export interface TranslationCandidate {
  entityKey: string;
  source: string;
}
export const listTranslationsApi = (params?: ListQuery) =>
  apiGet<PagedResult<ResourceRecord>>("/system/translations", { params });
/** Preset entities that have no English translation yet; feeds the create form. */
export const listTranslationCandidatesApi = (params?: { keyword?: string; lang?: string }) =>
  apiGet<TranslationCandidate[]>("/system/translations/candidates", { params });
export interface TranslateResult {
  /** Provider that produced the text ("mymemory", "google", or "cache"). */
  provider: string;
  text: string;
  cached: boolean;
}
/**
 * Machine-translates a preset text into another language.
 *
 * Both language columns of the translations page carry a translate button, so
 * either side can be the source: `source` defaults to Chinese and `target` to
 * English. It is deliberately a separate call rather than part of the save — the
 * operator can still correct the result, and a slow or blocked translation
 * service never blocks writing the record.
 */
export const translatePresetTextApi = (payload: {
  text: string;
  source?: string;
  target?: string;
}) => apiPost<TranslateResult>("/system/translations/translate", payload);
/**
 * Drops one entity's translation, so it falls back to the Chinese source. The
 * id a row shows is a display key (the list is one row per preset entity), so
 * the entity key is what actually addresses it; the path id only satisfies the
 * route shape.
 */
export const deleteTranslationApi = (id: number, entityKey: string) =>
  apiDelete<void>(`/system/translations/${id}`, { params: { entityKey } });
