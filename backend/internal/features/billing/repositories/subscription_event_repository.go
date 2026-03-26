package billing_repositories

import (
	"errors"

	"github.com/google/uuid"

	billing_models "databasus-backend/internal/features/billing/models"
	"databasus-backend/internal/storage"
)

type SubscriptionEventRepository struct{}

func (r *SubscriptionEventRepository) Create(event billing_models.SubscriptionEvent) error {
	if event.SubscriptionID == uuid.Nil {
		return errors.New("subscription id is required")
	}

	event.ID = uuid.New()
	return storage.GetDb().Create(&event).Error
}

func (r *SubscriptionEventRepository) FindByDatabaseID(
	databaseID uuid.UUID,
	limit, offset int,
) ([]*billing_models.SubscriptionEvent, error) {
	var events []*billing_models.SubscriptionEvent

	if err := storage.GetDb().Joins("JOIN subscriptions ON subscriptions.id = subscription_events.subscription_id").
		Where("subscriptions.database_id = ?", databaseID).
		Order("subscription_events.created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&events).Error; err != nil {
		return nil, err
	}

	return events, nil
}

func (r *SubscriptionEventRepository) CountByDatabaseID(databaseID uuid.UUID) (int64, error) {
	var count int64

	err := storage.GetDb().Model(&billing_models.SubscriptionEvent{}).
		Joins("JOIN subscriptions ON subscriptions.id = subscription_events.subscription_id").
		Where("subscriptions.database_id = ?", databaseID).
		Count(&count).Error

	return count, err
}
