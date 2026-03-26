import { InvoiceStatus } from './InvoiceStatus';

export interface Invoice {
  id: string;
  subscriptionId: string;
  providerInvoiceId: string;
  amountCents: number;
  storageGb: number;
  periodStart: string;
  periodEnd: string;
  status: InvoiceStatus;
  paidAt?: string;
  createdAt: string;
}
