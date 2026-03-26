import { SubscriptionEventType } from './SubscriptionEventType';
import { SubscriptionStatus } from './SubscriptionStatus';

export interface SubscriptionEvent {
  id: string;
  subscriptionId: string;
  providerEventId?: string;
  type: SubscriptionEventType;
  oldStorageGb?: number;
  newStorageGb?: number;
  oldStatus?: SubscriptionStatus;
  newStatus?: SubscriptionStatus;
  createdAt: string;
}
