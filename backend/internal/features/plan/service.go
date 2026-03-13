package plans

import (
	"log/slog"

	"github.com/google/uuid"

	"databasus-backend/internal/config"
	"databasus-backend/internal/util/period"
)

type DatabasePlanService struct {
	databasePlanRepository *DatabasePlanRepository
	logger                 *slog.Logger
}

func (s *DatabasePlanService) GetDatabasePlan(databaseID uuid.UUID) (*DatabasePlan, error) {
	plan, err := s.databasePlanRepository.GetDatabasePlan(databaseID)
	if err != nil {
		return nil, err
	}

	if plan == nil {
		s.logger.Info("no database plan found, creating default plan", "databaseID", databaseID)

		defaultPlan := s.createDefaultDatabasePlan(databaseID)

		err := s.databasePlanRepository.CreateDatabasePlan(defaultPlan)
		if err != nil {
			s.logger.Error("failed to create default database plan", "error", err)
			return nil, err
		}

		return defaultPlan, nil
	}

	return plan, nil
}

func (s *DatabasePlanService) createDefaultDatabasePlan(databaseID uuid.UUID) *DatabasePlan {
	var plan DatabasePlan

	isCloud := config.GetEnv().IsCloud
	if isCloud {
		s.logger.Info("creating default database plan for cloud", "databaseID", databaseID)

		// for playground we set limited storages enough to test,
		// but not too expensive to provide it for Databasus
		plan = DatabasePlan{
			DatabaseID:            databaseID,
			MaxBackupSizeMB:       100,  // ~ 1.5GB database
			MaxBackupsTotalSizeMB: 4000, // ~ 30 daily backups + 10 manual backups
			MaxStoragePeriod:      period.PeriodWeek,
		}
	} else {
		s.logger.Info("creating default database plan for self hosted", "databaseID", databaseID)

		// by default - everything is unlimited in self hosted mode
		plan = DatabasePlan{
			DatabaseID:            databaseID,
			MaxBackupSizeMB:       0,
			MaxBackupsTotalSizeMB: 0,
			MaxStoragePeriod:      period.PeriodForever,
		}
	}

	return &plan
}
