package billing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"databasus-backend/internal/config"
	billing_models "databasus-backend/internal/features/billing/models"
	billing_provider "databasus-backend/internal/features/billing/provider"
	billing_repositories "databasus-backend/internal/features/billing/repositories"
	"databasus-backend/internal/features/databases"
	users_models "databasus-backend/internal/features/users/models"
	workspaces_services "databasus-backend/internal/features/workspaces/services"
	"databasus-backend/internal/storage"
	"databasus-backend/internal/util/logger"
)

const billingTickerInterval = 5 * time.Minute

type BillingService struct {
	subscriptionRepository      *billing_repositories.SubscriptionRepository
	subscriptionEventRepository *billing_repositories.SubscriptionEventRepository
	invoiceRepository           *billing_repositories.InvoiceRepository

	billingProvider  billing_provider.BillingProvider
	workspaceService *workspaces_services.WorkspaceService
	databaseService  databases.DatabaseService

	runOnce sync.Once
	hasRun  atomic.Bool
}

func (s *BillingService) Run(ctx context.Context, logger slog.Logger) {
	wasAlreadyRun := s.hasRun.Load()

	s.runOnce.Do(func() {
		s.hasRun.Store(true)

		ticker := time.NewTicker(billingTickerInterval)
		defer ticker.Stop()

		// Run immediately on start
		expiredSubsLog := logger.With("task_name", "process_expired_subscriptions")
		if err := s.processExpiredSubscriptions(expiredSubsLog); err != nil {
			expiredSubsLog.Error("failed to process expired subscriptions", "error", err)
		}

		expiredTrialsLog := logger.With("task_name", "process_expired_trials")
		if err := s.processExpiredTrials(expiredTrialsLog); err != nil {
			expiredTrialsLog.Error("failed to process expired trials", "error", err)
		}

		reconcileSubsLog := logger.With("task_name", "reconcile_subscriptions")
		if err := s.reconcileSubscriptions(reconcileSubsLog); err != nil {
			reconcileSubsLog.Error("failed to reconcile subscriptions", "error", err)
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.processExpiredSubscriptions(expiredSubsLog); err != nil {
					expiredSubsLog.Error("failed to process expired subscriptions", "error", err)
				}

				if err := s.processExpiredTrials(expiredTrialsLog); err != nil {
					expiredTrialsLog.Error("failed to process expired trials", "error", err)
				}

				if err := s.reconcileSubscriptions(reconcileSubsLog); err != nil {
					reconcileSubsLog.Error("failed to reconcile subscriptions", "error", err)
				}
			}
		}
	})

	if wasAlreadyRun {
		panic(fmt.Sprintf("%T.Run() called multiple times", s))
	}
}

func (s *BillingService) SetBillingProvider(provider billing_provider.BillingProvider) {
	s.billingProvider = provider
}

func (s *BillingService) OnDatabaseCreated(databaseID uuid.UUID) {
	scopedLog := logger.GetLogger().With("database_id", databaseID)
	if err := s.createTrialSubscription(scopedLog, databaseID); err != nil {
		scopedLog.Error("failed to create trial subscription", "error", err)
	}
}

func (s *BillingService) CreateSubscription(
	logger *slog.Logger,
	user *users_models.User,
	databaseID uuid.UUID,
	storageGB int,
) (checkoutURL string, err error) {
	logger.Debug(fmt.Sprintf("creating subscription for storage %d GB", storageGB))

	if err := s.validateDatabaseAccess(logger, user, databaseID); err != nil {
		return "", err
	}

	// validate size
	if storageGB < config.GetEnv().MinStorageGB || storageGB > config.GetEnv().MaxStorageGB {
		logger.Error(
			fmt.Sprintf(
				"invalid storage requested: %d GB (allowed %d - %d)",
				storageGB,
				config.GetEnv().MinStorageGB,
				config.GetEnv().MaxStorageGB,
			),
		)
		return "", ErrInvalidStorage
	}

	// validate active subs (trial is allowed — it will be expired when the paid subscription activates)
	existingSub, err := s.getActiveSubscription(logger, databaseID)
	if err != nil && !errors.Is(err, ErrNoActiveSubscription) {
		logger.Error("failed to check existing subscriptions", "error", err)
		return "", err
	}

	if existingSub != nil && existingSub.Status != billing_models.StatusTrial {
		logger.Error("active subscription already exists")
		return "", ErrAlreadySubscribed
	}

	// create checkout session
	url, err := s.billingProvider.CreateCheckoutSession(logger, billing_provider.CheckoutRequest{
		DatabaseID: databaseID,
		Email:      user.Email,
		StorageGB:  storageGB,
		SuccessURL: "https://app.databasus.com/?payment_success=" + databaseID.String(),
		CancelURL:  "https://app.databasus.com/?payment_failed=" + databaseID.String(),
	})
	if err != nil {
		logger.Error("failed to create checkout session", "error", err)
		return "", err
	}

	logger.Debug("checkout session created", "url", url)

	return url, nil
}

