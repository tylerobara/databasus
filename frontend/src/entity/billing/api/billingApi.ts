import { getApplicationServer } from '../../../constants';
import RequestOptions from '../../../shared/api/RequestOptions';
import { apiHelper } from '../../../shared/api/apiHelper';
import type { ChangeStorageResponse } from '../model/ChangeStorageResponse';
import type { GetInvoicesResponse } from '../model/GetInvoicesResponse';
import type { GetSubscriptionEventsResponse } from '../model/GetSubscriptionEventsResponse';
import type { Subscription } from '../model/Subscription';

export const billingApi = {
  async createSubscription(databaseId: string, storageGb: number) {
    const requestOptions = new RequestOptions();
    requestOptions.setBody(JSON.stringify({ databaseId, storageGb }));

    return apiHelper.fetchPostJson<{ paddleTransactionId: string }>(
      `${getApplicationServer()}/api/v1/billing/subscription`,
      requestOptions,
    );
  },

  async changeStorage(databaseId: string, storageGb: number) {
    const requestOptions = new RequestOptions();
    requestOptions.setBody(JSON.stringify({ databaseId, storageGb }));

    return apiHelper.fetchPostJson<ChangeStorageResponse>(
      `${getApplicationServer()}/api/v1/billing/subscription/change-storage`,
      requestOptions,
    );
  },

  async getPortalSession(subscriptionId: string) {
    return apiHelper.fetchPostJson<{ url: string }>(
      `${getApplicationServer()}/api/v1/billing/subscription/portal/${subscriptionId}`,
      new RequestOptions(),
    );
  },

  async getSubscriptionEvents(subscriptionId: string, limit?: number, offset?: number) {
    const params = new URLSearchParams();
    if (limit !== undefined) params.append('limit', limit.toString());
    if (offset !== undefined) params.append('offset', offset.toString());

    const query = params.toString();
    const url = `${getApplicationServer()}/api/v1/billing/subscription/events/${subscriptionId}${query ? `?${query}` : ''}`;

    return apiHelper.fetchGetJson<GetSubscriptionEventsResponse>(url, undefined, true);
  },

  async getInvoices(subscriptionId: string, limit?: number, offset?: number) {
    const params = new URLSearchParams();
    if (limit !== undefined) params.append('limit', limit.toString());
    if (offset !== undefined) params.append('offset', offset.toString());

    const query = params.toString();
    const url = `${getApplicationServer()}/api/v1/billing/subscription/invoices/${subscriptionId}${query ? `?${query}` : ''}`;

    return apiHelper.fetchGetJson<GetInvoicesResponse>(url, undefined, true);
  },

  async getSubscription(databaseId: string) {
    return apiHelper.fetchGetJson<Subscription>(
      `${getApplicationServer()}/api/v1/billing/subscription/${databaseId}`,
      undefined,
      true,
    );
  },
};
