package billing_repositories

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	billing_models "databasus-backend/internal/features/billing/models"
	"databasus-backend/internal/storage"
)

type InvoiceRepository struct{}

func (r *InvoiceRepository) Save(invoice billing_models.Invoice) error {
	if invoice.SubscriptionID == uuid.Nil {
		return errors.New("subscription id is required")
	}

	db := storage.GetDb()

	if invoice.ID == uuid.Nil {
		invoice.ID = uuid.New()
		return db.Create(&invoice).Error
	}

	return db.Save(invoice).Error
}

func (r *InvoiceRepository) FindByProviderInvID(providerInvoiceID string) (*billing_models.Invoice, error) {
	var invoice billing_models.Invoice

	if err := storage.GetDb().Where("provider_invoice_id = ?", providerInvoiceID).
		First(&invoice).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &invoice, nil
}

func (r *InvoiceRepository) FindByDatabaseID(
	databaseID uuid.UUID,
	limit, offset int,
) ([]*billing_models.Invoice, error) {
	var invoices []*billing_models.Invoice

	if err := storage.GetDb().Joins("JOIN subscriptions ON subscriptions.id = invoices.subscription_id").
		Where("subscriptions.database_id = ?", databaseID).
		Order("invoices.created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&invoices).Error; err != nil {
		return nil, err
	}

	return invoices, nil
}

func (r *InvoiceRepository) CountByDatabaseID(databaseID uuid.UUID) (int64, error) {
	var count int64

	err := storage.GetDb().Model(&billing_models.Invoice{}).
		Joins("JOIN subscriptions ON subscriptions.id = invoices.subscription_id").
		Where("subscriptions.database_id = ?", databaseID).
		Count(&count).Error

	return count, err
}
