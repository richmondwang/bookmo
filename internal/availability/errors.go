package availability

import "errors"

var (
	ErrNotFound         = errors.New("availability: not found")
	ErrNoSlotsAvailable = errors.New("availability: no slots available")
	ErrSlotConflict     = errors.New("availability: slot conflict")
)
