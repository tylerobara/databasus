package plans

import (
	"github.com/google/uuid"

	"databasus-backend/internal/storage"
)

type DatabasePlanRepository struct{}

func (r *DatabasePlanRepository) GetDatabasePlan(databaseID uuid.UUID) (*DatabasePlan, error) {
	var databasePlan DatabasePlan

	if err := storage.GetDb().Where("database_id = ?", databaseID).First(&databasePlan).Error; err != nil {
		if err.Error() == "record not found" {
			return nil, nil
		}

		return nil, err
	}

	return &databasePlan, nil
}

func (r *DatabasePlanRepository) CreateDatabasePlan(databasePlan *DatabasePlan) error {
	return storage.GetDb().Create(&databasePlan).Error
}
