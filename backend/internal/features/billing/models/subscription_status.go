package billing_models

type SubscriptionStatus string

const (
	StatusTrial    SubscriptionStatus = "trial"    // trial period (~24h after DB creation)
	StatusActive   SubscriptionStatus = "active"   // paid, everything works
	StatusPastDue  SubscriptionStatus = "past_due" // payment failed, trying to charge again, but everything still works
	StatusCanceled SubscriptionStatus = "canceled" // subscription canceled by user or after past_due (grace period is active)
	StatusExpired  SubscriptionStatus = "expired"  // grace period ended, data marked for deletion, can come from canceled and trial
)
