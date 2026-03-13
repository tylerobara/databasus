package system_healthcheck

import (
	"databasus-backend/internal/features/backups/backups/backuping"
	"databasus-backend/internal/features/disk"
)

var healthcheckService = &HealthcheckService{
	disk.GetDiskService(),
	backuping.GetBackupsScheduler(),
	backuping.GetBackuperNode(),
}

var healthcheckController = &HealthcheckController{
	healthcheckService,
}

func GetHealthcheckController() *HealthcheckController {
	return healthcheckController
}