// SyncSubscriptionFromProvider - syncs subscription state from provider webhook (subscription.updated).
// Always applies quantity, status, and period from the webhook. Determines event type by comparing
// old vs new storage: upgraded, downgraded, or new billing cycle started.
// Important note: this is not the same as payment. Payments come separately via RecordPaymentSuccess.
func (s *BillingService) SyncSubscriptionFromProvider(
	logger *slog.Logger,
	subscription *billing_models.Subscription,
	webhookEvent billing_models.WebhookEvent,
) error {
	logger = logger.With("subscription_id", subscription.ID, "database_id", subscription.DatabaseID)
	logger.Debug("syncing subscription state from provider")

	return storage.GetDb().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", subscription.ID).First(&subscription).Error; err != nil {
			logger.Error("failed to lock subscription for provider sync", "error", err)
			return err
		}

		oldStorageGB := subscription.StorageGB
		oldStatus := subscription.Status

		subscription.StorageGB = webhookEvent.QuantityGB
		subscription.PendingStorageGB = nil
		subscription.Status = webhookEvent.Status
		subscription.UpdatedAt = time.Now().UTC()

		if webhookEvent.PeriodStart != nil {
			subscription.CurrentPeriodStart = *webhookEvent.PeriodStart
		}
		if webhookEvent.PeriodEnd != nil {
			subscription.CurrentPeriodEnd = *webhookEvent.PeriodEnd
		}

		if err := tx.Save(&subscription).Error; err != nil {
			logger.Error("failed to save subscription for provider sync", "error", err)
			return err
		}

		eventType := billing_models.EventNewBillingCycleStarted
		if oldStorageGB < webhookEvent.QuantityGB {
			eventType = billing_models.EventUpgraded
		} else if oldStorageGB > webhookEvent.QuantityGB {
			eventType = billing_models.EventDowngraded
		}

		event := billing_models.SubscriptionEvent{
			ID:              uuid.New(),
			SubscriptionID:  subscription.ID,
			Type:            eventType,
			OldStatus:       &oldStatus,
			NewStatus:       &subscription.Status,
			ProviderEventID: &webhookEvent.ProviderEventID,
			CreatedAt:       time.Now().UTC(),
		}

		if oldStorageGB != subscription.StorageGB {
			event.OldStorageGB = &oldStorageGB
			event.NewStorageGB = &subscription.StorageGB
		}

		if err := tx.Create(&event).Error; err != nil {
			logger.Error("failed to create subscription event for provider sync", "error", err)
			return err
		}

		logger.Info(
			fmt.Sprintf(
				"subscription synced from provider: %s -> %s, %d GB -> %d GB, period until %s",
				string(oldStatus),
				string(subscription.Status),
				oldStorageGB,
				subscription.StorageGB,
				subscription.CurrentPeriodEnd.Format(time.RFC3339),
			),
		)

		return nil
	})
}

func (s *BillingService) CancelSubscription(
	logger *slog.Logger,
	sub *billing_models.Subscription,
	webhookEvent billing_models.WebhookEvent,
) error {
	logger = logger.With("subscription_id", sub.ID, "database_id", sub.DatabaseID)
	logger.Debug(fmt.Sprintf("handling subscription cancel (was %s)", string(sub.Status)))

	return storage.GetDb().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", sub.ID).First(&sub).Error; err != nil {
			logger.Error("failed to lock subscription for cancel", "error", err)
			return err
		}

		now := time.Now().UTC()
		oldStatus := sub.Status
		sub.Status = billing_models.StatusCanceled
		sub.CanceledAt = &now
		sub.UpdatedAt = now

		if oldStatus == billing_models.StatusPastDue {
			// past_due -> canceled - immediae cancelation. User
			// is not paying so cannot create new backups (but
			// applying grace period to allow downloading\restore)
			sub.CurrentPeriodEnd = now
			retention := now.Add(config.GetEnv().GracePeriod)
			sub.DataRetentionGracePeriodUntil = &retention
		} else {
			// subscription is active, but will be canceled in the
			// end of the billing period. User allowed to do any
			// actions. Grace period will be applied after end of the billing period,
			// when subscription will be moved to expired status
			retention := sub.CurrentPeriodEnd.Add(config.GetEnv().GracePeriod)
			sub.DataRetentionGracePeriodUntil = &retention
		}

		if err := tx.Save(&sub).Error; err != nil {
			logger.Error("failed to save subscription for cancel", "error", err)
			return err
		}

		if err := tx.Create(&billing_models.SubscriptionEvent{
			ID:              uuid.New(),
			SubscriptionID:  sub.ID,
			Type:            billing_models.EventCanceled,
			OldStatus:       &oldStatus,
			NewStatus:       &sub.Status,
			ProviderEventID: &webhookEvent.ProviderEventID,
			CreatedAt:       time.Now().UTC(),
		}).Error; err != nil {
			logger.Error("failed to create subscription event for cancel", "error", err)
			return err
		}

		if oldStatus == billing_models.StatusPastDue {
			logger.Info(
				fmt.Sprintf(
					"subscription canceled immediately (was from past_due), applying grace period until %s",
					sub.DataRetentionGracePeriodUntil.Format(time.RFC3339),
				),
			)
		} else {
			logger.Info(
				fmt.Sprintf(
					"subscription cancelation scheduled at the end of billing period %s, applying grace period until %s",
					sub.CurrentPeriodEnd.Format(time.RFC3339),
					sub.DataRetentionGracePeriodUntil.Format(time.RFC3339),
				),
			)
		}

		return nil
	})
}

