package billing_models

import (
	"time"

	"github.com/google/uuid"

	"databasus-backend/internal/config"
)

type Subscription struct {
	ID         uuid.UUID          `json:"id"         gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	DatabaseID uuid.UUID          `json:"databaseId" gorm:"column:database_id;type:uuid;not null"`
	Status     SubscriptionStatus `json:"status"     gorm:"column:status;type:text;not null"`

	StorageGB        int  `json:"storageGb"                  gorm:"column:storage_gb;type:int;not null"`
	PendingStorageGB *int `json:"pendingStorageGb,omitempty" gorm:"column:pending_storage_gb;type:int"`

	CurrentPeriodStart time.Time  `json:"currentPeriodStart"   gorm:"column:current_period_start;type:timestamptz;not null"`
	CurrentPeriodEnd   time.Time  `json:"currentPeriodEnd"     gorm:"column:current_period_end;type:timestamptz;not null"`
	CanceledAt         *time.Time `json:"canceledAt,omitempty" gorm:"column:canceled_at;type:timestamptz"`

	DataRetentionGracePeriodUntil *time.Time `json:"dataRetentionGracePeriodUntil,omitempty" gorm:"column:data_retention_grace_period_until;type:timestamptz"`

	ProviderName       *string `json:"providerName,omitempty"       gorm:"column:provider_name;type:text"`
	ProviderSubID      *string `json:"providerSubId,omitempty"      gorm:"column:provider_sub_id;type:text"`
	ProviderCustomerID *string `json:"providerCustomerId,omitempty" gorm:"column:provider_customer_id;type:text"`

	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"column:updated_at;type:timestamptz;not null"`
}

func (Subscription) TableName() string {
	return "subscriptions"
}

func (s *Subscription) PriceCents() int64 {
	return int64(s.StorageGB) * config.GetEnv().PricePerGBCents
}

// CanCreateNewBackups - whether it is allowed to create new backups
// by scheduler or for user manually. Clarification: in grace period
// user can download, delete and restore backups, but cannot create new ones
func (s *Subscription) CanCreateNewBackups() bool {
	switch s.Status {
	case StatusActive, StatusPastDue:
		return true
	case StatusTrial, StatusCanceled:
		return time.Now().Before(s.CurrentPeriodEnd)
	case StatusExpired:
		return false
	default:
		panic("unknown subscription status")
	}
}

func (s *Subscription) GetBackupsStorageGB() int {
	switch s.Status {
	case StatusActive, StatusPastDue, StatusCanceled:
		return s.StorageGB
	case StatusTrial:
		if time.Now().Before(s.CurrentPeriodEnd) {
			return s.StorageGB
		}

		return 0
	case StatusExpired:
		return 0
	default:
		panic("unknown subscription status")
	}
}
