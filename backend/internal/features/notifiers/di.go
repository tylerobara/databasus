package notifiers

import (
	"sync"
	"sync/atomic"

	audit_logs "databasus-backend/internal/features/audit_logs"
	workspaces_services "databasus-backend/internal/features/workspaces/services"
	"databasus-backend/internal/util/encryption"
	"databasus-backend/internal/util/logger"
)

var (
	notifierRepository = &NotifierRepository{}
	notifierService    = &NotifierService{
		notifierRepository,
		logger.GetLogger(),
		workspaces_services.GetWorkspaceService(),
		audit_logs.GetAuditLogService(),
		encryption.GetFieldEncryptor(),
		nil,
	}
)

var notifierController = &NotifierController{
	notifierService,
	workspaces_services.GetWorkspaceService(),
}

func GetNotifierController() *NotifierController {
	return notifierController
}

func GetNotifierService() *NotifierService {
	return notifierService
}

func GetNotifierRepository() *NotifierRepository {
	return notifierRepository
}

var (
	setupOnce sync.Once
	isSetup   atomic.Bool
)

func SetupDependencies() {
	wasAlreadySetup := isSetup.Load()

	setupOnce.Do(func() {
		workspaces_services.GetWorkspaceService().AddWorkspaceDeletionListener(notifierService)

		isSetup.Store(true)
	})

	if wasAlreadySetup {
		logger.GetLogger().Warn("SetupDependencies called multiple times, ignoring subsequent call")
	}
}