func (s *BillingService) ReactivateSubscription(
	logger *slog.Logger,
	sub *billing_models.Subscription,
	webhookEvent billing_models.WebhookEvent,
) error {
	logger = logger.With("subscription_id", sub.ID, "database_id", sub.DatabaseID)
	logger.Debug("handling subscription reactivation (undo cancel)")

	return storage.GetDb().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", sub.ID).First(&sub).Error; err != nil {
			logger.Error("failed to lock subscription for reactivation", "error", err)
			return err
		}

		if sub.Status != billing_models.StatusCanceled {
			logger.Info(
				fmt.Sprintf(
					"subscription is no longer canceled (status: %s), skipping reactivation",
					string(sub.Status),
				),
			)
			return nil
		}

		oldStatus := sub.Status
		sub.Status = webhookEvent.Status
		sub.CanceledAt = nil
		sub.DataRetentionGracePeriodUntil = nil
		sub.StorageGB = webhookEvent.QuantityGB
		sub.PendingStorageGB = nil
		sub.UpdatedAt = time.Now().UTC()

		if webhookEvent.PeriodStart != nil {
			sub.CurrentPeriodStart = *webhookEvent.PeriodStart
		}
		if webhookEvent.PeriodEnd != nil {
			sub.CurrentPeriodEnd = *webhookEvent.PeriodEnd
		}

		if err := tx.Save(&sub).Error; err != nil {
			logger.Error("failed to save subscription for reactivation", "error", err)
			return err
		}

		if err := tx.Create(&billing_models.SubscriptionEvent{
			ID:              uuid.New(),
			SubscriptionID:  sub.ID,
			Type:            billing_models.EventReactivated,
			OldStatus:       &oldStatus,
			NewStatus:       &sub.Status,
			ProviderEventID: &webhookEvent.ProviderEventID,
			CreatedAt:       time.Now().UTC(),
		}).Error; err != nil {
			logger.Error("failed to create subscription event for reactivation", "error", err)
			return err
		}

		logger.Info(fmt.Sprintf("subscription reactivated: %s -> %s", string(oldStatus), string(sub.Status)))

		return nil
	})
}

func (s *BillingService) GetPortalURL(
	logger *slog.Logger,
	user *users_models.User,
	subscriptionID uuid.UUID,
) (string, error) {
	logger.Debug("getting billing portal URL")

	subscription, err := s.getSubscriptionWithAccessCheck(logger, user, subscriptionID)
	if err != nil {
		return "", err
	}

	logger = logger.With("database_id", subscription.DatabaseID)

	if subscription.Status != billing_models.StatusActive &&
		subscription.Status != billing_models.StatusPastDue &&
		subscription.Status != billing_models.StatusCanceled {
		logger.Error("subscription is not active", "status", subscription.Status)
		return "", fmt.Errorf("subscription is not active, past due, or canceled")
	}

	returnURL := "https://app.databasus.com"
	url, err := s.billingProvider.CreatePortalSession(logger, *subscription.ProviderCustomerID, returnURL)
	if err != nil {
		logger.Error("failed to create portal session", "error", err)
		return "", err
	}

	logger.Debug("billing portal session created", "url", url)

	return url, nil
}

