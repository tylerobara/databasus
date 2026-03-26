export enum SubscriptionEventType {
  Created = 'subscription.created',
  Upgraded = 'subscription.upgraded',
  Downgraded = 'subscription.downgraded',
  NewBillingCycleStarted = 'subscription.new_billing_cycle_started',
  Canceled = 'subscription.canceled',
  Reactivated = 'subscription.reactivated',
  Expired = 'subscription.expired',
  PastDue = 'subscription.past_due',
  RecoveredFromPastDue = 'subscription.recovered_from_past_due',
  Refund = 'payment.refund',
  Dispute = 'payment.dispute',
}
