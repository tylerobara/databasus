package backuping

import (
	"log/slog"

	"github.com/google/uuid"

	billing_models "databasus-backend/internal/features/billing/models"
)

type BillingService interface {
	GetSubscription(logger *slog.Logger, databaseID uuid.UUID) (*billing_models.Subscription, error)
}
