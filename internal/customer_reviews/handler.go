package customer_reviews

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler serves HTTP requests for the customer_reviews module.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all customer_reviews routes on the given router group.
// The caller is responsible for applying auth middleware to rg before passing it in.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	// POST /customer-reviews — owner only
	rg.POST("/customer-reviews", h.Submit)

	// GET /users/:id/customer-reviews — owners or the customer themselves
	rg.GET("/users/:id/customer-reviews", h.GetByCustomer)

	// POST /customer-reviews/:id/dispute — the reviewed customer only
	rg.POST("/customer-reviews/:id/dispute", h.Dispute)
}

// Submit handles POST /customer-reviews.
// Requires the caller to have role "owner".
func (h *Handler) Submit(c *gin.Context) {
	if c.GetString("user_role") != "owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "Only owners can submit customer reviews"})
		return
	}

	var req SubmitReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	ownerUserID := c.GetString("user_id")
	rev, err := h.svc.Submit(c.Request.Context(), &req, ownerUserID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, rev)
}

// GetByCustomer handles GET /users/:id/customer-reviews.
// Accessible by owners (any customer) or the customer viewing their own reviews.
func (h *Handler) GetByCustomer(c *gin.Context) {
	customerID := c.Param("id")
	callerRole := c.GetString("user_role")
	callerID := c.GetString("user_id")

	if callerRole != "owner" && callerID != customerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "You are not authorized to view these reviews"})
		return
	}

	reviews, err := h.svc.GetByCustomer(c.Request.Context(), customerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	if reviews == nil {
		reviews = []CustomerReview{}
	}
	c.JSON(http.StatusOK, reviews)
}

// Dispute handles POST /customer-reviews/:id/dispute.
// Only the reviewed customer can file a dispute.
func (h *Handler) Dispute(c *gin.Context) {
	reviewID := c.Param("id")

	var req DisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	customerUserID := c.GetString("user_id")
	if err := h.svc.Dispute(c.Request.Context(), reviewID, customerUserID, &req); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleError maps domain errors to HTTP responses.
func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrReviewNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": err.Error()})
	case errors.Is(err, ErrAlreadyReviewed):
		c.JSON(http.StatusConflict, gin.H{"error": "already_reviewed", "message": "This booking has already been reviewed"})
	case errors.Is(err, ErrDisputeAlreadyFiled):
		c.JSON(http.StatusConflict, gin.H{"error": "dispute_already_filed", "message": "You have already filed a dispute for this review"})
	case errors.Is(err, ErrBookingNotCompleted):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "booking_not_completed", "message": "Reviews can only be submitted for completed bookings"})
	case errors.Is(err, ErrReviewWindowExpired):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "review_window_expired", "message": "The 14-day review window for this booking has passed"})
	case errors.Is(err, ErrNotYourReview):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "You can only dispute reviews written about you"})
	case errors.Is(err, ErrUnauthorized):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "You are not authorized to perform this action"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
	}
}
