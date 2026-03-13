package backups_services

import (
	"sync"
	"sync/atomic"

	audit_logs "databasus-backend/internal/features/audit_logs"
	"databasus-backend/internal/features/backups/backups/backuping"
	backups_core "databasus-backend/internal/features/backups/backups/core"
	backups_download "databasus-backend/internal/features/backups/backups/download"
	"databasus-backend/internal/features/backups/backups/usecases"
	backups_config "databasus-backend/internal/features/backups/config"
	"databasus-backend/internal/features/databases"
	encryption_secrets "databasus-backend/internal/features/encryption/secrets"
	"databasus-backend/internal/features/notifiers"
	"databasus-backend/internal/features/storages"
	task_cancellation "databasus-backend/internal/features/tasks/cancellation"
	workspaces_services "databasus-backend/internal/features/workspaces/services"
	"databasus-backend/internal/util/encryption"
	"databasus-backend/internal/util/logger"
)

var taskCancelManager = task_cancellation.GetTaskCancelManager()

var backupService = &BackupService{
	databases.GetDatabaseService(),
	storages.GetStorageService(),
	backups_core.GetBackupRepository(),
	notifiers.GetNotifierService(),
	notifiers.GetNotifierService(),
	backups_config.GetBackupConfigService(),
	encryption_secrets.GetSecretKeyService(),
	encryption.GetFieldEncryptor(),
	usecases.GetCreateBackupUsecase(),
	logger.GetLogger(),
	[]backups_core.BackupRemoveListener{},
	workspaces_services.GetWorkspaceService(),
	audit_logs.GetAuditLogService(),
	taskCancelManager,
	backups_download.GetDownloadTokenService(),
	backuping.GetBackupsScheduler(),
	backuping.GetBackupCleaner(),
}

func GetBackupService() *BackupService {
	return backupService
}

var walService = &PostgreWalBackupService{
	backups_config.GetBackupConfigService(),
	backups_core.GetBackupRepository(),
	encryption.GetFieldEncryptor(),
	encryption_secrets.GetSecretKeyService(),
	logger.GetLogger(),
	backupService,
}

func GetWalService() *PostgreWalBackupService {
	return walService
}

var (
	setupOnce sync.Once
	isSetup   atomic.Bool
)

func SetupDependencies() {
	wasAlreadySetup := isSetup.Load()

	setupOnce.Do(func() {
		backups_config.
			GetBackupConfigService().
			SetDatabaseStorageChangeListener(backupService)

		databases.GetDatabaseService().AddDbRemoveListener(backupService)
		databases.GetDatabaseService().AddDbCopyListener(backups_config.GetBackupConfigService())

		isSetup.Store(true)
	})

	if wasAlreadySetup {
		logger.GetLogger().Warn("SetupDependencies called multiple times, ignoring subsequent call")
	}
}
