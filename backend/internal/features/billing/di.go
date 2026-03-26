package billing

import (
	"sync"
	"sync/atomic"

	billing_repositories "databasus-backend/internal/features/billing/repositories"
	"databasus-backend/internal/features/databases"
	workspaces_services "databasus-backend/internal/features/workspaces/services"
	"databasus-backend/internal/util/logger"
)

var (
	billingService = &BillingService{
		&billing_repositories.SubscriptionRepository{},
		&billing_repositories.SubscriptionEventRepository{},
		&billing_repositories.InvoiceRepository{},
		nil, // billing provider will be set later to avoid circular dependency
		workspaces_services.GetWorkspaceService(),
		*databases.GetDatabaseService(),
		sync.Once{},
		atomic.Bool{},
	}
	billingController = &BillingController{billingService}

	setupOnce sync.Once
	isSetup   atomic.Bool
)

func GetBillingService() *BillingService {
	return billingService
}

func GetBillingController() *BillingController {
	return billingController
}

func SetupDependencies() {
	wasAlreadySetup := isSetup.Load()

	setupOnce.Do(func() {
		databases.GetDatabaseService().AddDbCreationListener(billingService)
		isSetup.Store(true)
	})

	if wasAlreadySetup {
		logger.GetLogger().Warn("billing.SetupDependencies called multiple times, ignoring subsequent call")
	}
}
