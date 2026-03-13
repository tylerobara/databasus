package restores

import (
	"sync"
	"sync/atomic"

	audit_logs "databasus-backend/internal/features/audit_logs"
	"databasus-backend/internal/features/backups/backups/backuping"
	backups_services "databasus-backend/internal/features/backups/backups/services"
	backups_config "databasus-backend/internal/features/backups/config"
	"databasus-backend/internal/features/databases"
	"databasus-backend/internal/features/disk"
	restores_core "databasus-backend/internal/features/restores/core"
	"databasus-backend/internal/features/restores/usecases"
	"databasus-backend/internal/features/storages"
	tasks_cancellation "databasus-backend/internal/features/tasks/cancellation"
	workspaces_services "databasus-backend/internal/features/workspaces/services"
	"databasus-backend/internal/util/encryption"
	"databasus-backend/internal/util/logger"
)

var (
	restoreRepository = &restores_core.RestoreRepository{}
	restoreService    = &RestoreService{
		backups_services.GetBackupService(),
		restoreRepository,
		storages.GetStorageService(),
		backups_config.GetBackupConfigService(),
		usecases.GetRestoreBackupUsecase(),
		databases.GetDatabaseService(),
		logger.GetLogger(),
		workspaces_services.GetWorkspaceService(),
		audit_logs.GetAuditLogService(),
		encryption.GetFieldEncryptor(),
		disk.GetDiskService(),
		tasks_cancellation.GetTaskCancelManager(),
	}
)

var restoreController = &RestoreController{
	restoreService,
}

func GetRestoreController() *RestoreController {
	return restoreController
}

var (
	setupOnce sync.Once
	isSetup   atomic.Bool
)

func SetupDependencies() {
	wasAlreadySetup := isSetup.Load()

	setupOnce.Do(func() {
		backups_services.GetBackupService().AddBackupRemoveListener(restoreService)
		backuping.GetBackupCleaner().AddBackupRemoveListener(restoreService)

		isSetup.Store(true)
	})

	if wasAlreadySetup {
		logger.GetLogger().Warn("SetupDependencies called multiple times, ignoring subsequent call")
	}
}
