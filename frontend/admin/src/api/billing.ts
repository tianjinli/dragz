import type { PlatformInvoice } from "./platform";
import { apiGet } from "./request";

export interface BillingSubscription {
  id: number;
  planId: number;
  planCode: string;
  planName: string;
  status: string;
  startedAt: string;
  endedAt: string | null;
  trialEndsAt: string | null;
  graceEndsAt: string | null;
}

export interface BillingQuota {
  key: string;
  used: number;
  limit: number;
}

export interface BillingSummary {
  tenantId: number;
  subscription: BillingSubscription | null;
  quotas: BillingQuota[];
  invoices: PlatformInvoice[];
}

export interface BillingUsageCounter {
  quotaKey: string;
  periodKey: string;
  usedValue: number;
  updatedAt: string;
}

export interface BillingUsage {
  quotas: BillingQuota[];
  usage: BillingUsageCounter[];
}

export const getBillingSummary = () => apiGet<BillingSummary>("/billing/summary");

/**
 * Consumed counters (storage bytes, API calls per day, seats...). Separate from
 * the summary because it is a heavier aggregate, so the summary stays cheap.
 */
export const getBillingUsage = () => apiGet<BillingUsage>("/billing/usage");
