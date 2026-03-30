package reviews

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler serves HTTP requests for the reviews module.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all review-related routes on the provided router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/reviews", h.Submit)                         // auth required
	rg.GET("/services/:id/reviews", h.GetByService)       // public
	rg.GET("/services/:id/reviews/summary", h.GetSummary) // public
	rg.POST("/reviews/:id/flag", h.Flag)                  // auth required
	rg.POST("/reviews/:id/response", h.RespondAsOwner)    // auth required, owner
}

// Submit handles POST /reviews.
func (h *Handler) Submit(c *gin.Context) {
	var req SubmitReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	customerID := c.GetString("user_id")
	review, err := h.svc.Submit(c.Request.Context(), &req, customerID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, review)
}

// GetByService handles GET /services/:id/reviews.
func (h *Handler) GetByService(c *gin.Context) {
	serviceID := c.Param("id")
	views, err := h.svc.GetByService(c.Request.Context(), serviceID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, views)
}

// GetSummary handles GET /services/:id/reviews/summary.
func (h *Handler) GetSummary(c *gin.Context) {
	serviceID := c.Param("id")
	summary, err := h.svc.GetRatingSummary(c.Request.Context(), serviceID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"summary": summary})
}

// Flag handles POST /reviews/:id/flag.
func (h *Handler) Flag(c *gin.Context) {
	reviewID := c.Param("id")
	var req FlagReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	reportedBy := c.GetString("user_id")
	if err := h.svc.Flag(c.Request.Context(), reviewID, reportedBy, req.Reason); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// RespondAsOwner handles POST /reviews/:id/response.
func (h *Handler) RespondAsOwner(c *gin.Context) {
	reviewID := c.Param("id")
	var req ReviewResponseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	ownerID := c.GetString("user_id")
	if err := h.svc.RespondAsOwner(c.Request.Context(), reviewID, ownerID, req.Body); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleError maps domain errors to HTTP responses.
func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrReviewNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "Review not found"})
	case errors.Is(err, ErrAlreadyReviewed):
		c.JSON(http.StatusConflict, gin.H{"error": "already_reviewed", "message": "A review has already been submitted for this booking"})
	case errors.Is(err, ErrBookingNotCompleted):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "booking_not_completed", "message": "Reviews can only be submitted for completed bookings"})
	case errors.Is(err, ErrReviewWindowExpired):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "review_window_expired", "message": "The 14-day review window for this booking has expired"})
	case errors.Is(err, ErrNotBookingOwner):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "You are not the owner of this booking"})
	case errors.Is(err, ErrNotReviewOwner):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "You are not authorized to respond to this review"})
	case errors.Is(err, ErrAlreadyFlagged):
		c.JSON(http.StatusConflict, gin.H{"error": "already_flagged", "message": "You have already flagged this review"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
	}
}
