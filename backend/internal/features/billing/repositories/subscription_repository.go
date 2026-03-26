package billing_repositories

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	billing_models "databasus-backend/internal/features/billing/models"
	"databasus-backend/internal/storage"
)

type SubscriptionRepository struct{}

func (r *SubscriptionRepository) Save(sub billing_models.Subscription) error {
	db := storage.GetDb()

	if sub.ID == uuid.Nil {
		sub.ID = uuid.New()
		return db.Create(&sub).Error
	}

	return db.Save(&sub).Error
}

func (r *SubscriptionRepository) FindByID(id uuid.UUID) (*billing_models.Subscription, error) {
	var sub billing_models.Subscription

	if err := storage.GetDb().Where("id = ?", id).First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &sub, nil
}

func (r *SubscriptionRepository) FindByDatabaseIDAndStatuses(
	databaseID uuid.UUID,
	stauses []billing_models.SubscriptionStatus,
) ([]*billing_models.Subscription, error) {
	var subs []*billing_models.Subscription

	if err := storage.GetDb().Where("database_id = ? AND status IN ?", databaseID, stauses).
		Find(&subs).Error; err != nil {
		return nil, err
	}

	return subs, nil
}

func (r *SubscriptionRepository) FindLatestByDatabaseID(databaseID uuid.UUID) (*billing_models.Subscription, error) {
	var sub billing_models.Subscription

	if err := storage.GetDb().
		Where("database_id = ?", databaseID).
		Order("created_at DESC").
		First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &sub, nil
}

func (r *SubscriptionRepository) FindByProviderSubID(providerSubID string) (*billing_models.Subscription, error) {
	var sub billing_models.Subscription

	if err := storage.GetDb().Where("provider_sub_id = ?", providerSubID).
		First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &sub, nil
}

func (r *SubscriptionRepository) FindByStatuses(
	statuses []billing_models.SubscriptionStatus,
) ([]billing_models.Subscription, error) {
	var subs []billing_models.Subscription

	if err := storage.GetDb().Where("status IN ?", statuses).Find(&subs).Error; err != nil {
		return nil, err
	}

	return subs, nil
}

func (r *SubscriptionRepository) FindCanceledWithEndedGracePeriod(
	now time.Time,
) ([]billing_models.Subscription, error) {
	var subs []billing_models.Subscription

	if err := storage.GetDb().
		Where("status = ? AND data_retention_grace_period_until < ?", billing_models.StatusCanceled, now).
		Find(&subs).
		Error; err != nil {
		return nil, err
	}

	return subs, nil
}

func (r *SubscriptionRepository) FindExpiredTrials(now time.Time) ([]billing_models.Subscription, error) {
	var subs []billing_models.Subscription

	if err := storage.GetDb().Where("status = ? AND current_period_end < ?", billing_models.StatusTrial, now).
		Find(&subs).Error; err != nil {
		return nil, err
	}

	return subs, nil
}
