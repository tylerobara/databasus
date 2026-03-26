package billing

import "errors"

var (
	ErrInvalidStorage       = errors.New("storage must be between 20 and 10000 GB")
	ErrAlreadySubscribed    = errors.New("database already has an active subscription")
	ErrExceedsUsage         = errors.New("cannot downgrade below current storage usage")
	ErrNoChange             = errors.New("requested storage is the same as current")
	ErrDuplicate            = errors.New("duplicate event already processed")
	ErrProviderUnavailable  = errors.New("payment provider unavailable")
	ErrNoActiveSubscription = errors.New("no active subscription for this database")
	ErrAccessDenied         = errors.New("user does not have access to this database")
	ErrSubscriptionNotFound = errors.New("subscription not found")
)
