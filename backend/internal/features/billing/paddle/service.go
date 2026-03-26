package billing_paddle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/PaddleHQ/paddle-go-sdk"
	"github.com/google/uuid"

	"databasus-backend/internal/features/billing"
	billing_models "databasus-backend/internal/features/billing/models"
	billing_provider "databasus-backend/internal/features/billing/provider"
	billing_webhooks "databasus-backend/internal/features/billing/webhooks"
)

type PaddleBillingService struct {
	client            *paddle.SDK
	webhookVerified   *paddle.WebhookVerifier
	priceID           string
	webhookRepository billing_webhooks.WebhookRepository
	billingService    *billing.BillingService
}

func (s *PaddleBillingService) GetProviderName() billing_provider.ProviderName {
	return billing_provider.ProviderPaddle
}

func (s *PaddleBillingService) CreateCheckoutSession(
	logger *slog.Logger,
	request billing_provider.CheckoutRequest,
) (string, error) {
	logger = logger.With("database_id", request.DatabaseID)
	logger.Debug(fmt.Sprintf("paddle: creating checkout session for %d GB", request.StorageGB))

	txRequest := &paddle.CreateTransactionRequest{
		Items: []paddle.CreateTransactionItems{
			*paddle.NewCreateTransactionItemsCatalogItem(&paddle.CatalogItem{
				PriceID:  s.priceID,
				Quantity: request.StorageGB,
			}),
		},
		CustomData: paddle.CustomData{"database_id": request.DatabaseID.String()},
		Checkout: &paddle.TransactionCheckout{
			URL: &request.SuccessURL,
		},
	}

	tx, err := s.client.CreateTransaction(context.Background(), txRequest)
	if err != nil {
		logger.Error("paddle: failed to create transaction", "error", err)
		return "", err
	}

	return tx.ID, nil
}

func (s *PaddleBillingService) UpgradeQuantityWithSurcharge(
	logger *slog.Logger,
	providerSubscriptionID string,
	quantityGB int,
) error {
	logger = logger.With("provider_subscription_id", providerSubscriptionID)
	logger.Debug(fmt.Sprintf("paddle: applying upgrade: new storage %d GB", quantityGB))

	// important: paddle requires to send all items
	// in the subscription when updating, not just the changed one
	subscription, err := s.client.GetSubscription(context.Background(), &paddle.GetSubscriptionRequest{
		SubscriptionID: providerSubscriptionID,
	})
	if err != nil {
		logger.Error("paddle: failed to get subscription", "error", err)
		return err
	}

	currentQuantity := subscription.Items[0].Quantity
	if currentQuantity == quantityGB {
		logger.Info("paddle: subscription already at requested quantity, skipping upgrade",
			"current_quantity_gb", currentQuantity,
			"requested_quantity_gb", quantityGB,
		)
		return nil
	}

	priceID := subscription.Items[0].Price.ID

	_, err = s.client.UpdateSubscription(context.Background(), &paddle.UpdateSubscriptionRequest{
		SubscriptionID: providerSubscriptionID,
		Items: paddle.NewPatchField([]paddle.SubscriptionUpdateCatalogItem{
			{
				PriceID:  priceID,
				Quantity: quantityGB,
			},
		}),
		ProrationBillingMode: paddle.NewPatchField(paddle.ProrationBillingModeProratedImmediately),
	})
	if err != nil {
		logger.Error("paddle: failed to update subscription", "error", err)
		return err
	}

	logger.Debug("paddle: successfully applied upgrade")

	return nil
}

