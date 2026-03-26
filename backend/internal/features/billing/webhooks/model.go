package billing_webhooks

import (
	"time"

	"github.com/google/uuid"

	billing_provider "databasus-backend/internal/features/billing/provider"
)

type WebhookRecord struct {
	RequestID       uuid.UUID                     `gorm:"column:request_id;primaryKey;type:uuid;default:gen_random_uuid()"`
	ProviderName    billing_provider.ProviderName `gorm:"column:provider_name;type:text;not null"`
	EventType       string                        `gorm:"column:event_type;type:text;not null"`
	ProviderEventID string                        `gorm:"column:provider_event_id;type:text;not null;index"`
	RawPayload      string                        `gorm:"column:raw_payload;type:text;not null"`
	ProcessedAt     *time.Time                    `gorm:"column:processed_at"`
	IsSkipped       bool                          `gorm:"column:is_skipped;not null;default:false"`
	Error           *string                       `gorm:"column:error"`
	CreatedAt       time.Time                     `gorm:"column:created_at;not null"`
}

func (WebhookRecord) TableName() string {
	return "webhook_records"
}