func (s *BillingService) GetSubscriptionEvents(
	logger *slog.Logger,
	user *users_models.User,
	subscriptionID uuid.UUID,
	limit, offset int,
) (*GetSubscriptionEventsResponse, error) {
	subscription, err := s.getSubscriptionWithAccessCheck(logger, user, subscriptionID)
	if err != nil {
		return nil, err
	}

	logger = logger.With("database_id", subscription.DatabaseID)

	limit = normalizePaginationLimit(limit)
	offset = max(offset, 0)

	events, err := s.subscriptionEventRepository.FindByDatabaseID(subscription.DatabaseID, limit, offset)
	if err != nil {
		logger.Error("failed to get subscription events", "error", err)
		return nil, err
	}

	total, err := s.subscriptionEventRepository.CountByDatabaseID(subscription.DatabaseID)
	if err != nil {
		logger.Error("failed to count subscription events", "error", err)
		return nil, err
	}

	return &GetSubscriptionEventsResponse{
		Events: events,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (s *BillingService) GetSubscriptionInvoices(
	logger *slog.Logger,
	user *users_models.User,
	subscriptionID uuid.UUID,
	limit, offset int,
) (*GetInvoicesResponse, error) {
	subscription, err := s.getSubscriptionWithAccessCheck(logger, user, subscriptionID)
	if err != nil {
		return nil, err
	}

	logger = logger.With("database_id", subscription.DatabaseID)

	limit = normalizePaginationLimit(limit)
	offset = max(offset, 0)

	invoices, err := s.invoiceRepository.FindByDatabaseID(subscription.DatabaseID, limit, offset)
	if err != nil {
		logger.Error("failed to get subscription invoices", "error", err)
		return nil, err
	}

	total, err := s.invoiceRepository.CountByDatabaseID(subscription.DatabaseID)
	if err != nil {
		logger.Error("failed to count subscription invoices", "error", err)
		return nil, err
	}

	return &GetInvoicesResponse{
		Invoices: invoices,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}, nil
}

func normalizePaginationLimit(limit int) int {
	if limit <= 0 || limit > 1000 {
		return 100
	}

	return limit
}

func (s *BillingService) GetSubscriptionByDatabaseID(
	logger *slog.Logger,
	user *users_models.User,
	databaseID uuid.UUID,
) (*billing_models.Subscription, error) {
	logger = logger.With("database_id", databaseID)
	logger.Debug("getting subscription by database ID")

	if err := s.validateDatabaseAccess(logger, user, databaseID); err != nil {
		return nil, err
	}

	subscription, err := s.GetSubscription(logger, databaseID)
	if err != nil {
		logger.Error("failed to get subscription", "error", err)
		return nil, err
	}

	return subscription, nil
}

func (s *BillingService) RecordPaymentFailed(
	logger *slog.Logger,
	subscription *billing_models.Subscription,
	event billing_models.WebhookEvent,
) error {
	logger = logger.With(
		"subscription_id", subscription.ID,
		"database_id", subscription.DatabaseID,
		"provider_invoice_id", event.ProviderInvoiceID,
	)
	logger.Debug("recording payment failure for subscription")

	return storage.GetDb().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", subscription.ID).First(&subscription).Error; err != nil {
			logger.Error("failed to lock subscription for payment failure", "error", err)
			return err
		}

		oldStatus := subscription.Status
		subscription.Status = billing_models.StatusPastDue
		subscription.UpdatedAt = time.Now().UTC()

		if err := tx.Save(&subscription).Error; err != nil {
			logger.Error("failed to save subscription for payment failure", "error", err)
			return err
		}

		if err := tx.Create(&billing_models.SubscriptionEvent{
			ID:              uuid.New(),
			SubscriptionID:  subscription.ID,
			Type:            billing_models.EventPastDue,
			OldStatus:       &oldStatus,
			NewStatus:       &subscription.Status,
			ProviderEventID: &event.ProviderEventID,
			CreatedAt:       time.Now().UTC(),
		}).Error; err != nil {
			logger.Error("failed to create subscription event for payment failure", "error", err)
			return err
		}

		logger.Info(fmt.Sprintf("subscription marked as past_due due to payment failure (was %s)", string(oldStatus)))

		return nil
	})
}

func (s *BillingService) RecordDispute(logger *slog.Logger, event billing_models.WebhookEvent) error {
	logger = logger.With("provider_invoice_id", event.ProviderInvoiceID)
	logger.Debug("recording dispute for subscription")

	invoice, err := s.invoiceRepository.FindByProviderInvID(event.ProviderInvoiceID)
	if err != nil {
		logger.Error("failed to find invoice for dispute", "error", err)
		return err
	}

	subscription, err := s.subscriptionRepository.FindByID(invoice.SubscriptionID)
	if err != nil {
		logger.Error("failed to find subscription for dispute", "error", err)
		return err
	}

	logger = logger.With(
		"subscription_id", subscription.ID,
		"database_id", subscription.DatabaseID,
		"invoice_id", invoice.ID,
	)

	return storage.GetDb().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", subscription.ID).First(&subscription).Error; err != nil {
			logger.Error("failed to lock subscription for dispute", "error", err)
			return err
		}

		oldStatus := subscription.Status
		subscription.Status = billing_models.StatusCanceled
		subscription.UpdatedAt = time.Now().UTC()

		// Dispute does not have grace period. We provide grace period
		// for canceled subscriptions and accidentally expired

		now := time.Now().UTC()
		subscription.CanceledAt = &now
		subscription.UpdatedAt = now

		if err := tx.Save(&subscription).Error; err != nil {
			logger.Error("failed to save subscription for dispute", "error", err)
			return err
		}

		invoice.Status = billing_models.InvoiceStatusDisputed
		if err := tx.Save(&invoice).Error; err != nil {
			logger.Error("failed to save invoice for dispute", "error", err)
			return err
		}

		if err := tx.Create(&billing_models.SubscriptionEvent{
			ID:              uuid.New(),
			SubscriptionID:  subscription.ID,
			Type:            billing_models.EventDispute,
			OldStatus:       &oldStatus,
			NewStatus:       &subscription.Status,
			ProviderEventID: &event.ProviderEventID,
			CreatedAt:       time.Now().UTC(),
		}).Error; err != nil {
			logger.Error("failed to create subscription event for dispute", "error", err)
			return err
		}

		logger.Info(fmt.Sprintf("subscription canceled due to dispute (was %s)", string(oldStatus)))

		return nil
	})
}

