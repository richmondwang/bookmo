package participants

import "time"

// BookingParticipant represents a row in booking_participants.
type BookingParticipant struct {
	ID          string     `json:"id"`
	BookingID   string     `json:"booking_id"`
	UserID      string     `json:"user_id"`
	InvitedBy   string     `json:"invited_by"`
	Status      string     `json:"status"`
	InvitedAt   time.Time  `json:"invited_at"`
	RespondedAt *time.Time `json:"responded_at,omitempty"`
	LeftAt      *time.Time `json:"left_at,omitempty"`
}

// InviteRequest is the HTTP body for POST /bookings/:id/participants.
type InviteRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

// BookedWithUser is a user returned in the "booked with" history.
type BookedWithUser struct {
	UserID string `json:"user_id"`
}
