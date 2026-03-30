package payments

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler serves HTTP requests for the payments module.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers authenticated payment endpoints on rg.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/payments/intent", h.CreateIntent)
}

// RegisterWebhook registers the unauthenticated webhook endpoint on r.
func (h *Handler) RegisterWebhook(r gin.IRouter) {
	r.POST("/payments/webhook", h.HandleWebhook)
}

// CreateIntent handles POST /payments/intent.
// Requires auth middleware on the parent router group.
func (h *Handler) CreateIntent(c *gin.Context) {
	var req CreateIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	pi, err := h.svc.CreateIntent(c.Request.Context(), &req)
	if err != nil {
		log.Printf("payments.CreateIntent: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to create payment intent"})
		return
	}

	c.JSON(http.StatusCreated, toResponse(pi))
}

// HandleWebhook handles POST /payments/webhook.
// No auth middleware. Always returns 200 to satisfy PayMongo retry behaviour.
func (h *Handler) HandleWebhook(c *gin.Context) {
	rawBody, err := c.GetRawData()
	if err != nil {
		log.Printf("payments.HandleWebhook: read body: %v", err)
		c.Status(http.StatusOK)
		return
	}

	signature := c.GetHeader("Paymongo-Signature")

	if err := h.svc.HandleWebhook(c.Request.Context(), rawBody, signature); err != nil {
		switch {
		case errors.Is(err, ErrInvalidWebhookSignature):
			log.Printf("payments.HandleWebhook: invalid signature")
		case errors.Is(err, ErrDuplicateWebhookEvent):
			log.Printf("payments.HandleWebhook: duplicate event, ignoring")
		default:
			log.Printf("payments.HandleWebhook: %v", err)
		}
		// Always return 200 so PayMongo does not retry.
		c.Status(http.StatusOK)
		return
	}

	c.Status(http.StatusOK)
}
