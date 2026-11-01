import { apiGet, apiPost, apiPut, rawClient } from './request';
import type { AdminUserInfo, ApiEnvelope, MenuRoute } from '@/types';

export interface LoginInput {
  captchaCode?: string;
  captchaId?: string;
  deviceId?: string;
  deviceName?: string;
  password: string;
  username: string;
}
export interface TokenResult { accessToken: string; expiresIn: number; refreshExpiresIn: number; tokenType: 'Bearer' }
export interface CaptchaIssueResult { captchaId: string; expiresIn: number; imageBase64: string }
export interface LoginChallengeResult { captchaRequired: boolean }

export const loginApi = (data: LoginInput) => apiPost<TokenResult>('/auth/login', data);
export const issueCaptchaApi = () => apiGet<CaptchaIssueResult>('/auth/captcha');
export const loginChallengeApi = (username: string) =>
  apiGet<LoginChallengeResult>('/auth/login-challenge', { params: { username } });
export const getAccessCodesApi = () => apiGet<string[]>('/auth/codes');
export const getMenusApi = () => apiGet<MenuRoute[]>('/auth/menus');
export const getUserInfoApi = () => apiGet<AdminUserInfo>('/auth/profile');
export const updateProfileApi = (data: { email?: string; mobile?: string; realName: string }) => apiPut<void>('/auth/profile', data);
export const changePasswordApi = (data: { newPassword: string; oldPassword: string }) => apiPost<{ reloginRequired?: boolean; success: boolean }>('/auth/change-password', data);
export const publicAuthProvidersApi = () => apiGet<Array<Record<string, unknown>>>('/auth/providers');
export const startOAuthApi = (id: number, returnPath = '/auth/oauth-callback') => apiGet<{ authorizationUrl: string }>(`/auth/oauth/${id}/start`, { params: { returnPath } });
export const exchangeOAuthApi = (code: string) => apiPost<TokenResult>('/auth/oauth/exchange', { code });

export async function refreshTokenApi() {
  const response = await rawClient.post<ApiEnvelope<TokenResult>>('/auth/refresh');
  return response.data.data;
}
export async function logoutApi() {
  await rawClient.post('/auth/logout');
}

