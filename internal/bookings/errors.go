package bookings

import "errors"

var (
	ErrBookingNotFound         = errors.New("bookings: not found")
	ErrSlotUnavailable         = errors.New("bookings: slot unavailable")
	ErrIllegalStateTransition  = errors.New("bookings: illegal state transition")
	ErrRescheduleLimitReached  = errors.New("bookings: reschedule attempt limit reached")
	ErrPendingRescheduleExists = errors.New("bookings: pending reschedule already exists")
	ErrUnauthorized            = errors.New("bookings: unauthorized")
	ErrRescheduleNotFound      = errors.New("bookings: reschedule request not found")
	ErrLockNotFound            = errors.New("bookings: lock not found or expired")
)
