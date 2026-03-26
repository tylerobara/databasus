package billing_webhooks

import "errors"

var (
	ErrDuplicateWebhook     = errors.New("duplicate webhook event")
	ErrUnsupportedEventType = errors.New("unsupported webhook event type")
)
