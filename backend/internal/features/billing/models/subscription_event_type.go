package billing_models

type SubscriptionEventType string

const (
	EventCreated                SubscriptionEventType = "subscription.created"
	EventUpgraded               SubscriptionEventType = "subscription.upgraded"
	EventDowngraded             SubscriptionEventType = "subscription.downgraded"
	EventNewBillingCycleStarted SubscriptionEventType = "subscription.new_billing_cycle_started"
	EventCanceled               SubscriptionEventType = "subscription.canceled"
	EventReactivated            SubscriptionEventType = "subscription.reactivated"
	EventExpired                SubscriptionEventType = "subscription.expired"
	EventPastDue                SubscriptionEventType = "subscription.past_due"
	EventRecoveredFromPastDue   SubscriptionEventType = "subscription.recovered_from_past_due"
	EventRefund                 SubscriptionEventType = "payment.refund"
	EventDispute                SubscriptionEventType = "payment.dispute"
)