func (s *PaddleBillingService) ScheduleQuantityDowngradeFromNextBillingCycle(
	logger *slog.Logger,
	providerSubscriptionID string,
	quantityGB int,
) error {
	logger = logger.With("provider_subscription_id", providerSubscriptionID)
	logger.Debug(fmt.Sprintf("paddle: scheduling downgrade from next billing cycle: new storage %d GB", quantityGB))

	// important: paddle requires to send all items
	// in the subscription when updating, not just the changed one
	subscription, err := s.client.GetSubscription(context.Background(), &paddle.GetSubscriptionRequest{
		SubscriptionID: providerSubscriptionID,
	})
	if err != nil {
		logger.Error("paddle: failed to get subscription", "error", err)
		return err
	}

	currentQuantity := subscription.Items[0].Quantity
	if currentQuantity == quantityGB {
		logger.Info("paddle: subscription already at requested quantity, skipping downgrade",
			"current_quantity_gb", currentQuantity,
			"requested_quantity_gb", quantityGB,
		)
		return nil
	}

	if subscription.ScheduledChange != nil {
		logger.Info("paddle: subscription already has a scheduled change, skipping downgrade")
		return nil
	}

	priceID := subscription.Items[0].Price.ID

	// apply downgrade from next billing cycle by setting the proration billing mode to "prorate on next billing period"
	_, err = s.client.UpdateSubscription(context.Background(), &paddle.UpdateSubscriptionRequest{
		SubscriptionID: providerSubscriptionID,
		Items: paddle.NewPatchField([]paddle.SubscriptionUpdateCatalogItem{
			{
				PriceID:  priceID,
				Quantity: quantityGB,
			},
		}),
		ProrationBillingMode: paddle.NewPatchField(paddle.ProrationBillingModeFullNextBillingPeriod),
	})
	if err != nil {
		logger.Error("paddle: failed to update subscription for downgrade", "error", err)
		return fmt.Errorf("failed to update subscription: %w", err)
	}

	logger.Debug("paddle: successfully scheduled downgrade from next billing cycle")

	return nil
}

func (s *PaddleBillingService) GetSubscription(
	logger *slog.Logger,
	providerSubscriptionID string,
) (billing_provider.ProviderSubscription, error) {
	logger = logger.With("provider_subscription_id", providerSubscriptionID)
	logger.Debug("paddle: getting subscription details")

	subscription, err := s.client.GetSubscription(context.Background(), &paddle.GetSubscriptionRequest{
		SubscriptionID: providerSubscriptionID,
	})
	if err != nil {
		logger.Error("paddle: failed to get subscription", "error", err)
		return billing_provider.ProviderSubscription{}, err
	}

	logger.Debug(
		fmt.Sprintf(
			"paddle: successfully got subscription details: status=%s, quantity=%d",
			subscription.Status,
			subscription.Items[0].Quantity,
		),
	)

	return s.toProviderSubscription(logger, subscription)
}

func (s *PaddleBillingService) CreatePortalSession(
	logger *slog.Logger,
	providerCustomerID, returnURL string,
) (string, error) {
	logger = logger.With("provider_customer_id", providerCustomerID)
	logger.Debug("paddle: creating portal session")

	subscriptions, err := s.client.ListSubscriptions(context.Background(), &paddle.ListSubscriptionsRequest{
		CustomerID: []string{providerCustomerID},
		Status: []string{
			string(paddle.SubscriptionStatusActive),
			string(paddle.SubscriptionStatusPastDue),
		},
	})
	if err != nil {
		logger.Error("paddle: failed to list subscriptions for portal session", "error", err)
		return "", err
	}

	res := subscriptions.Next(context.Background())
	if !res.Ok() {
		if res.Err() != nil {
			logger.Error("paddle: failed to iterate subscriptions", "error", res.Err())
			return "", res.Err()
		}

		logger.Error("paddle: no active subscriptions found for customer")
		return "", fmt.Errorf("no active subscriptions found for customer %s", providerCustomerID)
	}

	subscription := res.Value()
	if subscription.ManagementURLs.UpdatePaymentMethod == nil {
		logger.Error("paddle: subscription has no management URL")
		return "", fmt.Errorf("subscription %s has no management URL", subscription.ID)
	}

	return *subscription.ManagementURLs.UpdatePaymentMethod, nil
}

func (s *PaddleBillingService) VerifyWebhookSignature(body []byte, headers map[string]string) error {
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/", bytes.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	ok, err := s.webhookVerified.Verify(req)
	if err != nil || !ok {
		return fmt.Errorf("failed to verify webhook signature: %w", err)
	}

	return nil
}

