package billing_models

type WebhookEventType string

const (
	WHEventSubscriptionCreated        WebhookEventType = "subscription.created"
	WHEventSubscriptionUpdated        WebhookEventType = "subscription.updated"
	WHEventSubscriptionCanceled       WebhookEventType = "subscription.canceled"
	WHEventSubscriptionPastDue        WebhookEventType = "subscription.past_due"
	WHEventSubscriptionReactivated    WebhookEventType = "subscription.reactivated"
	WHEventPaymentSucceeded           WebhookEventType = "payment.succeeded"
	WHEventSubscriptionDisputeCreated WebhookEventType = "dispute.created"
)