func (s *BillingService) ChangeSubscriptionStorage(
	logger *slog.Logger,
	user *users_models.User,
	databaseID uuid.UUID,
	newStorageGB int,
) (*ChangeStorageResult, error) {
	if err := s.validateDatabaseAccess(logger, user, databaseID); err != nil {
		return nil, err
	}

	if newStorageGB < config.GetEnv().MinStorageGB || newStorageGB > config.GetEnv().MaxStorageGB {
		logger.Error(
			fmt.Sprintf(
				"invalid storage requested for change: %d GB (allowed %d - %d)",
				newStorageGB,
				config.GetEnv().MinStorageGB,
				config.GetEnv().MaxStorageGB,
			),
		)
		return nil, ErrInvalidStorage
	}

	activeSub, err := s.getActiveSubscription(logger, databaseID)
	if err != nil {
		logger.Error("failed to find active subscription for storage change", "error", err)
		return nil, err
	}

	logger.Debug(fmt.Sprintf("changing subscription storage to %d GB", newStorageGB))
	logger = logger.With("subscription_id", activeSub.ID)

	if newStorageGB == activeSub.StorageGB {
		logger.Warn("requested storage is the same as current")
		return nil, ErrNoChange
	}

	if newStorageGB > activeSub.StorageGB {
		logger.Info(
			fmt.Sprintf(
				"requested storage is greater than current, applying upgrade: %d GB -> %d GB",
				activeSub.StorageGB,
				newStorageGB,
			),
		)
		return s.applyUpgrade(logger, activeSub, newStorageGB)
	} else {
		logger.Info(
			fmt.Sprintf(
				"requested storage is less than current, applying downgrade: %d GB -> %d GB",
				activeSub.StorageGB,
				newStorageGB,
			),
		)
		return s.applyDowngrade(logger, activeSub, newStorageGB)
	}
}

func (s *BillingService) ActivateSubscription(logger *slog.Logger, webhookEvent billing_models.WebhookEvent) error {
	logger.Debug("handling subscription created")

	databaseID := webhookEvent.DatabaseID
	if databaseID == nil {
		logger.Error("database ID is missing in webhook event")
		return fmt.Errorf("database ID is missing in webhook event")
	}

	logger = logger.With("database_id", *databaseID)

	existingSubscription, err := s.subscriptionRepository.FindByProviderSubID(webhookEvent.ProviderSubscriptionID)
	if err == nil && existingSubscription != nil {
		logger.Warn("subscription already existing, idempotent skip", "subscription_id", existingSubscription.ID)
		return nil
	}

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("failed to check existing subscription for activation", "error", err)
		return err
	}

	now := time.Now().UTC()
	providerName := string(s.billingProvider.GetProviderName())

	subscription := billing_models.Subscription{
		ID:                 uuid.New(),
		DatabaseID:         *databaseID,
		Status:             billing_models.StatusActive,
		StorageGB:          webhookEvent.QuantityGB,
		CurrentPeriodStart: *webhookEvent.PeriodStart,
		CurrentPeriodEnd:   *webhookEvent.PeriodEnd,
		ProviderName:       &providerName,
		ProviderSubID:      &webhookEvent.ProviderSubscriptionID,
		ProviderCustomerID: &webhookEvent.ProviderCustomerID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	logger = logger.With(
		"subscription_id", subscription.ID,
		"provider_subscription_id", webhookEvent.ProviderSubscriptionID,
		"provider_customer_id", webhookEvent.ProviderCustomerID,
	)

	if err := storage.GetDb().Transaction(func(tx *gorm.DB) error {
		// expire any existing trial subscription for this database
		trialSubs, findErr := s.subscriptionRepository.FindByDatabaseIDAndStatuses(
			*databaseID,
			[]billing_models.SubscriptionStatus{billing_models.StatusTrial},
		)
		if findErr != nil {
			logger.Error("failed to find trial subscriptions", "error", findErr)
			return findErr
		}

		for _, trialSub := range trialSubs {
			now := time.Now().UTC()
			trialSub.Status = billing_models.StatusExpired
			trialSub.CanceledAt = &now
			trialSub.UpdatedAt = now

			if err := tx.Save(trialSub).Error; err != nil {
				logger.Error(
					"failed to expire trial subscription during activation",
					"error",
					err,
					"subscription_id",
					trialSub.ID,
				)
				return err
			}

			logger.Info("expired trial subscription during paid activation", "trial_subscription_id", trialSub.ID)
		}

		if err := tx.Create(&subscription).Error; err != nil {
			logger.Error("failed to create subscription for activation", "error", err)
			return err
		}

		newStatus := subscription.Status
		return tx.Create(&billing_models.SubscriptionEvent{
			ID:              uuid.New(),
			SubscriptionID:  subscription.ID,
			Type:            billing_models.EventCreated,
			NewStorageGB:    &subscription.StorageGB,
			NewStatus:       &newStatus,
			ProviderEventID: &webhookEvent.ProviderEventID,
		}).Error
	}); err != nil {
		logger.Error("failed to activate subscription", "error", err)
		return err
	}

	logger.Info("subscription activated", "subscription_id", subscription.ID)

	return nil
}