func (s *PaddleBillingService) ProcessWebhookEvent(
	logger *slog.Logger,
	requestID uuid.UUID,
	webhookDTO PaddleWebhookDTO,
	rawBody []byte,
) error {
	webhookEvent, err := s.normalizeWebhookEvent(
		logger,
		requestID,
		webhookDTO.EventID,
		webhookDTO.EventType,
		webhookDTO.Data,
	)
	if err != nil {
		if errors.Is(err, billing_webhooks.ErrUnsupportedEventType) {
			return s.skipWebhookEvent(logger, requestID, webhookDTO, rawBody)
		}

		logger.Error("paddle: failed to normalize webhook event", "error", err)
		return err
	}

	logArgs := []any{
		"provider_event_id", webhookEvent.ProviderEventID,
		"provider_subscription_id", webhookEvent.ProviderSubscriptionID,
		"provider_customer_id", webhookEvent.ProviderCustomerID,
	}
	if webhookEvent.DatabaseID != nil {
		logArgs = append(logArgs, "database_id", webhookEvent.DatabaseID)
	}

	logger = logger.With(logArgs...)

	existingRecord, err := s.webhookRepository.FindSuccessfulByProviderEventID(webhookEvent.ProviderEventID)
	if err == nil && existingRecord != nil {
		logger.Info("paddle: webhook already processed successfully, skipping",
			"existing_request_id", existingRecord.RequestID,
		)
		return billing_webhooks.ErrDuplicateWebhook
	}

	webhookRecord := &billing_webhooks.WebhookRecord{
		RequestID:       requestID,
		ProviderName:    billing_provider.ProviderPaddle,
		EventType:       string(webhookEvent.Type),
		ProviderEventID: webhookEvent.ProviderEventID,
		RawPayload:      string(rawBody),
	}
	if err := s.webhookRepository.Insert(webhookRecord); err != nil {
		logger.Error("paddle: failed to save webhook record", "error", err)
		return err
	}

	if err := s.processWebhookEvent(logger, webhookEvent); err != nil {
		logger.Error("paddle: failed to process webhook event", "error", err)

		if markErr := s.webhookRepository.MarkError(requestID.String(), err.Error()); markErr != nil {
			logger.Error("paddle: failed to mark webhook as errored", "error", markErr)
		}

		return err
	}

	if markErr := s.webhookRepository.MarkProcessed(requestID.String()); markErr != nil {
		logger.Error("paddle: failed to mark webhook as processed", "error", markErr)
	}

	return nil
}

func (s *PaddleBillingService) skipWebhookEvent(
	logger *slog.Logger,
	requestID uuid.UUID,
	webhookDTO PaddleWebhookDTO,
	rawBody []byte,
) error {
	webhookRecord := &billing_webhooks.WebhookRecord{
		RequestID:       requestID,
		ProviderName:    billing_provider.ProviderPaddle,
		EventType:       webhookDTO.EventType,
		ProviderEventID: webhookDTO.EventID,
		RawPayload:      string(rawBody),
	}

	if err := s.webhookRepository.Insert(webhookRecord); err != nil {
		logger.Error("paddle: failed to save skipped webhook record", "error", err)
		return err
	}

	if err := s.webhookRepository.MarkSkipped(requestID.String()); err != nil {
		logger.Error("paddle: failed to mark webhook as skipped", "error", err)
	}

	return nil
}

func (s *PaddleBillingService) processWebhookEvent(
	logger *slog.Logger,
	webhookEvent billing_models.WebhookEvent,
) error {
	logger.Debug("processing webhook event")

	// subscription.created - there is no subscription in the database yet
	if webhookEvent.Type == billing_models.WHEventSubscriptionCreated {
		return s.billingService.ActivateSubscription(logger, webhookEvent)
	}

	// dispute - finds subscription via invoice, no provider subscription ID available
	if webhookEvent.Type == billing_models.WHEventSubscriptionDisputeCreated {
		return s.billingService.RecordDispute(logger, webhookEvent)
	}

	// for others - search subscription first
	subscription, err := s.billingService.GetSubscriptionByProviderSubID(logger, webhookEvent.ProviderSubscriptionID)
	if err != nil {
		logger.Error("paddle: failed to find subscription for webhook event", "error", err)
		return err
	}

	logger = logger.With("subscription_id", subscription.ID, "database_id", subscription.DatabaseID)
	logger.Debug(fmt.Sprintf("found subscription in DB with ID: %s", subscription.ID))

	switch webhookEvent.Type {
	case billing_models.WHEventSubscriptionUpdated:
		if subscription.Status == billing_models.StatusCanceled {
			return s.billingService.ReactivateSubscription(logger, subscription, webhookEvent)
		}

		return s.billingService.SyncSubscriptionFromProvider(logger, subscription, webhookEvent)
	case billing_models.WHEventSubscriptionCanceled:
		return s.billingService.CancelSubscription(logger, subscription, webhookEvent)
	case billing_models.WHEventPaymentSucceeded:
		return s.billingService.RecordPaymentSuccess(logger, subscription, webhookEvent)
	case billing_models.WHEventSubscriptionPastDue:
		return s.billingService.RecordPaymentFailed(logger, subscription, webhookEvent)
	default:
		logger.Error(fmt.Sprintf("unhandled webhook event type: %s", string(webhookEvent.Type)))
		return nil
	}
}

