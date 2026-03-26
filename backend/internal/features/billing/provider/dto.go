package billing_provider

import (
	"time"

	"github.com/google/uuid"

	billing_models "databasus-backend/internal/features/billing/models"
)

type CreateSubscriptionRequest struct {
	ProviderCustomerID string
	DatabaseID         uuid.UUID
	StorageGB          int
}

type ProviderSubscription struct {
	ProviderSubscriptionID string
	ProviderCustomerID     string
	Status                 billing_models.SubscriptionStatus
	QuantityGB             int
	PeriodStart            time.Time
	PeriodEnd              time.Time
}

type CheckoutRequest struct {
	DatabaseID uuid.UUID
	Email      string
	StorageGB  int
	SuccessURL string
	CancelURL  string
}

type ProviderName string

const (
	ProviderPaddle ProviderName = "paddle"
)
