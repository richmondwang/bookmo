package participants

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler serves HTTP requests for the participants module.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all participant-related routes onto the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/bookings/:id/participants", h.Invite)
	rg.GET("/bookings/:id/participants", h.GetByBooking)
	rg.POST("/bookings/:id/participants/:user_id/accept", h.Accept)
	rg.POST("/bookings/:id/participants/:user_id/decline", h.Decline)
	rg.DELETE("/bookings/:id/participants/me", h.Leave)
	rg.GET("/users/:id/booked-with", h.GetBookedWith)
}

// participantError maps a sentinel error to the appropriate HTTP status and error code.
func participantError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrParticipantsNotAllowed):
		c.JSON(http.StatusForbidden, gin.H{"error": "participants_not_allowed", "message": "This service does not allow booking participants"})
	case errors.Is(err, ErrNotBookingCreator):
		c.JSON(http.StatusForbidden, gin.H{"error": "not_booking_creator", "message": "Only the booking creator can invite participants"})
	case errors.Is(err, ErrCannotInviteSelf):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "cannot_invite_self", "message": "You cannot invite yourself to your own booking"})
	case errors.Is(err, ErrAlreadyInvited):
		c.JSON(http.StatusConflict, gin.H{"error": "already_invited", "message": "This user has already been invited to this booking"})
	case errors.Is(err, ErrBookingCompleted):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "booking_completed", "message": "No changes allowed once a booking is completed"})
	case errors.Is(err, ErrNotParticipant):
		c.JSON(http.StatusForbidden, gin.H{"error": "not_participant", "message": "You are not a participant on this booking"})
	case errors.Is(err, ErrParticipantNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "participant_not_found", "message": "Participant not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "An unexpected error occurred"})
	}
}

// Invite handles POST /bookings/:id/participants.
func (h *Handler) Invite(c *gin.Context) {
	bookingID := c.Param("id")
	callerID := c.GetString("user_id")

	var req InviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	p, err := h.svc.Invite(c.Request.Context(), bookingID, callerID, req.UserID)
	if err != nil {
		participantError(c, err)
		return
	}

	c.JSON(http.StatusCreated, p)
}

// GetByBooking handles GET /bookings/:id/participants.
func (h *Handler) GetByBooking(c *gin.Context) {
	bookingID := c.Param("id")

	participants, err := h.svc.GetByBooking(c.Request.Context(), bookingID)
	if err != nil {
		participantError(c, err)
		return
	}

	if participants == nil {
		participants = []BookingParticipant{}
	}
	c.JSON(http.StatusOK, gin.H{"participants": participants})
}

// Accept handles POST /bookings/:id/participants/:user_id/accept.
func (h *Handler) Accept(c *gin.Context) {
	bookingID := c.Param("id")
	userID := c.Param("user_id")
	callerID := c.GetString("user_id")

	if callerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "not_participant", "message": "You are not a participant on this booking"})
		return
	}

	if err := h.svc.Accept(c.Request.Context(), bookingID, callerID); err != nil {
		participantError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// Decline handles POST /bookings/:id/participants/:user_id/decline.
func (h *Handler) Decline(c *gin.Context) {
	bookingID := c.Param("id")
	userID := c.Param("user_id")
	callerID := c.GetString("user_id")

	if callerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "not_participant", "message": "You are not a participant on this booking"})
		return
	}

	if err := h.svc.Decline(c.Request.Context(), bookingID, callerID); err != nil {
		participantError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// Leave handles DELETE /bookings/:id/participants/me.
func (h *Handler) Leave(c *gin.Context) {
	bookingID := c.Param("id")
	callerID := c.GetString("user_id")

	if err := h.svc.Leave(c.Request.Context(), bookingID, callerID); err != nil {
		participantError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// GetBookedWith handles GET /users/:id/booked-with.
func (h *Handler) GetBookedWith(c *gin.Context) {
	userID := c.Param("id")

	users, err := h.svc.GetBookedWith(c.Request.Context(), userID)
	if err != nil {
		participantError(c, err)
		return
	}

	if users == nil {
		users = []BookedWithUser{}
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}
