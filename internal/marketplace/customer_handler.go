package marketplace

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CustomerHandler serves customer-facing marketplace routes.
type CustomerHandler struct {
	svc *Service
}

// NewCustomerHandler creates a CustomerHandler.
func NewCustomerHandler(svc *Service) *CustomerHandler {
	return &CustomerHandler{svc: svc}
}

// RegisterCustomerRoutes registers customer-facing routes on the given group.
// The caller must apply RequireAuth middleware.
func (h *CustomerHandler) RegisterCustomerRoutes(rg *gin.RouterGroup) {
	rg.POST("/bookings/:id/dispute", h.raiseDispute)
}

// raiseDispute handles POST /bookings/:id/dispute.
//
// swagger:route POST /bookings/{id}/dispute bookings raiseDispute
//
// # Raise a booking dispute
//
// Raises a formal dispute for a completed booking before the dispute window expires.
// The dispute window defaults to 48 hours after booking completion and is configurable
// in platform settings. Only the customer who made the booking can raise a dispute.
// All amount fields are in centavos.
//
// Security:
//
//	bearer:
//
// Responses:
//
//	201: disputeResponse
//	400: errorResponse
//	401: errorResponse
//	403: errorResponse
//	404: errorResponse
//	409: errorResponse
//	422: errorResponse
func (h *CustomerHandler) raiseDispute(c *gin.Context) {
	bookingID := c.Param("id")
	customerID := c.GetString("user_id")

	var req RaiseDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	dispute, err := h.svc.RaiseDispute(c.Request.Context(), bookingID, customerID, &req)
	if err != nil {
		customerHandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dispute)
}

func customerHandleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "Booking not found"})
	case errors.Is(err, ErrUnauthorized):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "You do not own this booking"})
	case errors.Is(err, ErrDisputeWindowClosed):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "dispute_window_closed", "message": err.Error()})
	case errors.Is(err, ErrAlreadyDisputed):
		c.JSON(http.StatusConflict, gin.H{"error": "already_disputed", "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
	}
}
