package billing

import (
	"github.com/google/uuid"

	billing_models "databasus-backend/internal/features/billing/models"
)

type CreateSubscriptionRequest struct {
	DatabaseID uuid.UUID `json:"databaseId" validate:"required"`
	StorageGB  int       `json:"storageGb"  validate:"required,min=1"`
}

type CreateSubscriptionResponse struct {
	PaddleTransactionID string `json:"paddleTransactionId"`
}

type ChangeStorageApplyMode string

const (
	ChangeStorageApplyImmediate ChangeStorageApplyMode = "immediate"
	ChangeStorageApplyNextCycle ChangeStorageApplyMode = "next_cycle"
)

type ChangeStorageRequest struct {
	DatabaseID uuid.UUID `json:"databaseId" validate:"required"`
	StorageGB  int       `json:"storageGb"  validate:"required,min=1"`
}

type ChangeStorageResponse struct {
	ApplyMode ChangeStorageApplyMode `json:"applyMode"`
	CurrentGB int                    `json:"currentGb"`
	PendingGB *int                   `json:"pendingGb,omitempty"`
}

type PortalResponse struct {
	URL string `json:"url"`
}

type ChangeStorageResult struct {
	ApplyMode ChangeStorageApplyMode
	CurrentGB int
	PendingGB *int
}

type GetPortalSessionResponse struct {
	PortalURL string `json:"url"`
}

type PaginatedRequest struct {
	Limit  int `form:"limit"  json:"limit"`
	Offset int `form:"offset" json:"offset"`
}

type GetSubscriptionEventsResponse struct {
	Events []*billing_models.SubscriptionEvent `json:"events"`
	Total  int64                               `json:"total"`
	Limit  int                                 `json:"limit"`
	Offset int                                 `json:"offset"`
}

type GetInvoicesResponse struct {
	Invoices []*billing_models.Invoice `json:"invoices"`
	Total    int64                     `json:"total"`
	Limit    int                       `json:"limit"`
	Offset   int                       `json:"offset"`
}