func (s *BillingService) RecordPaymentSuccess(
	logger *slog.Logger,
	subscription *billing_models.Subscription,
	webhookEvent billing_models.WebhookEvent,
) error {
	logger = logger.With(
		"subscription_id", subscription.ID,
		"database_id", subscription.DatabaseID,
		"provider_invoice_id", webhookEvent.ProviderInvoiceID,
	)
	logger.Debug("recording payment success for subscription")

	return storage.GetDb().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", subscription.ID).First(&subscription).Error; err != nil {
			logger.Error("failed to lock subscription for payment success", "error", err)
			return err
		}

		// if was past_due -> move back to active
		if subscription.Status == billing_models.StatusPastDue {
			oldStatus := subscription.Status
			subscription.Status = billing_models.StatusActive
			subscription.UpdatedAt = time.Now().UTC()

			if err := tx.Save(&subscription).Error; err != nil {
				logger.Error("failed to save subscription for payment success", "error", err)
				return err
			}

			if err := tx.Create(&billing_models.SubscriptionEvent{
				ID:              uuid.New(),
				SubscriptionID:  subscription.ID,
				Type:            billing_models.EventRecoveredFromPastDue,
				OldStatus:       &oldStatus,
				NewStatus:       &subscription.Status,
				ProviderEventID: &webhookEvent.ProviderEventID,
				CreatedAt:       time.Now().UTC(),
			}).Error; err != nil {
				logger.Error("failed to create subscription event for payment recovery", "error", err)
				return err
			}

			logger.Info(
				fmt.Sprintf(
					"subscription recovered from past_due due to successful payment (was %s)",
					string(oldStatus),
				),
			)
		}

		// check if invoice already exists (idempotency guard)
		var existingInvoice billing_models.Invoice
		invoiceLookupErr := tx.Where("provider_invoice_id = ?", webhookEvent.ProviderInvoiceID).
			First(&existingInvoice).Error

		if invoiceLookupErr == nil {
			logger.Info("invoice already exists, idempotent skip",
				"provider_invoice_id", webhookEvent.ProviderInvoiceID,
				"existing_invoice_id", existingInvoice.ID,
			)
			return nil
		}

		if !errors.Is(invoiceLookupErr, gorm.ErrRecordNotFound) {
			logger.Error("failed to check existing invoice", "error", invoiceLookupErr)
			return invoiceLookupErr
		}

		now := time.Now().UTC()
		invoice := billing_models.Invoice{
			ID:                uuid.New(),
			SubscriptionID:    subscription.ID,
			ProviderInvoiceID: webhookEvent.ProviderInvoiceID,
			AmountCents:       webhookEvent.AmountCents,
			Status:            billing_models.InvoiceStatusPaid,
			StorageGB:         webhookEvent.QuantityGB,
			PeriodStart:       *webhookEvent.PeriodStart,
			PeriodEnd:         *webhookEvent.PeriodEnd,
			PaidAt:            &now,
		}
		if err := tx.Create(&invoice).Error; err != nil {
			logger.Error("failed to create invoice for payment success", "error", err)
			return err
		}

		logger.Info(
			fmt.Sprintf("invoice recorded: %d cents USD for %d GB", webhookEvent.AmountCents, webhookEvent.QuantityGB),
		)

		return nil
	})
}

func (s *BillingService) GetSubscriptionByProviderSubID(
	logger *slog.Logger,
	providerSubID string,
) (*billing_models.Subscription, error) {
	logger = logger.With("provider_subscription_id", providerSubID)
	logger.Debug("getting subscription by provider subscription ID")

	subscription, err := s.subscriptionRepository.FindByProviderSubID(providerSubID)
	if err != nil {
		logger.Error("failed to find subscription by provider subscription ID", "error", err)
		return nil, err
	}

	if subscription == nil {
		return nil, ErrSubscriptionNotFound
	}

	return subscription, nil
}

func (s *BillingService) GetSubscription(
	logger *slog.Logger,
	databaseID uuid.UUID,
) (*billing_models.Subscription, error) {
	subscription, err := s.subscriptionRepository.FindLatestByDatabaseID(databaseID)
	if err != nil {
		return nil, err
	}

	if subscription == nil {
		return nil, ErrSubscriptionNotFound
	}

	return subscription, nil
}

func (s *BillingService) getActiveSubscription(
	logger *slog.Logger,
	databaseID uuid.UUID,
) (*billing_models.Subscription, error) {
	activeSubs, err := s.subscriptionRepository.FindByDatabaseIDAndStatuses(
		databaseID,
		[]billing_models.SubscriptionStatus{
			billing_models.StatusActive,
			billing_models.StatusTrial,
			billing_models.StatusPastDue,
		},
	)
	if err != nil {
		return nil, err
	}

	if len(activeSubs) == 0 {
		return nil, ErrNoActiveSubscription
	}

	if len(activeSubs) > 1 {
		logger.Error(fmt.Sprintf("multiple active subscriptions found: %d", len(activeSubs)))
	}

	return activeSubs[0], nil
}

