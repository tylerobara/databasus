import { SubscriptionStatus } from './SubscriptionStatus';

export interface Subscription {
  id: string;
  databaseId: string;
  status: SubscriptionStatus;
  storageGb: number;
  pendingStorageGb?: number;
  currentPeriodStart: string;
  currentPeriodEnd: string;
  canceledAt?: string;
  dataRetentionGracePeriodUntil?: string;
  providerName?: string;
  providerSubId?: string;
  providerCustomerId?: string;
  createdAt: string;
  updatedAt: string;
}
