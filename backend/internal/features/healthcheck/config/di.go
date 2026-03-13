package healthcheck_config

import (
	"sync"
	"sync/atomic"

	"databasus-backend/internal/features/audit_logs"
	"databasus-backend/internal/features/databases"
	workspaces_services "databasus-backend/internal/features/workspaces/services"
	"databasus-backend/internal/util/logger"
)

var (
	healthcheckConfigRepository = &HealthcheckConfigRepository{}
	healthcheckConfigService    = &HealthcheckConfigService{
		databases.GetDatabaseService(),
		healthcheckConfigRepository,
		workspaces_services.GetWorkspaceService(),
		audit_logs.GetAuditLogService(),
		logger.GetLogger(),
	}
)

var healthcheckConfigController = &HealthcheckConfigController{
	healthcheckConfigService,
}

func GetHealthcheckConfigService() *HealthcheckConfigService {
	return healthcheckConfigService
}

func GetHealthcheckConfigController() *HealthcheckConfigController {
	return healthcheckConfigController
}

var (
	setupOnce sync.Once
	isSetup   atomic.Bool
)

func SetupDependencies() {
	wasAlreadySetup := isSetup.Load()

	setupOnce.Do(func() {
		databases.
			GetDatabaseService().
			AddDbCreationListener(healthcheckConfigService)

		isSetup.Store(true)
	})

	if wasAlreadySetup {
		logger.GetLogger().Warn("SetupDependencies called multiple times, ignoring subsequent call")
	}
}