func (s *BillingService) reconcileSubscriptions(logger *slog.Logger) error {
	logger.Debug("starting subscription reconciliation")

	subscriptions, err := s.subscriptionRepository.FindByStatuses([]billing_models.SubscriptionStatus{
		billing_models.StatusActive,
		billing_models.StatusPastDue,
	})
	if err != nil {
		logger.Error("failed to find subscriptions for reconciliation", "error", err)
		return err
	}

	for _, subscription := range subscriptions {
		scopedLog := logger.With(
			"subscription_id", subscription.ID,
			"database_id", subscription.DatabaseID,
			"provider_subscription_id", subscription.ProviderSubID,
		)

		providerSubscription, err := s.billingProvider.GetSubscription(scopedLog, *subscription.ProviderSubID)
		if err != nil {
			scopedLog.Error("failed to get subscription from billing provider during reconciliation", "error", err)
			continue
		}

		if subscription.Status != providerSubscription.Status {
			scopedLog.Error(
				fmt.Sprintf(
					"subscription status mismatch with billing provider, local: %s, provider: %s",
					subscription.Status,
					providerSubscription.Status,
				),
			)
			continue
		}

		if subscription.StorageGB != providerSubscription.QuantityGB {
			scopedLog.Error(
				fmt.Sprintf(
					"subscription storage mismatch with billing provider, local: %d GB, provider: %d GB",
					subscription.StorageGB,
					providerSubscription.QuantityGB,
				),
			)
			continue
		}
	}

	logger.Debug("subscription reconciliation completed")

	return nil
}

func (s *BillingService) processExpiredSubscriptions(logger *slog.Logger) error {
	logger.Debug("started expiring subscriptions processing")

	subsWithEndedGracePeriod, err := s.subscriptionRepository.FindCanceledWithEndedGracePeriod(time.Now().UTC())
	if err != nil {
		logger.Error("failed to find canceled subscriptions with ended grace period", "error", err)
		return err
	}

	logger.Debug(fmt.Sprintf("found %d canceled subscriptions past retention", len(subsWithEndedGracePeriod)))

	for _, subscription := range subsWithEndedGracePeriod {
		scopedLog := logger.With("subscription_id", subscription.ID, "database_id", subscription.DatabaseID)
		err = s.expireSubscription(scopedLog, &subscription)
		if err != nil {
			scopedLog.Error("failed to expire subscription", "error", err)
		}
	}

	return nil
}

func (s *BillingService) expireSubscription(logger *slog.Logger, sub *billing_models.Subscription) error {
	logger = logger.With("subscription_id", sub.ID, "database_id", sub.DatabaseID)
	logger.Debug(fmt.Sprintf("expiring subscription (was %s)", string(sub.Status)))

	return storage.GetDb().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", sub.ID).First(&sub).Error; err != nil {
			logger.Error("failed to lock subscription for expire", "error", err)
			return err
		}

		oldStatus := sub.Status
		sub.Status = billing_models.StatusExpired
		sub.UpdatedAt = time.Now().UTC()

		if err := tx.Save(&sub).Error; err != nil {
			logger.Error("failed to save subscription for expire", "error", err)
			return err
		}

		if err := tx.Create(&billing_models.SubscriptionEvent{
			ID:             uuid.New(),
			SubscriptionID: sub.ID,
			Type:           billing_models.EventExpired,
			OldStatus:      &oldStatus,
			NewStatus:      &sub.Status,
			CreatedAt:      time.Now().UTC(),
		}).Error; err != nil {
			logger.Error("failed to create subscription event for expire", "error", err)
			return err
		}

		logger.Info(fmt.Sprintf("subscription expired (was %s)", string(oldStatus)))

		return nil
	})
}

func (s *BillingService) processExpiredTrials(logger *slog.Logger) error {
	logger.Debug("started expiring trial subscriptions processing")

	trials, err := s.subscriptionRepository.FindExpiredTrials(time.Now())
	if err != nil {
		logger.Error("failed to find expired trial subscriptions", "error", err)
		return err
	}

	logger.Debug(fmt.Sprintf("found %d expired trial subscriptions", len(trials)))

	for _, subscription := range trials {
		scopedLog := logger.With("subscription_id", subscription.ID, "database_id", subscription.DatabaseID)
		err = s.expireTrialSubscription(scopedLog, &subscription)
		if err != nil {
			scopedLog.Error("failed to expire trial subscription", "error", err)
		}
	}

	return nil
}

func (s *BillingService) expireTrialSubscription(logger *slog.Logger, sub *billing_models.Subscription) error {
	logger = logger.With("subscription_id", sub.ID, "database_id", sub.DatabaseID)
	logger.Debug("expiring trial subscription")

	return storage.GetDb().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", sub.ID).First(&sub).Error; err != nil {
			logger.Error("failed to lock subscription for trial expire", "error", err)
			return err
		}

		oldStatus := sub.Status
		sub.Status = billing_models.StatusExpired
		sub.UpdatedAt = time.Now().UTC()

		now := time.Now().UTC()
		sub.CanceledAt = &now
		sub.UpdatedAt = now

		if err := tx.Save(&sub).Error; err != nil {
			logger.Error("failed to save subscription for trial expire", "error", err)
			return err
		}

		if err := tx.Create(&billing_models.SubscriptionEvent{
			ID:             uuid.New(),
			SubscriptionID: sub.ID,
			Type:           billing_models.EventExpired,
			OldStatus:      &oldStatus,
			NewStatus:      &sub.Status,
			CreatedAt:      time.Now().UTC(),
		}).Error; err != nil {
			logger.Error("failed to create subscription event for trial expire", "error", err)
			return err
		}

		logger.Info(fmt.Sprintf("trial subscription expired (was %s)", string(oldStatus)))

		return nil
	})
}

