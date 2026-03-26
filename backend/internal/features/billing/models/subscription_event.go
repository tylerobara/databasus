package billing_models

import (
	"time"

	"github.com/google/uuid"
)

type SubscriptionEvent struct {
	ID              uuid.UUID             `json:"id"                        gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	SubscriptionID  uuid.UUID             `json:"subscriptionId"            gorm:"column:subscription_id;type:uuid;not null"`
	ProviderEventID *string               `json:"providerEventId,omitempty" gorm:"column:provider_event_id;type:text"`
	Type            SubscriptionEventType `json:"type"                      gorm:"column:type;type:text;not null"`

	OldStorageGB *int                `json:"oldStorageGb,omitempty" gorm:"column:old_storage_gb;type:int"`
	NewStorageGB *int                `json:"newStorageGb,omitempty" gorm:"column:new_storage_gb;type:int"`
	OldStatus    *SubscriptionStatus `json:"oldStatus,omitempty"    gorm:"column:old_status;type:text"`
	NewStatus    *SubscriptionStatus `json:"newStatus,omitempty"    gorm:"column:new_status;type:text"`

	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at;type:timestamptz;not null"`
}

func (SubscriptionEvent) TableName() string {
	return "subscription_events"
}
