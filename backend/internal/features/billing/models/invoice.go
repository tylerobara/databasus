package billing_models

import (
	"time"

	"github.com/google/uuid"
)

type Invoice struct {
	ID                uuid.UUID     `json:"id"                gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	SubscriptionID    uuid.UUID     `json:"subscriptionId"    gorm:"column:subscription_id;type:uuid;not null"`
	ProviderInvoiceID string        `json:"providerInvoiceId" gorm:"column:provider_invoice_id;type:text;not null"`
	AmountCents       int64         `json:"amountCents"       gorm:"column:amount_cents;type:bigint;not null"`
	StorageGB         int           `json:"storageGb"         gorm:"column:storage_gb;type:int;not null"`
	PeriodStart       time.Time     `json:"periodStart"       gorm:"column:period_start;type:timestamptz;not null"`
	PeriodEnd         time.Time     `json:"periodEnd"         gorm:"column:period_end;type:timestamptz;not null"`
	Status            InvoiceStatus `json:"status"            gorm:"column:status;type:text;not null"`
	PaidAt            *time.Time    `json:"paidAt,omitempty"  gorm:"column:paid_at;type:timestamptz"`
	CreatedAt         time.Time     `json:"createdAt"         gorm:"column:created_at;type:timestamptz;not null"`
}

func (Invoice) TableName() string {
	return "invoices"
}
