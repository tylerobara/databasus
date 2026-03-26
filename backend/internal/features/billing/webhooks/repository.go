package billing_webhooks

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"databasus-backend/internal/storage"
)

type WebhookRepository struct{}

func (r *WebhookRepository) FindSuccessfulByProviderEventID(providerEventID string) (*WebhookRecord, error) {
	var record WebhookRecord

	err := storage.GetDb().
		Where("provider_event_id = ? AND processed_at IS NOT NULL", providerEventID).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		return nil, err
	}

	return &record, nil
}

func (r *WebhookRepository) Insert(record *WebhookRecord) error {
	if record.ProviderEventID == "" {
		return errors.New("provider event ID is required")
	}

	record.CreatedAt = time.Now().UTC()

	return storage.GetDb().Create(record).Error
}

func (r *WebhookRepository) MarkProcessed(requestID string) error {
	now := time.Now().UTC()

	return storage.
		GetDb().
		Model(&WebhookRecord{}).
		Where("request_id = ?", requestID).
		Update("processed_at", now).
		Error
}

func (r *WebhookRepository) MarkSkipped(requestID string) error {
	now := time.Now().UTC()

	return storage.
		GetDb().
		Model(&WebhookRecord{}).
		Where("request_id = ?", requestID).
		Updates(map[string]any{
			"is_skipped":   true,
			"processed_at": now,
		}).
		Error
}

func (r *WebhookRepository) MarkError(requestID, errMsg string) error {
	return storage.
		GetDb().
		Model(&WebhookRecord{}).
		Where("request_id = ?", requestID).
		Update("error", errMsg).
		Error
}