func (s *BillingService) createTrialSubscription(logger *slog.Logger, databaseID uuid.UUID) error {
	logger = logger.With("database_id", databaseID)

	dbCreatedAt := time.Now().UTC()
	trialEnds := dbCreatedAt.Add(config.GetEnv().TrialDuration)

	logger.Debug(
		fmt.Sprintf(
			"creating trial subscription: %d GB, expires %s",
			config.GetEnv().TrialStorageGB,
			trialEnds.Format(time.RFC3339),
		),
	)

	sub := billing_models.Subscription{
		ID:                 uuid.New(),
		DatabaseID:         databaseID,
		Status:             billing_models.StatusTrial,
		StorageGB:          config.GetEnv().TrialStorageGB,
		CurrentPeriodStart: dbCreatedAt,
		CurrentPeriodEnd:   trialEnds,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}

	logger = logger.With("subscription_id", sub.ID)

	if err := s.subscriptionRepository.Save(sub); err != nil {
		logger.Error("failed to save trial subscription", "error", err)
		return err
	}

	logger.Info(
		fmt.Sprintf(
			"trial subscription created: %d GB, expires %s",
			config.GetEnv().TrialStorageGB,
			trialEnds.Format(time.RFC3339),
		),
	)

	return nil
}

func (s *BillingService) applyUpgrade(
	logger *slog.Logger,
	sub *billing_models.Subscription,
	newStorageGB int,
) (*ChangeStorageResult, error) {
	logger.Debug(fmt.Sprintf("applying upgrade for subscription: new storage %d GB", newStorageGB))

	err := s.billingProvider.UpgradeQuantityWithSurcharge(logger, *sub.ProviderSubID, newStorageGB)
	if err != nil {
		logger.Error("failed to apply upgrade with billing provider", "error", err)
		return nil, err
	}

	logger.Debug("upgrade requested, waiting for billing provider webhook to update subscription")

	return &ChangeStorageResult{
		ApplyMode: ChangeStorageApplyImmediate,
		CurrentGB: sub.StorageGB,
		PendingGB: &newStorageGB,
	}, nil
}

func (s *BillingService) applyDowngrade(
	logger *slog.Logger,
	sub *billing_models.Subscription,
	newStorageGB int,
) (*ChangeStorageResult, error) {
	logger.Debug(fmt.Sprintf("applying downgrade for subscription: new storage %d GB", newStorageGB))

	err := s.billingProvider.ScheduleQuantityDowngradeFromNextBillingCycle(logger, *sub.ProviderSubID, newStorageGB)
	if err != nil {
		logger.Error("failed to schedule downgrade with billing provider", "error", err)
		return nil, err
	}

	sub.PendingStorageGB = &newStorageGB
	sub.UpdatedAt = time.Now().UTC()
	if err := s.subscriptionRepository.Save(*sub); err != nil {
		logger.Error("failed to save subscription with pending downgrade", "error", err)
		return nil, err
	}

	logger.Debug("downgrade scheduled for next billing cycle")

	return &ChangeStorageResult{
		ApplyMode: ChangeStorageApplyNextCycle,
		CurrentGB: sub.StorageGB,
		PendingGB: &newStorageGB,
	}, nil
}

func (s *BillingService) validateDatabaseAccess(
	logger *slog.Logger,
	user *users_models.User,
	databaseID uuid.UUID,
) error {
	database, err := s.databaseService.GetDatabaseByID(databaseID)
	if err != nil {
		logger.Error("failed to get database", "error", err)
		return err
	}

	hasAccess, _, err := s.workspaceService.CanUserAccessWorkspace(*database.WorkspaceID, user)
	if err != nil {
		logger.Error("failed to check workspace access", "error", err)
		return err
	}

	if !hasAccess {
		logger.Error("user does not have access to the workspace")
		return ErrAccessDenied
	}

	return nil
}

func (s *BillingService) getSubscriptionWithAccessCheck(
	logger *slog.Logger,
	user *users_models.User,
	subscriptionID uuid.UUID,
) (*billing_models.Subscription, error) {
	subscription, err := s.subscriptionRepository.FindByID(subscriptionID)
	if err != nil {
		logger.Error("failed to find subscription", "error", err)
		return nil, err
	}

	if subscription == nil {
		logger.Error("subscription not found")
		return nil, ErrSubscriptionNotFound
	}

	if err := s.validateDatabaseAccess(logger, user, subscription.DatabaseID); err != nil {
		return nil, err
	}

	return subscription, nil
}