func (s *PaddleBillingService) normalizeWebhookEvent(
	logger *slog.Logger,
	requestID uuid.UUID,
	eventID, eventType string,
	data json.RawMessage,
) (billing_models.WebhookEvent, error) {
	webhookEvent := billing_models.WebhookEvent{
		RequestID:       requestID,
		ProviderEventID: eventID,
	}

	switch eventType {
	case "subscription.created":
		webhookEvent.Type = billing_models.WHEventSubscriptionCreated

		var subscription paddle.Subscription
		if err := json.Unmarshal(data, &subscription); err != nil {
			logger.Error("paddle: failed to unmarshal subscription.created webhook data", "error", err)
			return webhookEvent, err
		}

		webhookEvent.ProviderSubscriptionID = subscription.ID
		webhookEvent.ProviderCustomerID = subscription.CustomerID
		webhookEvent.QuantityGB = subscription.Items[0].Quantity
		status, err := mapPaddleStatus(logger, subscription.Status)
		if err != nil {
			return webhookEvent, err
		}

		webhookEvent.Status = status

		if subscription.CurrentBillingPeriod != nil {
			periodStart := mustParseRFC3339(logger, "period start", subscription.CurrentBillingPeriod.StartsAt)
			periodEnd := mustParseRFC3339(logger, "period end", subscription.CurrentBillingPeriod.EndsAt)

			webhookEvent.PeriodStart = &periodStart
			webhookEvent.PeriodEnd = &periodEnd
		}

		if subscription.CustomData == nil || subscription.CustomData["database_id"] == "" {
			logger.Error("paddle: subscription has no database_id in custom data")
		}

		databaseIDStr, isOk := subscription.CustomData["database_id"].(string)
		if !isOk {
			logger.Error("paddle: database_id in custom data is not a string")
			return webhookEvent, fmt.Errorf("invalid database_id type in custom data")
		}

		databaseID := uuid.MustParse(databaseIDStr)
		webhookEvent.DatabaseID = &databaseID

	case "subscription.updated":
		var subscription paddle.Subscription
		if err := json.Unmarshal(data, &subscription); err != nil {
			return webhookEvent, err
		}

		webhookEvent.ProviderSubscriptionID = subscription.ID
		webhookEvent.ProviderCustomerID = subscription.CustomerID
		webhookEvent.QuantityGB = subscription.Items[0].Quantity
		webhookEvent.Type = billing_models.WHEventSubscriptionUpdated

		status, err := mapPaddleStatus(logger, subscription.Status)
		if err != nil {
			return webhookEvent, err
		}

		webhookEvent.Status = status

		if subscription.CurrentBillingPeriod != nil {
			periodStart := mustParseRFC3339(logger, "period start", subscription.CurrentBillingPeriod.StartsAt)
			periodEnd := mustParseRFC3339(logger, "period end", subscription.CurrentBillingPeriod.EndsAt)

			webhookEvent.PeriodStart = &periodStart
			webhookEvent.PeriodEnd = &periodEnd
		}

		if subscription.ScheduledChange != nil &&
			subscription.ScheduledChange.Action == paddle.ScheduledChangeActionCancel {
			webhookEvent.Type = billing_models.WHEventSubscriptionCanceled
		}

	case "subscription.canceled":
		webhookEvent.Type = billing_models.WHEventSubscriptionCanceled

		var subscription paddle.Subscription
		if err := json.Unmarshal(data, &subscription); err != nil {
			return webhookEvent, err
		}

		webhookEvent.ProviderSubscriptionID = subscription.ID
		webhookEvent.ProviderCustomerID = subscription.CustomerID

		status, err := mapPaddleStatus(logger, subscription.Status)
		if err != nil {
			return webhookEvent, err
		}

		webhookEvent.Status = status

	case "transaction.completed":
		webhookEvent.Type = billing_models.WHEventPaymentSucceeded

		var transaction paddle.Transaction
		if err := json.Unmarshal(data, &transaction); err != nil {
			return webhookEvent, err
		}

		webhookEvent.ProviderInvoiceID = transaction.ID

		if len(transaction.Items) > 0 {
			webhookEvent.QuantityGB = transaction.Items[0].Quantity
		}

		if transaction.SubscriptionID != nil {
			webhookEvent.ProviderSubscriptionID = *transaction.SubscriptionID
		}

		if transaction.CustomerID != nil {
			webhookEvent.ProviderCustomerID = *transaction.CustomerID
		}

		amountCents, err := strconv.ParseInt(transaction.Details.Totals.Total, 10, 64)
		if err != nil {
			logger.Error("paddle: failed to parse transaction total", "error", err)
		} else {
			webhookEvent.AmountCents = amountCents
		}

		if transaction.BillingPeriod != nil {
			periodStart := mustParseRFC3339(logger, "period start", transaction.BillingPeriod.StartsAt)
			periodEnd := mustParseRFC3339(logger, "period end", transaction.BillingPeriod.EndsAt)

			webhookEvent.PeriodStart = &periodStart
			webhookEvent.PeriodEnd = &periodEnd
		}

	case "subscription.past_due":
		webhookEvent.Type = billing_models.WHEventSubscriptionPastDue

		var subscription paddle.Subscription
		if err := json.Unmarshal(data, &subscription); err != nil {
			return webhookEvent, err
		}

		webhookEvent.ProviderSubscriptionID = subscription.ID
		webhookEvent.ProviderCustomerID = subscription.CustomerID
		webhookEvent.QuantityGB = subscription.Items[0].Quantity

		status, err := mapPaddleStatus(logger, subscription.Status)
		if err != nil {
			return webhookEvent, err
		}

		webhookEvent.Status = status

		if subscription.CurrentBillingPeriod != nil {
			periodStart := mustParseRFC3339(logger, "period start", subscription.CurrentBillingPeriod.StartsAt)
			periodEnd := mustParseRFC3339(logger, "period end", subscription.CurrentBillingPeriod.EndsAt)

			webhookEvent.PeriodStart = &periodStart
			webhookEvent.PeriodEnd = &periodEnd
		}

	case "adjustment.created":
		webhookEvent.Type = billing_models.WHEventSubscriptionDisputeCreated

		var adjustment struct {
			TransactionID string `json:"transaction_id"`
		}
		if err := json.Unmarshal(data, &adjustment); err != nil {
			return webhookEvent, err
		}

		webhookEvent.ProviderInvoiceID = adjustment.TransactionID

	default:
		logger.Debug("unsupported paddle event type, skipping", "event_type", eventType)
		return webhookEvent, billing_webhooks.ErrUnsupportedEventType
	}

	return webhookEvent, nil
}

