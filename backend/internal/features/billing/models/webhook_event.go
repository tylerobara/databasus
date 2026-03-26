package billing_models

import (
	"time"

	"github.com/google/uuid"
)

type WebhookEvent struct {
	RequestID              uuid.UUID
	ProviderEventID        string
	DatabaseID             *uuid.UUID
	Type                   WebhookEventType
	ProviderSubscriptionID string
	ProviderCustomerID     string
	ProviderInvoiceID      string
	QuantityGB             int
	Status                 SubscriptionStatus
	PeriodStart            *time.Time
	PeriodEnd              *time.Time
	AmountCents            int64
}
