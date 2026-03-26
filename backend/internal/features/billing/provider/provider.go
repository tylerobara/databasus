package billing_provider

import "log/slog"

type BillingProvider interface {
	GetProviderName() ProviderName

	UpgradeQuantityWithSurcharge(logger *slog.Logger, providerSubscriptionID string, quantityGB int) error

	ScheduleQuantityDowngradeFromNextBillingCycle(
		logger *slog.Logger,
		providerSubscriptionID string,
		quantityGB int,
	) error

	GetSubscription(logger *slog.Logger, providerSubscriptionID string) (ProviderSubscription, error)

	CreateCheckoutSession(logger *slog.Logger, req CheckoutRequest) (checkoutURL string, err error)

	CreatePortalSession(logger *slog.Logger, providerCustomerID, returnURL string) (portalURL string, err error)
}
