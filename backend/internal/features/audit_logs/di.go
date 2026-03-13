package audit_logs

import (
	"sync"
	"sync/atomic"

	users_services "databasus-backend/internal/features/users/services"
	"databasus-backend/internal/util/logger"
)

var (
	auditLogRepository = &AuditLogRepository{}
	auditLogService    = &AuditLogService{
		auditLogRepository,
		logger.GetLogger(),
	}
)

var auditLogController = &AuditLogController{
	auditLogService,
}

var auditLogBackgroundService = &AuditLogBackgroundService{
	auditLogService: auditLogService,
	logger:          logger.GetLogger(),
	runOnce:         sync.Once{},
	hasRun:          atomic.Bool{},
}

func GetAuditLogService() *AuditLogService {
	return auditLogService
}

func GetAuditLogController() *AuditLogController {
	return auditLogController
}

func GetAuditLogBackgroundService() *AuditLogBackgroundService {
	return auditLogBackgroundService
}

var (
	setupOnce sync.Once
	isSetup   atomic.Bool
)

func SetupDependencies() {
	wasAlreadySetup := isSetup.Load()

	setupOnce.Do(func() {
		users_services.GetUserService().SetAuditLogWriter(auditLogService)
		users_services.GetSettingsService().SetAuditLogWriter(auditLogService)
		users_services.GetManagementService().SetAuditLogWriter(auditLogService)

		isSetup.Store(true)
	})

	if wasAlreadySetup {
		logger.GetLogger().Warn("SetupDependencies called multiple times, ignoring subsequent call")
	}
}