func (s *PaddleBillingService) toProviderSubscription(
	logger *slog.Logger,
	paddleSubscription *paddle.Subscription,
) (billing_provider.ProviderSubscription, error) {
	status, err := mapPaddleStatus(logger, paddleSubscription.Status)
	if err != nil {
		return billing_provider.ProviderSubscription{}, err
	}

	if len(paddleSubscription.Items) == 0 {
		return billing_provider.ProviderSubscription{}, fmt.Errorf(
			"paddle subscription %s has no items",
			paddleSubscription.ID,
		)
	}

	providerSubscription := &billing_provider.ProviderSubscription{
		ProviderSubscriptionID: paddleSubscription.ID,
		ProviderCustomerID:     paddleSubscription.CustomerID,
		Status:                 status,
		QuantityGB:             paddleSubscription.Items[0].Quantity,
	}

	if paddleSubscription.CurrentBillingPeriod != nil {
		providerSubscription.PeriodStart = mustParseRFC3339(
			logger,
			"period start",
			paddleSubscription.CurrentBillingPeriod.StartsAt,
		)
		providerSubscription.PeriodEnd = mustParseRFC3339(
			logger,
			"period end",
			paddleSubscription.CurrentBillingPeriod.EndsAt,
		)
	}

	return *providerSubscription, nil
}

func mustParseRFC3339(logger *slog.Logger, label, value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		logger.Error(fmt.Sprintf("paddle: failed to parse %s", label), "error", err)
	}

	return parsed
}

func mapPaddleStatus(logger *slog.Logger, s paddle.SubscriptionStatus) (billing_models.SubscriptionStatus, error) {
	switch s {
	case paddle.SubscriptionStatusActive:
		return billing_models.StatusActive, nil
	case paddle.SubscriptionStatusPastDue:
		return billing_models.StatusPastDue, nil
	case paddle.SubscriptionStatusCanceled:
		return billing_models.StatusCanceled, nil
	case paddle.SubscriptionStatusTrialing:
		return billing_models.StatusTrial, nil
	case paddle.SubscriptionStatusPaused:
		return billing_models.StatusCanceled, nil
	default:
		logger.Error(fmt.Sprintf("paddle: unknown subscription status: %s", string(s)))

		return "", fmt.Errorf("paddle: unknown subscription status: %s", string(s))
	}
}
