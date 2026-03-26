import type { Invoice } from './Invoice';

export interface GetInvoicesResponse {
  invoices: Invoice[];
  total: number;
  limit: number;
  offset: number;
}
