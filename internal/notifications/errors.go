package notifications

import "errors"

var (
	ErrNotFound       = errors.New("notifications: not found")
	ErrInvalidToken   = errors.New("notifications: invalid device token")
	ErrDeliveryFailed = errors.New("notifications: delivery failed")
)
