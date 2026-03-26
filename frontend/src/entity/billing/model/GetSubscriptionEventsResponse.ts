import type { SubscriptionEvent } from './SubscriptionEvent';

export interface GetSubscriptionEventsResponse {
  events: SubscriptionEvent[];
  total: number;
  limit: number;
  offset: number;
}
