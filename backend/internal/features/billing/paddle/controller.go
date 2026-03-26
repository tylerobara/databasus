package billing_paddle

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	billing_webhooks "databasus-backend/internal/features/billing/webhooks"
	"databasus-backend/internal/util/logger"
)

type PaddleBillingController struct {
	paddleBillingService *PaddleBillingService
}

func (c *PaddleBillingController) RegisterPublicRoutes(router *gin.RouterGroup) {
	router.POST("/billing/paddle/webhook", c.HandlePaddleWebhook)
}

// HandlePaddleWebhook
// @Summary Handle Paddle webhook
// @Description Process incoming webhook events from Paddle payment provider
// @Tags billing
// @Accept json
// @Produce json
// @Param Paddle-Signature header string true "Paddle webhook signature"
// @Success 200
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500
// @Router /billing/paddle/webhook [post]
func (c *PaddleBillingController) HandlePaddleWebhook(ctx *gin.Context) {
	requestID := uuid.New()
	log := logger.GetLogger().With("request_id", requestID)

	body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, 1<<20))
	if err != nil {
		log.Error("failed to read webhook request body", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	headers := make(map[string]string)
	for k := range ctx.Request.Header {
		headers[k] = ctx.GetHeader(k)
	}

	if err := c.paddleBillingService.VerifyWebhookSignature(body, headers); err != nil {
		log.Warn("paddle webhook signature verification failed", "error", err)
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook signature"})
		return
	}

	var webhookDTO PaddleWebhookDTO
	if err := json.Unmarshal(body, &webhookDTO); err != nil {
		log.Error("failed to unmarshal webhook payload", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook payload"})
		return
	}

	log = log.With(
		"provider_event_id", webhookDTO.EventID,
		"event_type", webhookDTO.EventType,
	)

	if err := c.paddleBillingService.ProcessWebhookEvent(log, requestID, webhookDTO, body); err != nil {
		if errors.Is(err, billing_webhooks.ErrDuplicateWebhook) {
			log.Info("duplicate webhook event, returning 200 to not force retry")
			ctx.Status(http.StatusOK)
			return
		}

		log.Error("Failed to process paddle webhook", "error", err)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.Status(http.StatusOK)
}
