package billing

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	users_middleware "databasus-backend/internal/features/users/middleware"
	"databasus-backend/internal/util/logger"
)

type BillingController struct {
	billingService *BillingService
}

func (c *BillingController) RegisterRoutes(router *gin.RouterGroup) {
	billing := router.Group("/billing")

	billing.POST("/subscription", c.CreateSubscription)
	billing.POST("/subscription/change-storage", c.ChangeSubscriptionStorage)
	billing.POST("/subscription/portal/:subscription_id", c.GetPortalSession)
	billing.GET("/subscription/events/:subscription_id", c.GetSubscriptionEvents)
	billing.GET("/subscription/invoices/:subscription_id", c.GetInvoices)
	billing.GET("/subscription/:database_id", c.GetSubscription)
}

// CreateSubscription
// @Summary Create a new subscription
// @Description Create a billing subscription for the specified database with the given storage
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateSubscriptionRequest true "Subscription creation data"
// @Success 200 {object} CreateSubscriptionResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /billing/subscription [post]
func (c *BillingController) CreateSubscription(ctx *gin.Context) {
	user, ok := users_middleware.GetUserFromContext(ctx)
	if !ok {
		ctx.JSON(401, gin.H{"error": "User not authenticated"})
		return
	}

	var request CreateSubscriptionRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	log := logger.GetLogger().With(
		"request_id", uuid.New(),
		"database_id", request.DatabaseID,
		"user_id", user.ID,
	)

	transactionID, err := c.billingService.CreateSubscription(
		log,
		user,
		request.DatabaseID,
		request.StorageGB,
	)
	if err != nil {
		log.Error("Failed to create subscription", "error", err)
		ctx.JSON(500, gin.H{"error": "Failed to create subscription"})
		return
	}

	ctx.JSON(200, CreateSubscriptionResponse{PaddleTransactionID: transactionID})
}

// ChangeSubscriptionStorage
// @Summary Change subscription storage
// @Description Update the storage allocation for an existing subscription
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ChangeStorageRequest true "New storage configuration"
// @Success 200 {object} ChangeStorageResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /billing/subscription/change-storage [post]
func (c *BillingController) ChangeSubscriptionStorage(ctx *gin.Context) {
	user, ok := users_middleware.GetUserFromContext(ctx)
	if !ok {
		ctx.JSON(401, gin.H{"error": "User not authenticated"})
		return
	}

	var request ChangeStorageRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	log := logger.GetLogger().With(
		"request_id", uuid.New(),
		"database_id", request.DatabaseID,
		"user_id", user.ID,
	)

	result, err := c.billingService.ChangeSubscriptionStorage(log, user, request.DatabaseID, request.StorageGB)
	if err != nil {
		log.Error("Failed to change subscription storage", "error", err)
		ctx.JSON(500, gin.H{"error": "Failed to change subscription storage"})
		return
	}

	ctx.JSON(200, ChangeStorageResponse{
		ApplyMode: result.ApplyMode,
		CurrentGB: result.CurrentGB,
		PendingGB: result.PendingGB,
	})
}

// GetPortalSession
// @Summary Get billing portal session
// @Description Generate a portal session URL for managing the subscription
// @Tags billing
// @Produce json
// @Security BearerAuth
// @Param subscription_id path string true "Subscription ID"
// @Success 200 {object} GetPortalSessionResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /billing/subscription/portal/{subscription_id} [post]
func (c *BillingController) GetPortalSession(ctx *gin.Context) {
	user, ok := users_middleware.GetUserFromContext(ctx)
	if !ok {
		ctx.JSON(401, gin.H{"error": "User not authenticated"})
		return
	}

	subscriptionID := ctx.Param("subscription_id")
	if subscriptionID == "" {
		ctx.JSON(400, gin.H{"error": "Subscription ID is required"})
		return
	}

	log := logger.GetLogger().With(
		"request_id", uuid.New(),
		"subscription_id", subscriptionID,
		"user_id", user.ID,
	)

	url, err := c.billingService.GetPortalURL(log, user, uuid.MustParse(subscriptionID))
	if err != nil {
		log.Error("Failed to get portal session", "error", err)
		ctx.JSON(500, gin.H{"error": "Failed to get portal session"})
		return
	}

	ctx.JSON(200, GetPortalSessionResponse{PortalURL: url})
}

