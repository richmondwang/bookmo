package participants

import "errors"

var (
	ErrNotBookingCreator      = errors.New("participants: only the booking creator can invite")
	ErrCannotInviteSelf       = errors.New("participants: cannot invite yourself")
	ErrAlreadyInvited         = errors.New("participants: user already invited to this booking")
	ErrBookingCompleted       = errors.New("participants: booking is completed, no changes allowed")
	ErrNotParticipant         = errors.New("participants: you are not a participant on this booking")
	ErrParticipantsNotAllowed = errors.New("participants: participants not allowed for this service category")
	ErrParticipantNotFound    = errors.New("participants: not found")
)
