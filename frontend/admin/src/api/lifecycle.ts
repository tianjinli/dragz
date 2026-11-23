import { apiDelete, apiGet, apiPost, apiPut } from './request';

/**
 * Tenant lifecycle: self-service signup, invitations, membership and the
 * platform staff directory. Kept apart from auth.ts because none of it is part
 * of signing in — signup runs before an account exists, and the rest is tenant
 * administration.
 */

export interface SignupStatus {
  enabled: boolean;
  /** Server-side minimum password length, so the form validates the same rule. */
  passwordMinLength: number;
}

export interface SignupInput {
  tenantName: string;
  tenantCode?: string;
  username: string;
  password: string;
  email?: string;
  mobile?: string;
  displayName?: string;
}

export interface SignupResult {
  tenantId: number;
  tenantCode: string;
  tenantName: string;
  userId: number;
}

export type InviteStatus = 'pending' | 'accepted' | 'revoked' | 'expired';

export interface InvitePreview {
  tenantName: string;
  email: string;
  status: InviteStatus;
  expiresAt: string;
  acceptedAt?: string | null;
}

export interface Invite {
  id: number;
  tenantId: number;
  tenantName?: string;
  email: string;
  roleIds: number[];
  departmentId?: number | null;
  isTenantAdmin: boolean;
  status: InviteStatus;
  invitedBy: number;
  expiresAt: string;
  acceptedBy?: number | null;
  acceptedAt?: string | null;
  createdAt: string;
}

export interface CreateInviteInput {
  email: string;
  roleIds?: number[];
  departmentId?: number | null;
  isTenantAdmin?: boolean;
  ttlHours?: number;
}

export interface CreateInviteResult {
  invite: Invite;
  /** Returned exactly once: only the hash is stored. */
  token: string;
}

export interface Member {
  userId: number;
  username: string;
  displayName: string;
  email: string;
  mobile: string;
  departmentId?: number | null;
  isTenantAdmin: boolean;
  isPlatformOperator: boolean;
  status: number;
  roleCodes: string[];
  createdAt: string;
}

export interface MemberUpdate {
  status?: number;
  isTenantAdmin?: boolean;
  departmentId?: number | null;
  roleIds?: number[];
}

export interface PlatformOperator {
  userId: number;
  username: string;
  displayName: string;
  email: string;
  grantedBy?: number | null;
  note: string;
  createdAt: string;
}

export interface AcceptInviteResult {
  tenantId: number;
  tenantName: string;
}

export const getSignupStatus = () => apiGet<SignupStatus>('/auth/signup-status');

export const registerTenant = (data: SignupInput) => apiPost<SignupResult>('/auth/register', data);

export const previewInvite = (token: string) => apiGet<InvitePreview>(`/auth/invites/${encodeURIComponent(token)}`);

export const acceptInvite = (token: string) =>
  apiPost<AcceptInviteResult>(`/auth/invites/${encodeURIComponent(token)}/accept`);

export const listMembers = () => apiGet<Member[]>('/tenant/members');

export const updateMember = (userId: number, data: MemberUpdate) =>
  apiPut<{ success: boolean }>(`/tenant/members/${userId}`, data);

export const removeMember = (userId: number) => apiDelete<{ success: boolean }>(`/tenant/members/${userId}`);

export const listInvites = () => apiGet<Invite[]>('/tenant/invites');

export const createInvite = (data: CreateInviteInput) => apiPost<CreateInviteResult>('/tenant/invites', data);

export const revokeInvite = (id: number) => apiDelete<{ success: boolean }>(`/tenant/invites/${id}`);

export const listPlatformOperators = () => apiGet<PlatformOperator[]>('/platform/operators');

export const grantPlatformOperator = (userId: number, note: string) =>
  apiPost<{ success: boolean }>('/platform/operators', { userId, note });

export const revokePlatformOperator = (userId: number) => apiDelete<{ success: boolean }>(`/platform/operators/${userId}`);

/** The accept link an administrator copies out of the invite dialog. */
export function inviteLink(token: string) {
  const { origin } = window.location;
  return `${origin}/auth/invite/${token}`;
}
