import { apiDelete, apiGet, apiPost, apiPut } from "./request";

export interface PlatformTenant {
  id: number;
  code: string;
  name: string;
  status: number;
  remark?: string;
  slug?: string;
  createdAt?: string;
  purgeAfter?: string | null;
  purgeStatus?: string;
}

export interface TenantExportSnapshot {
  tenant: PlatformTenant;
  members: Array<Record<string, unknown>>;
  departments: Array<Record<string, unknown>>;
  roles: Array<Record<string, unknown>>;
  subscription: Record<string, unknown> | null;
}

export interface PlanEntitlement {
  key: string;
  value: string;
}

export interface PlatformPlan {
  id: number;
  code: string;
  name: string;
  remark: string;
  status: number;
  sort: number;
  entitlements: PlanEntitlement[];
}

export interface ImpersonationToken {
  accessToken: string;
  tokenType: string;
  expiresIn: number;
  impersonationId: number;
  tenantId: number;
}

export const listTenants = () => apiGet<PlatformTenant[]>("/platform/tenants");

export const createTenant = (data: { code: string; name: string }) =>
  apiPost<PlatformTenant>("/platform/tenants", data);

export const updateTenantStatus = (id: number, status: number) =>
  apiPut<{ success: boolean }>(`/platform/tenants/${id}/status`, { status });

export const listPlans = () => apiGet<PlatformPlan[]>("/platform/plans");

export interface PlanWrite {
  code: string;
  name: string;
  remark?: string;
  status?: number;
  sort?: number;
  entitlements?: PlanEntitlement[];
}

export const createPlan = (data: PlanWrite) => apiPost<PlatformPlan>("/platform/plans", data);

export const updatePlan = (id: number, data: PlanWrite) =>
  apiPut<PlatformPlan>(`/platform/plans/${id}`, data);

export const updatePlanStatus = (id: number, status: number) =>
  apiPut<{ success: boolean }>(`/platform/plans/${id}/status`, { status });

export const updateTenantSubscription = (
  id: number,
  data: { planId: number; status: string },
) =>
  apiPut<{ id: number; tenantId: number; planId: number; status: string }>(
    `/platform/tenants/${id}/subscription`,
    data,
  );

export const impersonateTenant = (
  id: number,
  data: { reason: string; hours: number },
) => apiPost<ImpersonationToken>(`/platform/tenants/${id}/impersonate`, data);

export const exportTenant = (id: number) =>
  apiPost<TenantExportSnapshot>(`/platform/tenants/${id}/export`);

export const scheduleTenantPurge = (id: number, days: number) =>
  apiPost<{ success: boolean; days: number }>(`/platform/tenants/${id}/schedule-purge`, { days });

export const cancelTenantPurge = (id: number) =>
  apiPost<{ success: boolean }>(`/platform/tenants/${id}/cancel-purge`);

export interface TenantDomain {
  id: number;
  tenantId: number;
  host: string;
  verifiedAt?: string | null;
  isPrimary: boolean;
  createdAt: string;
}

export const updateTenantSlug = (id: number, slug: string) =>
  apiPut<{ slug: string; success: boolean }>(`/platform/tenants/${id}/slug`, { slug });

export const listTenantDomains = (id: number) =>
  apiGet<TenantDomain[]>(`/platform/tenants/${id}/domains`);

export const addTenantDomain = (id: number, host: string) =>
  apiPost<TenantDomain>(`/platform/tenants/${id}/domains`, { host });

export const verifyTenantDomain = (id: number, domainId: number) =>
  apiPost<{ success: boolean }>(`/platform/tenants/${id}/domains/${domainId}/verify`);

export const removeTenantDomain = (id: number, domainId: number) =>
  apiDelete<{ success: boolean }>(`/platform/tenants/${id}/domains/${domainId}`);