// GetSubscriptionEvents
// @Summary Get subscription events
// @Description Retrieve the event history for a subscription
// @Tags billing
// @Produce json
// @Security BearerAuth
// @Param subscription_id path string true "Subscription ID"
// @Param limit query int false "Limit number of results" default(100)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} GetSubscriptionEventsResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /billing/subscription/events/{subscription_id} [get]
func (c *BillingController) GetSubscriptionEvents(ctx *gin.Context) {
	user, ok := users_middleware.GetUserFromContext(ctx)
	if !ok {
		ctx.JSON(401, gin.H{"error": "User not authenticated"})
		return
	}

	subscriptionID, err := uuid.Parse(ctx.Param("subscription_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription ID"})
		return
	}

	var request PaginatedRequest
	if err := ctx.ShouldBindQuery(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
		return
	}

	log := logger.GetLogger().With(
		"request_id", uuid.New(),
		"subscription_id", subscriptionID,
		"user_id", user.ID,
	)

	response, err := c.billingService.GetSubscriptionEvents(log, user, subscriptionID, request.Limit, request.Offset)
	if err != nil {
		log.Error("Failed to get subscription events", "error", err)
		ctx.JSON(500, gin.H{"error": "Failed to get subscription events"})
		return
	}

	ctx.JSON(200, response)
}

// GetInvoices
// @Summary Get subscription invoices
// @Description Retrieve all invoices for a subscription
// @Tags billing
// @Produce json
// @Security BearerAuth
// @Param subscription_id path string true "Subscription ID"
// @Param limit query int false "Limit number of results" default(100)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} GetInvoicesResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /billing/subscription/invoices/{subscription_id} [get]
func (c *BillingController) GetInvoices(ctx *gin.Context) {
	user, ok := users_middleware.GetUserFromContext(ctx)
	if !ok {
		ctx.JSON(401, gin.H{"error": "User not authenticated"})
		return
	}

	subscriptionID, err := uuid.Parse(ctx.Param("subscription_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription ID"})
		return
	}

	var request PaginatedRequest
	if err := ctx.ShouldBindQuery(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
		return
	}

	log := logger.GetLogger().With(
		"request_id", uuid.New(),
		"subscription_id", subscriptionID,
		"user_id", user.ID,
	)

	response, err := c.billingService.GetSubscriptionInvoices(log, user, subscriptionID, request.Limit, request.Offset)
	if err != nil {
		log.Error("Failed to get invoices", "error", err)
		ctx.JSON(500, gin.H{"error": "Failed to get invoices"})
		return
	}

	ctx.JSON(200, response)
}

// GetSubscription
// @Summary Get subscription by database
// @Description Retrieve the subscription associated with a specific database
// @Tags billing
// @Produce json
// @Security BearerAuth
// @Param database_id path string true "Database ID"
// @Success 200 {object} billing_models.Subscription
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /billing/subscription/{database_id} [get]
func (c *BillingController) GetSubscription(ctx *gin.Context) {
	user, ok := users_middleware.GetUserFromContext(ctx)
	if !ok {
		ctx.JSON(401, gin.H{"error": "User not authenticated"})
		return
	}

	databaseID, err := uuid.Parse(ctx.Param("database_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid database ID"})
		return
	}

	log := logger.GetLogger().With(
		"request_id", uuid.New(),
		"database_id", databaseID,
		"user_id", user.ID,
	)

	subscription, err := c.billingService.GetSubscriptionByDatabaseID(log, user, databaseID)
	if err != nil {
		if errors.Is(err, ErrSubscriptionNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found"})
			return
		}

		log.Error("failed to get subscription", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get subscription"})
		return
	}

	ctx.JSON(200, subscription)
}
