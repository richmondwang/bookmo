package bookings

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler serves HTTP requests for the bookings module.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers customer-facing booking routes and owner-facing routes.
// The caller is responsible for applying auth middleware to rg before passing it in.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	// Customer-facing routes.
	rg.POST("/bookings/lock", h.lockSlot)
	rg.POST("/bookings", h.createBooking)
	rg.GET("/bookings", h.listBookings)
	rg.POST("/bookings/:id/cancel", h.cancelBooking)
	rg.POST("/bookings/:id/reschedule", h.requestReschedule)

	// Owner-facing routes.
	owner := rg.Group("/owner")
	owner.GET("/queue", h.getOwnerQueue)
	owner.POST("/bookings/:id/approve", h.approveBooking)
	owner.POST("/bookings/:id/reject", h.rejectBooking)
	owner.POST("/reschedules/:id/approve", h.approveReschedule)
	owner.POST("/reschedules/:id/reject", h.rejectReschedule)
}

// --- Customer handlers ---

// lockSlot handles POST /bookings/lock.
func (h *Handler) lockSlot(c *gin.Context) {
	var req CreateLockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	lock, err := h.svc.LockSlot(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, lock)
}

// createBooking handles POST /bookings.
func (h *Handler) createBooking(c *gin.Context) {
	var req CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	customerID := c.GetString("user_id")
	booking, err := h.svc.CreateBooking(c.Request.Context(), &req, customerID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	resp := toResponse(booking)
	c.JSON(http.StatusCreated, resp)
}

// listBookings handles GET /bookings.
func (h *Handler) listBookings(c *gin.Context) {
	customerID := c.GetString("user_id")
	bs, err := h.svc.GetCustomerBookings(c.Request.Context(), customerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	resps := make([]BookingResponse, 0, len(bs))
	for i := range bs {
		resps = append(resps, toResponse(&bs[i]))
	}
	c.JSON(http.StatusOK, resps)
}

// cancelBooking handles POST /bookings/:id/cancel.
func (h *Handler) cancelBooking(c *gin.Context) {
	id := c.Param("id")
	callerID := c.GetString("user_id")
	callerRole := c.GetString("user_role")

	if err := h.svc.Cancel(c.Request.Context(), id, callerID, callerRole); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// requestReschedule handles POST /bookings/:id/reschedule.
func (h *Handler) requestReschedule(c *gin.Context) {
	id := c.Param("id")
	var req RescheduleBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	customerID := c.GetString("user_id")
	if err := h.svc.RequestReschedule(c.Request.Context(), id, customerID, &req); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Owner handlers ---

// getOwnerQueue handles GET /owner/queue.
func (h *Handler) getOwnerQueue(c *gin.Context) {
	ownerID := c.GetString("user_id")
	items, err := h.svc.GetOwnerQueue(c.Request.Context(), ownerID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	resps := make([]QueueItemResponse, 0, len(items))
	for i := range items {
		resps = append(resps, QueueItemResponse{
			BookingResponse: toResponse(&items[i].Booking),
			ServiceName:     items[i].ServiceName,
			BranchName:      items[i].BranchName,
			CustomerName:    items[i].CustomerName,
			CustomerTrust:   items[i].CustomerTrust,
		})
	}
	c.JSON(http.StatusOK, resps)
}

// approveBooking handles POST /owner/bookings/:id/approve.
func (h *Handler) approveBooking(c *gin.Context) {
	id := c.Param("id")
	ownerID := c.GetString("user_id")

	if err := h.svc.Approve(c.Request.Context(), id, ownerID); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// rejectBooking handles POST /owner/bookings/:id/reject.
func (h *Handler) rejectBooking(c *gin.Context) {
	id := c.Param("id")
	ownerID := c.GetString("user_id")

	var req RejectRequest
	_ = c.ShouldBindJSON(&req) // reason is optional

	if err := h.svc.Reject(c.Request.Context(), id, ownerID, req.Reason); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// approveReschedule handles POST /owner/reschedules/:id/approve.
func (h *Handler) approveReschedule(c *gin.Context) {
	id := c.Param("id")
	ownerID := c.GetString("user_id")

	if err := h.svc.ApproveReschedule(c.Request.Context(), id, ownerID); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// rejectReschedule handles POST /owner/reschedules/:id/reject.
func (h *Handler) rejectReschedule(c *gin.Context) {
	id := c.Param("id")
	ownerID := c.GetString("user_id")

	if err := h.svc.RejectReschedule(c.Request.Context(), id, ownerID); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleError maps domain errors to HTTP responses.
func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrBookingNotFound), errors.Is(err, ErrRescheduleNotFound), errors.Is(err, ErrLockNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": err.Error()})
	case errors.Is(err, ErrSlotUnavailable):
		c.JSON(http.StatusConflict, gin.H{"error": "slot_unavailable", "message": "The selected time slot is no longer available"})
	case errors.Is(err, ErrIllegalStateTransition):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "illegal_state_transition", "message": "This action cannot be performed on a booking in its current state"})
	case errors.Is(err, ErrRescheduleLimitReached):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "reschedule_limit_reached", "message": "Maximum reschedule attempts reached for this booking"})
	case errors.Is(err, ErrPendingRescheduleExists):
		c.JSON(http.StatusConflict, gin.H{"error": "pending_reschedule_exists", "message": "A pending reschedule request already exists for this booking"})
	case errors.Is(err, ErrUnauthorized):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "You are not authorized to perform this action"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
	}
}
